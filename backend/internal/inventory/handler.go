package inventory

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"vendrix/backend/internal/httpx"
)

// Handlers: admin inventory API (7.7 Inventory).
type Handlers struct {
	db  *sql.DB
	log *slog.Logger
}

func NewHandlers(d *sql.DB, log *slog.Logger) *Handlers { return &Handlers{db: d, log: log} }

// Routes (11.2: staff/vendor_staff "stock entry ar wastage"): shob inventory role; vendor scope service e.
func (h *Handlers) Routes(mux *http.ServeMux, staff httpx.Middleware) {
	all := httpx.RequireRole("owner", "admin", "staff", "vendor_owner", "vendor_staff")
	admin := func(f http.HandlerFunc) http.Handler { return staff(all(f)) }
	mux.Handle("GET /api/admin/inventory/variants", admin(h.Variants))
	mux.Handle("GET /api/admin/variants/{id}/batches", admin(h.Batches))
	mux.Handle("POST /api/admin/batches", admin(h.CreateBatch))
	mux.Handle("POST /api/admin/inventory/adjust", admin(h.Adjust))
	mux.Handle("GET /api/admin/inventory/movements", admin(h.Movements))
}

func invalid(w http.ResponseWriter, r *http.Request, field, reason string) {
	httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{field: reason}})
}

// fail: service er error → 7.1.
func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	var oos *OutOfStockError
	var fe *FieldError
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, httpx.ErrorBody{Code: "not_found"})
	case errors.As(err, &oos):
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "out_of_stock", Details: map[string]any{"available": oos.Available}})
	case errors.As(err, &fe):
		invalid(w, r, fe.Field, fe.Reason)
	default:
		h.log.ErrorContext(r.Context(), "inventory", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
	}
}

func list[T any](h *Handlers, w http.ResponseWriter, r *http.Request, page, limit int, items []T, total int64, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

// Variants: GET /api/admin/inventory/variants?q=&low=&page=&limit= (11.7 item 1).
func (h *Handlers) Variants(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	q, low := strings.TrimSpace(r.URL.Query().Get("q")), r.URL.Query().Get("low")
	if utf8.RuneCountInString(q) > 100 {
		invalid(w, r, "q", "invalid")
		return
	}
	if low != "" && low != "0" && low != "1" {
		invalid(w, r, "low", "invalid")
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	items, total, err := Variants(r.Context(), h.db, st, q, low == "1", page, limit)
	list(h, w, r, page, limit, items, total, err)
}

// Batches: GET /api/admin/variants/{id}/batches (FIFO kram).
func (h *Handlers) Batches(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.fail(w, r, ErrNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	items, total, err := Batches(r.Context(), h.db, st, id, page, limit)
	list(h, w, r, page, limit, items, total, err)
}

// Movements: GET /api/admin/inventory/movements?variant_id=&page=&limit= (notun age).
func (h *Handlers) Movements(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	var id int64
	if v := r.URL.Query().Get("variant_id"); v != "" {
		var err error
		if id, err = strconv.ParseInt(v, 10, 64); err != nil || id <= 0 {
			invalid(w, r, "variant_id", "invalid")
			return
		}
	}
	st, _ := httpx.StaffFrom(r.Context())
	items, total, err := Movements(r.Context(), h.db, st, id, page, limit)
	list(h, w, r, page, limit, items, total, err)
}

const maxQty = 1_000_000

// date: YYYY-MM-DD ba nil.
func date(s *string) bool {
	if s == nil {
		return true
	}
	_, err := time.Parse(time.DateOnly, *s)
	return err == nil
}

// CreateBatch: POST /api/admin/batches {variant_id, quantity, cost_price?, batch_code?, harvest_date?, expiry_date?} → 201.
func (h *Handlers) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		VariantID   int64   `json:"variant_id"`
		Quantity    int     `json:"quantity"`
		CostPrice   *int64  `json:"cost_price"`
		BatchCode   *string `json:"batch_code"`
		HarvestDate *string `json:"harvest_date"`
		ExpiryDate  *string `json:"expiry_date"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	if in.BatchCode != nil {
		if c := strings.TrimSpace(*in.BatchCode); c == "" {
			in.BatchCode = nil
		} else {
			in.BatchCode = &c
		}
	}
	field := ""
	switch {
	case in.VariantID <= 0:
		field = "variant_id"
	case in.Quantity < 1 || in.Quantity > maxQty:
		field = "quantity"
	case in.CostPrice != nil && *in.CostPrice < 0:
		field = "cost_price"
	case in.BatchCode != nil && utf8.RuneCountInString(*in.BatchCode) > 64:
		field = "batch_code"
	case !date(in.HarvestDate):
		field = "harvest_date"
	case !date(in.ExpiryDate), in.ExpiryDate != nil && in.HarvestDate != nil && *in.ExpiryDate < *in.HarvestDate:
		field = "expiry_date"
	}
	if field != "" {
		invalid(w, r, field, "invalid")
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	b, err := CreateBatch(r.Context(), h.db, st, BatchInput{VariantID: in.VariantID, Quantity: in.Quantity, CostPrice: in.CostPrice,
		BatchCode: in.BatchCode, HarvestDate: in.HarvestDate, ExpiryDate: in.ExpiryDate})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, b)
}

// Adjust: POST /api/admin/inventory/adjust {variant_id, change < 0, reason wastage|adjust, note 1–500, batch_id?}.
func (h *Handlers) Adjust(w http.ResponseWriter, r *http.Request) {
	var in struct {
		VariantID int64  `json:"variant_id"`
		Change    int    `json:"change"`
		Reason    string `json:"reason"`
		Note      string `json:"note"`
		BatchID   *int64 `json:"batch_id"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	in.Note = strings.TrimSpace(in.Note)
	field := ""
	switch n := utf8.RuneCountInString(in.Note); {
	case in.VariantID <= 0:
		field = "variant_id"
	case in.Change >= 0 || in.Change < -maxQty:
		field = "change"
	case in.Reason != "wastage" && in.Reason != "adjust":
		field = "reason"
	case n < 1 || n > 500:
		field = "note"
	case in.BatchID != nil && *in.BatchID <= 0:
		field = "batch_id"
	}
	if field != "" {
		invalid(w, r, field, "invalid")
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	stock, err := Adjust(r.Context(), h.db, st, AdjustInput{VariantID: in.VariantID, Change: in.Change, Reason: in.Reason,
		Note: in.Note, BatchID: in.BatchID})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]int64{"variant_id": in.VariantID, "stock": int64(stock)})
}
