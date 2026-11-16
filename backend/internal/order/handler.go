package order

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/coupon"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/phone"
	"vendrix/backend/internal/shipping"
)

// Handlers: checkout ar order route (7.5).
type Handlers struct {
	db  *sql.DB
	log *slog.Logger
}

func NewHandlers(d *sql.DB, log *slog.Logger) *Handlers { return &Handlers{db: d, log: log} }

// Routes: wrap = public + OptionalCustomer (cart er moto: login e account cart, na hole X-Cart-Key).
func (h *Handlers) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("POST /api/checkout/quote", wrap(http.HandlerFunc(h.Quote)))
}

func invalid(w http.ResponseWriter, r *http.Request, field, code string) {
	httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed",
		Fields: map[string]string{field: code}})
}

// place: district/area 1–100 rune (8.19 item 2).
func place(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != "" && utf8.RuneCountInString(s) <= 100
}

// Quote: POST /api/checkout/quote. Dam/total er field nai (DisallowUnknownFields e 400, Rule 1).
func (h *Handlers) Quote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CouponCode    string `json:"coupon_code"`
		District      string `json:"district"`
		Area          string `json:"area"`
		PaymentMethod string `json:"payment_method"`
		Phone         string `json:"phone"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	p, err := phone.Normalize(in.Phone)
	if err != nil {
		invalid(w, r, "phone", "invalid_phone")
		return
	}
	code, ok := coupon.Normalize(in.CouponCode)
	if !ok {
		invalid(w, r, "coupon_code", "invalid")
		return
	}
	district, ok := place(in.District)
	if !ok {
		invalid(w, r, "district", "invalid")
		return
	}
	area, ok := place(in.Area)
	if !ok {
		invalid(w, r, "area", "invalid")
		return
	}
	s, _ := httpx.ShopFrom(r.Context())
	q, err := QuoteCart(r.Context(), h.db, s.ID, cart.OwnerFrom(r),
		QuoteInput{CouponCode: code, District: district, Area: area, PaymentMethod: in.PaymentMethod, Phone: p},
		httpx.IP(r.Context()))
	var fe FieldError
	switch {
	case errors.As(err, &fe):
		invalid(w, r, string(fe), "invalid")
	case errors.Is(err, shipping.ErrAreaNotServed):
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "area_not_served"})
	case errors.Is(err, ErrCartEmpty):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "cart_empty"})
	case err != nil:
		h.log.ErrorContext(r.Context(), "checkout quote", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
	default:
		w.Header().Set("Cache-Control", "no-store")
		httpx.WriteJSON(w, http.StatusOK, q)
	}
}
