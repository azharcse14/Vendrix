package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vendrix/backend/internal/config"
	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/shop"
)

type fakeDB struct{ err error }

func (f fakeDB) PingContext(context.Context) error { return f.err }

func quiet() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func testConfig() config.Config {
	return config.Config{Env: "local", Addr: "127.0.0.1:0", AdminOrigins: []string{"http://localhost:3001"},
		JasprInternalSecret: "0123456789abcdef0123456789abcdef"}
}

// TestRouterBuild (M0, M1, 12.2 Server 7): puro router panic chhara bane; /healthz database er obostha dekhay;
// admin route staff chhara 401, public route shop chhara 404 (v1.10).
func TestRouterBuild(t *testing.T) {
	h, err := newHandler(testConfig(), quiet(), nil, fakeDB{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) || rec.Header().Get("X-Request-Id") == "" {
		t.Errorf("healthz: %d %s", rec.Code, rec.Body.String())
	}
	down, err := newHandler(testConfig(), quiet(), nil, fakeDB{err: errors.New("down")})
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	down.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "db_unavailable") {
		t.Errorf("database bondho: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nai", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("ochena path JSON na: %d %s", rec.Code, rec.Body.String())
	}
	for _, p := range []string{"/api/admin/categories", "/api/admin/products", "/api/admin/products/1"} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "unauthenticated") {
			t.Errorf("%s staff chhara: %d %s", p, rec.Code, rec.Body.String())
		}
	}
	for _, p := range []string{"/api/settings", "/api/categories", "/api/products", "/api/products/x", "/api/translations"} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "shop_not_found") {
			t.Errorf("%s X-Shop-Host chhara: %d %s", p, rec.Code, rec.Body.String())
		}
	}
	if _, err := newHandler(config.Config{AdminOrigins: []string{"not a url"}}, quiet(), nil, fakeDB{}); err == nil {
		t.Error("bhul ADMIN_ORIGINS e error ashe ni")
	}
}

// TestCrossOrigin (12.2 Server 3): onno site er browser theke POST 403; admin origin CrossOriginProtection paar hoy.
func TestCrossOrigin(t *testing.T) {
	h, err := newHandler(testConfig(), quiet(), nil, fakeDB{})
	if err != nil {
		t.Fatal(err)
	}
	post := func(origin string) int {
		r := httptest.NewRequest(http.MethodPost, "/healthz", nil)
		r.Header.Set("Origin", origin)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	if c := post("https://evil.example"); c != http.StatusForbidden {
		t.Errorf("onno site: %d", c)
	}
	if c := post("http://localhost:3001"); c != http.StatusMethodNotAllowed { // CrossOriginProtection paar, tarpor mux er 405
		t.Errorf("admin origin: %d", c)
	}
}

// TestServeShutdown (2.2): ctx bondho hole server thame, timeout 4.2 er moto.
func TestServeShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv := newServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	done := make(chan error, 1)
	go func() { done <- serve(ctx, srv, ln, quiet()) }()
	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown hoyni")
	}
	if srv.ReadHeaderTimeout != 5*time.Second || srv.ReadTimeout != 15*time.Second || srv.WriteTimeout != 30*time.Second || srv.IdleTimeout != 60*time.Second {
		t.Error("timeout 4.2 er moto na")
	}
}

