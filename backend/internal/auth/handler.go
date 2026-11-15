package auth

import (
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

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
