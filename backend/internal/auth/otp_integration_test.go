package auth_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/jobs"
	"vendrix/backend/internal/notify"
	"vendrix/backend/internal/shop"
)

const otpSecret = "test-otp-secret-0123456789abcdef"

// fakeSiteverify: Cloudflare siteverify er moto; token "ok:<host>" → success, hostname <host>; baki fail.
func fakeSiteverify(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.PostForm.Get("secret") != "ts-secret" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		host, ok := strings.CutPrefix(r.PostForm.Get("response"), "ok:")
		json.NewEncoder(w).Encode(map[string]any{"success": ok, "hostname": host})
	}))
	t.Cleanup(srv.Close)
	return srv
}

type otpFixture struct {
	d     *sql.DB
	srv   http.Handler
	shops map[string]int64 // "a.com", "b.com"
}

func newOTPFixture(t *testing.T) *otpFixture {
	t.Helper()
	return newOTPFixtureMerge(t, func(ctx context.Context, tx *sql.Tx, shopID, userID int64, key, locale string) (any, error) {
		return cart.Merge(ctx, tx, shopID, userID, key, locale)
	})
}

// newOTPFixtureMerge: verify er cart merge bodle (fail er test).
func newOTPFixtureMerge(t *testing.T, merge auth.CartMerge) *otpFixture {
	t.Helper()
	d := dbtest.New(t)
	f := &otpFixture{d: d, shops: map[string]int64{}}
	for _, s := range []string{"a", "b"} {
		id, _, err := shop.Create(context.Background(), d, shop.Params{Slug: s, Prefix: "S" + strings.ToUpper(s),
			Domain: s + ".com", NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		f.shops[s+".com"] = id
	}
	log := slog.New(slog.DiscardHandler)
	h := auth.NewHandlers(d, log, otpSecret, auth.Turnstile{URL: fakeSiteverify(t).URL, Secret: "ts-secret"}, merge)
	mux := http.NewServeMux()
	shopMW := httpx.ShopFromHost(log, shop.NewResolver(d).Resolve)
	mux.Handle("POST /api/auth/otp/send", shopMW(http.HandlerFunc(h.SendOTP)))
	mux.Handle("POST /api/auth/otp/verify", shopMW(http.HandlerFunc(h.VerifyOTP)))
	customer := func(hf http.HandlerFunc) http.Handler {
		return shopMW(httpx.RequireCustomer(log, auth.LookupSession(d))(hf))
	}
	mux.Handle("POST /api/auth/logout", customer(h.Logout))
	mux.Handle("GET /api/me", customer(h.Me))
	mux.Handle("PATCH /api/me", customer(h.UpdateMe))
	f.srv = httpx.ClientIP("")(httpx.JSONErrors(mux))
	return f
}

func (f *otpFixture) send(t *testing.T, host, phone, token string) *httptest.ResponseRecorder {
	t.Helper()
	return f.post(t, host, map[string]any{"phone": phone, "purpose": "login", "turnstile_token": token})
}

func (f *otpFixture) post(t *testing.T, host string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return f.serve(newReq(http.MethodPost, "/api/auth/otp/send", host, "", body))
}

// newReq: X-Shop-Host, token thakle Authorization: Bearer, body thakle JSON.
func newReq(method, path, host, token string, body any) *http.Request {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, rd)
	r.Header.Set("X-Shop-Host", host)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func (f *otpFixture) serve(r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r)
	return rec
}

func (f *otpFixture) verify(t *testing.T, host, phone, code string) *httptest.ResponseRecorder {
	t.Helper()
	return f.serve(newReq(http.MethodPost, "/api/auth/otp/verify", host, "", map[string]string{"phone": phone, "code": code}))
}

// otpRow: login OTP jar code jana (SMS chhara verify er test); expires "5 minutes", "-1 second".
func (f *otpFixture) otpRow(t *testing.T, shopID int64, e164, code, expires string) int64 {
	t.Helper()
	var id int64
	if err := f.d.QueryRow(`INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, expires_at)
		VALUES ($1, $2, 'login', $3, now() + $4::interval) RETURNING id`,
		shopID, e164, auth.HashOTP(otpSecret, e164, "login", code), expires).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// wantErr: status ar error code.
func wantErr(t *testing.T, rec *httptest.ResponseRecorder, status int, code, what string) {
	t.Helper()
	if rec.Code != status || errCode(rec) != code {
		t.Errorf("%s: %d %s chai, pelam %d %s", what, status, code, rec.Code, rec.Body.String())
	}
}

func (f *otpFixture) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := f.d.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func (f *otpFixture) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := f.d.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

// sent: 200 {"sent": true} (8.2 step 1, Q10 vii).
func sent(t *testing.T, rec *httptest.ResponseRecorder, what string) {
	t.Helper()
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"sent":true}` {
		t.Fatalf("%s: %d %s", what, rec.Code, rec.Body.String())
	}
}

// limited: 429 rate_limited; Retry-After (second) ferot.
func limited(t *testing.T, rec *httptest.ResponseRecorder, what string) int {
	t.Helper()
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `"code":"rate_limited"`) {
		t.Fatalf("%s: 429 rate_limited chai, pelam %d %s", what, rec.Code, rec.Body.String())
	}
	s, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || s < 1 {
		t.Fatalf("%s: Retry-After %q", what, rec.Header().Get("Retry-After"))
	}
	return s
}

func errCode(rec *httptest.ResponseRecorder) string {
	var b struct{ Error httpx.ErrorBody }
	json.Unmarshal(rec.Body.Bytes(), &b)
	return b.Error.Code
}

// TestOTPSendLimitsIntegration (8.2 step 1, 7.8, 14.2 test 10): phone e 15 minute e 3, 24 ghontay 10 (shob
// purpose), shop alada gona, parallel e o 3 er beshi na; blocked phone 403; bhul input 400; otp_codes row.
func TestOTPSendLimitsIntegration(t *testing.T) {
	f := newOTPFixture(t)
	a := f.shops["a.com"]
	const ph, e164 = "01711-000001", "+8801711000001"
	for i := range 3 {
		sent(t, f.send(t, "a.com", ph, "ok:a.com"), "send "+strconv.Itoa(i+1))
	}
	if s := limited(t, f.send(t, "a.com", ph, "ok:a.com"), "15 minute e 4th"); s > 900 {
		t.Errorf("15 minute er limit e Retry-After %d > 900", s)
	}
	sent(t, f.send(t, "b.com", ph, "ok:b.com"), "onno shop e ekoi phone (key shop_id:phone)")

	var hashLen int
	var expiresIn float64
	var purpose string
	if err := f.d.QueryRow(`SELECT length(code_hash), extract(epoch FROM expires_at - now()), purpose FROM otp_codes
		WHERE shop_id = $1 AND phone = $2 ORDER BY id DESC LIMIT 1`, a, e164).Scan(&hashLen, &expiresIn, &purpose); err != nil {
		t.Fatal(err)
	}
	if hashLen != 32 || expiresIn < 290 || expiresIn > 301 || purpose != "login" {
		t.Errorf("otp_codes: hash %d byte, expire %v second, purpose %q", hashLen, expiresIn, purpose)
	}

	backdate := func() {
		f.exec(t, `UPDATE otp_codes SET created_at = created_at - interval '16 minutes' WHERE shop_id = $1`, a)
	}
	backdate()
	for range 3 {
		sent(t, f.send(t, "a.com", ph, "ok:a.com"), "16 minute por")
	}
	backdate()
	for range 3 {
		sent(t, f.send(t, "a.com", ph, "ok:a.com"), "32 minute por")
	}
	// 10 tomo ta checkout er OTP: dine 10 shob purpose mile (7.8)
	f.exec(t, `INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, expires_at, created_at)
		VALUES ($1, $2, 'checkout', '\x00', now(), now())`, a, e164)
	backdate()
	if s := limited(t, f.send(t, "a.com", ph, "ok:a.com"), "24 ghontay 11th"); s < 22*3600 || s > 24*3600 {
		t.Errorf("24 ghontar limit e Retry-After %d (shob theke purono row 48 minute ager)", s)
	}
	if n := f.count(t, `SELECT count(*) FROM otp_codes WHERE shop_id = $1 AND phone = $2`, a, e164); n != 10 {
		t.Errorf("limit er por-o row: %d", n)
	}

	f.exec(t, `INSERT INTO blocked_phones (shop_id, phone) VALUES ($1, '+8801711000002')`, a)
	if rec := f.send(t, "a.com", "01711000002", "ok:a.com"); rec.Code != http.StatusForbidden || errCode(rec) != "phone_blocked" {
		t.Errorf("blocked phone: %d %s", rec.Code, rec.Body.String())
	}
	for name, c := range map[string]struct {
		body any
		code string
	}{
		"bhul phone":     {map[string]any{"phone": "12345", "purpose": "login", "turnstile_token": "ok:a.com"}, "invalid_phone"},
		"ochena purpose": {map[string]any{"phone": "01711000003", "purpose": "signup", "turnstile_token": "ok:a.com"}, "validation_failed"},
		"ochena field":   {map[string]any{"phone": "01711000003", "purpose": "login", "turnstile_token": "ok:a.com", "x": 1}, "validation_failed"},
	} {
		if rec := f.post(t, "a.com", c.body); rec.Code != http.StatusBadRequest || errCode(rec) != c.code {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}

	// parallel: advisory lock er karone 10 ta request e thik 3 ta
	var wg sync.WaitGroup
	codes := make(chan int, 10)
	for range 10 {
		wg.Go(func() { codes <- f.send(t, "a.com", "01811000004", "ok:a.com").Code })
	}
	wg.Wait()
	close(codes)
	got := map[int]int{}
	for c := range codes {
		got[c]++
	}
	if got[200] != 3 || got[429] != 7 {
		t.Errorf("parallel 10 send: %v, chai 3 ta 200 ar 7 ta 429", got)
	}
}

// TestOTPPhoneLockIntegration (7.8 v1.19, Q3 (a)): shesh 24 ghontay otp_codes er bhul (attempts, consumed row e
// ek kom) 10 hole 429 rate_limited, Retry-After rolling window theke; alert.security ekbar (dedupe_key).
func TestOTPPhoneLockIntegration(t *testing.T) {
	f := newOTPFixture(t)
	a := f.shops["a.com"]
	row := func(phone string, attempts int, ago string, consumed bool) {
		f.exec(t, `INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, attempts, expires_at, created_at, consumed_at)
			VALUES ($1, $2, 'login', '\x00', $3, now() - $4::interval + interval '5 minutes', now() - $4::interval,
			  CASE WHEN $5 THEN now() - $4::interval END)`, a, phone, attempts, ago, consumed)
	}
	row("+8801711000001", 5, "2 hours", false)
	row("+8801711000001", 5, "2 hours", false)
	s := limited(t, f.send(t, "a.com", "01711000001", "ok:a.com"), "10 bhul")
	if want := 22 * 3600; math.Abs(float64(s-want)) > 60 {
		t.Errorf("Retry-After %d, chai ~%d (purono bhul 24 ghonta par hole)", s, want)
	}
	limited(t, f.send(t, "a.com", "01711000001", "ok:a.com"), "abar")
	if n := f.count(t, `SELECT count(*) FROM otp_codes WHERE phone = '+8801711000001'`); n != 2 {
		t.Errorf("lock e notun OTP row hoyeche: %d", n)
	}
	var key string
	var payload []byte
	if err := f.d.QueryRow(`SELECT dedupe_key, payload FROM jobs WHERE kind = 'alert.security'`).Scan(&key, &payload); err != nil {
		t.Fatalf("alert.security ekta chai: %v", err)
	}
	day := time.Now().In(mustLoc(t, "Asia/Dhaka")).Format(time.DateOnly)
	// v1.26 (7.8): jobs row 30 din thake, tai key e plain phone na, HMAC er 16 hex
	if want := regexp.MustCompile(`^alert\.otp_lock:` + strconv.FormatInt(a, 10) + `:[0-9a-f]{16}:` + day + `$`); !want.MatchString(key) {
		t.Errorf("dedupe_key %q, chai %s", key, want)
	}
	if strings.Contains(string(payload), "1711000001") {
		t.Errorf("alert payload e puro phone (12.2): %s", payload)
	}
	if n := f.count(t, `SELECT count(*) FROM jobs WHERE kind = 'alert.security'`); n != 1 {
		t.Errorf("alert %d bar, chai ekbar", n)
	}

	// consumed row er shesh attempt thik: 5 + (5-1) = 9 bhul, lock na
	row("+8801711000002", 5, "1 hour", false)
	row("+8801711000002", 5, "1 hour", true)
	sent(t, f.send(t, "a.com", "01711000002", "ok:a.com"), "9 bhul")
	// 24 ghontar ager bhul gona hoy na
	row("+8801711000003", 5, "25 hours", false)
	row("+8801711000003", 5, "1 hour", false)
	sent(t, f.send(t, "a.com", "01711000003", "ok:a.com"), "purono bhul")
	// rolling: purono 5 ta 10 minute por window er baire
	row("+8801711000004", 5, "23 hours 50 minutes", false)
	row("+8801711000004", 5, "1 hour", false)
	if s := limited(t, f.send(t, "a.com", "01711000004", "ok:a.com"), "rolling"); s < 540 || s > 660 {
		t.Errorf("rolling Retry-After %d, chai ~600", s)
	}
	// onno shop e ekoi phone er lock nai
	sent(t, f.send(t, "b.com", "01711000001", "ok:b.com"), "onno shop")
}

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	l, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// TestOTPSMSCapIntegration (7.8 v1.7, Q10 v): shop e dine checkout.sms_daily_cap er beshi OTP na; din shop er
// timezone er calendar din, Retry-After shop er rat 12 ta porjonto; onno shop alada.
func TestOTPSMSCapIntegration(t *testing.T) {
	f := newOTPFixture(t)
	a := f.shops["a.com"]
	f.exec(t, `UPDATE shop_settings SET published = jsonb_set(published, '{checkout,sms_daily_cap}', '2') WHERE shop_id = $1`, a)
	// shop er gotokal (Asia/Dhaka) er shesh minute: ajker cap e gona hoy na
	f.exec(t, `INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, expires_at, created_at)
		VALUES ($1, '+8801911000009', 'login', '\x00', now(),
		  date_trunc('day', now() AT TIME ZONE 'Asia/Dhaka') AT TIME ZONE 'Asia/Dhaka' - interval '1 minute')`, a)
	sent(t, f.send(t, "a.com", "01711000001", "ok:a.com"), "1")
	sent(t, f.send(t, "a.com", "01711000002", "ok:a.com"), "2")
	s := limited(t, f.send(t, "a.com", "01711000003", "ok:a.com"), "cap")
	now := time.Now().In(mustLoc(t, "Asia/Dhaka"))
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	if want := midnight.Sub(now).Seconds(); math.Abs(float64(s)-want) > 5 {
		t.Errorf("cap Retry-After %d, chai ~%.0f (shop er rat 12 ta)", s, want)
	}
	sent(t, f.send(t, "b.com", "01711000003", "ok:b.com"), "onno shop er cap alada")
}

// TestTurnstile (12.2 Website 1, 10.11, 14.2 test 23 er OTP ongsho): siteverify e secret, token, remoteip;
// success ar hostname = shop er domain hole-i thik; bhul token e send 400 turnstile_failed, DB te jay na.
func TestTurnstile(t *testing.T) {
	var form url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		form = r.PostForm
		switch tok := r.PostForm.Get("response"); {
		case tok == "500":
			http.Error(w, "down", http.StatusInternalServerError)
		case tok == "json":
			w.Write([]byte("{"))
		default:
			host, ok := strings.CutPrefix(tok, "ok:")
			json.NewEncoder(w).Encode(map[string]any{"success": ok, "hostname": host, "error-codes": []string{}})
		}
	}))
	defer srv.Close()
	ts := auth.Turnstile{URL: srv.URL, Secret: "ts-secret"}
	ctx := context.Background()
	ok, err := ts.Verify(ctx, "ok:a.com", "203.0.113.7", "a.com")
	if !ok || err != nil {
		t.Fatalf("thik token: %v %v", ok, err)
	}
	if form.Get("secret") != "ts-secret" || form.Get("response") != "ok:a.com" || form.Get("remoteip") != "203.0.113.7" {
		t.Errorf("siteverify form: %v", form)
	}
	for tok, host := range map[string]string{"ok:evil.com": "a.com", "bad": "a.com", "ok:": "", "ok:A.com": "b.com"} {
		if ok, err := ts.Verify(ctx, tok, "203.0.113.7", host); ok || err != nil {
			t.Errorf("%q/%q: %v %v, chai false", tok, host, ok, err)
		}
	}
	// v1.22 (10.11): Cloudflare er test secret shob shomoy hostname "example.com" dey, tai shudhu oi secret e
	// hostname check na; success false hole tobu fail. Asol secret e upore "ok:evil.com" fail, "ok:a.com" pass.
	for secret, want := range map[string]bool{"1x0000000000000000000000000000000AA": true,
		"3x0000000000000000000000000000000AA": true, "ts-secret": false} {
		test := auth.Turnstile{URL: srv.URL, Secret: secret}
		if ok, err := test.Verify(ctx, "ok:example.com", "", "localhost"); ok != want || err != nil {
			t.Errorf("%s + example.com: %v %v, chai %v", secret, ok, err, want)
		}
		if ok, err := test.Verify(ctx, "bad", "", "localhost"); ok || err != nil {
			t.Errorf("%s + bad token: %v %v, chai false", secret, ok, err)
		}
	}
	form = nil
	if ok, err := ts.Verify(ctx, "", "203.0.113.7", "a.com"); ok || err != nil || form != nil {
		t.Errorf("khali token: %v %v, siteverify dakche %v", ok, err, form != nil)
	}
	for _, tok := range []string{"500", "json"} {
		if ok, err := ts.Verify(ctx, tok, "", "a.com"); ok || err == nil {
			t.Errorf("%s: error chai", tok)
		}
	}

	// handler: Turnstile DB er age, tai database chhara-i 400
	h := auth.NewHandlers(nil, slog.New(slog.DiscardHandler), otpSecret, ts, nil)
	for _, tok := range []string{"bad", "", "ok:evil.com"} {
		b, _ := json.Marshal(map[string]string{"phone": "01711000001", "purpose": "login", "turnstile_token": tok})
		rec := httptest.NewRecorder()
		h.SendOTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/otp/send", bytes.NewReader(b)))
		if rec.Code != http.StatusBadRequest || errCode(rec) != "turnstile_failed" {
			t.Errorf("%q: %d %s", tok, rec.Code, rec.Body.String())
		}
	}
}

type smsFunc func(ctx context.Context, shopID int64, phone, text string) error

func (f smsFunc) Send(ctx context.Context, shopID int64, phone, text string) error {
	return f(ctx, shopID, phone, text)
}

// TestOTPJobPayload (8.2 step 4 v1.19, 8.17, Q1 (a)): sms.send payload {otp_id, nonce, ct}, plain code na;
// AES-GCM AAD otp_codes.id (onno id ba shop e decrypt na); max_attempts 2; worker pathale payload {}.
func TestOTPJobPayload(t *testing.T) {
	f := newOTPFixture(t)
	a := f.shops["a.com"]
	sent(t, f.send(t, "a.com", "01711000001", "ok:a.com"), "send")
	var otpID int64
	var hash []byte
	if err := f.d.QueryRow(`SELECT id, code_hash FROM otp_codes WHERE shop_id = $1`, a).Scan(&otpID, &hash); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	var maxAttempts int
	var shopID int64
	if err := f.d.QueryRow(`SELECT payload, max_attempts, shop_id FROM jobs WHERE kind = 'sms.send'`).
		Scan(&raw, &maxAttempts, &shopID); err != nil {
		t.Fatal(err)
	}
	if maxAttempts != 2 || shopID != a {
		t.Errorf("max_attempts %d (chai 2: 1 minute por ekbar), shop %d", maxAttempts, shopID)
	}
	var keys map[string]json.RawMessage
	json.Unmarshal(raw, &keys)
	if len(keys) != 3 || keys["otp_id"] == nil || keys["nonce"] == nil || keys["ct"] == nil {
		t.Fatalf("payload {otp_id, nonce, ct} chai: %s", raw)
	}
	var p struct {
		OTPID int64  `json:"otp_id"`
		Nonce []byte `json:"nonce"`
		CT    []byte `json:"ct"`
	}
	json.Unmarshal(raw, &p)
	if p.OTPID != otpID {
		t.Errorf("otp_id %d, chai %d", p.OTPID, otpID)
	}
	src := auth.SMSCode(f.d, otpSecret)
	ctx := context.Background()
	o, err := src(ctx, a, otpID, p.Nonce, p.CT)
	phone, code := o.Phone, o.Code
	if err != nil || phone != "+8801711000001" || !auth.CheckOTP(otpSecret, phone, "login", code, hash) ||
		o.Locale != "" || len(o.Override) != 0 {
		t.Fatalf("decrypt (user nai: locale khali, override nai): %+v %v", o, err)
	}
	// customer er locale ar shop er sms.otp override (8.16); onno shop er override ase na (Rule 3)
	f.exec(t, `INSERT INTO users (shop_id, phone, locale) VALUES ($1, '+8801711000001', 'en')`, a)
	f.exec(t, `INSERT INTO translations (shop_id, locale, key, value) VALUES ($1, 'en', 'sms.otp', 'A {code}'),
		($1, 'bn', 'cart.add', 'x'), ($2, 'bn', 'sms.otp', 'B {code}')`, a, f.shops["b.com"])
	if o, err := src(ctx, a, otpID, p.Nonce, p.CT); err != nil || o.Locale != "en" ||
		len(o.Override) != 1 || o.Override["en"] != "A {code}" {
		t.Fatalf("locale/override: %+v %v", o, err)
	}
	if strings.Contains(string(raw), code) || strings.Contains(string(raw), "1711000001") {
		t.Errorf("payload e plain code ba phone: %s", raw)
	}
	if _, err := src(ctx, f.shops["b.com"], otpID, p.Nonce, p.CT); err == nil {
		t.Error("onno shop e decrypt holo")
	}
	f.exec(t, `INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, expires_at) VALUES ($1, '+8801711000001', 'login', '\x00', now())`, a)
	if _, err := src(ctx, a, otpID+1, p.Nonce, p.CT); err == nil {
		t.Error("onno otp_codes.id (AAD) e decrypt holo")
	}
	if _, err := auth.SMSCode(f.d, "onno-secret")(ctx, a, otpID, p.Nonce, p.CT); err == nil {
		t.Error("onno OTP_SECRET e decrypt holo")
	}

	var got []string
	w := jobs.NewWorker(f.d, slog.New(slog.DiscardHandler), map[string]jobs.Handler{
		"sms.send": notify.SendSMSJob(smsFunc(func(_ context.Context, shopID int64, phone, text string) error {
			got = append(got, strconv.FormatInt(shopID, 10)+"|"+phone+"|"+text)
			return nil
		}), src),
	})
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != strconv.FormatInt(a, 10)+"|+8801711000001|A "+code {
		t.Fatalf("SMS: %q", got)
	}
	var after string
	var done bool
	if err := f.d.QueryRow(`SELECT payload::text, done_at IS NOT NULL FROM jobs WHERE kind = 'sms.send'`).Scan(&after, &done); err != nil {
		t.Fatal(err)
	}
	if !done || after != "{}" {
		t.Errorf("done %v, payload %s (chai {})", done, after)
	}
}

// parallel: n ta request ek shathe, protiti response er "status code" (error hole code) gona.
func parallel(n int, do func() *httptest.ResponseRecorder) map[string]int {
	var wg sync.WaitGroup
	var mu sync.Mutex
	got := map[string]int{}
	for range n {
		wg.Go(func() {
			rec := do()
			k := strconv.Itoa(rec.Code) + " " + errCode(rec)
			mu.Lock()
			got[k]++
			mu.Unlock()
		})
	}
	wg.Wait()
	return got
}

// TestOTPAttemptsIntegration (8.2 verify step 2-3, 7.8, 14.2 test 10): ek OTP e 5 bhul er por thik code o
// otp_too_many_attempts (bhul er attempt commit thake); parallel bhul e-o attempts 5 er beshi na; 24 ghontay
// 10 bhul e 429 rate_limited, alert.security ekbar, lock e thik code o chole na.
func TestOTPAttemptsIntegration(t *testing.T) {
	f := newOTPFixture(t)
	a := f.shops["a.com"]
	id := f.otpRow(t, a, "+8801711000001", "123456", "5 minutes")
	for i := range 5 {
		wantErr(t, f.verify(t, "a.com", "01711000001", "000000"), 400, "otp_invalid", "bhul "+strconv.Itoa(i+1))
	}
	wantErr(t, f.verify(t, "a.com", "01711000001", "123456"), 400, "otp_too_many_attempts", "5 bhul er por thik code")
	if n := f.count(t, `SELECT attempts FROM otp_codes WHERE id = $1`, id); n != 5 {
		t.Errorf("attempts %d, chai 5", n)
	}
	if n := f.count(t, `SELECT count(*) FROM sessions`); n != 0 {
		t.Errorf("session hoyeche: %d", n)
	}

	id = f.otpRow(t, a, "+8801711000002", "123456", "5 minutes")
	got := parallel(10, func() *httptest.ResponseRecorder { return f.verify(t, "a.com", "01711000002", "000000") })
	if got["400 otp_invalid"] != 5 || got["400 otp_too_many_attempts"] != 5 {
		t.Errorf("parallel 10 bhul: %v, chai 5 otp_invalid ar 5 otp_too_many_attempts", got)
	}
	if n := f.count(t, `SELECT attempts FROM otp_codes WHERE id = $1`, id); n != 5 {
		t.Errorf("parallel er por attempts %d, chai 5", n)
	}

	// 24 ghontar lock: purono OTP e 5 bhul, notun e 4 bhul (9), 5 tomo bhul e 10 → 429 ar alert
	f.exec(t, `INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, attempts, expires_at, created_at)
		VALUES ($1, '+8801711000003', 'login', '\x00', 5, now() - interval '2 hours', now() - interval '2 hours 5 minutes')`, a)
	f.otpRow(t, a, "+8801711000003", "123456", "5 minutes")
	for i := range 4 {
		wantErr(t, f.verify(t, "a.com", "01711000003", "000000"), 400, "otp_invalid", "lock er age bhul "+strconv.Itoa(i+1))
	}
	limited(t, f.verify(t, "a.com", "01711000003", "000000"), "10 tomo bhul")
	limited(t, f.verify(t, "a.com", "01711000003", "123456"), "lock e thik code")
	if n := f.count(t, `SELECT count(*) FROM jobs WHERE kind = 'alert.security' AND dedupe_key LIKE 'alert.otp_lock:%'
		  AND dedupe_key NOT LIKE '%1711000003%'`); n != 1 { // key e phone na, HMAC tag (7.8 v1.26)
		t.Errorf("lock alert %d bar, chai ekbar", n)
	}
	if n := f.count(t, `SELECT count(*) FROM sessions`); n != 0 {
		t.Errorf("lock e session hoyeche: %d", n)
	}
}

// TestOTPExpiredIntegration (8.2 verify step 1, 3): expire hole thik code o otp_invalid (attempt gona hoy na);
// ek code ekbar-i; shudhu shesh OTP (purono code na); checkout er OTP login e na; bhul input 400.
func TestOTPExpiredIntegration(t *testing.T) {
	f := newOTPFixture(t)
	a := f.shops["a.com"]
	const ph, e164 = "01711000001", "+8801711000001"
	wantErr(t, f.verify(t, "a.com", ph, "123456"), 400, "otp_invalid", "OTP nai")
	id := f.otpRow(t, a, e164, "123456", "-1 second")
	wantErr(t, f.verify(t, "a.com", ph, "123456"), 400, "otp_invalid", "expire")
	if n := f.count(t, `SELECT attempts FROM otp_codes WHERE id = $1`, id); n != 0 {
		t.Errorf("expire OTP e attempt gona holo: %d", n)
	}

	f.otpRow(t, a, e164, "222222", "5 minutes")
	if rec := f.verify(t, "a.com", ph, "222222"); rec.Code != http.StatusOK {
		t.Fatalf("thik code: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, f.verify(t, "a.com", ph, "222222"), 400, "otp_invalid", "ekoi code duibar")

	f.otpRow(t, a, e164, "333333", "5 minutes")
	f.otpRow(t, a, e164, "444444", "5 minutes")
	wantErr(t, f.verify(t, "a.com", ph, "333333"), 400, "otp_invalid", "purono OTP er code")
	if rec := f.verify(t, "a.com", ph, "444444"); rec.Code != http.StatusOK {
		t.Errorf("shesh OTP: %d %s", rec.Code, rec.Body.String())
	}

	f.exec(t, `INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, expires_at)
		VALUES ($1, $2, 'checkout', $3, now() + interval '5 minutes')`, a, e164, auth.HashOTP(otpSecret, e164, "checkout", "555555"))
	wantErr(t, f.verify(t, "a.com", ph, "555555"), 400, "otp_invalid", "checkout er OTP")

	wantErr(t, f.verify(t, "a.com", "12345", "123456"), 400, "invalid_phone", "bhul phone")
	wantErr(t, f.serve(newReq(http.MethodPost, "/api/auth/otp/verify", "a.com", "",
		map[string]any{"phone": ph, "code": "123456", "x": 1})), 400, "validation_failed", "ochena field")
}

// TestOTPParallelVerifyIntegration (8.2 verify step 3 v1.7, 14.2 test 10): ekoi code e parallel verify →
// ekta session, ekta user; baki otp_invalid.
func TestOTPParallelVerifyIntegration(t *testing.T) {
	f := newOTPFixture(t)
	f.otpRow(t, f.shops["a.com"], "+8801711000001", "123456", "5 minutes")
	got := parallel(5, func() *httptest.ResponseRecorder { return f.verify(t, "a.com", "01711000001", "123456") })
	if got["200 "] != 1 || got["400 otp_invalid"] != 4 {
		t.Errorf("parallel verify: %v, chai ekta 200 ar 4 ta otp_invalid", got)
	}
	if n := f.count(t, `SELECT count(*) FROM sessions`); n != 1 {
		t.Errorf("session %d, chai 1", n)
	}
	if n := f.count(t, `SELECT count(*) FROM users`); n != 1 {
		t.Errorf("user %d, chai 1", n)
	}
}

// guestOrder: orders e choto row (order M6 e; ekhane shudhu phone ar user_id dekhi).
func (f *otpFixture) guestOrder(t *testing.T, shopID int64, num, phone string, userID any) int64 {
	t.Helper()
	var id int64
	if err := f.d.QueryRow(`INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash,
		  user_id, customer_name, customer_phone, recipient_name, recipient_phone, ship_district, ship_area, ship_address,
		  subtotal, total, payment_method)
		VALUES ($1, $2, $2, $2, '\x00', $4, 'K', $3, 'K', $3, 'Dhaka', 'Mirpur', 'Road 1', 100, 100, 'cod') RETURNING id`,
		shopID, num, phone, userID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// TestGuestOrdersLinkIntegration (8.2 verify step 5, 14.2 test 11 er ongsho): oi phone er guest order account e
// jog shudhu thik OTP verify er por; onno phone, onno shop ar onno user er order bodlay na.
func TestGuestOrdersLinkIntegration(t *testing.T) {
	f := newOTPFixture(t)
	a, b := f.shops["a.com"], f.shops["b.com"]
	const e164 = "+8801711000001"
	var other int64
	if err := f.d.QueryRow(`INSERT INTO users (shop_id, phone) VALUES ($1, '+8801711000009') RETURNING id`, a).Scan(&other); err != nil {
		t.Fatal(err)
	}
	mine := []int64{f.guestOrder(t, a, "SA-1", e164, nil), f.guestOrder(t, a, "SA-2", e164, nil)}
	otherPhone := f.guestOrder(t, a, "SA-3", "+8801711000002", nil)
	otherUser := f.guestOrder(t, a, "SA-4", e164, other)
	otherShop := f.guestOrder(t, b, "SB-1", e164, nil)
	owner := func(id int64) int64 {
		return int64(f.count(t, `SELECT coalesce(user_id, 0) FROM orders WHERE id = $1`, id))
	}

	f.otpRow(t, a, e164, "123456", "5 minutes")
	wantErr(t, f.verify(t, "a.com", "01711000001", "000000"), 400, "otp_invalid", "bhul code")
	if owner(mine[0]) != 0 {
		t.Fatal("bhul code e order jog holo")
	}
	if rec := f.verify(t, "a.com", "01711000001", "123456"); rec.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	user := int64(f.count(t, `SELECT id FROM users WHERE shop_id = $1 AND phone = $2`, a, e164))
	for _, id := range mine {
		if owner(id) != user {
			t.Errorf("order %d: user %d, chai %d", id, owner(id), user)
		}
	}
	if owner(otherPhone) != 0 || owner(otherShop) != 0 || owner(otherUser) != other {
		t.Errorf("onno order bodleche: phone %d, shop %d, user %d", owner(otherPhone), owner(otherShop), owner(otherUser))
	}
}

// guestCart: shop e n ta product+variant (slug prefix+i, naam bn "পণ্য <slug>", stock 10) ar tader guest cart
// (qty 1, kram e); key ferot. user != 0 hole guest na, oi user er account cart (key "").
func (f *otpFixture) guestCart(t *testing.T, shopID int64, prefix string, n int, user int64) string {
	t.Helper()
	key, hash := cart.NewKey()
	var cartID int64
	var err error
	if user != 0 {
		key = ""
		err = f.d.QueryRow(`INSERT INTO carts (shop_id, user_id) VALUES ($1, $2) RETURNING id`, shopID, user).Scan(&cartID)
	} else {
		err = f.d.QueryRow(`INSERT INTO carts (shop_id, session_key) VALUES ($1, $2) RETURNING id`, shopID, hash).Scan(&cartID)
	}
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `WITH p AS (
		  INSERT INTO products (shop_id, vendor_id, slug, name)
		  SELECT $1, v.id, $2 || g, jsonb_build_object('bn', 'পণ্য ' || $2 || g, 'en', 'Item ' || $2 || g)
		  FROM generate_series(1, $3::int) g, vendors v WHERE v.shop_id = $1 AND v.is_house
		  RETURNING id, vendor_id, slug),
		v AS (
		  INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock)
		  SELECT $1, id, vendor_id, slug, 100, 10 FROM p RETURNING id, sku)
		INSERT INTO cart_items (shop_id, cart_id, variant_id, quantity, added_at)
		SELECT $1, $4, id, 1, now() + (substr(sku, length($2) + 1)::int * interval '1 second') FROM v`,
		shopID, prefix, n, cartID)
	return key
}

func (f *otpFixture) verifyKey(t *testing.T, host, phone, code, key string) *httptest.ResponseRecorder {
	t.Helper()
	r := newReq(http.MethodPost, "/api/auth/otp/verify", host, "", map[string]string{"phone": phone, "code": code})
	r.Header.Set("X-Cart-Key", key)
	return f.serve(r)
}

// TestVerifyMergesCartIntegration (8.2 step 6, 8.4 item 6, 7.3 v1.30): verify er transaction e X-Cart-Key er
// guest cart account e merge; bhul code e merge na; response e cart_notices (khali hole []); 50 variant er
// beshi e item_dropped; onno shop er key chhoy na.
func TestVerifyMergesCartIntegration(t *testing.T) {
	f := newOTPFixture(t)
	a, b := f.shops["a.com"], f.shops["b.com"]
	key := f.guestCart(t, a, "g", 2, 0)
	keyB := f.guestCart(t, b, "x", 1, 0)
	guestItems := func(k string) int {
		sum := sha256.Sum256([]byte(k))
		return f.count(t, `SELECT count(*) FROM cart_items ci JOIN carts c ON c.id = ci.cart_id WHERE c.session_key = $1`, sum[:])
	}
	accountItems := func(phone string) int {
		return f.count(t, `SELECT count(*) FROM cart_items ci JOIN carts c ON c.id = ci.cart_id
			JOIN users u ON u.shop_id = c.shop_id AND u.id = c.user_id WHERE u.phone = $1`, phone)
	}
	type resp struct {
		Token       string `json:"token"`
		CartNotices []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"cart_notices"`
	}

	f.otpRow(t, a, "+8801711000001", "123456", "5 minutes")
	wantErr(t, f.verifyKey(t, "a.com", "01711000001", "000000", key), 400, "otp_invalid", "bhul code")
	if guestItems(key) != 2 || f.count(t, `SELECT count(*) FROM carts WHERE user_id IS NOT NULL`) != 0 {
		t.Fatal("bhul code e merge holo")
	}
	rec := f.verifyKey(t, "a.com", "01711000001", "123456", key)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"cart_notices":[]`) {
		t.Fatalf("verify: %d %s (cart_notices [] chai)", rec.Code, rec.Body.String())
	}
	if guestItems(key) != 0 || f.count(t, `SELECT count(*) FROM carts WHERE session_key IS NOT NULL AND shop_id = $1`, a) != 0 ||
		accountItems("+8801711000001") != 2 {
		t.Errorf("merge: guest %d, account %d", guestItems(key), accountItems("+8801711000001"))
	}

	// key chhara verify: cart_notices [] o
	f.otpRow(t, a, "+8801711000002", "123456", "5 minutes")
	if rec := f.verify(t, "a.com", "01711000002", "123456"); !strings.Contains(rec.Body.String(), `"cart_notices":[]`) {
		t.Errorf("key chhara: %s", rec.Body.String())
	}

	// onno shop er key: chhoy na
	f.otpRow(t, a, "+8801711000003", "123456", "5 minutes")
	if rec := f.verifyKey(t, "a.com", "01711000003", "123456", keyB); rec.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	if guestItems(keyB) != 1 || accountItems("+8801711000003") != 0 {
		t.Error("shop b er guest cart merge holo")
	}

	// 50 er limit: account e 50, guest e notun 1 → item_dropped (naam request er bhashay, bn)
	var u int64
	if err := f.d.QueryRow(`INSERT INTO users (shop_id, phone) VALUES ($1, '+8801711000004') RETURNING id`, a).Scan(&u); err != nil {
		t.Fatal(err)
	}
	f.guestCart(t, a, "acc", 50, u)
	key4 := f.guestCart(t, a, "new", 1, 0)
	f.otpRow(t, a, "+8801711000004", "123456", "5 minutes")
	rec = f.verifyKey(t, "a.com", "01711000004", "123456", key4)
	var got resp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK || got.Token == "" {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	if len(got.CartNotices) != 1 || got.CartNotices[0].Code != "item_dropped" || got.CartNotices[0].Name != "পণ্য new1" {
		t.Errorf("cart_notices %+v, chai item_dropped পণ্য new1", got.CartNotices)
	}
	if accountItems("+8801711000004") != 50 || guestItems(key4) != 0 {
		t.Errorf("limit: account %d, guest %d", accountItems("+8801711000004"), guestItems(key4))
	}
}

// TestVerifyMergeFailRollsBackIntegration (8.2 step 6, 8.4 item 6): merge cart bodlanor por fail korle verify er
// puro transaction rollback: guest cart, OTP (abar use kora jay), user ar session kichui bodlay na.
func TestVerifyMergeFailRollsBackIntegration(t *testing.T) {
	f := newOTPFixtureMerge(t, func(ctx context.Context, tx *sql.Tx, shopID, userID int64, key, locale string) (any, error) {
		if _, err := cart.Merge(ctx, tx, shopID, userID, key, locale); err != nil {
			return nil, err
		}
		return nil, errors.New("merge er por bhul")
	})
	a := f.shops["a.com"]
	key := f.guestCart(t, a, "g", 2, 0)
	f.otpRow(t, a, "+8801711000001", "123456", "5 minutes")
	if rec := f.verifyKey(t, "a.com", "01711000001", "123456", key); rec.Code != http.StatusInternalServerError {
		t.Fatalf("verify: %d %s, chai 500", rec.Code, rec.Body.String())
	}
	sum := sha256.Sum256([]byte(key))
	if n := f.count(t, `SELECT count(*) FROM cart_items ci JOIN carts c ON c.id = ci.cart_id WHERE c.session_key = $1`, sum[:]); n != 2 {
		t.Errorf("guest cart e %d item, chai 2 (rollback)", n)
	}
	for q, want := range map[string]int{
		`SELECT count(*) FROM carts WHERE user_id IS NOT NULL`:      0,
		`SELECT count(*) FROM users WHERE phone = '+8801711000001'`: 0,
		`SELECT count(*) FROM sessions`:                             0,
		`SELECT count(*) FROM otp_codes WHERE consumed_at IS NULL`:  1,
	} {
		if n := f.count(t, q); n != want {
			t.Errorf("%s: %d, chai %d", q, n, want)
		}
	}
}