func TestRun(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	noEnv := func(string) (string, bool) { return "", false }
	if err := run(ctx, []string{"version"}, noEnv, nil, &out); err != nil || out.Len() == 0 {
		t.Errorf("version: %v", err)
	}
	if err := run(ctx, []string{"nai"}, noEnv, nil, &out); err == nil {
		t.Error("ochena command e error ashe ni")
	}
	for _, cmd := range []string{"serve", "migrate"} {
		if err := run(ctx, []string{cmd}, noEnv, nil, &out); err == nil || !strings.Contains(err.Error(), "APP_ENV") {
			t.Errorf("%s env chhara: %v", cmd, err)
		}
	}
	env := map[string]string{"APP_ENV": "local", "DATABASE_URL": "postgres://x@127.0.0.1:1/db?connect_timeout=1",
		"JASPR_INTERNAL_SECRET": strings.Repeat("s", 32), "OTP_SECRET": strings.Repeat("o", 32), "RECOVERY_SECRET": strings.Repeat("r", 32),
		"CONFIRM_SECRET":   strings.Repeat("c", 32),
		"TURNSTILE_SECRET": "1x0000000000000000000000000000000AA", "TOTP_KEY": "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	// config thik, tai error database theke (config er error e na)
	if err := run(ctx, nil, lookup, nil, &out); err == nil || strings.Contains(err.Error(), "_SECRET") || strings.Contains(err.Error(), "TOTP_KEY") {
		t.Errorf("database chhara serve: %v", err)
	}
	for _, args := range [][]string{{"credentials"}, {"credentials", "get"}, {"credentials", "set", "--shop", "x"},
		{"credentials", "set", "--provider", "sms"}, {"credentials", "set", "--shop", "x", "--provider", "sms", "extra"}} {
		if err := run(ctx, args, noEnv, strings.NewReader("{}"), &out); err == nil || strings.Contains(err.Error(), "APP_ENV") {
			t.Errorf("%q: argument er error chai, pelam %v", args, err)
		}
	}
	for _, args := range [][]string{{"shop"}, {"shop", "delete"}, {"shop", "create", "--slug", "x"}, {"shop", "create", "--nai"}} {
		if err := run(ctx, args, noEnv, nil, &out); err == nil || strings.Contains(err.Error(), "APP_ENV") {
			t.Errorf("%q: argument er error chai, pelam %v", args, err)
		}
	}
	full := []string{"shop", "create", "--slug", "x", "--prefix", "XX", "--domain", "x.com", "--name-bn", "ক", "--owner-email", "a@x.com", "--owner-name", "A"}
	if err := run(ctx, full, noEnv, nil, &out); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("env chhara shop create: %v", err)
	}
	// staff reset-2fa (8.3 item 6): bhul argument e usage, env er age
	for _, args := range [][]string{{"staff"}, {"staff", "nai"}, {"staff", "reset-2fa"}, {"staff", "reset-2fa", "--shop", "x"},
		{"staff", "reset-2fa", "--email", "a@x.com"}, {"staff", "reset-2fa", "--shop", "x", "--email", "a@x.com", "extra"}} {
		if err := run(ctx, args, noEnv, nil, &out); err == nil || !strings.Contains(err.Error(), "reset-2fa --shop") {
			t.Errorf("%q: usage error chai, pelam %v", args, err)
		}
	}
	if err := run(ctx, []string{"staff", "reset-2fa", "--shop", "x", "--email", "a@x.com"}, noEnv, nil, &out); err == nil ||
		!strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("env chhara staff reset-2fa: %v", err)
	}
	args := []string{"credentials", "set", "--shop", "x", "--provider", "sms"}
	if err := run(ctx, args, noEnv, strings.NewReader("{}"), &out); err == nil || !strings.Contains(err.Error(), "CREDENTIALS_KEY") {
		t.Errorf("env chhara credentials: %v", err)
	}
}

