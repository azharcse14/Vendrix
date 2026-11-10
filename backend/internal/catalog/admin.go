package catalog

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"vendrix/backend/internal/audit"
	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
)

// Routes: public route (public = ShopFromHost → RateLimit) ar admin route (staff ar role check ekhanei, 11.2).
func (h *Handlers) Routes(mux *http.ServeMux, public httpx.Middleware) {
	pub := func(f http.HandlerFunc) http.Handler { return public(f) }
	mux.Handle("GET /api/categories", pub(h.Categories))
	mux.Handle("GET /api/products", pub(h.Products))
	mux.Handle("GET /api/products/{slug}", pub(h.Product))

	admin := func(f http.HandlerFunc, roles ...string) http.Handler {
		return httpx.RequireStaff(httpx.RequireRole(roles...)(f))
	}
	cat := []string{"owner", "admin"}
	mux.Handle("GET /api/admin/categories", admin(h.AdminCategories, cat...))
	mux.Handle("POST /api/admin/categories", admin(h.CreateCategory, cat...))
	mux.Handle("PATCH /api/admin/categories/{id}", admin(h.UpdateCategory, cat...))
	mux.Handle("DELETE /api/admin/categories/{id}", admin(h.DeleteCategory, cat...))
	prod := []string{"owner", "admin", "vendor_owner"}
	mux.Handle("GET /api/admin/products", admin(h.AdminProducts, prod...))
	mux.Handle("POST /api/admin/products", admin(h.CreateProduct, prod...))
	mux.Handle("GET /api/admin/products/{id}", admin(h.AdminProduct, prod...))
	mux.Handle("PATCH /api/admin/products/{id}", admin(h.UpdateProduct, prod...))
	mux.Handle("POST /api/admin/products/{id}/variants", admin(h.CreateVariant, prod...))
	mux.Handle("PATCH /api/admin/variants/{id}", admin(h.UpdateVariant, prod...))
}

// apiErr: handler er jana error (status ar 7.1 body).
type apiErr struct {
	status int
	body   httpx.ErrorBody
}

func (e apiErr) Error() string { return e.body.Code }

func badField(field, reason string) apiErr {
	return apiErr{http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{field: reason}}}
}

var errNotFound = apiErr{http.StatusNotFound, httpx.ErrorBody{Code: "not_found"}}

// writeErr: apiErr, nai (404), ar DB constraint (unique 409, FK/CHECK 400) er error; baki 500.
func (h *Handlers) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var ae apiErr
	var pg *pgconn.PgError
	switch {
	case errors.As(err, &ae):
	case errors.Is(err, sql.ErrNoRows):
		ae = errNotFound
	case errors.As(err, &pg) && pg.Code == "23505" && strings.Contains(pg.ConstraintName, "sku"):
		ae = apiErr{http.StatusConflict, httpx.ErrorBody{Code: "sku_taken"}}
	case errors.As(err, &pg) && pg.Code == "23505":
		ae = apiErr{http.StatusConflict, httpx.ErrorBody{Code: "slug_taken"}}
	case errors.As(err, &pg) && (pg.Code == "23503" || pg.Code == "23514"):
		ae = badField(constraintField(pg.ConstraintName), "invalid")
	default:
		h.fail(w, r, err)
		return
	}
	httpx.WriteError(w, r, ae.status, ae.body)
}

// constraintField: constraint er naam theke field (onno shop er category/vendor/parent → oi field).
func constraintField(c string) string {
	for _, f := range [][2]string{{"parent_id", "parent_id"}, {"category_id", "category_id"}, {"product_id", "product_id"},
		{"vendor_id", "vendor_id"}, {"variable_weight", "variable_weight"}, {"unit", "unit"}, {"compare_at", "compare_at_price"},
		{"cost_price", "cost_price"}, {"nominal_weight", "nominal_weight_g"}, {"price", "price"}} {
		if strings.Contains(c, f[0]) {
			return f[1]
		}
	}
	return "body"
}

