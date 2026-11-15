package auth_test

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/httpx"
)

const (
	staffPW        = "correct-horse-battery"
	recoverySecret = "test-recovery-secret-0123456789ab"
)

var totpKey = bytes.Repeat([]byte{7}, 32)

type staffFixture struct {
	*otpFixture
	admin  http.Handler
	owner  map[string]int64 // "a", "b" → owner staff id
	secret map[int64][]byte // staff id → TOTP secret
	ipSeq  atomic.Int64
}

// newStaffFixture: duita shop (a, b), owner er password staffPW (test e 1000 iteration, druto), TOTP ache,
// must_change_password false. Admin login route ClientIP er pore (RemoteAddr = client IP).
func newStaffFixture(t *testing.T) *staffFixture {
	t.Helper()
	f := &staffFixture{otpFixture: newOTPFixture(t), owner: map[string]int64{}, secret: map[int64][]byte{}}
	for _, s := range []string{"a", "b"} {
		shopID := f.shops[s+".com"]
		var id int64
		if err := f.d.QueryRow(`SELECT id FROM staff WHERE shop_id = $1 AND role = 'owner'`, shopID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		f.owner[s] = id
		f.exec(t, `UPDATE staff SET password_hash = $3, must_change_password = false WHERE id = $1 AND shop_id = $2`,
			id, shopID, fastHash(t, staffPW))
		f.setTOTP(t, shopID, id)
	}
	h := auth.NewAdminHandlers(f.d, slog.New(slog.DiscardHandler), totpKey, recoverySecret)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/admin/auth/login", h.Login)
	mux.HandleFunc("POST /api/admin/auth/totp", h.TOTP)
	mux.HandleFunc("POST /api/admin/auth/recovery", h.Recovery)
	// setup ar admin session (8.3 step 4, 11.3): __Host-asid cookie theke staff
	sess := httpx.AdminSession(slog.New(slog.DiscardHandler), auth.LookupStaffSession(f.d))
	mux.Handle("POST /api/admin/auth/password", sess(http.HandlerFunc(h.Password)))
	mux.Handle("POST /api/admin/auth/totp/setup", sess(http.HandlerFunc(h.TOTPSetup)))
	mux.Handle("POST /api/admin/auth/totp/enable", sess(http.HandlerFunc(h.TOTPEnable)))
	mux.Handle("POST /api/admin/auth/logout", sess(http.HandlerFunc(h.Logout)))
	mux.Handle("GET /api/admin/me", sess(httpx.RequireStaff(http.HandlerFunc(h.Me))))
	f.admin = httpx.ClientIP("")(httpx.JSONErrors(mux))
	return f
}

// fastHash: HashPassword er format, kintu 1000 iteration (600000 e test dhire).
func fastHash(t *testing.T, pw string) string {
	salt := []byte("0123456789abcdef")
	k, err := pbkdf2.Key(sha256.New, pw, salt, 1000, 32)
	if err != nil {
		t.Fatal(err)
	}
	return "pbkdf2-sha256$1000$" + base64.StdEncoding.EncodeToString(salt) + "$" + base64.StdEncoding.EncodeToString(k)
}

func (f *staffFixture) setTOTP(t *testing.T, shopID, staffID int64) {
	t.Helper()
	raw, _ := auth.NewTOTPSecret()
	ct, nonce, err := auth.SealTOTPSecret(totpKey, shopID, staffID, 1, raw)
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `UPDATE staff SET totp_secret_enc = $3, totp_nonce = $4, totp_key_version = 1 WHERE id = $1 AND shop_id = $2`,
		staffID, shopID, ct, nonce)
	f.secret[staffID] = raw
}

// code: ei staff er ekhon er TOTP code (offset step shoho).
func (f *staffFixture) code(staffID int64, offset int64) string {
	return auth.TOTPCode(f.secret[staffID], time.Now().Unix()/30+offset)
}

// adminPost: ip theke JSON POST.
func (f *staffFixture) adminPost(path, ip string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	r.RemoteAddr = ip + ":1234"
	r.Header.Set("User-Agent", "admin-agent")
	rec := httptest.NewRecorder()
	f.admin.ServeHTTP(rec, r)
	return rec
}

// login: protibar notun client IP, jate IP er limit (TestLoginIPLimit) account er test e na lage.
func (f *staffFixture) login(shop, email, pw string) *httptest.ResponseRecorder {
	n := f.ipSeq.Add(1)
	ip := fmt.Sprintf("10.0.%d.%d", n/256, n%256)
	return f.adminPost("/api/admin/auth/login", ip, map[string]string{"shop": shop, "email": email, "password": pw})
}

