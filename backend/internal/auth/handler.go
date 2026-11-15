package auth

import (
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"unicode/utf8"

	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/phone"
)

// Handlers: customer login er route (7.3). Public route ShopFromHost er pore boshe.
type Handlers struct {
	db        *sql.DB
	log       *slog.Logger
	otpSecret string
	turnstile Turnstile
}

func NewHandlers(d *sql.DB, log *slog.Logger, otpSecret string, ts Turnstile) *Handlers {
	return &Handlers{db: d, log: log, otpSecret: otpSecret, turnstile: ts}
}

// SendOTP: POST /api/auth/otp/send {phone, purpose: "login", turnstile_token} → {"sent": true} (8.2, 7.8).
// Phone er account ache kina bole na (Q10 vii).
func (h *Handlers) SendOTP(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone          string `json:"phone"`
		Purpose        string `json:"purpose"`
		TurnstileToken string `json:"turnstile_token"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed"})
		return
	}
	if in.Purpose != "login" { // checkout er OTP POST /api/orders theke (8.6)
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{"purpose": "invalid"}})
		return
	}
	p, err := phone.Normalize(in.Phone)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "invalid_phone", Fields: map[string]string{"phone": "invalid_phone"}})
		return
	}
	s, _ := httpx.ShopFrom(r.Context())
	err = sendOTP(r.Context(), h.db, h.otpSecret, h.turnstile, sendInput{ShopID: s.ID, Timezone: s.Timezone, Host: s.Domain,
		Phone: p, Purpose: in.Purpose, Token: in.TurnstileToken, IP: httpx.IP(r.Context())})
	var limit limitError
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"sent": true})
	case errors.As(err, &limit):
		w.Header().Set("Retry-After", strconv.Itoa(max(int(math.Ceil(limit.retry.Seconds())), 1)))
		httpx.WriteError(w, r, http.StatusTooManyRequests, httpx.ErrorBody{Code: "rate_limited"})
	case errors.Is(err, errTurnstile):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "turnstile_failed"})
	case errors.Is(err, errPhoneBlocked):
		httpx.WriteError(w, r, http.StatusForbidden, httpx.ErrorBody{Code: "phone_blocked"})
	default:
		h.log.ErrorContext(r.Context(), "otp send", "request_id", httpx.RequestIDFrom(r.Context()), "shop_id", s.ID,
			"phone", phone.Mask(p), "err", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
	}
}

// VerifyOTP: POST /api/auth/otp/verify {phone, code} → {token, user} (8.2, 7.3). Turnstile lage na (14.2
// test 23): OTP nijei 5 attempt ar phone lock e bandha.
func (h *Handlers) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed"})
		return
	}
	p, err := phone.Normalize(in.Phone)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "invalid_phone", Fields: map[string]string{"phone": "invalid_phone"}})
		return
	}
	s, _ := httpx.ShopFrom(r.Context())
	ua := r.UserAgent()
	if len(ua) > 512 { // client er lekha, tai shima; table na bare
		ua = strings.ToValidUTF8(ua[:512], "")
	}
	token, u, err := verifyOTP(r.Context(), h.db, h.otpSecret, verifyInput{ShopID: s.ID, Timezone: s.Timezone, Phone: p,
		Code: in.Code, Locale: httpx.Locale(r), IP: httpx.IP(r.Context()), UserAgent: ua})
	var limit limitError
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
	case errors.As(err, &limit):
		w.Header().Set("Retry-After", strconv.Itoa(max(int(math.Ceil(limit.retry.Seconds())), 1)))
		httpx.WriteError(w, r, http.StatusTooManyRequests, httpx.ErrorBody{Code: "rate_limited"})
	case errors.Is(err, errOTPInvalid):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "otp_invalid"})
	case errors.Is(err, errOTPAttempts):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "otp_too_many_attempts"})
	default:
		h.internal(w, r, "otp verify", err, "phone", phone.Mask(p))
	}
}

// Logout: POST /api/auth/logout (RequireCustomer er pore) → 204; ei session revoke (12.2 Login 5).
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	s, _ := httpx.ShopFrom(r.Context())
	c, _ := httpx.CustomerFrom(r.Context())
	if err := revokeSession(r.Context(), h.db, s.ID, c.SessionID); err != nil {
		h.internal(w, r, "logout", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Me: GET /api/me (RequireCustomer er pore) → User.
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	s, _ := httpx.ShopFrom(r.Context())
	c, _ := httpx.CustomerFrom(r.Context())
	u, err := getUser(r.Context(), h.db, s.ID, c.UserID)
	h.writeUser(w, r, u, err)
}

// UpdateMe: PATCH /api/me {name?, email?, locale?} (7.3, Q8). name max 100 okkhor, email ekta thikana
// (max 254), "" dile muche; locale bn ba en.
func (h *Handlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var p UserPatch
	var in struct {
		Name   *string `json:"name"`
		Email  *string `json:"email"`
		Locale *string `json:"locale"`
	}
	bad := map[string]string{}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed"})
		return
	}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if utf8.RuneCountInString(n) > 100 {
			bad["name"] = "too_long"
		}
		p.Name = &n
	}
	if in.Email != nil {
		e := strings.TrimSpace(*in.Email)
		if a, err := mail.ParseAddress(e); e != "" && (err != nil || a.Address != e || len(e) > 254) {
			bad["email"] = "invalid"
		}
		p.Email = &e
	}
	if in.Locale != nil {
		if *in.Locale != "bn" && *in.Locale != "en" {
			bad["locale"] = "invalid"
		}
		p.Locale = in.Locale
	}
	if len(bad) > 0 {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: bad})
		return
	}
	s, _ := httpx.ShopFrom(r.Context())
	c, _ := httpx.CustomerFrom(r.Context())
	u, err := updateUser(r.Context(), h.db, s.ID, c.UserID, p)
	h.writeUser(w, r, u, err)
}

// writeUser: user delete hoye gele (session thakleo) 401.
func (h *Handlers) writeUser(w http.ResponseWriter, r *http.Request, u User, err error) {
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusOK, u)
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteError(w, r, http.StatusUnauthorized, httpx.ErrorBody{Code: "unauthenticated"})
	default:
		h.internal(w, r, "me", err)
	}
}

// internal: 500; details shudhu log e (7.1).
func (h *Handlers) internal(w http.ResponseWriter, r *http.Request, what string, err error, attrs ...any) {
	s, _ := httpx.ShopFrom(r.Context())
	h.log.ErrorContext(r.Context(), what, append([]any{"request_id", httpx.RequestIDFrom(r.Context()), "shop_id", s.ID, "err", err}, attrs...)...)
	httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
}
