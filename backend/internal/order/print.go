package order

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
)

const (
	PrintPackingSlip = "packing_slip"
	PrintInvoice     = "invoice"
	PrintPickList    = "pick_list"

	printTTL      = 10 * time.Minute
	maxPrintOrder = 50
)

// PrintToken (11.7 item 8): stateless; DB te kichu na, tai verify e scope ar staff abar dekha hoy.
type PrintToken struct {
	ShopID   int64   `json:"shop_id"`
	Kind     string  `json:"kind"`
	IDs      []int64 `json:"ids,omitempty"`
	Date     string  `json:"date,omitempty"`
	StaffID  int64   `json:"staff_id"`
	VendorID int64   `json:"vendor_id,omitempty"`
	Exp      int64   `json:"exp"`
}

// printMAC: "print:" prefix, jate PRINT_SECRET onno kaje reuse holeo token mile na.
func printMAC(secret, payload string) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("print:" + payload))
	return m.Sum(nil)
}

// SignPrint: base64url(json) + "." + base64url(HMAC).
func SignPrint(secret string, t PrintToken) string {
	b, _ := json.Marshal(t)
	p := base64.RawURLEncoding.EncodeToString(b)
	return p + "." + base64.RawURLEncoding.EncodeToString(printMAC(secret, p))
}

// VerifyPrint: signature (hmac.Equal, JSON porar age), exp, kind ar ids|date er mil. Kind/shop/staff er mil caller e.
func VerifyPrint(secret, tok string, now time.Time) (PrintToken, bool) {
	var t PrintToken
	p, s, ok := strings.Cut(tok, ".")
	if !ok || strings.Contains(s, ".") {
		return t, false
	}
	sig, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || !hmac.Equal(sig, printMAC(secret, p)) {
		return t, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil {
		return t, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&t) != nil || t.ShopID <= 0 || t.StaffID <= 0 || now.Unix() >= t.Exp {
		return PrintToken{}, false
	}
	if !validPrintShape(t.Kind, t.IDs, t.Date) {
		return PrintToken{}, false
	}
	return t, true
}

// validPrintShape: packing_slip/invoice e 1–50 alada id, date na; pick_list e date, id na.
func validPrintShape(kind string, ids []int64, date string) bool {
	switch kind {
	case PrintPackingSlip, PrintInvoice:
		return date == "" && printIDsField(ids) == ""
	case PrintPickList:
		_, err := time.Parse(time.DateOnly, date)
		return ids == nil && err == nil
	}
	return false
}

// printIDsField: order_ids bhul hole "order_ids".
func printIDsField(ids []int64) string {
	if len(ids) == 0 || len(ids) > maxPrintOrder {
		return "order_ids"
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return "order_ids"
		}
		seen[id] = true
	}
	return ""
}