// tempToken: thik password e {totp_required: true, temp_token}.
func (f *staffFixture) tempToken(t *testing.T, shop string) string {
	t.Helper()
	rec := f.login(shop, "o@"+shop+".com", staffPW)
	var out struct {
		TOTPRequired bool   `json:"totp_required"`
		TempToken    string `json:"temp_token"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || !out.TOTPRequired || out.TempToken == "" {
		t.Fatalf("login %s: %d %s", shop, rec.Code, rec.Body.String())
	}
	return out.TempToken
}

// mfaToken: DB te sora-sori mfa_pending session (5 minute): duita parallel login er moto, jekhane notun login
// er purono temp_token revoke (8.3 step 2 v1.24) er age duitai password par hoyeche.
func (f *staffFixture) mfaToken(t *testing.T, shop string) string {
	t.Helper()
	tok := rand.Text()
	f.exec(t, `INSERT INTO sessions (shop_id, token_hash, subject_type, staff_id, stage, expires_at)
		VALUES ($1, $2, 'staff', $3, 'mfa_pending', now() + interval '5 minutes')`, f.shops[shop+".com"], hashOf(tok), f.owner[shop])
	return tok
}

func (f *staffFixture) totp(temp, code string) *httptest.ResponseRecorder {
	return f.adminPost("/api/admin/auth/totp", "192.0.2.1", map[string]string{"temp_token": temp, "code": code})
}

func (f *staffFixture) recovery(temp, code string) *httptest.ResponseRecorder {
	return f.adminPost("/api/admin/auth/recovery", "192.0.2.1", map[string]string{"temp_token": temp, "code": code})
}

// adminCookie (11.3 v1.19): __Host-asid HttpOnly, Secure, SameSite=Strict, Path=/, Domain na, Max-Age maxAge;
// value ferot.
func adminCookie(t *testing.T, rec *httptest.ResponseRecorder, maxAge int, what string) string {
	t.Helper()
	cs := rec.Result().Cookies()
	if len(cs) != 1 {
		t.Fatalf("%s: cookie %d ta (%d %s)", what, len(cs), rec.Code, rec.Body.String())
	}
	c := cs[0]
	if c.Name != "__Host-asid" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" ||
		c.Domain != "" || c.MaxAge != maxAge || c.Value == "" {
		t.Errorf("%s: cookie %q", what, rec.Header().Get("Set-Cookie"))
	}
	return c.Value
}

func noCookie(t *testing.T, rec *httptest.ResponseRecorder, what string) {
	t.Helper()
	if h := rec.Header().Get("Set-Cookie"); h != "" {
		t.Errorf("%s: cookie dewa uchit na: %q", what, h)
	}
}

func hashOf(tok string) []byte {
	h := sha256.Sum256([]byte(tok))
	return h[:]
}

// session: token er session er stage ar expire hote koto second.
func (f *staffFixture) session(t *testing.T, tok string) (string, float64) {
	t.Helper()
	var stage string
	var secs float64
	if err := f.d.QueryRow(`SELECT stage, extract(epoch FROM expires_at - now()) FROM sessions
		WHERE token_hash = $1 AND subject_type = 'staff'`, hashOf(tok)).Scan(&stage, &secs); err != nil {
		t.Fatalf("session: %v", err)
	}
	return stage, secs
}

func near(got, want float64) bool { return got > want-10 && got <= want+1 }

func (f *staffFixture) audits(t *testing.T, shop, action string) int {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND action = $2`, f.shops[shop+".com"], action)
}

