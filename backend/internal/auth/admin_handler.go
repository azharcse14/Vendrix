package auth

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"vendrix/backend/internal/httpx"
)

// AdminHandlers: staff login er route (7.7 Login). Admin domain e shop context nai: login e shop body er slug
// theke, TOTP/recovery te temp_token er session theke.
type AdminHandlers struct {
	db             *sql.DB
	log            *slog.Logger
	totpKey        []byte
	recoverySecret string
	ipFails        *httpx.FailLimit // 7.8: client IP te 15 minute e 10 bhul login
}

func NewAdminHandlers(d *sql.DB, log *slog.Logger, totpKey []byte, recoverySecret string) *AdminHandlers {
	return &AdminHandlers{db: d, log: log, totpKey: totpKey, recoverySecret: recoverySecret,
		ipFails: httpx.NewFailLimit(10, 15*time.Minute)}
}

// Login: POST /api/admin/auth/login {shop, email, password} (8.3 step 1-2, 4) → TOTP thakle
// {totp_required, temp_token} (body te, cookie na), na hole {setup_required} + __Host-asid (900).
// Ochena shop/email, inactive ar bhul password ekoi 401 invalid_credentials.
func (h *AdminHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Shop     string `json:"shop"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed"})
		return
	}
	ctx := r.Context()
	if ok, retry := h.ipFails.Take(ctx); !ok {
		writeLimit(w, r, time.Duration(retry)*time.Second)
		return
	}
	token, stage, err := staffLogin(ctx, h.db, loginInput{Shop: in.Shop, Email: in.Email, Password: in.Password,
		IP: httpx.IP(ctx), UserAgent: userAgent(r)})
	if !errors.Is(err, errBadCredentials) { // shudhu bhul gona (7.8)
		h.ipFails.Refund(ctx)
	}
	var limit limitError
	switch {
	case err == nil && stage == "mfa_pending":
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"totp_required": true, "temp_token": token})
	case err == nil:
		httpx.AdminCookie(w, token, int(setupTTL.Seconds()))
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"setup_required": true})
	case errors.As(err, &limit):
		writeLimit(w, r, limit.retry)
	case errors.Is(err, errBadCredentials):
		httpx.WriteError(w, r, http.StatusUnauthorized, httpx.ErrorBody{Code: "invalid_credentials"})
	default:
		h.internal(w, r, "staff login", err)
	}
}

// TOTP: POST /api/admin/auth/totp {temp_token, code} (8.3 step 2-3) → __Host-asid + {setup_required}.
func (h *AdminHandlers) TOTP(w http.ResponseWriter, r *http.Request) { h.mfa(w, r, false) }

// Recovery: POST /api/admin/auth/recovery {temp_token, code} (8.3 step 3); TOTP er moto, code ekbar.
func (h *AdminHandlers) Recovery(w http.ResponseWriter, r *http.Request) { h.mfa(w, r, true) }

func (h *AdminHandlers) mfa(w http.ResponseWriter, r *http.Request, recovery bool) {
	var in struct {
		TempToken string `json:"temp_token"`
		Code      string `json:"code"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed"})
		return
	}
	token, setup, err := verifyMFA(r.Context(), h.db, h.totpKey, h.recoverySecret,
		mfaInput{Token: in.TempToken, Code: in.Code, IP: httpx.IP(r.Context()), Recovery: recovery})
	var limit limitError
	switch {
	case err == nil:
		ttl := adminTTL
		if setup {
			ttl = setupTTL
		}
		httpx.AdminCookie(w, token, int(ttl.Seconds()))
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"setup_required": setup})
	case errors.As(err, &limit):
		writeLimit(w, r, limit.retry)
	case errors.Is(err, httpx.ErrNoSession):
		httpx.WriteError(w, r, http.StatusUnauthorized, httpx.ErrorBody{Code: "unauthenticated"})
	case errors.Is(err, httpx.ErrSessionExpired):
		httpx.WriteError(w, r, http.StatusUnauthorized, httpx.ErrorBody{Code: "session_expired"})
	case errors.Is(err, errMFAInvalid):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "mfa_invalid"})
	case errors.Is(err, errMFAAttempts):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "mfa_too_many_attempts"})
	default:
		h.internal(w, r, "staff mfa", err)
	}
}

// internal: 500; details shudhu log e (7.1).
func (h *AdminHandlers) internal(w http.ResponseWriter, r *http.Request, what string, err error) {
	h.log.ErrorContext(r.Context(), what, "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
	httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
}