// write: admin kaj ek transaction e, audit shoho (12 Operation 3). fn ferot dey entity id, before, after.
func (h *Handlers) write(w http.ResponseWriter, r *http.Request, status int, action string,
	fn func(tx *sql.Tx, st httpx.Staff) (int64, json.RawMessage, json.RawMessage, error)) {
	st, _ := httpx.StaffFrom(r.Context())
	var after json.RawMessage
	err := db.InTx(r.Context(), h.db, func(tx *sql.Tx) error {
		id, before, a, err := fn(tx, st)
		if err != nil {
			return err
		}
		after = a
		entity, _, _ := strings.Cut(action, ".")
		return audit.Log(r.Context(), tx, audit.Entry{ShopID: st.ShopID, StaffID: st.ID, Action: action,
			EntityType: entity, EntityID: id, Before: before, After: a, IP: httpx.IP(r.Context())})
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if after == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(after)
}

// decode: body (DisallowUnknownFields); bhul hole 400 (boro hole 413).
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	err := httpx.DecodeJSON(w, r, v)
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		httpx.WriteError(w, r, http.StatusRequestEntityTooLarge, httpx.ErrorBody{Code: "validation_failed"})
	case err != nil:
		invalid(w, r, map[string]string{"body": "invalid"})
	}
	return err == nil
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// opt: PATCH er nullable field: key thakle Set (null hole V nil = muche dao).
type opt[T any] struct {
	Set bool
	V   *T
}

func (o *opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	return json.Unmarshal(b, &o.V)
}

// AdminCategories: GET /api/admin/categories, shob (inactive o), puro {"bn","en"}.
func (h *Handlers) AdminCategories(w http.ResponseWriter, r *http.Request) {
	st, _ := httpx.StaffFrom(r.Context())
	rows, err := categories(r.Context(), h.db, st.ShopID, false)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if rows == nil {
		rows = []categoryRow{}
	}
	httpx.WriteJSON(w, http.StatusOK, rows)
}

type categoryInput struct {
	ParentID  opt[int64]  `json:"parent_id"`
	Slug      *string     `json:"slug"`
	Name      Text        `json:"name"`
	ImageKey  opt[string] `json:"image_key"`
	SortOrder *int        `json:"sort_order"`
	IsActive  *bool       `json:"is_active"`
}

// check: create e slug ar name lagbe; PATCH e je field ache shudhu sheta.
func (in categoryInput) check(create bool) error {
	switch {
	case (create || in.Slug != nil) && (in.Slug == nil || !validSlug(*in.Slug)):
		return badField("slug", "invalid")
	case (create || in.Name != nil) && !in.Name.valid():
		return badField("name", "invalid")
	}
	return nil
}

// CreateCategory: POST /api/admin/categories → 201 notun row.
func (h *Handlers) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var in categoryInput
	if !decode(w, r, &in) {
		return
	}
	h.write(w, r, http.StatusCreated, "category.create", func(tx *sql.Tx, st httpx.Staff) (int64, json.RawMessage, json.RawMessage, error) {
		if err := in.check(true); err != nil {
			return 0, nil, nil, err
		}
		return createCategory(r.Context(), tx, st.ShopID, in)
	})
}

// UpdateCategory: PATCH /api/admin/categories/{id}; parent bodlale cycle check (shop er category lock kore).
func (h *Handlers) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	var in categoryInput
	if !ok {
		h.writeErr(w, r, errNotFound)
		return
	}
	if !decode(w, r, &in) {
		return
	}
	h.write(w, r, http.StatusOK, "category.update", func(tx *sql.Tx, st httpx.Staff) (int64, json.RawMessage, json.RawMessage, error) {
		if err := in.check(false); err != nil {
			return 0, nil, nil, err
		}
		if in.ParentID.Set && in.ParentID.V != nil {
			parents, err := lockCategoryParents(r.Context(), tx, st.ShopID)
			if err != nil {
				return 0, nil, nil, err
			}
			if _, ok := parents[id]; !ok {
				return 0, nil, nil, errNotFound
			}
			if makesCycle(parents, id, *in.ParentID.V) {
				return 0, nil, nil, badField("parent_id", "cycle")
			}
		}
		return updateCategory(r.Context(), tx, st.ShopID, id, in)
	})
}

// DeleteCategory: DELETE /api/admin/categories/{id}; product ba child thakle 409 category_not_empty (v1.10).
func (h *Handlers) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.writeErr(w, r, errNotFound)
		return
	}
	h.write(w, r, http.StatusNoContent, "category.delete", func(tx *sql.Tx, st httpx.Staff) (int64, json.RawMessage, json.RawMessage, error) {
		before, err := deleteCategory(r.Context(), tx, st.ShopID, id)
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23503" {
			err = apiErr{http.StatusConflict, httpx.ErrorBody{Code: "category_not_empty"}}
		}
		return id, before, nil, err
	})
}
