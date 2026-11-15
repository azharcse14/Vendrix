package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

// Customer: customer route er login kora user (8.2, Rule 16); RequireCustomer context e rakhe.
type Customer struct {
	UserID    int64
	SessionID int64
}

var (
	ErrNoSession      = errors.New("session nai")            // 401 unauthenticated
	ErrSessionExpired = errors.New("session expire hoyeche") // 401 session_expired
)

// LookupCustomer: shop er customer session (Bearer token) → Customer; na pele ErrNoSession ba
// ErrSessionExpired. Lookup auth e, karon auth httpx import kore.
type LookupCustomer func(ctx context.Context, shopID int64, token string) (Customer, error)

// RequireCustomer (3.2, Rule 16): ShopFromHost er pore. Authorization: Bearer token oi shop er active
// customer session na hole 401; thakle context e Customer.
func RequireCustomer(log *slog.Logger, lookup LookupCustomer) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, _ := ShopFrom(r.Context())
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			cust, err := Customer{}, ErrNoSession
			if ok && token != "" && s.ID != 0 {
				cust, err = lookup(r.Context(), s.ID, token)
			}
			switch {
			case err == nil:
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), customerKey, cust)))
			case errors.Is(err, ErrSessionExpired):
				WriteError(w, r, http.StatusUnauthorized, ErrorBody{Code: "session_expired"})
			case errors.Is(err, ErrNoSession):
				WriteError(w, r, http.StatusUnauthorized, ErrorBody{Code: "unauthenticated"})
			default:
				log.ErrorContext(r.Context(), "session lookup", "request_id", RequestIDFrom(r.Context()), "err", err)
				WriteError(w, r, http.StatusInternalServerError, ErrorBody{Code: "internal"})
			}
		})
	}
}

// OptionalCustomer (8.4 v1.28 item 2): cart route. Authorization header na thakle guest (Customer nai);
// thakle RequireCustomer hubohu, tai bhul/expire token 401, chupchap guest cart e jay na.
func OptionalCustomer(log *slog.Logger, lookup LookupCustomer) Middleware {
	require := RequireCustomer(log, lookup)
	return func(next http.Handler) http.Handler {
		req := require(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				next.ServeHTTP(w, r)
				return
			}
			req.ServeHTTP(w, r)
		})
	}
}

// CustomerFrom: RequireCustomer er Customer; na thakle ok false.
func CustomerFrom(ctx context.Context) (Customer, bool) {
	c, ok := ctx.Value(customerKey).(Customer)
	return c, ok
}

// LookupStaff: __Host-asid token → staff session (active ba setup_pending; mfa_pending kokhono na) er Staff
// (SessionID, Setup shoho); na pele ErrNoSession ba ErrSessionExpired. Lookup auth e.
type LookupStaff func(ctx context.Context, token string) (Staff, error)

// AdminSession (3.2 Niyom 3, 11.3, Rule 16): admin route e __Host-asid cookie theke staff; shop context
// session theke (X-Shop-Host na). Bearer dekhe na, tai customer token admin e 401. Na pele 401; thakle
// WithStaff. Setup gate RequireStaff e.
func AdminSession(log *slog.Logger, lookup LookupStaff) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			st, err := Staff{}, ErrNoSession
			if c, cerr := r.Cookie(AdminCookieName); cerr == nil && c.Value != "" {
				st, err = lookup(r.Context(), c.Value)
			}
			switch {
			case err == nil:
				next.ServeHTTP(w, r.WithContext(WithStaff(r.Context(), st)))
			case errors.Is(err, ErrSessionExpired):
				WriteError(w, r, http.StatusUnauthorized, ErrorBody{Code: "session_expired"})
			case errors.Is(err, ErrNoSession):
				WriteError(w, r, http.StatusUnauthorized, ErrorBody{Code: "unauthenticated"})
			default:
				log.ErrorContext(r.Context(), "staff session lookup", "request_id", RequestIDFrom(r.Context()), "err", err)
				WriteError(w, r, http.StatusInternalServerError, ErrorBody{Code: "internal"})
			}
		})
	}
}