// AdminPrintLink: POST /api/admin/print-links {kind, order_ids | date} → 200 {url, expires_at} (11.7 item 8, v1.44).
func (h *Handlers) AdminPrintLink(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind     string  `json:"kind"`
		OrderIDs []int64 `json:"order_ids"`
		Date     *string `json:"date"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	field := ""
	switch in.Kind {
	case PrintPackingSlip, PrintInvoice:
		if field = printIDsField(in.OrderIDs); field == "" && in.Date != nil {
			field = "date"
		}
	case PrintPickList:
		if in.OrderIDs != nil {
			field = "order_ids"
		} else if in.Date != nil {
			if _, err := time.Parse(time.DateOnly, *in.Date); err != nil {
				field = "date"
			}
		}
	default:
		field = "kind"
	}
	if field != "" {
		invalid(w, r, field, "invalid")
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	by := ByStaff(st)
	vendor := scope(by)
	if vendor < 0 {
		h.adminError(w, r, ErrForbidden)
		return
	}
	if vendor > 0 && in.Kind == PrintInvoice {
		h.adminError(w, r, ErrForbidden) // vendor e invoice na (puro order er taka, customer)
		return
	}
	t := PrintToken{ShopID: st.ShopID, Kind: in.Kind, IDs: in.OrderIDs, StaffID: st.ID, VendorID: vendor}
	ctx := r.Context()
	if in.Kind == PrintPickList {
		if in.Date != nil {
			t.Date = *in.Date
		} else {
			// issue er shomoy-i date thik: midnight er por khulleo ekoi list
			var err error
			if t.Date, err = shopToday(ctx, h.db, st.ShopID); err != nil {
				h.adminError(w, r, err)
				return
			}
		}
	}
	var domain string
	err := h.db.QueryRowContext(ctx, `SELECT domain FROM shop_domains WHERE shop_id = $1 AND is_primary`, st.ShopID).Scan(&domain)
	if err != nil {
		h.adminError(w, r, err)
		return
	}
	exp := time.Now().Add(printTTL).Truncate(time.Second)
	t.Exp = exp.Unix()
	after := map[string]any{"kind": t.Kind}
	if t.Date != "" {
		after["date"] = t.Date
	} else {
		after["count"] = len(t.IDs)
	}
	ab, _ := json.Marshal(after)
	// audit: print link e customer er thikana/phone khole, tai ke banalo thake (id ba PII na)
	if err := db.InTx(ctx, h.db, func(tx *sql.Tx) error {
		return logAudit(ctx, tx, st, "print.link", t.Kind, 0, nil, ab)
	}); err != nil {
		h.adminError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"url":        "https://" + domain + "/print/" + t.Kind + "?token=" + SignPrint(h.guard.PrintSecret, t),
		"expires_at": exp.UTC().Format(time.RFC3339),
	})
}

// SlipItem: packing slip er line (taka na).
type SlipItem struct {
	ProductName    json.RawMessage `json:"product_name"`
	VariantLabel   json.RawMessage `json:"variant_label"`
	SKU            string          `json:"sku"`
	Quantity       int             `json:"quantity"`
	VariableWeight bool            `json:"variable_weight"`
	OrderedWeightG *int64          `json:"ordered_weight_g"`
	PackedWeightG  *int64          `json:"packed_weight_g"`
}

// Slip: fulfillment onujayi packing slip (marketplace e "SB-100245-2").
type Slip struct {
	OrderNumber    string     `json:"order_number"`
	Seq            int        `json:"seq"`
	VendorID       int64      `json:"vendor_id"`
	RecipientName  string     `json:"recipient_name"`
	RecipientPhone string     `json:"recipient_phone"`
	District       string     `json:"district"`
	Area           string     `json:"area"`
	Address        string     `json:"address"`
	Note           *string    `json:"note"`
	PaymentMethod  string     `json:"payment_method"`
	CODAmount      int64      `json:"cod_amount"`
	Items          []SlipItem `json:"items"`
}

type InvoiceItem struct {
	SlipItem
	UnitPrice int64 `json:"unit_price"`
	LineTotal int64 `json:"line_total"`
}

// Invoice: puro order (shudhu shop er staff).
type Invoice struct {
	OrderNumber    string        `json:"order_number"`
	PlacedAt       time.Time     `json:"placed_at"`
	CustomerName   string        `json:"customer_name"`
	CustomerPhone  string        `json:"customer_phone"`
	RecipientName  string        `json:"recipient_name"`
	RecipientPhone string        `json:"recipient_phone"`
	District       string        `json:"district"`
	Area           string        `json:"area"`
	Address        string        `json:"address"`
	PaymentMethod  string        `json:"payment_method"`
	PaymentStatus  string        `json:"payment_status"`
	Subtotal       int64         `json:"subtotal"`
	Discount       int64         `json:"discount"`
	DeliveryCharge int64         `json:"delivery_charge"`
	Total          int64         `json:"total"`
	Items          []InvoiceItem `json:"items"`
}

// PrintData: GET /api/print er response; kind onujayi ekta field.
type PrintData struct {
	Kind     string    `json:"kind"`
	Slips    []Slip    `json:"slips,omitempty"`
	Invoices []Invoice `json:"invoices,omitempty"`
	PickList *PickList `json:"pick_list,omitempty"`
}

// LoadPrint (11.7 item 8): verify (path kind, host er shop, exp, issuing staff active) tarpor scope abar; kono ta na
// mille ba scope e kichu na thakle ErrNotFound.
func LoadPrint(ctx context.Context, d *sql.DB, secret string, shopID int64, kind, tok string, now time.Time) (PrintData, error) {
	out := PrintData{Kind: kind}
	t, ok := VerifyPrint(secret, tok, now)
	if !ok || t.Kind != kind || t.ShopID != shopID || (t.Kind == PrintInvoice && t.VendorID != 0) {
		return out, ErrNotFound
	}
	var active bool
	err := d.QueryRowContext(ctx, `SELECT is_active FROM staff WHERE shop_id = $1 AND id = $2`, shopID, t.StaffID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !active) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	switch kind {
	case PrintPickList:
		p, err := LoadPickList(ctx, d, shopID, t.Date, t.VendorID)
		out.PickList = &p
		return out, err
	case PrintPackingSlip:
		out.Slips, err = loadSlips(ctx, d, shopID, t.IDs, t.VendorID)
	default:
		out.Invoices, err = loadInvoices(ctx, d, shopID, t.IDs)
	}
	if err == nil && len(out.Slips) == 0 && len(out.Invoices) == 0 {
		err = ErrNotFound
	}
	return out, err
}

// loadSlips: ids er kram e; cancelled fulfillment ar removed item bad; vendor > 0 hole shudhu tar.
func loadSlips(ctx context.Context, d *sql.DB, shopID int64, ids []int64, vendor int64) ([]Slip, error) {
	var out []Slip
	for _, oid := range ids { // ponytail: order protiti query (max 50), print e jothesto
		rows, err := d.QueryContext(ctx, `SELECT f.id, o.order_number, f.seq, f.vendor_id, o.recipient_name, o.recipient_phone,
			o.ship_district, o.ship_area, o.ship_address, o.note, o.payment_method, CASE WHEN o.payment_method = 'cod' THEN f.total ELSE 0 END
			FROM order_fulfillments f JOIN orders o ON o.shop_id = f.shop_id AND o.id = f.order_id
			WHERE f.shop_id = $1 AND f.order_id = $2 AND ($3::bigint = 0 OR f.vendor_id = $3) AND f.status <> 'cancelled'
			ORDER BY f.seq`, shopID, oid, vendor)
		if err != nil {
			return nil, err
		}
		var fids []int64
		for rows.Next() {
			var s Slip
			var fid int64
			if err := rows.Scan(&fid, &s.OrderNumber, &s.Seq, &s.VendorID, &s.RecipientName, &s.RecipientPhone, &s.District, &s.Area,
				&s.Address, &s.Note, &s.PaymentMethod, &s.CODAmount); err != nil {
				rows.Close()
				return nil, err
			}
			fids = append(fids, fid)
			out = append(out, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for i, fid := range fids {
			s := &out[len(out)-len(fids)+i]
			s.Items = []SlipItem{}
			if err := scanItems(ctx, d, `SELECT product_name, variant_label, sku, quantity, variable_weight, ordered_weight_g, packed_weight_g FROM order_items WHERE shop_id = $1 AND fulfillment_id = $2
				AND status = 'active' ORDER BY id`, func(sc func(...any) error) error {
				var it SlipItem
				if err := sc(it.dest()...); err != nil {
					return err
				}
				s.Items = append(s.Items, it)
				return nil
			}, shopID, fid); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func (it *SlipItem) dest() []any {
	return []any{&it.ProductName, &it.VariantLabel, &it.SKU, &it.Quantity, &it.VariableWeight, &it.OrderedWeightG, &it.PackedWeightG}
}

// loadInvoices: ids er kram e; cancelled order bad (bhul bill); taka snapshot theke, hishab na.
func loadInvoices(ctx context.Context, d *sql.DB, shopID int64, ids []int64) ([]Invoice, error) {
	var out []Invoice
	for _, oid := range ids {
		var v Invoice
		err := d.QueryRowContext(ctx, `SELECT order_number, placed_at, customer_name, customer_phone, recipient_name, recipient_phone,
			ship_district, ship_area, ship_address, payment_method, payment_status, subtotal, discount, delivery_charge, total
			FROM orders WHERE shop_id = $1 AND id = $2 AND status <> 'cancelled'`, shopID, oid).Scan(&v.OrderNumber, &v.PlacedAt,
			&v.CustomerName, &v.CustomerPhone, &v.RecipientName, &v.RecipientPhone, &v.District, &v.Area, &v.Address,
			&v.PaymentMethod, &v.PaymentStatus, &v.Subtotal, &v.Discount, &v.DeliveryCharge, &v.Total)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		v.Items = []InvoiceItem{}
		if err := scanItems(ctx, d, `SELECT product_name, variant_label, sku, quantity, variable_weight, ordered_weight_g, packed_weight_g, unit_price, line_total FROM order_items
			WHERE shop_id = $1 AND order_id = $2 AND status = 'active' ORDER BY id`, func(sc func(...any) error) error {
			var it InvoiceItem
			if err := sc(append(it.dest(), &it.UnitPrice, &it.LineTotal)...); err != nil {
				return err
			}
			v.Items = append(v.Items, it)
			return nil
		}, shopID, oid); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func scanItems(ctx context.Context, d *sql.DB, query string, each func(func(...any) error) error, args ...any) error {
	rows, err := d.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := each(rows.Scan); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Print: GET /api/print?kind=&token= (public, website SSR; shop X-Shop-Host theke). Shob bhul 404 (kon ta bola na).
func (h *Handlers) Print(w http.ResponseWriter, r *http.Request) {
	sh, _ := httpx.ShopFrom(r.Context())
	q := r.URL.Query()
	data, err := LoadPrint(r.Context(), h.db, h.guard.PrintSecret, sh.ID, q.Get("kind"), q.Get("token"), time.Now())
	w.Header().Set("Cache-Control", "no-store")
	if err != nil {
		h.transitionError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, data)
}
