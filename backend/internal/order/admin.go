package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"vendrix/backend/internal/audit"
	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/phone"
)

// ErrForbidden: role er niyom (11.2), CanTransition* er age; 403 forbidden.
var ErrForbidden = errors.New("order: ei role e na")

// AdminRoutes (7.7): staff = AdminSession; role ekhane (11.2), role er baki niyom service e.
func (h *Handlers) AdminRoutes(mux *http.ServeMux, staff httpx.Middleware) {
	all := []string{"owner", "admin", "staff", "vendor_owner", "vendor_staff"}
	shop := []string{"owner", "admin", "staff"}
	admin := func(f http.HandlerFunc, roles []string) http.Handler { return staff(httpx.RequireRole(roles...)(f)) }
	mux.Handle("GET /api/admin/orders", admin(h.AdminOrders, all))
	mux.Handle("GET /api/admin/orders/{id}", admin(h.AdminOrder, all))
	mux.Handle("POST /api/admin/orders/{id}/transition", admin(h.AdminTransitionOrder, shop))
	mux.Handle("POST /api/admin/orders/{id}/notes", admin(h.AdminNote, shop))
	mux.Handle("PATCH /api/admin/orders/{id}/items", admin(h.AdminEdit, shop))
	mux.Handle("GET /api/admin/fulfillments", admin(h.AdminFulfillments, all))
	mux.Handle("POST /api/admin/fulfillments/{id}/transition", admin(h.AdminTransitionFulfillment, all))
	mux.Handle("POST /api/admin/fulfillments/{id}/pack", admin(h.AdminPack, all))
}

// scope (Rule 13, 11.7 item 13 i): shop staff 0 (shob vendor); vendor role e nijer vendor, vendor_id chhara -1
// (kichu na, fail-closed).
func scope(by By) int64 {
	switch {
	case by.Actor == Staff:
		return 0
	case by.Actor == Vendor && by.VendorID > 0:
		return by.VendorID
	}
	return -1
}

var displayStatuses = []string{"pending", "confirmed", "packed", "shipped", "delivered", "returned", "cancelled", "partial"}
var fStatuses = []string{"open", "packed", "shipped", "delivered", "cancelled", "returned"}

func adminID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// dateParam: YYYY-MM-DD (shop er timezone e din, SQL e); bhul hole ok false.
func dateParam(r *http.Request, name string) (string, bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return "", true
	}
	_, err := time.Parse(time.DateOnly, v)
	return v, err == nil
}

