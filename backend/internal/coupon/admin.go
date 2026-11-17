package coupon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"vendrix/backend/internal/audit"
	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
)

// Handlers: coupon admin (7.7, 11.7 item 13 v).
type Handlers struct {
	db  *sql.DB
	log *slog.Logger
}

func NewHandlers(d *sql.DB, log *slog.Logger) *Handlers { return &Handlers{db: d, log: log} }

// AdminRoutes: shudhu owner/admin (11.2); delete na (is_active); vendor coupon MV e.
func (h *Handlers) AdminRoutes(mux *http.ServeMux, staff httpx.Middleware) {
	admin := func(f http.HandlerFunc) http.Handler { return staff(httpx.RequireRole("owner", "admin")(f)) }
	mux.Handle("GET /api/admin/coupons", admin(h.AdminCoupons))
	mux.Handle("POST /api/admin/coupons", admin(h.CreateCoupon))
	mux.Handle("PATCH /api/admin/coupons/{id}", admin(h.UpdateCoupon))
}

var codeRe = regexp.MustCompile(`^[A-Z0-9_-]{1,32}$`)

// opt: PATCH er nullable field: key thakle Set (null = muche).
type opt[T any] struct {
	Set bool
	V   *T
}

func (o *opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	return json.Unmarshal(b, &o.V)
}

// couponIn: vendor_id/funded_by/used_count ochena field (400); tarikh RFC 3339 string, nijei parse (fields.<f>).
type couponIn struct {
	Code          *string     `json:"code"`
	Kind          *string     `json:"kind"`
	Value         *int64      `json:"value"`
	MinOrder      *int64      `json:"min_order"`
	MaxDiscount   opt[int64]  `json:"max_discount"`
	UsageLimit    opt[int64]  `json:"usage_limit"`
	PerPhoneLimit *int64      `json:"per_phone_limit"`
	StartsAt      opt[string] `json:"starts_at"`
	ExpiresAt     opt[string] `json:"expires_at"`
	IsActive      *bool       `json:"is_active"`
}

// row: DB er coupon (admin er bodlano jay emon field); input er por ei rup validate hoy.
type row struct {
	Code                string
	Kind                string
	Value, MinOrder     int64
	MaxDiscount, Limit  *int64
	PerPhone, Used      int64
	StartsAt, ExpiresAt *time.Time
	IsActive            bool
}

// fieldErr: 400 validation_failed + fields.{field: reason}.
type fieldErr struct{ field, reason string }

func (e fieldErr) Error() string { return e.field + ": " + e.reason }

func bad(f string) error { return fieldErr{f, "invalid"} }

func when(o opt[string], field string) (*time.Time, error) {
	if o.V == nil {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *o.V)
	if err != nil {
		return nil, bad(field)
	}
	t = t.UTC()
	return &t, nil
}

// apply: input row e boshay (create e row = default). used_count > 0 hole code/kind/value lock, kintu ekoi
// value pathale chole (form puro object pathay, v1.45).
func (c *row) apply(in couponIn, create bool) error {
	used := c.Used > 0
	if create || in.Code != nil {
		code := ""
		if in.Code != nil {
			code, _ = Normalize(*in.Code)
		}
		switch {
		case !codeRe.MatchString(code):
			return bad("code")
		case strings.ToUpper(c.Code) == code:
		case used:
			return fieldErr{"code", "locked"}
		default:
			c.Code = code
		}
	}
	if create || in.Kind != nil {
		switch {
		case in.Kind == nil || (*in.Kind != "percent" && *in.Kind != "fixed"):
			return bad("kind")
		case used && *in.Kind != c.Kind:
			return fieldErr{"kind", "locked"}
		}
		c.Kind = *in.Kind
	}
	if create || in.Value != nil {
		switch {
		case in.Value == nil || *in.Value <= 0:
			return bad("value")
		case used && *in.Value != c.Value:
			return fieldErr{"value", "locked"}
		}
		c.Value = *in.Value
	}
	if in.MinOrder != nil {
		if *in.MinOrder < 0 {
			return bad("min_order")
		}
		c.MinOrder = *in.MinOrder
	}
	if in.MaxDiscount.Set {
		if v := in.MaxDiscount.V; v != nil && *v <= 0 {
			return bad("max_discount")
		}
		c.MaxDiscount = in.MaxDiscount.V
	}
	if in.UsageLimit.Set {
		if v := in.UsageLimit.V; v != nil && (*v <= 0 || *v > math.MaxInt32) {
			return bad("usage_limit")
		}
		c.Limit = in.UsageLimit.V
	}
	if in.PerPhoneLimit != nil {
		if v := *in.PerPhoneLimit; v < 1 || v > math.MaxInt32 {
			return bad("per_phone_limit")
		}
		c.PerPhone = *in.PerPhoneLimit
	}
	var err error
	if in.StartsAt.Set {
		if c.StartsAt, err = when(in.StartsAt, "starts_at"); err != nil {
			return err
		}
	}
	if in.ExpiresAt.Set {
		if c.ExpiresAt, err = when(in.ExpiresAt, "expires_at"); err != nil {
			return err
		}
	}
	if in.IsActive != nil {
		c.IsActive = *in.IsActive
	}
	// DB er CHECK er moto, kintu field shoho
	switch {
	case c.Kind == "percent" && c.Value > 100:
		return bad("value")
	case c.StartsAt != nil && c.ExpiresAt != nil && !c.StartsAt.Before(*c.ExpiresAt):
		return bad("expires_at")
	case c.Limit != nil && *c.Limit < c.Used:
		return fieldErr{"usage_limit", "below_used"}
	}
	return nil
}

