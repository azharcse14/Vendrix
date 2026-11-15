package auth_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type meUser struct {
	Phone  string  `json:"phone"`
	Name   *string `json:"name"`
	Email  *string `json:"email"`
	Locale string  `json:"locale"`
}

// login: notun OTP (code jana) → verify (Accept-Language lang) → token ar user.
func (f *otpFixture) login(t *testing.T, host, e164, lang string) (string, meUser) {
	t.Helper()
	f.otpRow(t, f.shops[host], e164, "135790", "5 minutes")
	r := newReq(http.MethodPost, "/api/auth/otp/verify", host, "", map[string]string{"phone": e164, "code": "135790"})
	r.Header.Set("Accept-Language", lang)
	r.Header.Set("User-Agent", "test-agent")
	rec := f.serve(r)
	var out struct {
		Token string `json:"token"`
		User  meUser `json:"user"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Token == "" {
		t.Fatalf("login %s %s: %d %s", host, e164, rec.Code, rec.Body.String())
	}
	return out.Token, out.User
}

// me: GET /api/me; 200 hole user.
func (f *otpFixture) me(t *testing.T, host, token string) (meUser, *bytes.Buffer, int) {
	t.Helper()
	rec := f.serve(newReq(http.MethodGet, "/api/me", host, token, nil))
	var u meUser
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
			t.Fatal(err)
		}
	}
	return u, rec.Body, rec.Code
}

func str(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// TestCustomerSessionIntegration (8.2 verify step 4, 7; 3.2; 7.3; Rule 16; 12.2 Login 1, 5): token rand.Text,
// DB te shudhu sha256, 30 din, ip ar user_agent; notun user er locale Accept-Language theke (purono user er
// bodlay na); /api/me GET/PATCH; logout e revoke; expire session_expired, revoke/bhul/staff token
// unauthenticated; last_used_at ghontay ekbar.
func TestCustomerSessionIntegration(t *testing.T) {
	f := newOTPFixture(t)
	a := f.shops["a.com"]
	const e164 = "+8801711000001"
	tok, u := f.login(t, "a.com", e164, "en-US,en;q=0.9")
	if u.Phone != e164 || u.Name != nil || u.Email != nil || u.Locale != "en" {
		t.Errorf("verify er user: %+v", u)
	}
	var hash []byte
	var subject, stage, ip, ua string
	var days float64
	if err := f.d.QueryRow(`SELECT token_hash, subject_type, stage, ip, user_agent, extract(epoch FROM expires_at - now()) / 86400
		FROM sessions WHERE shop_id = $1`, a).Scan(&hash, &subject, &stage, &ip, &ua, &days); err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256([]byte(tok)); !bytes.Equal(hash, sum[:]) {
		t.Error("token_hash sha256(token) na")
	}
	if len(tok) < 26 || subject != "customer" || stage != "active" || ip != "192.0.2.1" || ua != "test-agent" ||
		days < 29.99 || days > 30.01 {
		t.Errorf("session: token %d okkhor, %s %s ip %q ua %q, %.3f din", len(tok), subject, stage, ip, ua, days)
	}

	got, _, code := f.me(t, "a.com", tok)
	if code != http.StatusOK || got != u {
		t.Errorf("GET /api/me: %d %+v", code, got)
	}
	tok2, u2 := f.login(t, "a.com", e164, "bn")
	if u2.Locale != "en" || f.count(t, `SELECT count(*) FROM users`) != 1 {
		t.Errorf("abar login e locale bodleche ba notun user: %+v", u2)
	}

	for name, h := range map[string]string{"header nai": "", "bhul token": "Bearer abc", "Basic": "Basic " + tok, "khali": "Bearer "} {
		r := newReq(http.MethodGet, "/api/me", "a.com", "", nil)
		if h != "" {
			r.Header.Set("Authorization", h)
		}
		wantErr(t, f.serve(r), 401, "unauthenticated", name)
	}
	staffTok := "staff-token-0123456789abcdef"
	sum := sha256.Sum256([]byte(staffTok))
	f.exec(t, `INSERT INTO sessions (shop_id, token_hash, subject_type, staff_id, expires_at)
		SELECT shop_id, $2, 'staff', id, now() + interval '1 hour' FROM staff WHERE shop_id = $1`, a, sum[:])
	wantErr(t, f.serve(newReq(http.MethodGet, "/api/me", "a.com", staffTok, nil)), 401, "unauthenticated", "staff token (Rule 16)")

	patch := func(body any) (int, string) {
		rec := f.serve(newReq(http.MethodPatch, "/api/me", "a.com", tok, body))
		return rec.Code, rec.Body.String()
	}
	if c, b := patch(map[string]any{"name": "  Rahim  ", "email": "r@x.com", "locale": "bn"}); c != http.StatusOK {
		t.Fatalf("PATCH: %d %s", c, b)
	}
	got, _, _ = f.me(t, "a.com", tok)
	if str(got.Name) != "Rahim" || str(got.Email) != "r@x.com" || got.Locale != "bn" || got.Phone != e164 {
		t.Errorf("PATCH er por: %+v %s %s", got, str(got.Name), str(got.Email))
	}
	if c, b := patch(map[string]any{"email": ""}); c != http.StatusOK {
		t.Fatalf("PATCH email khali: %d %s", c, b)
	}
	got, _, _ = f.me(t, "a.com", tok)
	if got.Email != nil || str(got.Name) != "Rahim" {
		t.Errorf("email khali korle NULL, naam thakbe: %s %s", str(got.Name), str(got.Email))
	}
	for name, body := range map[string]any{
		"locale fr":   map[string]any{"locale": "fr"},
		"bhul email":  map[string]any{"email": "nope"},
		"lomba naam":  map[string]any{"name": strings.Repeat("ক", 101)},
		"phone":       map[string]any{"phone": "+8801711000002"},
		"ochena json": "x",
	} {
		rec := f.serve(newReq(http.MethodPatch, "/api/me", "a.com", tok, body))
		wantErr(t, rec, 400, "validation_failed", name)
	}

	// last_used_at: ghontar kom hole bodlay na, beshi hole now
	f.exec(t, `UPDATE sessions SET last_used_at = now() - interval '30 minutes' WHERE token_hash = $1`, hash)
	f.me(t, "a.com", tok)
	age := func() float64 {
		var s float64
		f.d.QueryRow(`SELECT extract(epoch FROM now() - last_used_at) FROM sessions WHERE token_hash = $1`, hash).Scan(&s)
		return s
	}
	if s := age(); s < 1790 {
		t.Errorf("ghontar kom e last_used_at bodleche (%v second ager)", s)
	}
	f.exec(t, `UPDATE sessions SET last_used_at = now() - interval '2 hours' WHERE token_hash = $1`, hash)
	f.me(t, "a.com", tok)
	if s := age(); s > 5 {
		t.Errorf("ghonta par hole last_used_at bodlay na (%v second ager)", s)
	}

	sum2 := sha256.Sum256([]byte(tok2))
	f.exec(t, `UPDATE sessions SET expires_at = now() - interval '1 second' WHERE token_hash = $1`, sum2[:])
	wantErr(t, f.serve(newReq(http.MethodGet, "/api/me", "a.com", tok2, nil)), 401, "session_expired", "expire")

	if rec := f.serve(newReq(http.MethodPost, "/api/auth/logout", "a.com", tok, nil)); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
	}
	if n := f.count(t, `SELECT count(*) FROM sessions WHERE token_hash = $1 AND revoked_at IS NOT NULL`, hash); n != 1 {
		t.Error("logout e revoked_at nai")
	}
	wantErr(t, f.serve(newReq(http.MethodGet, "/api/me", "a.com", tok, nil)), 401, "unauthenticated", "logout er por")
	wantErr(t, f.serve(newReq(http.MethodPost, "/api/auth/logout", "a.com", tok, nil)), 401, "unauthenticated", "abar logout")
}

// TestSessionShopIsolationIntegration (3.2, Rule 3, 16; 12.2 Data 1): shop A er token shop B er X-Shop-Host e
// 401 (logout o kaj kore na); OTP o shop er; ekoi phone protiti shop e alada user, PATCH shudhu nijer shop e.
func TestSessionShopIsolationIntegration(t *testing.T) {
	f := newOTPFixture(t)
	const e164 = "+8801711000001"
	tokA, _ := f.login(t, "a.com", e164, "bn")
	wantErr(t, f.serve(newReq(http.MethodGet, "/api/me", "b.com", tokA, nil)), 401, "unauthenticated", "A er token B te")
	wantErr(t, f.serve(newReq(http.MethodPost, "/api/auth/logout", "b.com", tokA, nil)), 401, "unauthenticated", "A er token e B te logout")
	if _, _, c := f.me(t, "a.com", tokA); c != http.StatusOK {
		t.Fatalf("B te logout er chesta e A er session gelo: %d", c)
	}

	f.otpRow(t, f.shops["a.com"], e164, "246802", "5 minutes")
	wantErr(t, f.verify(t, "b.com", e164, "246802"), 400, "otp_invalid", "A er OTP B te")

	tokB, _ := f.login(t, "b.com", e164, "en")
	if n := f.count(t, `SELECT count(DISTINCT shop_id) FROM users WHERE phone = $1`, e164); n != 2 {
		t.Errorf("ekoi phone duita shop e user: %d shop, chai 2", n)
	}
	wantErr(t, f.serve(newReq(http.MethodGet, "/api/me", "a.com", tokB, nil)), 401, "unauthenticated", "B er token A te")
	if rec := f.serve(newReq(http.MethodPatch, "/api/me", "b.com", tokB, map[string]string{"name": "B"})); rec.Code != http.StatusOK {
		t.Fatalf("PATCH B: %d %s", rec.Code, rec.Body.String())
	}
	if ua, _, _ := f.me(t, "a.com", tokA); ua.Name != nil || ua.Locale != "bn" {
		t.Errorf("B er PATCH A er user e: %+v", ua)
	}
}
