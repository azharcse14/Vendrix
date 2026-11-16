package order

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"regexp"
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
	mux.Handle("POST /api/orders", wrap(http.HandlerFunc(h.Place)))
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

// idemKeyRE (8.19 item 12 i).
var idemKeyRE = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

type personIn struct {
	Name  string  `json:"name"`
	Phone string  `json:"phone"`
	Email *string `json:"email"`
}

type recipientIn struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type addressIn struct {
	District string `json:"district"`
	Area     string `json:"area"`
	Line     string `json:"line"`
}

// hashed: idempotency_hash er field, nirdishto kram e (8.6 step 1, 8.19 item 12 ii); turnstile_token,
// otp_code, website na (retry te bodlay).
type hashed struct {
	Customer      personIn     `json:"customer"`
	Recipient     *recipientIn `json:"recipient"`
	Address       addressIn    `json:"address"`
	Note          string       `json:"note"`
	CouponCode    string       `json:"coupon_code"`
	PaymentMethod string       `json:"payment_method"`
	ExpectedTotal *int64       `json:"expected_total"`
	Consent       bool         `json:"consent"`
}

// text: trim, min–max rune; ok false = bhul.
func text(s string, min, max int) (string, bool) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	return s, n >= min && n <= max
}

// Place: POST /api/orders (8.6). Dam/total er field nai (Rule 1); expected_total shudhu tulona.
func (h *Handlers) Place(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if !idemKeyRE.MatchString(key) {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "missing_idempotency_key"})
		return
	}
	var in struct {
		hashed
		OTPCode        *string `json:"otp_code"`
		TurnstileToken string  `json:"turnstile_token"` // ponytail: Turnstile verify M6 step 6 e
		Website        string  `json:"website"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	if in.ExpectedTotal == nil {
		invalid(w, r, "expected_total", "required")
		return
	}
	raw, err := json.Marshal(in.hashed)
	if err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	sum := sha256.Sum256(raw)
	p, field := placeInput(in.hashed)
	switch {
	case in.Website != "": // honeypot (8.6 step 3)
		field = "website"
	case !in.Consent:
		field = "consent"
	}
	if field != "" {
		invalid(w, r, field, "invalid")
		return
	}
	if in.OTPCode != nil {
		p.OTPCode = *in.OTPCode
	}
	p.Key, p.Hash, p.Locale, p.IP = key, sum[:], httpx.Locale(r), httpx.IP(r.Context())
	s, _ := httpx.ShopFrom(r.Context())
	out, err := PlaceOrder(r.Context(), h.db, s.ID, cart.OwnerFrom(r), p)
	h.placeResult(w, r, out, err)
}

// placeInput (8.19 item 10): format check; bhul field er naam ferot.
func placeInput(in hashed) (PlaceInput, string) {
	var p PlaceInput
	var ok bool
	var err error
	if p.Customer.Name, ok = text(in.Customer.Name, 1, 100); !ok {
		return p, "customer.name"
	}
	if p.Customer.Phone, err = phone.Normalize(in.Customer.Phone); err != nil {
		return p, "customer.phone"
	}
	if in.Customer.Email != nil {
		e := strings.TrimSpace(*in.Customer.Email)
		if a, err := mail.ParseAddress(e); e != "" && (err != nil || a.Address != e || len(e) > 254) {
			return p, "customer.email"
		}
		p.Customer.Email = e
	}
	p.Recipient = Person{Name: p.Customer.Name, Phone: p.Customer.Phone}
	if in.Recipient != nil {
		if p.Recipient.Name, ok = text(in.Recipient.Name, 1, 100); !ok {
			return p, "recipient.name"
		}
		if p.Recipient.Phone, err = phone.Normalize(in.Recipient.Phone); err != nil {
			return p, "recipient.phone"
		}
	}
	if p.District, ok = place(in.Address.District); !ok {
		return p, "address.district"
	}
	if p.Area, ok = place(in.Address.Area); !ok {
		return p, "address.area"
	}
	if p.Line, ok = text(in.Address.Line, 5, 500); !ok {
		return p, "address.line"
	}
	if p.Note, ok = text(in.Note, 0, 500); !ok {
		return p, "note"
	}
	if p.CouponCode, ok = coupon.Normalize(in.CouponCode); !ok {
		return p, "coupon_code"
	}
	p.PaymentMethod, p.ExpectedTotal = in.PaymentMethod, *in.ExpectedTotal
	return p, ""
}

func (h *Handlers) placeResult(w http.ResponseWriter, r *http.Request, out Placed, err error) {
	var fe FieldError
	var oos OutOfStockError
	var pc PriceChangedError
	conflict := func(code string, details map[string]any) {
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: code, Details: details})
	}
	switch {
	case err == nil:
		w.Header().Set("Cache-Control", "no-store")
		status := http.StatusOK
		if out.Created {
			status = http.StatusCreated
		}
		httpx.WriteJSON(w, status, out)
	case errors.As(err, &fe):
		invalid(w, r, string(fe), "invalid")
	case errors.Is(err, ErrKeyReused):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, httpx.ErrorBody{Code: "idempotency_key_reused"})
	case errors.Is(err, ErrPhoneBlocked):
		httpx.WriteError(w, r, http.StatusForbidden, httpx.ErrorBody{Code: "phone_blocked"})
	case errors.Is(err, ErrCartEmpty):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "cart_empty"})
	case errors.Is(err, ErrOTPInvalid):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "otp_invalid"})
	case errors.Is(err, shipping.ErrAreaNotServed):
		conflict("area_not_served", nil)
	case errors.As(err, &oos):
		conflict("out_of_stock", map[string]any{"variant_id": oos.VariantID})
	case errors.Is(err, ErrVendorClosed):
		conflict("vendor_unavailable", nil)
	case coupon.ErrorCode(err) != "":
		conflict(coupon.ErrorCode(err), nil)
	case errors.Is(err, ErrBelowMinOrder):
		conflict("below_min_order", nil)
	case errors.As(err, &pc):
		conflict("price_changed", map[string]any{"total": pc.Total, "subtotal": pc.Subtotal, "discount": pc.Discount,
			"delivery_charge": pc.DeliveryCharge})
	case errors.Is(err, ErrOTPRequired):
		conflict("otp_required", nil)
	default:
		h.log.ErrorContext(r.Context(), "place order", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
	}
}