// TestOTPSendIPLimit (7.8, 3.2 Niyom 5, 14.2 test 18 er OTP ongsho): router diye OTP send e client IP te
// ghontay 10. Website (loopback + X-Internal-Auth) theke alada X-Client-IP e 11 ta 429 na, ekoi IP te 11 tomo
// 429; secret chhara X-Client-IP mane na (shob ek IP).
func TestOTPSendIPLimit(t *testing.T) {
	d := dbtest.New(t)
	if _, _, err := shop.Create(context.Background(), d, shop.Params{Slug: "a", Prefix: "SA", Domain: "a.com",
		NameBn: "দোকান", OwnerEmail: "o@a.com", OwnerName: "O", Preset: "organic"}); err != nil {
		t.Fatal(err)
	}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true,"hostname":"a.com"}`))
	}))
	defer fake.Close()
	old := siteverifyURL
	siteverifyURL = fake.URL
	defer func() { siteverifyURL = old }()
	cfg := testConfig()
	cfg.OTPSecret, cfg.TurnstileSecret = "otp-secret-0123456789abcdef012345", "ts-secret"

	n := 0
	send := func(h http.Handler, ip, secret string) int {
		n++ // protibar notun phone, jate phone er 3/15 minute limit na lage
		r := httptest.NewRequest(http.MethodPost, "/api/auth/otp/send", strings.NewReader(
			`{"phone":"0171100`+fmt.Sprintf("%04d", n)+`","purpose":"login","turnstile_token":"x"}`))
		r.RemoteAddr = "127.0.0.1:40000"
		r.Header.Set("X-Shop-Host", "a.com")
		r.Header.Set("X-Client-IP", ip)
		r.Header.Set("X-Internal-Auth", secret)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	for name, c := range map[string]struct {
		sameIP bool
		secret string
		last   int
	}{
		"alada IP":      {false, cfg.JasprInternalSecret, http.StatusOK},
		"ekoi IP":       {true, cfg.JasprInternalSecret, http.StatusTooManyRequests},
		"secret chhara": {false, "", http.StatusTooManyRequests},
		"bhul secret":   {false, "bhul", http.StatusTooManyRequests},
	} {
		h, err := newHandler(cfg, quiet(), d, fakeDB{})
		if err != nil {
			t.Fatal(err)
		}
		for i := range 11 {
			ip := "203.0.113.1"
			if !c.sameIP {
				ip = fmt.Sprintf("203.0.113.%d", i+1)
			}
			code := send(h, ip, c.secret)
			if i < 10 && code != http.StatusOK {
				t.Fatalf("%s: %d tomo send %d", name, i+1, code)
			}
			if i == 10 && code != c.last {
				t.Errorf("%s: 11 tomo send %d, chai %d", name, code, c.last)
			}
		}
	}
}

// TestAdminMFAIPLimit (7.8 v1.24): /api/admin/auth/totp ar /recovery e ek client IP te minute e 120 (duita
// route mile), tarpor 429 + Retry-After; onno IP te chole.
func TestAdminMFAIPLimit(t *testing.T) {
	d := dbtest.New(t)
	cfg := testConfig()
	cfg.TOTPKey = bytes.Repeat([]byte{7}, 32)
	h, err := newHandler(cfg, quiet(), d, fakeDB{})
	if err != nil {
		t.Fatal(err)
	}
	post := func(path, ip string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"temp_token":"x","code":"000000"}`))
		r.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}
	for i := range 120 {
		path := "/api/admin/auth/totp"
		if i%2 == 1 {
			path = "/api/admin/auth/recovery"
		}
		if rec := post(path, "198.51.100.7"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%d tomo: %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	for _, p := range []string{"/api/admin/auth/totp", "/api/admin/auth/recovery"} {
		if rec := post(p, "198.51.100.7"); rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
			t.Errorf("%s 121 tomo: %d %q", p, rec.Code, rec.Header().Get("Retry-After"))
		}
	}
	if rec := post("/api/admin/auth/totp", "198.51.100.8"); rec.Code != http.StatusUnauthorized {
		t.Errorf("onno IP: %d", rec.Code)
	}
}

