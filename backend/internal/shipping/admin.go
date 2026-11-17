package shipping

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

	"vendrix/backend/internal/audit"
	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
)

// AdminRoutes (7.7, 11.7 item 13 vi): zone shudhu owner/admin (11.2); delete na (is_active).
func (h *Handlers) AdminRoutes(mux *http.ServeMux, staff httpx.Middleware) {
	admin := func(f http.HandlerFunc) http.Handler { return staff(httpx.RequireRole("owner", "admin")(f)) }
	mux.Handle("GET /api/admin/delivery-zones", admin(h.AdminZones))
	mux.Handle("POST /api/admin/delivery-zones", admin(h.CreateZone))
	mux.Handle("PATCH /api/admin/delivery-zones/{id}", admin(h.UpdateZone))
}

// opt: PATCH er nullable field: key thakle Set (null = muche).
type opt[T any] struct {
	Set bool
	V   *T
}

func (o *opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	return json.Unmarshal(b, &o.V)
}

// zoneIn: same_day_cutoff, own_rider Phase 2, tai ochena field (400).
type zoneIn struct {
	Name             map[string]string `json:"name"`
	District         *string           `json:"district"`
	Area             opt[string]       `json:"area"`
	Charge           *int64            `json:"charge"`
	MinOrder         *int64            `json:"min_order"`
	FreeDeliveryOver opt[int64]        `json:"free_delivery_over"`
	SameCity         *bool             `json:"same_city"`
	IsActive         *bool             `json:"is_active"`
}

// fieldErr: 400 validation_failed + fields.<f>: invalid.
type fieldErr string

func (e fieldErr) Error() string { return string(e) + ": invalid" }

// place: trim, 1–100 rune (11.7 item 13 vi).
func place(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != "" && utf8.RuneCountInString(s) <= 100
}

// check: create e name, district, charge, same_city lagbe; PATCH e je field ache. Trim kora value in e boshay.
func (in *zoneIn) check(create bool) error {
	if create || in.Name != nil {
		bn := strings.TrimSpace(in.Name["bn"])
		for k := range in.Name {
			if k != "bn" && k != "en" {
				bn = ""
			}
		}
		if bn == "" {
			return fieldErr("name")
		}
	}
	if create || in.District != nil {
		if in.District == nil {
			return fieldErr("district")
		}
		d, ok := place(*in.District)
		if !ok {
			return fieldErr("district")
		}
		in.District = &d
	}
	// area khali = puro district (NULL, v1.45)
	if in.Area.V != nil {
		if a := strings.TrimSpace(*in.Area.V); a == "" {
			in.Area.V = nil
		} else if a, ok := place(a); ok {
			in.Area.V = &a
		} else {
			return fieldErr("area")
		}
	}
	switch {
	case (create && in.Charge == nil) || (in.Charge != nil && *in.Charge < 0):
		return fieldErr("charge")
	case in.MinOrder != nil && *in.MinOrder < 0:
		return fieldErr("min_order")
	case in.FreeDeliveryOver.V != nil && *in.FreeDeliveryOver.V <= 0:
		return fieldErr("free_delivery_over")
	case create && in.SameCity == nil:
		return fieldErr("same_city")
	}
	return nil
}

func (h *Handlers) adminErr(w http.ResponseWriter, r *http.Request, err error) {
	var fe fieldErr
	var pg *pgconn.PgError
	switch {
	case errors.As(err, &fe):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{string(fe): "invalid"}})
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteError(w, r, http.StatusNotFound, httpx.ErrorBody{Code: "not_found"})
	case errors.As(err, &pg) && pg.Code == "23505":
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "zone_exists"})
	default:
		h.log.ErrorContext(r.Context(), "zone admin", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
	}
}

