package auth

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"

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
	noStore(w)
	if ok, retry := h.ipFails.Take(ctx); !ok {
		writeLimit(w, r, time.Duration(retry)*time.Second)
		return
	}
	token, stage, err := staffLogin(ctx, h.db, loginInput{Shop: in.Shop, Email: in.Email, Password: in.Password,
		IP: httpx.IP(ctx), UserAgent: userAgent(r)})
	if !errors.Is(err, errBadCredentials) { // shudhu bhul gona (7.8)
		h.ipFails.Refund(ctx)
	}
	switch {
	case err == nil && stage == "mfa_pending":
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"totp_required": true, "temp_token": token})
	case err == nil:
		httpx.AdminCookie(w, token, int(setupTTL.Seconds()))
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"setup_required": true})
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
	noStore(w)
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

// Password: POST /api/admin/auth/password {old, new} (AdminSession er pore; active ba setup_pending) → 204
// (8.3 step 4, 6). new 12–128 okkhor, old er sathe ekoi na; bhul old e 400 (fields.old). v1.25: TOTP live ar
// must_change_password e session active, notun __Host-asid (43200).
func (h *AdminHandlers) Password(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed"})
		return
	}
	bad := ""
	switch n := utf8.RuneCountInString(in.New); {
	case n < minPassword:
		bad = "too_short"
	case n > maxPassword:
		bad = "too_long"
	case in.New == in.Old:
		bad = "same_as_old"
	}
	if bad != "" {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{"new": bad}})
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	token, err := changePassword(r.Context(), h.db, st, in.Old, in.New, httpx.IP(r.Context()))
	var limit limitError
	switch {
	case err == nil:
		if token != "" {
			noStore(w)
			httpx.AdminCookie(w, token, int(adminTTL.Seconds()))
		}
		w.WriteHeader(http.StatusNoContent)
	case errors.As(err, &limit):
		writeLimit(w, r, limit.retry)
	case errors.Is(err, errWrongPassword):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed", Fields: map[string]string{"old": "wrong"}})
	default:
		h.internal(w, r, "staff password", err)
	}
}

// TOTPSetup: POST /api/admin/auth/totp/setup (shudhu setup_pending) → {otpauth_url, secret} (8.3 step 4, 6).
func (h *AdminHandlers) TOTPSetup(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	st, ok := h.setupOnly(w, r)
	if !ok {
		return
	}
	secret, u, err := totpSetup(r.Context(), h.db, h.totpKey, st, httpx.IP(r.Context()))
	if err != nil {
		h.setupErr(w, r, "totp setup", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"otpauth_url": u, "secret": secret})
}

// TOTPEnable: POST /api/admin/auth/totp/enable {code} (shudhu setup_pending) → {recovery_codes} + __Host-asid
// (43200), session active (8.3 step 4, 6).
func (h *AdminHandlers) TOTPEnable(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	st, ok := h.setupOnly(w, r)
	if !ok {
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "validation_failed"})
		return
	}
	token, codes, err := totpEnable(r.Context(), h.db, h.totpKey, h.recoverySecret, st, in.Code, httpx.IP(r.Context()))
	if err != nil {
		h.setupErr(w, r, "totp enable", err)
		return
	}
	httpx.AdminCookie(w, token, int(adminTTL.Seconds()))
	httpx.WriteJSON(w, http.StatusOK, map[string][]string{"recovery_codes": codes})
}

// setupOnly: TOTP setup/enable shudhu setup_pending e; active session e 403 forbidden, jate churi kora
// cookie diye TOTP bodle dewa na jay (TOTP reset staff er reset-2fa diye).
func (h *AdminHandlers) setupOnly(w http.ResponseWriter, r *http.Request) (httpx.Staff, bool) {
	st, _ := httpx.StaffFrom(r.Context())
	if !st.Setup {
		httpx.WriteError(w, r, http.StatusForbidden, httpx.ErrorBody{Code: "forbidden"})
	}
	return st, st.Setup
}

func (h *AdminHandlers) setupErr(w http.ResponseWriter, r *http.Request, what string, err error) {
	switch {
	case errors.Is(err, errPasswordChangeRequired):
		httpx.WriteError(w, r, http.StatusForbidden, httpx.ErrorBody{Code: "password_change_required"})
	case errors.Is(err, errTOTPLive):
		httpx.WriteError(w, r, http.StatusForbidden, httpx.ErrorBody{Code: "forbidden"})
	case errors.Is(err, errMFAInvalid):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.ErrorBody{Code: "mfa_invalid"})
	case errors.Is(err, httpx.ErrNoSession):
		httpx.WriteError(w, r, http.StatusUnauthorized, httpx.ErrorBody{Code: "unauthenticated"})
	default:
		h.internal(w, r, what, err)
	}
}

// Logout: POST /api/admin/auth/logout (active ba setup_pending) → 204, session revoke, cookie Max-Age=0.
func (h *AdminHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	st, _ := httpx.StaffFrom(r.Context())
	if err := revokeSession(r.Context(), h.db, st.ShopID, st.SessionID); err != nil {
		h.internal(w, r, "staff logout", err)
		return
	}
	httpx.AdminCookie(w, "", 0)
	w.WriteHeader(http.StatusNoContent)
}

// Me: GET /api/admin/me (RequireStaff er pore, shudhu active) → StaffProfile.
func (h *AdminHandlers) Me(w http.ResponseWriter, r *http.Request) {
	st, _ := httpx.StaffFrom(r.Context())
	p, err := staffProfile(r.Context(), h.db, st.ShopID, st.ID)
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusOK, p)
	case errors.Is(err, sql.ErrNoRows):
		httpx.WriteError(w, r, http.StatusUnauthorized, httpx.ErrorBody{Code: "unauthenticated"})
	default:
		h.internal(w, r, "staff me", err)
	}
}

// internal: 500; details shudhu log e (7.1).
func (h *AdminHandlers) internal(w http.ResponseWriter, r *http.Request, what string, err error) {
	h.log.ErrorContext(r.Context(), what, "request_id", httpx.RequestIDFrom(r.Context()), "err", err)
	httpx.WriteError(w, r, http.StatusInternalServerError, httpx.ErrorBody{Code: "internal"})
}
