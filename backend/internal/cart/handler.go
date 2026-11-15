package cart

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"vendrix/backend/internal/httpx"
)

// Handlers: cart route (7.4). Router e ShopFromHost → RateLimit → OptionalCustomer er pore.
type Handlers struct {
	db  *sql.DB
	log *slog.Logger
}

func NewHandlers(d *sql.DB, log *slog.Logger) *Handlers { return &Handlers{db: d, log: log} }

// Routes: wrap = public + OptionalCustomer.
func (h *Handlers) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /api/cart", wrap(http.HandlerFunc(h.Get)))
	mux.Handle("POST /api/cart/items", wrap(http.HandlerFunc(h.Add)))
	mux.Handle("PATCH /api/cart/items/{variant_id}", wrap(http.HandlerFunc(h.Patch)))
	mux.Handle("DELETE /api/cart/items/{variant_id}", wrap(http.HandlerFunc(h.Delete)))
}

// owner (8.4 item 2): login thakle account cart, key ignore; na hole X-Cart-Key (bhul format = nai).
func owner(r *http.Request) Owner {
	if c, ok := httpx.CustomerFrom(r.Context()); ok {
		return Owner{UserID: c.UserID}
	}
	hash, _ := KeyHash(r.Header.Get("X-Cart-Key"))
	return Owner{KeyHash: hash}
}

func shopID(r *http.Request) int64 {
	s, _ := httpx.ShopFrom(r.Context())
	return s.ID
}

func invalid(w http.ResponseWriter, r *http.Request, field string) {
	httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed",
		Fields: map[string]string{field: "invalid"}})
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	var oos OutOfStockError
	switch {
	case errors.As(err, &oos):
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "out_of_stock",
			Details: map[string]any{"variant_id": oos.VariantID, "max_quantity": oos.MaxQuantity}})
	case errors.Is(err, ErrInvalidQuantity):
		invalid(w, r, "quantity")
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, httpx.ErrorBody{Code: "not_found"})
	case errors.Is(err, ErrCartFull):
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "cart_full"})
	default:
		h.log.ErrorContext(r.Context(), "cart handler", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
	}
}

// reply: puro cart, no-store (7.4); newKey shudhu notun guest cart e.
func (h *Handlers) reply(w http.ResponseWriter, r *http.Request, o Owner, newKey string) {
	if newKey != "" {
		o.KeyHash, _ = KeyHash(newKey)
	}
	c, err := View(r.Context(), h.db, shopID(r), o, httpx.Locale(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	c.CartKey = newKey
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, c)
}

func variantPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("variant_id"), 10, 64)
	if err != nil || id <= 0 {
		invalid(w, r, "variant_id")
		return 0, false
	}
	return id, true
}

// Get: GET /api/cart.
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) { h.reply(w, r, owner(r), "") }

// Add: POST /api/cart/items {variant_id, quantity}; dam/total er field nai (DisallowUnknownFields, Rule 1).
func (h *Handlers) Add(w http.ResponseWriter, r *http.Request) {
	var in struct {
		VariantID int64 `json:"variant_id"`
		Quantity  int   `json:"quantity"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body")
		return
	}
	if in.VariantID <= 0 {
		invalid(w, r, "variant_id")
		return
	}
	o := owner(r)
	key, err := Add(r.Context(), h.db, shopID(r), o, in.VariantID, in.Quantity)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.reply(w, r, o, key)
}

// Patch: PATCH /api/cart/items/{variant_id} {quantity}.
func (h *Handlers) Patch(w http.ResponseWriter, r *http.Request) {
	id, ok := variantPath(w, r)
	if !ok {
		return
	}
	var in struct {
		Quantity int `json:"quantity"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body")
		return
	}
	o := owner(r)
	if err := SetQuantity(r.Context(), h.db, shopID(r), o, id, in.Quantity); err != nil {
		h.fail(w, r, err)
		return
	}
	h.reply(w, r, o, "")
}

// Delete: DELETE /api/cart/items/{variant_id}; na thakleo 200 (8.4 item 7).
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := variantPath(w, r)
	if !ok {
		return
	}
	o := owner(r)
	if err := Remove(r.Context(), h.db, shopID(r), o, id); err != nil {
		h.fail(w, r, err)
		return
	}
	h.reply(w, r, o, "")
}