// AdminZones: GET /api/admin/delivery-zones?page=&limit= (inactive o, puro {bn, en}).
func (h *Handlers) AdminZones(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	rows, err := h.db.QueryContext(r.Context(), `SELECT to_jsonb(z) - 'shop_id', count(*) OVER () FROM delivery_zones z
		WHERE shop_id = $1 ORDER BY district, area NULLS FIRST, id LIMIT $2 OFFSET $3`, st.ShopID, limit, (page-1)*limit)
	if err != nil {
		h.adminErr(w, r, err)
		return
	}
	defer rows.Close()
	var items []json.RawMessage
	var total int64
	for rows.Next() {
		var z json.RawMessage
		if err := rows.Scan(&z, &total); err != nil {
			h.adminErr(w, r, err)
			return
		}
		items = append(items, z)
	}
	if err := rows.Err(); err != nil {
		h.adminErr(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

// CreateZone: POST /api/admin/delivery-zones → 201 row.
func (h *Handlers) CreateZone(w http.ResponseWriter, r *http.Request) {
	h.writeZone(w, r, 0)
}

// UpdateZone: PATCH /api/admin/delivery-zones/{id} → 200 row.
func (h *Handlers) UpdateZone(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.adminErr(w, r, sql.ErrNoRows)
		return
	}
	h.writeZone(w, r, id)
}

// writeZone: id 0 = create. Audit ek tx e (11.7 item 13 x).
func (h *Handlers) writeZone(w http.ResponseWriter, r *http.Request, id int64) {
	var in zoneIn
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		h.adminErr(w, r, fieldErr("body"))
		return
	}
	if err := in.check(id == 0); err != nil {
		h.adminErr(w, r, err)
		return
	}
	var name any
	if in.Name != nil {
		b, _ := json.Marshal(in.Name)
		name = string(b)
	}
	st, _ := httpx.StaffFrom(r.Context())
	var before, after json.RawMessage
	err := db.InTx(r.Context(), h.db, func(tx *sql.Tx) error {
		var err error
		action := "zone.update"
		if id == 0 {
			action = "zone.create"
			id, after, err = insertZone(r.Context(), tx, st.ShopID, in, name)
		} else {
			before, after, err = updateZone(r.Context(), tx, st.ShopID, id, in, name)
		}
		if err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, audit.Entry{ShopID: st.ShopID, StaffID: st.ID, Action: action, EntityType: "zone",
			EntityID: id, Before: before, After: after, IP: httpx.IP(r.Context())})
	})
	if err != nil {
		h.adminErr(w, r, err)
		return
	}
	status := http.StatusOK
	if before == nil {
		status = http.StatusCreated
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(after)
}

func insertZone(ctx context.Context, tx *sql.Tx, shopID int64, in zoneIn, name any) (id int64, after json.RawMessage, err error) {
	minOrder := int64(0)
	if in.MinOrder != nil {
		minOrder = *in.MinOrder
	}
	active := in.IsActive == nil || *in.IsActive
	err = tx.QueryRowContext(ctx, `INSERT INTO delivery_zones (shop_id, name, district, area, charge, min_order, free_delivery_over, same_city, is_active)
		VALUES ($1, $2::jsonb, $3, $4, $5, $6, $7, $8, $9) RETURNING id, to_jsonb(delivery_zones.*) - 'shop_id'`,
		shopID, name, *in.District, in.Area.V, *in.Charge, minOrder, in.FreeDeliveryOver.V, *in.SameCity, active).Scan(&id, &after)
	return id, after, err
}

func updateZone(ctx context.Context, tx *sql.Tx, shopID, id int64, in zoneIn, name any) (before, after json.RawMessage, err error) {
	if err = tx.QueryRowContext(ctx, `SELECT to_jsonb(z) - 'shop_id' FROM delivery_zones z WHERE shop_id = $1 AND id = $2 FOR UPDATE`,
		shopID, id).Scan(&before); err != nil {
		return nil, nil, err
	}
	err = tx.QueryRowContext(ctx, `UPDATE delivery_zones SET name = coalesce($3::jsonb, name), district = coalesce($4, district),
		area = CASE WHEN $5 THEN $6::text ELSE area END, charge = coalesce($7, charge), min_order = coalesce($8, min_order),
		free_delivery_over = CASE WHEN $9 THEN $10::bigint ELSE free_delivery_over END,
		same_city = coalesce($11, same_city), is_active = coalesce($12, is_active)
		WHERE shop_id = $1 AND id = $2 RETURNING to_jsonb(delivery_zones.*) - 'shop_id'`,
		shopID, id, name, in.District, in.Area.Set, in.Area.V, in.Charge, in.MinOrder,
		in.FreeDeliveryOver.Set, in.FreeDeliveryOver.V, in.SameCity, in.IsActive).Scan(&after)
	return before, after, err
}