// TestStaffLoginIntegration (8.3 step 1-4, 7.7, 7.8, Q7, Q9): thik password e TOTP thakle mfa_pending temp_token
// (body te, 5 minute, cookie na), na hole setup_pending + __Host-asid (900); ochena shop/email, bhul password ar
// inactive ekoi 401 invalid_credentials; failed_logins, 5 er por protiti bhul e 1 minute lock (v1.24: lock e o 401,
// Retry-After na), thik hole 0;
// 10 ar 50 e alert.security; shob login audit_logs e; closed/suspended shop e login chole; shop A er staff
// shop B er slug e na.
func TestStaffLoginIntegration(t *testing.T) {
	f := newStaffFixture(t)
	a, own := f.shops["a.com"], f.owner["a"]

	temp := f.tempToken(t, "a")
	if stage, secs := f.session(t, temp); stage != "mfa_pending" || !near(secs, 300) {
		t.Errorf("temp_token session: %s, %.0f second", stage, secs)
	}
	noCookie(t, f.login("a", "o@a.com", staffPW), "mfa_pending")
	if rec := f.login("a", "O@A.COM", staffPW); rec.Code != http.StatusOK {
		t.Errorf("email boro haat e: %d %s", rec.Code, rec.Body.String())
	}
	if n := f.audits(t, "a", "staff.login"); n != 3 {
		t.Errorf("thik login er audit %d, chai 3", n)
	}

	for name, c := range map[string][3]string{
		"bhul password":        {"a", "o@a.com", "wrong-password-1"},
		"ochena email":         {"a", "x@a.com", staffPW},
		"ochena shop":          {"zz", "o@a.com", staffPW},
		"onno shop er slug":    {"b", "o@a.com", staffPW},
		"onno shop er owner":   {"a", "o@b.com", staffPW},
		"khali password":       {"a", "o@a.com", ""},
		"password e bhul haat": {"a", "o@a.com", strings.ToUpper(staffPW)},
	} {
		rec := f.login(c[0], c[1], c[2])
		wantErr(t, rec, 401, "invalid_credentials", name)
		noCookie(t, rec, name)
	}
	if n := f.count(t, `SELECT failed_logins FROM staff WHERE id = $1`, own); n != 3 {
		t.Errorf("failed_logins %d, chai 3 (bhul password, khali, bhul haat)", n)
	}
	if n := f.count(t, `SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND action = 'staff.login_failed' AND staff_id IS NULL`, a); n != 2 {
		t.Errorf("ochena email er audit (staff_id NULL) %d, chai 2", n)
	}
	if n := f.audits(t, "a", "staff.login_failed"); n != 5 {
		t.Errorf("bhul login er audit %d, chai 5", n)
	}

	// 5 tomo bhul e lock; lock e thik password o 401, Retry-After na (8.3 step 1 v1.24: lock dekhe email chena na)
	f.login("a", "o@a.com", "wrong-password-1")
	wantErr(t, f.login("a", "o@a.com", "wrong-password-1"), 401, "invalid_credentials", "5 tomo bhul")
	rec := f.login("a", "o@a.com", staffPW)
	wantErr(t, rec, 401, "invalid_credentials", "lock e thik password")
	if h := rec.Header().Get("Retry-After"); h != "" {
		t.Errorf("lock e Retry-After %q", h)
	}
	if n := f.count(t, `SELECT failed_logins FROM staff WHERE id = $1`, own); n != 5 {
		t.Errorf("lock e attempt gona hoyeche: failed_logins %d", n)
	}
	f.exec(t, `UPDATE staff SET locked_until = now() - interval '1 second' WHERE id = $1`, own)
	f.tempToken(t, "a")
	if n := f.count(t, `SELECT failed_logins FROM staff WHERE id = $1`, own); n != 0 {
		t.Errorf("thik login e failed_logins 0 hoyni: %d", n)
	}
	f.exec(t, `UPDATE staff SET failed_logins = 4 WHERE id = $1`, own)
	wantErr(t, f.login("a", "o@a.com", "nope-nope-nope"), 401, "invalid_credentials", "5 tomo bhul (abar)")
	wantErr(t, f.login("a", "o@a.com", staffPW), 401, "invalid_credentials", "5 er por lock")

	// 10 ar 50 e alert, 11 e na
	for _, c := range []struct{ before, alerts int }{{8, 0}, {9, 1}, {10, 1}, {49, 2}, {50, 2}} {
		f.exec(t, `UPDATE staff SET failed_logins = $2, locked_until = NULL WHERE id = $1`, own, c.before)
		f.login("a", "o@a.com", "wrong-password-1")
		if n := f.count(t, `SELECT count(*) FROM jobs WHERE kind = 'alert.security' AND shop_id = $1
			AND payload->>'event' = 'staff_login_failed' AND (payload->>'staff_id')::bigint = $2`, a, own); n != c.alerts {
			t.Errorf("failed_logins %d → %d: alert %d, chai %d", c.before, c.before+1, n, c.alerts)
		}
	}
	f.exec(t, `UPDATE staff SET failed_logins = 0, locked_until = NULL WHERE id = $1`, own)

	f.exec(t, `UPDATE staff SET is_active = false WHERE id = $1`, own)
	wantErr(t, f.login("a", "o@a.com", staffPW), 401, "invalid_credentials", "inactive staff")
	f.exec(t, `UPDATE staff SET is_active = true WHERE id = $1`, own)

	// setup_pending: must_change_password ba TOTP nai
	f.exec(t, `UPDATE staff SET must_change_password = true, totp_secret_enc = NULL, totp_nonce = NULL, totp_key_version = NULL WHERE id = $1`, own)
	rec = f.login("a", "o@a.com", staffPW)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"setup_required":true}` {
		t.Fatalf("setup login: %d %s", rec.Code, rec.Body.String())
	}
	tok := adminCookie(t, rec, 900, "setup_pending")
	if stage, secs := f.session(t, tok); stage != "setup_pending" || !near(secs, 900) {
		t.Errorf("setup session: %s, %.0f second", stage, secs)
	}
	var ip, ua string
	if err := f.d.QueryRow(`SELECT ip, user_agent FROM sessions WHERE token_hash = $1`, hashOf(tok)).Scan(&ip, &ua); err != nil ||
		!strings.HasPrefix(ip, "10.0.") || ua != "admin-agent" {
		t.Errorf("session ip %q ua %q (%v)", ip, ua, err)
	}

	// closed ar suspended shop e login chole (3.2 Niyom 2, Q7)
	for _, st := range []string{"closed", "suspended"} {
		f.exec(t, `UPDATE shops SET status = $2 WHERE id = $1`, f.shops["b.com"], st)
		f.tempToken(t, "b")
	}

	wantErr(t, f.adminPost("/api/admin/auth/login", "192.0.2.1", map[string]any{"shop": "a", "email": "o@a.com", "password": staffPW, "x": 1}),
		400, "validation_failed", "ochena field")
}

// TestLoginIPLimit (7.8): ek client IP te 15 minute e 10 bhul login er por oi IP theke thik password o 429;
// shudhu bhul gone (thik login gone na); onno IP te chole.
func TestLoginIPLimit(t *testing.T) {
	f := newStaffFixture(t)
	post := func(ip, email, pw string) *httptest.ResponseRecorder {
		return f.adminPost("/api/admin/auth/login", ip, map[string]string{"shop": "a", "email": email, "password": pw})
	}
	// ochena email: account lock na, shudhu IP er gona
	for i := range 9 {
		wantErr(t, post("198.51.100.7", fmt.Sprintf("x%d@a.com", i), staffPW), 401, "invalid_credentials", "bhul "+strconv.Itoa(i+1))
	}
	for range 3 {
		if rec := post("198.51.100.7", "o@a.com", staffPW); rec.Code != http.StatusOK {
			t.Fatalf("9 bhul er por thik login: %d %s", rec.Code, rec.Body.String())
		}
	}
	wantErr(t, post("198.51.100.7", "x9@a.com", staffPW), 401, "invalid_credentials", "10 tomo bhul")
	if s := limited(t, post("198.51.100.7", "o@a.com", staffPW), "10 bhul er por thik password"); s > 900 {
		t.Errorf("Retry-After %d > 900", s)
	}
	if rec := post("198.51.100.8", "o@a.com", staffPW); rec.Code != http.StatusOK {
		t.Errorf("onno IP: %d %s", rec.Code, rec.Body.String())
	}
}

// TestMFAAttemptsIntegration (8.3 step 2, 7.8): ek temp_token e 5 bar (conditional, parallel e o), tarpor thik
// code o mfa_too_many_attempts; temp_token 5 minute (session_expired); bhul/customer/active token 401; TOTP
// thik hole oi row-i active 12 ghonta + __Host-asid 43200, token notun (temp_token cookie hoy na),
// failed_mfa 0, audit; must_change_password thakle TOTP er por setup_pending (900).
func TestMFAAttemptsIntegration(t *testing.T) {
	f := newStaffFixture(t)
	own := f.owner["a"]
	temp := f.tempToken(t, "a")
	for i := range 5 {
		wantErr(t, f.totp(temp, "000000"), 400, "mfa_invalid", "bhul "+strconv.Itoa(i+1))
	}
	wantErr(t, f.totp(temp, f.code(own, 0)), 400, "mfa_too_many_attempts", "5 bhul er por thik code")
	wantErr(t, f.recovery(temp, "AAAAA-AAAAA"), 400, "mfa_too_many_attempts", "5 bhul er por recovery")
	if n := f.count(t, `SELECT mfa_attempts FROM sessions WHERE token_hash = $1`, hashOf(temp)); n != 5 {
		t.Errorf("mfa_attempts %d, chai 5", n)
	}

	f.exec(t, `UPDATE staff SET failed_mfa = 0 WHERE id = $1`, own)
	temp = f.tempToken(t, "a")
	got := parallel(10, func() *httptest.ResponseRecorder { return f.totp(temp, "000000") })
	if got["400 mfa_invalid"] != 5 || got["400 mfa_too_many_attempts"] != 5 {
		t.Errorf("parallel 10 bhul: %v", got)
	}
	if n := f.count(t, `SELECT failed_mfa FROM staff WHERE id = $1`, own); n != 5 {
		t.Errorf("failed_mfa %d, chai 5", n)
	}

	temp = f.tempToken(t, "a")
	f.exec(t, `UPDATE sessions SET expires_at = now() - interval '1 second' WHERE token_hash = $1`, hashOf(temp))
	wantErr(t, f.totp(temp, f.code(own, 0)), 401, "session_expired", "expire temp_token")
	wantErr(t, f.totp("no-such-token", f.code(own, 0)), 401, "unauthenticated", "bhul token")
	cust, _ := f.otpFixture.login(t, "a.com", "+8801711000001", "bn")
	wantErr(t, f.totp(cust, f.code(own, 0)), 401, "unauthenticated", "customer token")
	wantErr(t, f.adminPost("/api/admin/auth/totp", "192.0.2.1", map[string]any{"temp_token": f.tempToken(t, "a")}), 400, "mfa_invalid", "code nai")

	temp = f.tempToken(t, "a")
	f.totp(temp, "000000")
	rec := f.totp(temp, f.code(own, 0))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"setup_required":false}` {
		t.Fatalf("thik TOTP: %d %s", rec.Code, rec.Body.String())
	}
	tok := adminCookie(t, rec, 43200, "active")
	if tok == temp {
		t.Error("temp_token-i cookie hoyeche (JS temp_token dekheche)")
	}
	if stage, secs := f.session(t, tok); stage != "active" || !near(secs, 43200) {
		t.Errorf("active session: %s, %.0f second", stage, secs)
	}
	if n := f.count(t, `SELECT count(*) FROM sessions WHERE token_hash = $1`, hashOf(temp)); n != 0 {
		t.Error("temp_token er hash ekhono DB te")
	}
	wantErr(t, f.totp(temp, f.code(own, 1)), 401, "unauthenticated", "active hoyar por temp_token")
	if n := f.count(t, `SELECT count(*) FROM staff WHERE id = $1 AND failed_mfa = 0 AND last_login_at > now() - interval '1 minute'`, own); n != 1 {
		t.Error("thik TOTP e failed_mfa 0 ba last_login_at hoyni")
	}
	if n := f.audits(t, "a", "staff.login_mfa"); n != 1 {
		t.Errorf("staff.login_mfa audit %d, chai 1", n)
	}
	if n := f.audits(t, "a", "staff.login_mfa_failed"); n < 11 {
		t.Errorf("staff.login_mfa_failed audit %d", n)
	}

	// TOTP ache kintu must_change_password: TOTP er por setup_pending (8.3 step 4)
	f.exec(t, `UPDATE staff SET must_change_password = true WHERE id = $1`, own)
	temp = f.tempToken(t, "a")
	rec = f.totp(temp, f.code(own, 1))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"setup_required":true}` {
		t.Fatalf("must_change TOTP: %d %s", rec.Code, rec.Body.String())
	}
	if stage, secs := f.session(t, adminCookie(t, rec, 900, "setup_pending")); stage != "setup_pending" || !near(secs, 900) {
		t.Errorf("setup session: %s, %.0f", stage, secs)
	}
}

// TestTOTPReplayIntegration (8.3 step 2 v1.5, 14.2 test 10): ekoi code (ba ager step er code) duibar na,
// parallel e duita temp_token e ekoi code → ekta session; onno staff er secret er code na.
func TestTOTPReplayIntegration(t *testing.T) {
	f := newStaffFixture(t)
	own := f.owner["a"]
	code := f.code(own, 0)
	if rec := f.totp(f.tempToken(t, "a"), code); rec.Code != http.StatusOK {
		t.Fatalf("prothom TOTP: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, f.totp(f.tempToken(t, "a"), code), 400, "mfa_invalid", "ekoi code abar")
	wantErr(t, f.totp(f.tempToken(t, "a"), f.code(own, -1)), 400, "mfa_invalid", "ager step er code")
	wantErr(t, f.totp(f.tempToken(t, "a"), f.code(f.owner["b"], 1)), 400, "mfa_invalid", "onno staff er code")

	t1, t2, t3 := f.mfaToken(t, "a"), f.mfaToken(t, "a"), f.mfaToken(t, "a")
	next := f.code(own, 1)
	toks := make(chan string, 3)
	toks <- t1
	toks <- t2
	toks <- t3
	got := parallel(3, func() *httptest.ResponseRecorder { return f.totp(<-toks, next) })
	if got["200 "] != 1 || got["400 mfa_invalid"] != 2 {
		t.Errorf("parallel ekoi code: %v, chai ekta 200", got)
	}
	if n := f.count(t, `SELECT count(*) FROM sessions WHERE staff_id = $1 AND stage = 'active'`, own); n != 2 {
		t.Errorf("active session %d, chai 2", n)
	}
	if n := f.count(t, `SELECT (totp_last_step = $2)::int FROM staff WHERE id = $1`, own, time.Now().Unix()/30+1); n != 1 {
		t.Error("totp_last_step notun step na")
	}
}

// TestFailedMFAAcrossTokensIntegration (8.3 step 2 v1.3, v1.24, 7.8, 14.2 test 10): temp_token bodle-o
// staff.failed_mfa jure 10 bhul e 30 minute lock (429) ar alert ekbar (dedupe_key din shoho); notun login e purono
// temp_token revoke; lock e thik code na, login 401; lock khulle thik code e 0.
func TestFailedMFAAcrossTokensIntegration(t *testing.T) {
	f := newStaffFixture(t)
	a, own := f.shops["a.com"], f.owner["a"]
	t1 := f.tempToken(t, "a")
	for i := range 5 {
		wantErr(t, f.totp(t1, "000000"), 400, "mfa_invalid", "t1 bhul "+strconv.Itoa(i+1))
	}
	t2 := f.tempToken(t, "a")
	wantErr(t, f.totp(t1, f.code(own, 0)), 401, "unauthenticated", "notun login er por purono temp_token")
	for i := range 3 {
		wantErr(t, f.recovery(t2, "AAAAA-AAAAA"), 400, "mfa_invalid", "t2 recovery bhul "+strconv.Itoa(i+1))
	}
	wantErr(t, f.totp(t2, "000000"), 400, "mfa_invalid", "9 tomo bhul")
	t3 := f.tempToken(t, "a")
	if s := limited(t, f.totp(t3, "000000"), "10 tomo bhul"); s < 1790 || s > 1800 {
		t.Errorf("failed_mfa lock Retry-After %d, chai ~1800", s)
	}
	limited(t, f.totp(t3, f.code(own, 0)), "lock e thik code")
	wantErr(t, f.login("a", "o@a.com", staffPW), 401, "invalid_credentials", "lock e login")
	var key string
	if err := f.d.QueryRow(`SELECT dedupe_key FROM jobs WHERE kind = 'alert.security' AND shop_id = $1
		AND payload->>'event' = 'staff_mfa_lock' AND (payload->>'staff_id')::bigint = $2`, a, own).Scan(&key); err != nil {
		t.Fatalf("mfa lock alert ekta chai: %v", err)
	}
	day := time.Now().In(mustLoc(t, "Asia/Dhaka")).Format(time.DateOnly)
	if want := fmt.Sprintf("alert.staff_mfa_lock:%d:%d:%s", a, own, day); key != want {
		t.Errorf("dedupe_key %q, chai %q", key, want)
	}
	if n := f.count(t, `SELECT count(*) FROM sessions WHERE staff_id = $1 AND stage = 'active'`, own); n != 0 {
		t.Errorf("lock e session active: %d", n)
	}

	f.exec(t, `UPDATE staff SET locked_until = now() - interval '1 second' WHERE id = $1`, own)
	if rec := f.totp(t3, f.code(own, 0)); rec.Code != http.StatusOK {
		t.Fatalf("lock khulle thik code: %d %s", rec.Code, rec.Body.String())
	}
	if n := f.count(t, `SELECT count(*) FROM staff WHERE id = $1 AND failed_mfa = 0 AND locked_until IS NULL`, own); n != 1 {
		t.Error("thik code e failed_mfa 0 ar lock khole nai")
	}
}

// TestRecoveryCodeIntegration (8.3 step 3, 5, 6): recovery code ekbar-i (parallel e o), `-`/space/case bad,
// onno staff er code na, bhul e failed_mfa; thik hole active + cookie.
func TestRecoveryCodeIntegration(t *testing.T) {
	f := newStaffFixture(t)
	a, own := f.shops["a.com"], f.owner["a"]
	codes := auth.NewRecoveryCodes()
	for _, c := range codes {
		f.exec(t, `INSERT INTO staff_recovery_codes (shop_id, staff_id, code_hash) VALUES ($1, $2, $3)`,
			a, own, auth.HashRecoveryCode(recoverySecret, own, c))
	}
	b, ownB := f.shops["b.com"], f.owner["b"]
	other := auth.NewRecoveryCodes()[0]
	f.exec(t, `INSERT INTO staff_recovery_codes (shop_id, staff_id, code_hash) VALUES ($1, $2, $3)`,
		b, ownB, auth.HashRecoveryCode(recoverySecret, ownB, other))

	rec := f.recovery(f.tempToken(t, "a"), " "+strings.ToLower(strings.ReplaceAll(codes[0], "-", " "))+" ")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"setup_required":false}` {
		t.Fatalf("recovery: %d %s", rec.Code, rec.Body.String())
	}
	if stage, _ := f.session(t, adminCookie(t, rec, 43200, "recovery")); stage != "active" {
		t.Errorf("recovery er por stage %s", stage)
	}
	if n := f.count(t, `SELECT count(*) FROM staff_recovery_codes WHERE staff_id = $1 AND used_at IS NOT NULL`, own); n != 1 {
		t.Errorf("used code %d, chai 1", n)
	}
	wantErr(t, f.recovery(f.tempToken(t, "a"), codes[0]), 400, "mfa_invalid", "ekoi code abar")
	wantErr(t, f.recovery(f.tempToken(t, "a"), other), 400, "mfa_invalid", "onno staff er code")
	if n := f.count(t, `SELECT failed_mfa FROM staff WHERE id = $1`, own); n != 2 {
		t.Errorf("failed_mfa %d, chai 2", n)
	}

	toks := make(chan string, 4)
	for range 4 {
		toks <- f.mfaToken(t, "a")
	}
	got := parallel(4, func() *httptest.ResponseRecorder { return f.recovery(<-toks, codes[1]) })
	if got["200 "] != 1 || got["400 mfa_invalid"] != 3 {
		t.Errorf("parallel ekoi recovery code: %v, chai ekta 200", got)
	}
	// parallel er bhul gulo success er pore-o gona hote pare, tai aro ekta thik code er por failed_mfa dekho
	if rec := f.recovery(f.tempToken(t, "a"), codes[2]); rec.Code != http.StatusOK {
		t.Fatalf("tritiyo code: %d %s", rec.Code, rec.Body.String())
	}
	if n := f.count(t, `SELECT count(*) FROM staff_recovery_codes WHERE staff_id = $1 AND used_at IS NULL`, own); n != 7 {
		t.Errorf("baki code %d, chai 7", n)
	}
	if n := f.count(t, `SELECT failed_mfa FROM staff WHERE id = $1`, own); n != 0 {
		t.Errorf("thik recovery er por failed_mfa %d", n)
	}
}