// AdminOrders: GET /api/admin/orders?status=&tab=&risk=&q=&from=&to=&vendor_id=&page=&limit= (11.7 item 13 vii).
func (h *Handlers) AdminOrders(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	q := r.URL.Query()
	f := ListFilter{Status: q.Get("status"), Tab: q.Get("tab"), Risk: q.Get("risk") == "1", Vendor: scope(ByStaff(st))}
	f.VendorRole = f.Vendor != 0
	var ok1, ok2 bool
	f.From, ok1 = dateParam(r, "from")
	f.To, ok2 = dateParam(r, "to")
	field := ""
	switch {
	case f.Status != "" && !slices.Contains(displayStatuses, f.Status):
		field = "status"
	case f.Tab != "" && f.Tab != TabAwaitingConfirm && f.Tab != TabDeadlineSoon:
		field = "tab"
	case q.Get("risk") != "" && !f.Risk:
		field = "risk"
	case !ok1:
		field = "from"
	case !ok2:
		field = "to"
	}
	if v := q.Get("vendor_id"); v != "" && field == "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			field = "vendor_id"
		} else if f.Vendor == 0 { // vendor role e session er vendor-i
			f.Vendor = id
		}
	}
	if s := strings.TrimSpace(q.Get("q")); s != "" && field == "" {
		if p, err := phone.Normalize(s); err == nil {
			f.Phone = p
		} else if len(s) <= 64 {
			f.Number = strings.ToUpper(s)
		} else {
			field = "q"
		}
	}
	if field != "" {
		invalid(w, r, field, "invalid")
		return
	}
	items, total, err := listOrders(r.Context(), h.db, st.ShopID, f, page, limit)
	if err != nil {
		h.adminError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

// AdminOrder: GET /api/admin/orders/{id}: fulfillment, item, history, note (vendor e shudhu nijer, note na).
func (h *Handlers) AdminOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := adminID(r)
	if !ok {
		h.adminError(w, r, ErrNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	d, err := orderDetail(r.Context(), h.db, st.ShopID, id, scope(ByStaff(st)))
	if err != nil {
		h.adminError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, d)
}

type transitionIn struct {
	To   string `json:"to"`
	Note string `json:"note"`
}

// decodeTransition: body {to, note}; note max 500.
func decodeTransition(w http.ResponseWriter, r *http.Request, allowed []string) (transitionIn, bool) {
	var in transitionIn
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return in, false
	}
	if !slices.Contains(allowed, in.To) {
		invalid(w, r, "to", "invalid")
		return in, false
	}
	var ok bool
	if in.Note, ok = text(in.Note, 0, 500); !ok {
		invalid(w, r, "note", "invalid")
	}
	return in, ok
}

// AdminTransitionOrder: POST /api/admin/orders/{id}/transition {to: confirmed|cancelled, note}.
func (h *Handlers) AdminTransitionOrder(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeTransition(w, r, []string{string(Confirmed), string(Cancelled)})
	if !ok {
		return
	}
	id, ok := adminID(r)
	if !ok {
		h.adminError(w, r, ErrNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	display, err := StaffTransitionOrder(r.Context(), h.db, st, id, Status(in.To), in.Note)
	if err != nil {
		h.adminError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": in.To, "display_status": display})
}

// AdminTransitionFulfillment: POST /api/admin/fulfillments/{id}/transition {to, note} (packed er path /pack).
func (h *Handlers) AdminTransitionFulfillment(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeTransition(w, r, []string{string(Open), string(Shipped), string(Delivered), string(Returned), string(FCancelled)})
	if !ok {
		return
	}
	id, ok := adminID(r)
	if !ok {
		h.adminError(w, r, ErrNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	if err := StaffTransitionFulfillment(r.Context(), h.db, st, id, FStatus(in.To), in.Note); err != nil {
		h.adminError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": in.To})
}

// AdminNote: POST /api/admin/orders/{id}/notes {kind: note|call, note 1–1000}.
func (h *Handlers) AdminNote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind string `json:"kind"`
		Note string `json:"note"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	if in.Kind == "" {
		in.Kind = "note"
	}
	if in.Kind != "note" && in.Kind != "call" {
		invalid(w, r, "kind", "invalid")
		return
	}
	var ok bool
	if in.Note, ok = text(in.Note, 1, 1000); !ok {
		invalid(w, r, "note", "invalid")
		return
	}
	id, ok := adminID(r)
	if !ok {
		h.adminError(w, r, ErrNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	n, err := AddNote(r.Context(), h.db, st, id, in.Kind, in.Note)
	if err != nil {
		h.adminError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, n)
}

// AdminFulfillments: GET /api/admin/fulfillments?status=&vendor_id=&date= (date = order er din, shop timezone).
func (h *Handlers) AdminFulfillments(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	q := r.URL.Query()
	status, vendor := q.Get("status"), scope(ByStaff(st))
	date, ok := dateParam(r, "date")
	field := ""
	switch {
	case status != "" && !slices.Contains(fStatuses, status):
		field = "status"
	case !ok:
		field = "date"
	}
	if v := q.Get("vendor_id"); v != "" && field == "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			field = "vendor_id"
		} else if vendor == 0 {
			vendor = id
		}
	}
	if field != "" {
		invalid(w, r, field, "invalid")
		return
	}
	items, total, err := listFulfillments(r.Context(), h.db, st.ShopID, status, date, vendor, page, limit)
	if err != nil {
		h.adminError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

// adminError: ErrForbidden 403; ErrNotImplemented (M7a er porer step) 409 invalid_transition; baki transitionError.
func (h *Handlers) adminError(w http.ResponseWriter, r *http.Request, err error) {
	var fe FieldError
	switch {
	case errors.As(err, &fe):
		invalid(w, r, string(fe), "invalid")
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, r, http.StatusForbidden, httpx.ErrorBody{Code: "forbidden"})
	case errors.Is(err, ErrNotImplemented):
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "invalid_transition"})
	default:
		h.transitionError(w, r, err)
	}
}

// statusJSON: audit er before/after; shudhu status (note e phone thakte pare, 12.2).
func statusJSON(s string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"status": s})
	return b
}

func logAudit(ctx context.Context, tx *sql.Tx, st httpx.Staff, action, entity string, id int64, before, after json.RawMessage) error {
	return audit.Log(ctx, tx, audit.Entry{ShopID: st.ShopID, StaffID: st.ID, Action: action, EntityType: entity,
		EntityID: id, Before: before, After: after, IP: httpx.IP(ctx)})
}

// StaffTransitionOrder (11.2, 11.7 item 13 i, x): cancel shudhu owner/admin; TransitionOrder + audit ek tx e.
// display_status ferot.
func StaffTransitionOrder(ctx context.Context, d *sql.DB, st httpx.Staff, orderID int64, to Status, note string) (string, error) {
	if to == Cancelled && st.Role != "owner" && st.Role != "admin" {
		return "", ErrForbidden
	}
	var display string
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		var from string
		// lockOrder er moto order age lock (kram ek)
		err := tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE shop_id = $1 AND id = $2 FOR UPDATE`,
			st.ShopID, orderID).Scan(&from)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := TransitionOrder(ctx, tx, st.ShopID, orderID, to, ByStaff(st), note); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `SELECT display_status FROM orders WHERE shop_id = $1 AND id = $2`,
			st.ShopID, orderID).Scan(&display); err != nil {
			return err
		}
		return logAudit(ctx, tx, st, "order.transition", "order", orderID, statusJSON(from), statusJSON(string(to)))
	})
	return display, err
}

// StaffTransitionFulfillment (11.2 v1.7; 14.2 test 35): cancel staff ar vendor_staff na; vendor scope
// TransitionFulfillment e (onno vendor 404). Audit before = history er from_status.
func StaffTransitionFulfillment(ctx context.Context, d *sql.DB, st httpx.Staff, fid int64, to FStatus, note string) error {
	if to == FCancelled && (st.Role == "staff" || st.Role == "vendor_staff") {
		return ErrForbidden
	}
	return db.InTx(ctx, d, func(tx *sql.Tx) error {
		if err := TransitionFulfillment(ctx, tx, st.ShopID, fid, to, ByStaff(st), note); err != nil {
			return err
		}
		var from string
		if err := tx.QueryRowContext(ctx, `SELECT from_status FROM order_status_history WHERE shop_id = $1 AND fulfillment_id = $2
			ORDER BY id DESC LIMIT 1`, st.ShopID, fid).Scan(&from); err != nil {
			return err
		}
		return logAudit(ctx, tx, st, "fulfillment.transition", "fulfillment", fid, statusJSON(from), statusJSON(string(to)))
	})
}

// Note: order_notes er row.
type Note struct {
	ID        int64     `json:"id"`
	StaffID   int64     `json:"staff_id"`
	Kind      string    `json:"kind"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// AddNote (7.7): shop er order e note; audit e note er lekha na (phone thakte pare).
func AddNote(ctx context.Context, d *sql.DB, st httpx.Staff, orderID int64, kind, text string) (Note, error) {
	n := Note{StaffID: st.ID, Kind: kind, Note: text}
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `INSERT INTO order_notes (shop_id, order_id, staff_id, kind, note)
			SELECT shop_id, id, $3, $4, $5 FROM orders WHERE shop_id = $1 AND id = $2 RETURNING id, created_at`,
			st.ShopID, orderID, st.ID, kind, text).Scan(&n.ID, &n.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		after, _ := json.Marshal(map[string]any{"note_id": n.ID, "kind": kind})
		return logAudit(ctx, tx, st, "order.note", "order", orderID, nil, after)
	})
	return n, err
}
