package auth_test

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"vendrix/backend/internal/auth"
)

// cookieReq: __Host-asid cookie (tok khali hole na) shoho admin request.
func (f *staffFixture) cookieReq(method, path, tok string, body any) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.RemoteAddr = "192.0.2.9:1234"
	if tok != "" {
		r.AddCookie(&http.Cookie{Name: "__Host-asid", Value: tok})
	}
	rec := httptest.NewRecorder()
	f.admin.ServeHTTP(rec, r)
	return rec
}

// setupSession: thik password e login → setup_pending cookie (900).
func (f *staffFixture) setupSession(t *testing.T, shop, pw string) string {
	t.Helper()
	rec := f.login(shop, "o@"+shop+".com", pw)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"setup_required":true}` {
		t.Fatalf("setup login %s: %d %s", shop, rec.Code, rec.Body.String())
	}
	return adminCookie(t, rec, 900, "setup login")
}

var recoveryFormat = regexp.MustCompile(`^[A-Z2-7]{5}-[A-Z2-7]{5}$`)

// TestStaffSetupIntegration (8.3 step 4, 6, 7.7, 11.3, Q10): must_change_password ar TOTP nai → setup_pending
// (900); setup session e admin API 403 setup_required; age password (old thik, 12–128 okkhor, purono na;
// must_change false, onno shob session revoke, audit), tarpor /auth/totp/setup ({otpauth_url, secret}, abar
// dakle replace), /auth/totp/enable (bhul/purono secret er code na; thik hole 10 recovery code, purono code
// muche, session active 12 ghonta + notun token); active e setup route na; /me; logout revoke + Max-Age=0;
// expire session_expired; inactive staff 401.
func TestStaffSetupIntegration(t *testing.T) {
	f := newStaffFixture(t)
	a, own := f.shops["a.com"], f.owner["a"]
	f.exec(t, `UPDATE staff SET must_change_password = true, totp_secret_enc = NULL, totp_nonce = NULL, totp_key_version = NULL
		WHERE id = $1 AND shop_id = $2`, own, a)
	f.exec(t, `INSERT INTO staff_recovery_codes (shop_id, staff_id, code_hash) VALUES ($1, $2, $3)`,
		a, own, auth.HashRecoveryCode(recoverySecret, own, "AAAAA-AAAAA"))

	other := f.setupSession(t, "a", staffPW) // password bodlale revoke hobe
	wantErr(t, f.cookieReq("GET", "/api/admin/me", "", nil), 401, "unauthenticated", "cookie chhara /me")
	tok := f.setupSession(t, "a", staffPW)
	if stage, secs := f.session(t, tok); stage != "setup_pending" || !near(secs, 900) {
		t.Errorf("setup session: %s %.0f", stage, secs)
	}

	wantErr(t, f.cookieReq("GET", "/api/admin/me", tok, nil), 403, "setup_required", "setup e /me")
	wantErr(t, f.cookieReq("POST", "/api/admin/auth/totp/setup", tok, nil), 403, "password_change_required", "password er age TOTP")
	wantErr(t, f.cookieReq("POST", "/api/admin/auth/totp/enable", tok, map[string]string{"code": "123456"}), 403,
		"password_change_required", "password er age enable")

	newPW := strings.Repeat("ক", 11) + "x" // 12 okkhor (36 byte): byte na, okkhor gone
	for name, c := range map[string]struct{ old, new, field string }{
		"bhul old":   {"bhul-password-xx", newPW, "old"},
		"11 okkhor":  {staffPW, strings.Repeat("ক", 11), "new"},
		"129 okkhor": {staffPW, strings.Repeat("a", 129), "new"},
		"purono":     {staffPW, staffPW, "new"},
	} {
		rec := f.cookieReq("POST", "/api/admin/auth/password", tok, map[string]string{"old": c.old, "new": c.new})
		wantErr(t, rec, 400, "validation_failed", name)
		if !strings.Contains(rec.Body.String(), `"`+c.field+`"`) {
			t.Errorf("%s: fields e %s nai: %s", name, c.field, rec.Body.String())
		}
	}
	if rec := f.cookieReq("POST", "/api/admin/auth/password", tok, map[string]string{"old": staffPW, "new": newPW}); rec.Code != http.StatusNoContent {
		t.Fatalf("password: %d %s", rec.Code, rec.Body.String())
	}
	var hash string
	var must bool
	if err := f.d.QueryRow(`SELECT password_hash, must_change_password FROM staff WHERE id = $1 AND password_changed_at IS NOT NULL`,
		own).Scan(&hash, &must); err != nil || must || !auth.CheckPassword(hash, newPW) {
		t.Errorf("password er por: must %v, err %v", must, err)
	}
	if n := f.count(t, `SELECT count(*) FROM sessions WHERE token_hash = $1 AND revoked_at IS NOT NULL`, hashOf(other)); n != 1 {
		t.Error("password bodlale onno session revoke hoyni")
	}
	if stage, _ := f.session(t, tok); stage != "setup_pending" {
		t.Errorf("password er por ei session %s", stage)
	}
	wantErr(t, f.cookieReq("GET", "/api/admin/me", other, nil), 401, "unauthenticated", "revoke kora session")
	if n := f.audits(t, "a", "staff.password_changed"); n != 1 {
		t.Errorf("audit staff.password_changed %d", n)
	}

	setup := func(what string) (string, []byte) {
		t.Helper()
		rec := f.cookieReq("POST", "/api/admin/auth/totp/setup", tok, nil)
		var out struct {
			URL    string `json:"otpauth_url"`
			Secret string `json:"secret"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
			t.Fatalf("%s: %d %s", what, rec.Code, rec.Body.String())
		}
		raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(out.Secret)
		u, uerr := url.Parse(out.URL)
		if err != nil || len(raw) != 20 || uerr != nil || u.Scheme != "otpauth" || u.Host != "totp" ||
			u.Query().Get("secret") != out.Secret || u.Query().Get("issuer") == "" || !strings.Contains(u.Path, "o@a.com") {
			t.Errorf("%s: %+v", what, out)
		}
		return out.Secret, raw
	}
	_, first := setup("setup")
	_, raw := setup("abar setup") // enable er age replace
	var ct, nonce []byte          // v1.25: setup er secret pending e, live na
	if err := f.d.QueryRow(`SELECT totp_pending_enc, totp_pending_nonce FROM staff WHERE id = $1 AND totp_pending_key_version = 1
		AND totp_secret_enc IS NULL`, own).Scan(&ct, &nonce); err != nil {
		t.Fatal(err)
	}
	if got, err := auth.OpenTOTPSecret(totpKey, a, own, 1, ct, nonce); err != nil || !bytes.Equal(got, raw) || bytes.Equal(raw, first) {
		t.Errorf("DB te shesh secret na: %v", err)
	}

	step := time.Now().Unix() / 30
	enable := func(code string) *httptest.ResponseRecorder {
		return f.cookieReq("POST", "/api/admin/auth/totp/enable", tok, map[string]string{"code": code})
	}
	wantErr(t, enable("000000"), 400, "mfa_invalid", "bhul code")
	wantErr(t, enable(auth.TOTPCode(first, step)), 400, "mfa_invalid", "purono secret er code")
	rec := enable(auth.TOTPCode(raw, step))
	var out struct {
		Codes []string `json:"recovery_codes"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || len(out.Codes) != 10 {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body.String())
	}
	for _, c := range out.Codes {
		if !recoveryFormat.MatchString(c) {
			t.Errorf("recovery code %q", c)
		}
	}
	active := adminCookie(t, rec, 43200, "enable")
	if stage, secs := f.session(t, active); active == tok || stage != "active" || !near(secs, 43200) {
		t.Errorf("enable er por: %s %.0f (token notun %v)", stage, secs, active != tok)
	}
	if n := f.count(t, `SELECT count(*) FROM staff_recovery_codes WHERE staff_id = $1 AND used_at IS NULL`, own); n != 10 {
		t.Errorf("recovery code %d, chai 10 (purono muche)", n)
	}
	if n := f.count(t, `SELECT count(*) FROM staff_recovery_codes WHERE staff_id = $1 AND code_hash = $2`,
		own, auth.HashRecoveryCode(recoverySecret, own, out.Codes[0])); n != 1 {
		t.Error("recovery code er hash DB te nai")
	}
	if n := f.audits(t, "a", "staff.totp_enabled"); n != 1 {
		t.Errorf("audit staff.totp_enabled %d", n)
	}
	wantErr(t, f.cookieReq("GET", "/api/admin/me", tok, nil), 401, "unauthenticated", "setup er purono token")

	rec = f.cookieReq("GET", "/api/admin/me", active, nil)
	var me map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &me) != nil || me["email"] != "o@a.com" ||
		me["role"] != "owner" || me["vendor_id"] != nil || me["name"] != "O" {
		t.Errorf("/me: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, f.cookieReq("POST", "/api/admin/auth/totp/setup", active, nil), 403, "forbidden", "active e TOTP setup")
	wantErr(t, f.cookieReq("POST", "/api/admin/auth/totp/enable", active, map[string]string{"code": "123456"}), 403,
		"forbidden", "active e enable")
	if rec := f.login("a", "o@a.com", newPW); !strings.Contains(rec.Body.String(), `"totp_required":true`) {
		t.Errorf("enable er por login e TOTP chay na: %d %s", rec.Code, rec.Body.String())
	}

	rec = f.cookieReq("POST", "/api/admin/auth/logout", active, nil)
	if rec.Code != http.StatusNoContent || !strings.Contains(rec.Header().Get("Set-Cookie"), "__Host-asid=; Path=/; Max-Age=0") {
		t.Errorf("logout: %d %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}
	wantErr(t, f.cookieReq("GET", "/api/admin/me", active, nil), 401, "unauthenticated", "logout er por")

	// shop b: must_change na, TOTP nai (jemon reset-2fa er por) → setup e sora-sori TOTP
	b, ownB := f.shops["b.com"], f.owner["b"]
	f.exec(t, `UPDATE staff SET totp_secret_enc = NULL, totp_nonce = NULL, totp_key_version = NULL WHERE id = $1 AND shop_id = $2`, ownB, b)
	tokB, expired := f.setupSession(t, "b", staffPW), f.setupSession(t, "b", staffPW)
	f.exec(t, `UPDATE sessions SET expires_at = now() - interval '1 second' WHERE token_hash = $1`, hashOf(expired))
	wantErr(t, f.cookieReq("POST", "/api/admin/auth/totp/setup", expired, nil), 401, "session_expired", "expire setup session")
	if rec := f.cookieReq("POST", "/api/admin/auth/totp/setup", tokB, nil); rec.Code != http.StatusOK {
		t.Errorf("must_change chhara setup: %d %s", rec.Code, rec.Body.String())
	}
	f.exec(t, `UPDATE staff SET is_active = false WHERE id = $1 AND shop_id = $2`, ownB, b)
	wantErr(t, f.cookieReq("POST", "/api/admin/auth/logout", tokB, nil), 401, "unauthenticated", "inactive staff")
}

// totpState: staff er live ar pending TOTP secret ache kina.
func (f *staffFixture) totpState(t *testing.T, staffID int64) (live, pending bool) {
	t.Helper()
	if err := f.d.QueryRow(`SELECT totp_secret_enc IS NOT NULL, totp_pending_enc IS NOT NULL FROM staff WHERE id = $1`,
		staffID).Scan(&live, &pending); err != nil {
		t.Fatal(err)
	}
	return live, pending
}

// setupSecret: /auth/totp/setup er secret (raw).
func (f *staffFixture) setupSecret(t *testing.T, tok string) []byte {
	t.Helper()
	rec := f.cookieReq("POST", "/api/admin/auth/totp/setup", tok, nil)
	var out struct {
		Secret string `json:"secret"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(out.Secret)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestTOTPPendingIntegration (8.3 step 4, 6 v1.25, 7.7, 16.1): setup er secret pending e; enable na korle porer
// login TOTP chay na (abar setup_required); enable e bhul code e pending thake, live na; thik code e pending →
// live, pending NULL, login e TOTP chay ar oi secret e code chole. Live TOTP thakle onno setup_pending session e
// setup 403. TOTP live ar must_change_password: TOTP er por setup_pending, password bodlale oi session active
// (notun token, 43200), setup lage na; active e setup 403.
func TestTOTPPendingIntegration(t *testing.T) {
	f := newStaffFixture(t)
	a, own := f.shops["a.com"], f.owner["a"]
	f.exec(t, `UPDATE staff SET totp_secret_enc = NULL, totp_nonce = NULL, totp_key_version = NULL WHERE id = $1 AND shop_id = $2`, own, a)

	f.setupSecret(t, f.setupSession(t, "a", staffPW)) // setup, enable na
	if live, pending := f.totpState(t, own); live || !pending {
		t.Errorf("setup er por live %v pending %v; chai false, true", live, pending)
	}
	tok := f.setupSession(t, "a", staffPW) // abar login e TOTP chay na
	other := f.setupSession(t, "a", staffPW)
	raw := f.setupSecret(t, tok)
	step := time.Now().Unix() / 30
	enable := func(code string) *httptest.ResponseRecorder {
		return f.cookieReq("POST", "/api/admin/auth/totp/enable", tok, map[string]string{"code": code})
	}
	wantErr(t, enable(auth.TOTPCode(raw, step+5)), 400, "mfa_invalid", "bhul code")
	if live, pending := f.totpState(t, own); live || !pending {
		t.Errorf("bhul code er por live %v pending %v; chai false, true", live, pending)
	}
	if rec := enable(auth.TOTPCode(raw, step)); rec.Code != http.StatusOK {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body.String())
	}
	if live, pending := f.totpState(t, own); !live || pending {
		t.Errorf("enable er por live %v pending %v; chai true, false", live, pending)
	}
	if n := f.count(t, `SELECT count(*) FROM staff WHERE id = $1 AND totp_key_version = 1 AND totp_pending_nonce IS NULL
		AND totp_pending_key_version IS NULL AND totp_last_step = $2`, own, step); n != 1 {
		t.Error("enable er por key_version/last_step/pending thik na")
	}
	wantErr(t, f.cookieReq("POST", "/api/admin/auth/totp/setup", other, nil), 403, "forbidden", "live TOTP e onno session e setup")
	f.secret[own] = raw
	rec := f.totp(f.tempToken(t, "a"), f.code(own, 1))
	adminCookie(t, rec, 43200, "live secret e TOTP")

	// shop b: TOTP live, must_change_password → TOTP er por setup_pending, password bodlale active
	b, ownB := f.shops["b.com"], f.owner["b"]
	f.exec(t, `UPDATE staff SET must_change_password = true WHERE id = $1 AND shop_id = $2`, ownB, b)
	setup := adminCookie(t, f.totp(f.tempToken(t, "b"), f.code(ownB, 0)), 900, "must_change e TOTP")
	wantErr(t, f.cookieReq("POST", "/api/admin/auth/totp/setup", setup, nil), 403, "password_change_required", "password er age setup")
	rec = f.cookieReq("POST", "/api/admin/auth/password", setup, map[string]string{"old": staffPW, "new": "notun-password-123"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("password: %d %s", rec.Code, rec.Body.String())
	}
	active := adminCookie(t, rec, 43200, "TOTP live e password")
	if stage, secs := f.session(t, active); active == setup || stage != "active" || !near(secs, 43200) {
		t.Errorf("password er por: %s %.0f (token notun %v)", stage, secs, active != setup)
	}
	wantErr(t, f.cookieReq("GET", "/api/admin/me", setup, nil), 401, "unauthenticated", "setup er purono token")
	if rec := f.cookieReq("GET", "/api/admin/me", active, nil); rec.Code != http.StatusOK {
		t.Errorf("/me: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, f.cookieReq("POST", "/api/admin/auth/totp/setup", active, nil), 403, "forbidden", "active e setup")
	if live, pending := f.totpState(t, ownB); !live || pending {
		t.Errorf("shop b live %v pending %v", live, pending)
	}
}