// TestStaffLockRaceIntegration (8.3 step 1, 2, 4 v1.24, 7.8, 8.17, 12.2): lock check er age atomic reservation.
// Parallel bhul password e lock er age 5 tar beshi asol PBKDF2 na (baki "locked" audit), lock er por thik password
// o 401; parallel temp_token e 10 tar beshi TOTP guess na, lock e thik code o 429; 30 minute lock password er
// bhul e choto hoy na; login alert er dedupe_key; /auth/password e bhul old gona ar audit; ochena email er audit e
// email na, shudhu domain; auth response e Cache-Control: no-store.
func TestStaffLockRaceIntegration(t *testing.T) {
	f := newStaffFixture(t)
	a, own := f.shops["a.com"], f.owner["a"]
	reason := func(r string) int {
		return f.count(t, `SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND action = 'staff.login_failed'
			AND after->>'reason' = $2`, a, r)
	}

	got := parallel(20, func() *httptest.ResponseRecorder { return f.login("a", "o@a.com", "wrong-password-1") })
	if got["401 invalid_credentials"] != 20 {
		t.Errorf("parallel bhul password: %v, chai 20 ta 401", got)
	}
	if n, p := f.count(t, `SELECT failed_logins FROM staff WHERE id = $1`, own), reason("password"); n != 5 || p != 5 {
		t.Errorf("parallel 20 bhul: failed_logins %d, password audit %d; chai 5, 5", n, p)
	}
	if n := reason("locked"); n != 15 {
		t.Errorf("locked audit %d, chai 15", n)
	}
	rec := f.login("a", "o@a.com", staffPW)
	wantErr(t, rec, 401, "invalid_credentials", "lock er por thik password")
	if h := rec.Header().Get("Retry-After"); h != "" {
		t.Errorf("lock e Retry-After %q", h)
	}

	// failed_mfa: 4 ta parallel temp_token e 20 bhul → 10 guess, baki 429; lock e thik code o 429
	f.exec(t, `UPDATE staff SET failed_logins = 0, locked_until = NULL WHERE id = $1`, own)
	toks := make(chan string, 20)
	for range 4 {
		tok := f.mfaToken(t, "a")
		for range 5 {
			toks <- tok
		}
	}
	got = parallel(20, func() *httptest.ResponseRecorder { return f.totp(<-toks, "000000") })
	if got["400 mfa_invalid"]+got["429 rate_limited"] != 20 || got["429 rate_limited"] < 10 {
		t.Errorf("parallel temp_token 20 bhul: %v", got)
	}
	if n, m := f.count(t, `SELECT failed_mfa FROM staff WHERE id = $1`, own), f.audits(t, "a", "staff.login_mfa_failed"); n != 10 || m != 10 {
		t.Errorf("failed_mfa %d, mfa_failed audit %d; chai 10, 10", n, m)
	}
	limited(t, f.totp(f.mfaToken(t, "a"), f.code(own, 0)), "lock e thik code")

	// 30 minute lock password er bhul e choto hoy na (GREATEST)
	f.exec(t, `UPDATE staff SET failed_logins = 9 WHERE id = $1`, own)
	f.login("a", "o@a.com", "wrong-password-1")
	if n := f.count(t, `SELECT count(*) FROM staff WHERE id = $1 AND locked_until > now() + interval '29 minutes'`, own); n != 1 {
		t.Error("lambha lock choto hoyeche")
	}

	// login alert er dedupe_key (8.17 v1.24)
	f.exec(t, `UPDATE staff SET failed_logins = 9, failed_mfa = 0, locked_until = NULL WHERE id = $1`, own)
	f.login("a", "o@a.com", "wrong-password-1")
	var key string
	if err := f.d.QueryRow(`SELECT dedupe_key FROM jobs WHERE kind = 'alert.security' AND shop_id = $1
		AND payload->>'event' = 'staff_login_failed'`, a).Scan(&key); err != nil {
		t.Fatalf("login alert: %v", err)
	}
	day := time.Now().In(mustLoc(t, "Asia/Dhaka")).Format(time.DateOnly)
	if want := fmt.Sprintf("alert.staff_login_failed:%d:%d:10:%s", a, own, day); key != want {
		t.Errorf("dedupe_key %q, chai %q", key, want)
	}

	// ochena email: audit e email na (keu password likhe phellte pare), shudhu domain
	f.login("a", "Secret-PW-123@a.com", staffPW)
	if n := f.count(t, `SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND after->>'reason' = 'unknown_email'
		AND NOT after ? 'email' AND after->>'email_domain' = 'a.com' AND after::text NOT LIKE '%Secret-PW%'`, a); n != 1 {
		t.Error("ochena email er audit e email (ba domain nai)")
	}

	// no-store: temp_token, session cookie
	f.exec(t, `UPDATE staff SET failed_logins = 0, locked_until = NULL WHERE id = $1`, own)
	rec = f.login("a", "o@a.com", staffPW)
	if h := rec.Header().Get("Cache-Control"); h != "no-store" {
		t.Errorf("login Cache-Control %q", h)
	}
	if h := f.totp(f.mfaToken(t, "a"), f.code(own, 1)).Header().Get("Cache-Control"); h != "no-store" {
		t.Errorf("totp Cache-Control %q", h)
	}
	if h := f.verify(t, "a.com", "+8801711000001", "000000").Header().Get("Cache-Control"); h != "no-store" {
		t.Errorf("otp verify Cache-Control %q", h)
	}
	cust, _ := f.otpFixture.login(t, "a.com", "+8801711000002", "bn")
	if h := f.serve(newReq(http.MethodGet, "/api/me", "a.com", cust, nil)).Header().Get("Cache-Control"); h != "no-store" {
		t.Errorf("/api/me Cache-Control %q", h)
	}

	// /auth/password: bhul old e failed_logins gona ar audit (8.3 step 4 v1.24)
	f.exec(t, `UPDATE staff SET must_change_password = true, totp_secret_enc = NULL, totp_nonce = NULL WHERE id = $1`, own)
	tok := f.setupSession(t, "a", staffPW)
	wantErr(t, f.cookieReq("POST", "/api/admin/auth/password", tok, map[string]string{"old": "bhul-password-xx", "new": "notun-password-123"}),
		400, "validation_failed", "bhul old")
	if n := f.count(t, `SELECT failed_logins FROM staff WHERE id = $1`, own); n != 1 {
		t.Errorf("bhul old e failed_logins %d, chai 1", n)
	}
	if n := f.audits(t, "a", "staff.password_change_failed"); n != 1 {
		t.Errorf("password_change_failed audit %d", n)
	}
}
