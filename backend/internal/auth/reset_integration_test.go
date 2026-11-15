package auth_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"vendrix/backend/internal/auth"
)

// TestStaffReset2FAIntegration (8.3 item 6, 7.7, Q8): reset-2fa ek transaction e TOTP (secret, nonce, key_version,
// last_step) ar recovery code muche, shob session (active, mfa_pending) revoke, audit staff.reset_2fa (actor CLI:
// staff_id NULL, entity_id = staff); porer login setup_pending. Onno shop er staff e hat na; ochena shop/email e
// spashto error, kichu bodlay na.
func TestStaffReset2FAIntegration(t *testing.T) {
	f := newStaffFixture(t)
	ctx := context.Background()
	a, own := f.shops["a.com"], f.owner["a"]
	b, ownB := f.shops["b.com"], f.owner["b"]

	rec := f.totp(f.tempToken(t, "a"), f.code(own, 0))
	active := adminCookie(t, rec, 43200, "active")
	f.tempToken(t, "a") // mfa_pending session o thake
	f.tempToken(t, "b")
	f.exec(t, `INSERT INTO staff_recovery_codes (shop_id, staff_id, code_hash) VALUES ($1, $2, '\x01'), ($3, $4, '\x02')`, a, own, b, ownB)

	for _, c := range []struct {
		slug, email string
		want        error
	}{
		{"nai", "o@a.com", auth.ErrShopNotFound},
		{"a", "nai@a.com", auth.ErrStaffNotFound},
		{"a", "o@b.com", auth.ErrStaffNotFound}, // shop b er staff shop a te na
	} {
		if err := auth.ResetStaff2FA(ctx, f.d, c.slug, c.email); !errors.Is(err, c.want) {
			t.Errorf("%s %s: %v, chai %v", c.slug, c.email, err, c.want)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM audit_logs WHERE action = 'staff.reset_2fa'`); n != 0 {
		t.Errorf("ochena e audit %d", n)
	}

	if err := auth.ResetStaff2FA(ctx, f.d, "a", "O@A.com"); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM staff WHERE id = $1 AND shop_id = $2 AND totp_secret_enc IS NULL
		AND totp_nonce IS NULL AND totp_key_version IS NULL AND totp_last_step IS NULL`, own, a); n != 1 {
		t.Error("TOTP muche ni")
	}
	if n := f.count(t, `SELECT count(*) FROM staff_recovery_codes WHERE shop_id = $1 AND staff_id = $2`, a, own); n != 0 {
		t.Errorf("recovery code %d ta baki", n)
	}
	if n := f.count(t, `SELECT count(*) FROM sessions WHERE shop_id = $1 AND staff_id = $2 AND revoked_at IS NULL`, a, own); n != 0 {
		t.Errorf("session %d ta revoke hoyni", n)
	}
	wantErr(t, f.cookieReq("GET", "/api/admin/me", active, nil), 401, "unauthenticated", "reset er por purono cookie")
	if n := f.count(t, `SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND action = 'staff.reset_2fa'
		AND staff_id IS NULL AND entity_type = 'staff' AND entity_id = $2 AND after->>'actor' = 'cli'`, a, own); n != 1 {
		t.Errorf("audit row %d ta", n)
	}
	f.setupSession(t, "a", staffPW) // porer login setup_pending (TOTP setup)

	// shop b er owner e hat na
	if n := f.count(t, `SELECT count(*) FROM staff_recovery_codes WHERE shop_id = $1`, b); n != 1 {
		t.Errorf("shop b er recovery code %d", n)
	}
	if n := f.count(t, `SELECT count(*) FROM sessions WHERE shop_id = $1 AND revoked_at IS NULL`, b); n != 1 {
		t.Errorf("shop b er session %d", n)
	}
	if rec := f.login("b", "o@b.com", staffPW); rec.Code != http.StatusOK || f.count(t,
		`SELECT count(*) FROM staff WHERE id = $1 AND totp_secret_enc IS NOT NULL`, ownB) != 1 {
		t.Errorf("shop b: %d %s", rec.Code, rec.Body.String())
	}
}
