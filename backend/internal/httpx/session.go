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

// CustomerFrom: RequireCustomer er Customer; na thakle ok false.
func CustomerFrom(ctx context.Context) (Customer, bool) {
	c, ok := ctx.Value(customerKey).(Customer)
	return c, ok
}
