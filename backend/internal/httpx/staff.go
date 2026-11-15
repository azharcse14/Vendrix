package httpx

import (
	"context"
	"net/http"
	"slices"
)

// Staff: admin request er staff (3.2 Niyom 3). VendorID nil = shop er staff (shob vendor); thakle shudhu
// oi vendor er (Rule 13). AdminSession session theke boshay.
type Staff struct {
	ID        int64
	ShopID    int64
	VendorID  *int64
	Role      string // owner, admin, staff, vendor_owner, vendor_staff (11.2)
	SessionID int64
	Setup     bool // setup_pending (8.3 step 4): shudhu setup route
}

// WithStaff: context e staff (AdminSession, test).
func WithStaff(ctx context.Context, s Staff) context.Context {
	return context.WithValue(ctx, staffKey, s)
}

// StaffFrom: WithStaff er staff; na thakle ok false.
func StaffFrom(ctx context.Context) (Staff, bool) {
	s, ok := ctx.Value(staffKey).(Staff)
	return s, ok
}

// RequireStaff (v1.10, 8.3 step 4): staff na thakle 401 unauthenticated (fail-closed); setup_pending session
// e 403 setup_required, karon password/TOTP setup er age admin er kaj na.
func RequireStaff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := StaffFrom(r.Context())
		switch {
		case !ok:
			WriteError(w, r, http.StatusUnauthorized, ErrorBody{Code: "unauthenticated"})
		case s.Setup:
			WriteError(w, r, http.StatusForbidden, ErrorBody{Code: "setup_required"})
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// RequireRole (11.2): staff er role roles e na thakle 403 forbidden (staff na thakle 401).
func RequireRole(roles ...string) Middleware {
	return func(next http.Handler) http.Handler {
		return RequireStaff(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s, _ := StaffFrom(r.Context()); !slices.Contains(roles, s.Role) {
				WriteError(w, r, http.StatusForbidden, ErrorBody{Code: "forbidden"})
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}
