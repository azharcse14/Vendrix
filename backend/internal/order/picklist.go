package order

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"vendrix/backend/internal/httpx"
)

// PickVariant: pick list e ek variant er mot (ojon-variable e ordered gram er mot o).
type PickVariant struct {
	VariantID      int64           `json:"variant_id"`
	ProductName    json.RawMessage `json:"product_name"`
	VariantLabel   json.RawMessage `json:"variant_label"`
	SKU            string          `json:"sku"`
	VariableWeight bool            `json:"variable_weight"`
	Quantity       int             `json:"quantity"`
	OrderedWeightG *int64          `json:"ordered_weight_g"`
}

type PickArea struct {
	District string        `json:"district"`
	Area     string        `json:"area"`
	Variants []PickVariant `json:"variants"`
}

type PickFulfillment struct {
	ID            int64     `json:"id"`
	OrderID       int64     `json:"order_id"`
	OrderNumber   string    `json:"order_number"`
	Seq           int       `json:"seq"`
	VendorID      int64     `json:"vendor_id"`
	RecipientName string    `json:"recipient_name"`
	District      string    `json:"district"`
	Area          string    `json:"area"`
	ConfirmedAt   time.Time `json:"confirmed_at"`
}

// PickList (11.7 item 4, v1.44): GET /packing/pick-list ar print er pick_list.
type PickList struct {
	Date         string            `json:"date"`
	Variants     []PickVariant     `json:"variants"`
	Areas        []PickArea        `json:"areas"`
	Fulfillments []PickFulfillment `json:"fulfillments"`
}

// shopToday: shop er timezone e ajker tarikh (Rule 5).
func shopToday(ctx context.Context, d *sql.DB, shopID int64) (string, error) {
	var s string
	err := d.QueryRowContext(ctx, `SELECT to_char((now() AT TIME ZONE timezone)::date, 'YYYY-MM-DD') FROM shops WHERE id = $1`,
		shopID).Scan(&s)
	return s, err
}

// LoadPickList (8.11 item 1): date (shop timezone, khali = aj) er shesh porjonto confirmed order er shob open fulfillment,
// ager diner baki o. vendor 0 = shob, -1 = kichu na. confirmed_at NULL bad (TransitionOrder shob shomoy dey; v1.44).
func LoadPickList(ctx context.Context, d *sql.DB, shopID int64, date string, vendor int64) (PickList, error) {
	p := PickList{Date: date, Variants: []PickVariant{}, Areas: []PickArea{}, Fulfillments: []PickFulfillment{}}
	if p.Date == "" {
		var err error
		if p.Date, err = shopToday(ctx, d, shopID); err != nil {
			return p, err
		}
	}
	rows, err := d.QueryContext(ctx, `SELECT f.id, f.order_id, o.order_number, f.seq, f.vendor_id, o.recipient_name, o.ship_district,
		o.ship_area, o.confirmed_at, i.variant_id, i.product_name, i.variant_label, i.sku, i.variable_weight, i.quantity, i.ordered_weight_g
		FROM order_fulfillments f JOIN orders o ON o.shop_id = f.shop_id AND o.id = f.order_id
		JOIN order_items i ON i.shop_id = f.shop_id AND i.fulfillment_id = f.id AND i.status = 'active'
		WHERE f.shop_id = $1 AND ($2::bigint = 0 OR f.vendor_id = $2) AND f.status = 'open' AND o.status = 'confirmed'
		AND o.confirmed_at < ($3::date + 1)::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1)
		ORDER BY o.confirmed_at, f.id, i.id`, shopID, vendor, p.Date)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	type areaKey struct{ district, area string }
	all := map[int64]*PickVariant{}
	byArea := map[areaKey]map[int64]*PickVariant{}
	add := func(m map[int64]*PickVariant, v PickVariant) {
		x, ok := m[v.VariantID]
		if !ok {
			c := v
			c.Quantity, c.OrderedWeightG = 0, nil
			if c.VariableWeight {
				c.OrderedWeightG = new(int64)
			}
			m[v.VariantID], x = &c, &c
		}
		x.Quantity += v.Quantity
		if x.OrderedWeightG != nil && v.OrderedWeightG != nil {
			*x.OrderedWeightG += *v.OrderedWeightG
		}
	}
	for rows.Next() {
		var fl PickFulfillment
		var v PickVariant
		if err := rows.Scan(&fl.ID, &fl.OrderID, &fl.OrderNumber, &fl.Seq, &fl.VendorID, &fl.RecipientName, &fl.District, &fl.Area,
			&fl.ConfirmedAt, &v.VariantID, &v.ProductName, &v.VariantLabel, &v.SKU, &v.VariableWeight, &v.Quantity, &v.OrderedWeightG); err != nil {
			return p, err
		}
		if n := len(p.Fulfillments); n == 0 || p.Fulfillments[n-1].ID != fl.ID {
			p.Fulfillments = append(p.Fulfillments, fl)
		}
		add(all, v)
		k := areaKey{fl.District, fl.Area}
		if byArea[k] == nil {
			byArea[k] = map[int64]*PickVariant{}
		}
		add(byArea[k], v)
	}
	if err := rows.Err(); err != nil {
		return p, err
	}
	sorted := func(m map[int64]*PickVariant) []PickVariant {
		out := make([]PickVariant, 0, len(m))
		for _, v := range m {
			out = append(out, *v)
		}
		slices.SortFunc(out, func(a, b PickVariant) int { return cmp.Compare(a.VariantID, b.VariantID) })
		return out
	}
	p.Variants = sorted(all)
	for k, m := range byArea {
		p.Areas = append(p.Areas, PickArea{District: k.district, Area: k.area, Variants: sorted(m)})
	}
	slices.SortFunc(p.Areas, func(a, b PickArea) int {
		return cmp.Or(cmp.Compare(a.District, b.District), cmp.Compare(a.Area, b.Area))
	})
	return p, nil
}

// AdminPickList: GET /api/admin/packing/pick-list?date= (vendor role e nijer vendor).
func (h *Handlers) AdminPickList(w http.ResponseWriter, r *http.Request) {
	date, ok := dateParam(r, "date")
	if !ok {
		invalid(w, r, "date", "invalid")
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	p, err := LoadPickList(r.Context(), h.db, st.ShopID, date, scope(ByStaff(st)))
	if err != nil {
		h.adminError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, p)
}
