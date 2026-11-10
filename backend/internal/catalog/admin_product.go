package catalog

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"vendrix/backend/internal/httpx"
)

// vendorScope (Rule 13): vendor staff hole tar vendor (SQL e `$n IS NULL OR vendor_id = $n`), shop staff e nil.
func vendorScope(st httpx.Staff) any {
	if st.VendorID == nil {
		return nil
	}
	return *st.VendorID
}

type productInput struct {
	VendorID       *int64          `json:"vendor_id"` // shudhu create e
	CategoryID     opt[int64]      `json:"category_id"`
	Slug           *string         `json:"slug"`
	Name           Text            `json:"name"`
	Description    Text            `json:"description"`
	Unit           *string         `json:"unit"`
	VariableWeight *bool           `json:"variable_weight"`
	Attributes     json.RawMessage `json:"attributes"`
	SearchKeywords *string         `json:"search_keywords"`
	SEO            json.RawMessage `json:"seo"`
	IsActive       *bool           `json:"is_active"`
}

// obj: JSON object (nil = dey nai).
func obj(m json.RawMessage) bool {
	var v map[string]any
	return m == nil || json.Unmarshal(m, &v) == nil && v != nil
}

func rawArg(m json.RawMessage) any {
	if m == nil {
		return nil
	}
	return string(m)
}

// check: unit ar variable_weight er mil DB CHECK dhore (400 variable_weight).
func (in productInput) check(create bool) error {
	switch {
	case (create || in.Slug != nil) && (in.Slug == nil || !validSlug(*in.Slug)):
		return badField("slug", "invalid")
	case (create || in.Name != nil) && !in.Name.valid():
		return badField("name", "invalid")
	case !in.Description.keysOK():
		return badField("description", "invalid")
	case !obj(in.Attributes):
		return badField("attributes", "invalid")
	case !obj(in.SEO):
		return badField("seo", "invalid")
	case !create && in.VendorID != nil:
		return badField("vendor_id", "read_only")
	}
	return nil
}

// AdminProducts: GET /api/admin/products (paging; shop staff `?vendor_id=`, vendor staff shudhu nijer, Rule 13).
func (h *Handlers) AdminProducts(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	vendor := vendorScope(st)
	if v := r.URL.Query().Get("vendor_id"); v != "" && vendor == nil {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			invalid(w, r, map[string]string{"vendor_id": "invalid"})
			return
		}
		vendor = n
	}
	items, total, err := adminProducts(r.Context(), h.db, st.ShopID, vendor, limit, (page-1)*limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

// AdminProduct: GET /api/admin/products/{id}: puro row, shob variant ar chobi; onno shop/vendor → 404.
func (h *Handlers) AdminProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.writeErr(w, r, errNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	p, err := adminProduct(r.Context(), h.db, st.ShopID, id, vendorScope(st))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

// CreateProduct: POST /api/admin/products (v1.10): vendor_id na dile shop staff e house vendor, vendor staff e
// nijer; vendor staff onno vendor dile 404.
func (h *Handlers) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var in productInput
	if !decode(w, r, &in) {
		return
	}
	h.write(w, r, http.StatusCreated, "product.create", func(tx *sql.Tx, st httpx.Staff) (int64, json.RawMessage, json.RawMessage, error) {
		if err := in.check(true); err != nil {
			return 0, nil, nil, err
		}
		if st.VendorID != nil {
			if in.VendorID != nil && *in.VendorID != *st.VendorID {
				return 0, nil, nil, errNotFound
			}
			in.VendorID = st.VendorID
		}
		return createProduct(r.Context(), tx, st.ShopID, in)
	})
}

// UpdateProduct: PATCH /api/admin/products/{id}; hard delete na, is_active diye bondho (6.1).
func (h *Handlers) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		h.writeErr(w, r, errNotFound)
		return
	}
	var in productInput
	if !decode(w, r, &in) {
		return
	}
	h.write(w, r, http.StatusOK, "product.update", func(tx *sql.Tx, st httpx.Staff) (int64, json.RawMessage, json.RawMessage, error) {
		if err := in.check(false); err != nil {
			return 0, nil, nil, err
		}
		return updateProduct(r.Context(), tx, st.ShopID, id, vendorScope(st), in)
	})
}
