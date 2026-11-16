package order

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/coupon"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/phone"
	"vendrix/backend/internal/shipping"
)

// Handlers: checkout ar order route (7.5).
type Handlers struct {
	db    *sql.DB
	log   *slog.Logger
	guard Guard
}

// Guard: 8.6 step 3 er Turnstile ar step 8 er checkout OTP (auth er login er ekoi niyom).
type Guard struct {
	Turnstile     auth.Turnstile
	OTPSecret     string
	ConfirmSecret string       // COD confirm token er HMAC (8.10)
	OTPIP         *httpx.Limit // login er OTP send er ekoi client IP limit (7.8); nil = OTP pathano na (fail closed)
}

func NewHandlers(d *sql.DB, log *slog.Logger, g Guard) *Handlers {
	return &Handlers{db: d, log: log, guard: g}
}

// Routes: wrap = public + OptionalCustomer (cart er moto: login e account cart, na hole X-Cart-Key).
func (h *Handlers) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	// quote: client IP te minute e 30 (7.8); cart/coupon/zone er query bot e na chale
	mux.Handle("POST /api/checkout/quote", wrap(httpx.RateLimit(30, time.Minute)(http.HandlerFunc(h.Quote))))
	mux.Handle("POST /api/orders", wrap(http.HandlerFunc(h.Place)))
	mux.Handle("GET /api/orders/{token}", wrap(http.HandlerFunc(h.View)))
	mux.Handle("POST /api/orders/{token}/cancel", wrap(http.HandlerFunc(h.Cancel)))
	// track: public er upore client IP te minute e 10 (7.8); number + phone guess atkate
	mux.Handle("POST /api/orders/track", wrap(httpx.RateLimit(10, time.Minute)(http.HandlerFunc(h.Track))))
	mux.Handle("GET /api/confirm/{confirm_token}", wrap(http.HandlerFunc(h.ConfirmView)))
	mux.Handle("POST /api/confirm/{confirm_token}", wrap(http.HandlerFunc(h.Confirm)))
}

// MeRoutes (7.3): wrap = public + RequireCustomer (Rule 16).
func (h *Handlers) MeRoutes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /api/me/orders", wrap(http.HandlerFunc(h.MyOrders)))
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
		QuoteInput{CouponCode: code, District: district, Area: area, PaymentMethod: in.PaymentMethod, Phone: p})
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
		TurnstileToken string  `json:"turnstile_token"`
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
	s, _ := httpx.ShopFrom(r.Context())
	p.Key, p.Hash, p.Locale, p.IP = key, sum[:], httpx.Locale(r), httpx.IP(r.Context())
	p.TurnstileToken, p.Host, p.Timezone = in.TurnstileToken, s.Domain, s.Timezone
	out, err := PlaceOrder(r.Context(), h.db, h.guard, s.ID, cart.OwnerFrom(r), p)
	h.placeResult(w, r, out, err, p.Customer.Phone)
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

func (h *Handlers) placeResult(w http.ResponseWriter, r *http.Request, out Placed, err error, customerPhone string) {
	var fe FieldError
	var le LimitError
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
	case errors.Is(err, ErrOTPAttempts):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "otp_too_many_attempts"})
	case errors.Is(err, ErrTurnstile):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "turnstile_failed"})
	case errors.As(err, &le):
		w.Header().Set("Retry-After", strconv.Itoa(max(int(math.Ceil(le.Retry.Seconds())), 1)))
		httpx.WriteError(w, r, http.StatusTooManyRequests, httpx.ErrorBody{Code: "rate_limited"})
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
		conflict("otp_required", map[string]any{"phone_masked": maskPhone(customerPhone)})
	default:
		h.log.ErrorContext(r.Context(), "place order", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
	}
}

// maskPhone (8.19 item 4, 6): +88017****5678; customer dekhe kon number e code gelo.
func maskPhone(p string) string {
	if len(p) < 10 {
		return "****"
	}
	return p[:6] + "****" + p[len(p)-4:]
}

// View: GET /api/orders/{token} (7.5, Rule 11).
func (h *Handlers) View(w http.ResponseWriter, r *http.Request) {
	s, _ := httpx.ShopFrom(r.Context())
	v, err := ViewByToken(r.Context(), h.db, s.ID, r.PathValue("token"))
	if err != nil {
		h.transitionError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, v)
}

// Track: POST /api/orders/track {number, phone} (7.5; phone body te, URL/log e na).
func (h *Handlers) Track(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Number string `json:"number"`
		Phone  string `json:"phone"`
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
	s, _ := httpx.ShopFrom(r.Context())
	// order number prefix boro hater; customer chhoto hater e likhleo mile
	t, err := Track(r.Context(), h.db, s.ID, strings.ToUpper(strings.TrimSpace(in.Number)), p)
	if err != nil {
		h.transitionError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, t)
}

// MyOrders: GET /api/me/orders (7.3, Rule 16: session er user_id).
func (h *Handlers) MyOrders(w http.ResponseWriter, r *http.Request) {
	page, limit, ok := httpx.Page(w, r)
	if !ok {
		return
	}
	s, _ := httpx.ShopFrom(r.Context())
	c, _ := httpx.CustomerFrom(r.Context())
	items, total, err := MyOrders(r.Context(), h.db, s.ID, c.UserID, page, limit)
	if err != nil {
		h.transitionError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, httpx.NewPaged(items, page, limit, total))
}

// Cancel: POST /api/orders/{token}/cancel (7.5, 8.19 item 12 vii): body {reason?} max 200.
func (h *Handlers) Cancel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	reason, ok := text(in.Reason, 0, 200)
	if !ok {
		invalid(w, r, "reason", "invalid")
		return
	}
	by := By{Actor: Customer}
	if c, ok := httpx.CustomerFrom(r.Context()); ok {
		by.ID = c.UserID
	}
	s, _ := httpx.ShopFrom(r.Context())
	if err := CancelByToken(r.Context(), h.db, s.ID, r.PathValue("token"), by, reason); err != nil {
		h.transitionError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": string(Cancelled)})
}

// ConfirmView: GET /api/confirm/{confirm_token} (8.10 item 2); public_token diye hoy na (Rule 11).
func (h *Handlers) ConfirmView(w http.ResponseWriter, r *http.Request) {
	s, _ := httpx.ShopFrom(r.Context())
	v, err := ConfirmView(r.Context(), h.db, s.ID, r.PathValue("confirm_token"))
	if err != nil {
		h.transitionError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, v)
}

// Confirm: POST /api/confirm/{confirm_token}; body lage na. Abar confirm 409 invalid_transition.
func (h *Handlers) Confirm(w http.ResponseWriter, r *http.Request) {
	by := By{Actor: Customer}
	if c, ok := httpx.CustomerFrom(r.Context()); ok {
		by.ID = c.UserID
	}
	s, _ := httpx.ShopFrom(r.Context())
	if err := ConfirmByToken(r.Context(), h.db, s.ID, r.PathValue("confirm_token"), by); err != nil {
		h.transitionError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": string(Confirmed)})
}

// transitionError: TransitionOrder/TransitionFulfillment ar order khoja (view, track, me/orders) er error → 7.1.
func (h *Handlers) transitionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, httpx.ErrorBody{Code: "not_found"})
	case errors.Is(err, ErrInvalidTransition):
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "invalid_transition"})
	case errors.Is(err, ErrPaymentRequired):
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "payment_required"})
	default:
		h.log.ErrorContext(r.Context(), "order transition", "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
	}
}