func (h *Handlers) adminErr(w http.ResponseWriter, r *http.Request, err error) {
	var fe fieldErr
	var pg *pgconn.PgError
	switch {
	case errors.As(err, &fe):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{fe.field: fe.reason}})
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteError(w, r, http.StatusNotFound, httpx.ErrorBody{Code: "not_found"})
	case errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "coupons_code_uniq":
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "coupon_exists"})
	case errors.As(err, &pg) && pg.Code == "23514" && pg.ConstraintName == "coupons_used_le_limit_check":
		// lock er porei checkout used_count barate pare na (row lock), tobu shesh pahara
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{"usage_limit": "below_used"}})
	default:
		h.log.ErrorContext(r.Context(), "coupon admin", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
	}
}

// AdminCoupons: GET /api/admin/coupons?page=&limit=, notun age.
func (h *Handlers) AdminCoupons(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	rows, err := h.db.QueryContext(r.Context(), `SELECT to_jsonb(c) - 'shop_id', count(*) OVER () FROM coupons c
		WHERE shop_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, st.ShopID, limit, (page-1)*limit)
	if err != nil {
		h.adminErr(w, r, err)
		return
	}
	defer rows.Close()
	var items []json.RawMessage
	var total int64
	for rows.Next() {
		var c json.RawMessage
		if err := rows.Scan(&c, &total); err != nil {
			h.adminErr(w, r, err)
			return
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		h.adminErr(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

// CreateCoupon: POST /api/admin/coupons → 201 row.
func (h *Handlers) CreateCoupon(w http.ResponseWriter, r *http.Request) { h.write(w, r, 0) }

// UpdateCoupon: PATCH /api/admin/coupons/{id} → 200 row.
func (h *Handlers) UpdateCoupon(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.adminErr(w, r, sql.ErrNoRows)
		return
	}
	h.write(w, r, id)
}

// write: id 0 = create. PATCH e row FOR UPDATE (checkout er Use er sathe used_count thik thake), audit ek tx e.
func (h *Handlers) write(w http.ResponseWriter, r *http.Request, id int64) {
	var in couponIn
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		h.adminErr(w, r, bad("body"))
		return
	}
	ctx := r.Context()
	st, _ := httpx.StaffFrom(ctx)
	var before, after json.RawMessage
	err := db.InTx(ctx, h.db, func(tx *sql.Tx) error {
		c, action := row{PerPhone: 1, IsActive: true}, "coupon.create"
		if id != 0 {
			action = "coupon.update"
			var err error
			if c, before, err = lockCoupon(ctx, tx, st.ShopID, id); err != nil {
				return err
			}
		}
		if err := c.apply(in, id == 0); err != nil {
			return err
		}
		var err error
		if id, after, err = saveCoupon(ctx, tx, st.ShopID, id, c); err != nil {
			return err
		}
		return audit.Log(ctx, tx, audit.Entry{ShopID: st.ShopID, StaffID: st.ID, Action: action, EntityType: "coupon",
			EntityID: id, Before: before, After: after, IP: httpx.IP(ctx)})
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

func lockCoupon(ctx context.Context, tx *sql.Tx, shopID, id int64) (c row, before json.RawMessage, err error) {
	err = tx.QueryRowContext(ctx, `SELECT code, kind, value, min_order, max_discount, usage_limit, per_phone_limit, used_count,
		starts_at, expires_at, is_active, to_jsonb(c) - 'shop_id' FROM coupons c WHERE shop_id = $1 AND id = $2 FOR UPDATE`, shopID, id).
		Scan(&c.Code, &c.Kind, &c.Value, &c.MinOrder, &c.MaxDiscount, &c.Limit, &c.PerPhone, &c.Used, &c.StartsAt, &c.ExpiresAt, &c.IsActive, &before)
	return c, before, err
}

// saveCoupon: id 0 = INSERT; used_count kokhono ekhane bodlay na.
func saveCoupon(ctx context.Context, tx *sql.Tx, shopID, id int64, c row) (int64, json.RawMessage, error) {
	var after json.RawMessage
	args := []any{shopID, id, c.Code, c.Kind, c.Value, c.MinOrder, c.MaxDiscount, c.Limit, c.PerPhone, c.StartsAt, c.ExpiresAt, c.IsActive}
	q := `UPDATE coupons SET code = $3, kind = $4, value = $5, min_order = $6, max_discount = $7, usage_limit = $8,
		per_phone_limit = $9, starts_at = $10, expires_at = $11, is_active = $12
		WHERE shop_id = $1 AND id = $2 RETURNING id, to_jsonb(coupons.*) - 'shop_id'`
	if id == 0 {
		q = `INSERT INTO coupons (shop_id, code, kind, value, min_order, max_discount, usage_limit, per_phone_limit, starts_at, expires_at, is_active)
		SELECT $1, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12 WHERE $2::bigint = 0 RETURNING id, to_jsonb(coupons.*) - 'shop_id'`
	}
	err := tx.QueryRowContext(ctx, q, args...).Scan(&id, &after)
	return id, after, err
}