// TestSessionRouteKind (14.2 test 19, 3.2 Niyom 3, Rule 16): router diye session er dhoron. Customer token admin
// route e (Bearer ba cookie) 401; mfa_pending temp_token admin API te 401; setup_pending e setup route chole,
// baki admin API 403 setup_required; active staff cookie admin e chole kintu customer route e (cookie ba Bearer) 401.
func TestSessionRouteKind(t *testing.T) {
	d := dbtest.New(t)
	shopID, _, err := shop.Create(context.Background(), d, shop.Params{Slug: "a", Prefix: "SA", Domain: "a.com",
		NameBn: "দোকান", OwnerEmail: "o@a.com", OwnerName: "O", Preset: "organic"})
	if err != nil {
		t.Fatal(err)
	}
	var userID, staffID int64
	if err := d.QueryRow(`INSERT INTO users (shop_id, phone) VALUES ($1, '+8801711000001') RETURNING id`, shopID).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	// must_change_password false: setup_pending e totp/setup sora-sori chole (password er age 403 na)
	if err := d.QueryRow(`UPDATE staff SET must_change_password = false WHERE shop_id = $1 RETURNING id`, shopID).Scan(&staffID); err != nil {
		t.Fatal(err)
	}
	session := func(tok, kind, stage string) {
		h := sha256.Sum256([]byte(tok))
		var user, staff any
		if kind == "customer" {
			user = userID
		} else {
			staff = staffID
		}
		if _, err := d.Exec(`INSERT INTO sessions (shop_id, token_hash, subject_type, user_id, staff_id, stage, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, now() + interval '1 hour')`, shopID, h[:], kind, user, staff, stage); err != nil {
			t.Fatal(err)
		}
	}
	session("cust-token", "customer", "active")
	session("mfa-token", "staff", "mfa_pending")
	session("setup-token", "staff", "setup_pending")
	session("staff-token", "staff", "active")

	cfg := testConfig()
	cfg.TOTPKey = bytes.Repeat([]byte{7}, 32)
	h, err := newHandler(cfg, quiet(), d, fakeDB{})
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path, bearer, cookie string) (int, string) {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("X-Shop-Host", "a.com")
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: "__Host-asid", Value: cookie})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code, rec.Body.String()
	}
	for _, c := range []struct {
		name, method, path, bearer, cookie string
		status                             int
		code                               string
	}{
		{"customer Bearer admin e", "GET", "/api/admin/me", "cust-token", "", 401, "unauthenticated"},
		{"customer token cookie te", "GET", "/api/admin/products", "", "cust-token", 401, "unauthenticated"},
		{"mfa_pending cookie te", "GET", "/api/admin/me", "", "mfa-token", 401, "unauthenticated"},
		{"mfa_pending Bearer", "GET", "/api/admin/products", "mfa-token", "", 401, "unauthenticated"},
		{"mfa_pending setup route e", "POST", "/api/admin/auth/totp/setup", "", "mfa-token", 401, "unauthenticated"},
		{"setup_pending product", "GET", "/api/admin/products", "", "setup-token", 403, "setup_required"},
		{"setup_pending category", "POST", "/api/admin/categories", "", "setup-token", 403, "setup_required"},
		{"setup_pending /me", "GET", "/api/admin/me", "", "setup-token", 403, "setup_required"},
		{"setup_pending setup route", "POST", "/api/admin/auth/totp/setup", "", "setup-token", 200, ""},
		{"active staff /me", "GET", "/api/admin/me", "", "staff-token", 200, ""},
		{"active staff product", "GET", "/api/admin/products", "", "staff-token", 200, ""},
		{"staff cookie customer route e", "GET", "/api/me", "", "staff-token", 401, "unauthenticated"},
		{"staff token Bearer customer route e", "GET", "/api/me", "staff-token", "", 401, "unauthenticated"},
		{"customer route", "GET", "/api/me", "cust-token", "", 200, ""},
		{"guest cart", "GET", "/api/cart", "", "", 200, ""},
		{"customer cart", "GET", "/api/cart", "cust-token", "", 200, ""},
		{"staff token Bearer cart e", "GET", "/api/cart", "staff-token", "", 401, "unauthenticated"},
		{"guest cart count", "GET", "/api/cart/count", "", "", 200, ""},
		{"customer cart count", "GET", "/api/cart/count", "cust-token", "", 200, ""},
		{"staff token Bearer cart count e", "GET", "/api/cart/count", "staff-token", "", 401, "unauthenticated"},
		{"guest wishlist", "GET", "/api/wishlist", "", "", 401, "unauthenticated"},
		{"customer wishlist", "GET", "/api/wishlist", "cust-token", "", 200, ""},
		{"guest quote (body nai)", "POST", "/api/checkout/quote", "", "", 400, "validation_failed"},
		{"staff token Bearer quote e", "POST", "/api/checkout/quote", "staff-token", "", 401, "unauthenticated"},
		{"guest order (key nai)", "POST", "/api/orders", "", "", 400, "missing_idempotency_key"},
		{"staff token Bearer order e", "POST", "/api/orders", "staff-token", "", 401, "unauthenticated"},
		{"guest order cancel (body nai)", "POST", "/api/orders/x/cancel", "", "", 400, "validation_failed"},
		{"staff token Bearer cancel e", "POST", "/api/orders/x/cancel", "staff-token", "", 401, "unauthenticated"},
		{"guest confirm view (ochena token)", "GET", "/api/confirm/x", "", "", 404, "not_found"},
		{"guest confirm (ochena token)", "POST", "/api/confirm/x", "", "", 404, "not_found"},
		{"staff token Bearer confirm e", "POST", "/api/confirm/x", "staff-token", "", 401, "unauthenticated"},
		{"guest order view (ochena token)", "GET", "/api/orders/x", "", "", 404, "not_found"},
		{"staff token Bearer order view e", "GET", "/api/orders/x", "staff-token", "", 401, "unauthenticated"},
		{"guest track (body nai)", "POST", "/api/orders/track", "", "", 400, "validation_failed"},
		{"guest me/orders", "GET", "/api/me/orders", "", "", 401, "unauthenticated"},
		{"staff cookie me/orders e", "GET", "/api/me/orders", "", "staff-token", 401, "unauthenticated"},
		{"customer me/orders", "GET", "/api/me/orders", "cust-token", "", 200, ""},
		{"setup_pending logout", "POST", "/api/admin/auth/logout", "", "setup-token", 204, ""},
		{"logout er por", "POST", "/api/admin/auth/totp/setup", "", "setup-token", 401, "unauthenticated"},
	} {
		status, body := do(c.method, c.path, c.bearer, c.cookie)
		if status != c.status || (c.code != "" && !strings.Contains(body, `"code":"`+c.code+`"`)) {
			t.Errorf("%s: %d %s, chai %d %s", c.name, status, body, c.status, c.code)
		}
	}
}
