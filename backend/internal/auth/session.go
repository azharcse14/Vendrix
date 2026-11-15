package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"vendrix/backend/internal/httpx"
)

// customerSessionTTL (8.2 verify step 7): customer session 30 din (__Host-sid er Max-Age o, 10.3).
const customerSessionTTL = 30 * 24 * time.Hour

// newSessionToken (12.2 Login 1): rand.Text (26 okkhor base32, 130 bit); DB te shudhu sha256, jate DB
// leak e token na pay.
func newSessionToken() (string, []byte) {
	t := rand.Text()
	return t, tokenHash(t)
}

func tokenHash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// LookupSession (3.2, Rule 16): httpx.RequireCustomer er lookup. Shudhu ei shop er active customer session
// (revoke na, user delete na); expire hole ErrSessionExpired, baki shob ErrNoSession (onno shop er token
// ar bhul token ekoi, kichu bole na). last_used_at ghontay ekbar, jate protiti request e write na hoy.
func LookupSession(d *sql.DB) httpx.LookupCustomer {
	return func(ctx context.Context, shopID int64, token string) (httpx.Customer, error) {
		s, err := sessionByHash(ctx, d, shopID, tokenHash(token))
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return httpx.Customer{}, httpx.ErrNoSession
		case err != nil:
			return httpx.Customer{}, err
		case s.expired:
			return httpx.Customer{}, httpx.ErrSessionExpired
		}
		if s.stale {
			if err := touchSession(ctx, d, shopID, s.id); err != nil {
				return httpx.Customer{}, err
			}
		}
		return httpx.Customer{UserID: s.userID, SessionID: s.id}, nil
	}
}

// LookupStaffSession (3.2 Niyom 3, 11.3): httpx.AdminSession er lookup. Shudhu active ba setup_pending staff
// session, staff active; mfa_pending, customer, revoke ba onno token ErrNoSession, expire ErrSessionExpired.
// Shop session er row theke; last_used_at ghontay ekbar.
func LookupStaffSession(d *sql.DB) httpx.LookupStaff {
	return func(ctx context.Context, token string) (httpx.Staff, error) {
		s, err := staffSessionByHash(ctx, d, tokenHash(token))
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return httpx.Staff{}, httpx.ErrNoSession
		case err != nil:
			return httpx.Staff{}, err
		case s.expired:
			return httpx.Staff{}, httpx.ErrSessionExpired
		}
		if s.stale {
			if err := touchSession(ctx, d, s.staff.ShopID, s.staff.SessionID); err != nil {
				return httpx.Staff{}, err
			}
		}
		return s.staff, nil
	}
}
