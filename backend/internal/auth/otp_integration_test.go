package auth_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"vendrix/backend/internal/auth"
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
	h := auth.NewHandlers(d, log, otpSecret, auth.Turnstile{URL: fakeSiteverify(t).URL, Secret: "ts-secret"})
	mux := http.NewServeMux()
	mux.Handle("POST /api/auth/otp/send", httpx.ShopFromHost(log, shop.NewResolver(d).Resolve)(http.HandlerFunc(h.SendOTP)))
	f.srv = httpx.ClientIP("")(httpx.JSONErrors(mux))
	return f
}

func (f *otpFixture) send(t *testing.T, host, phone, token string) *httptest.ResponseRecorder {
	t.Helper()
	return f.post(t, host, map[string]any{"phone": phone, "purpose": "login", "turnstile_token": token})
}

func (f *otpFixture) post(t *testing.T, host string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/api/auth/otp/send", bytes.NewReader(b))
	r.Header.Set("X-Shop-Host", host)
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r)
	return rec
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
	if want := "alert.otp_lock:" + strconv.FormatInt(a, 10) + ":+8801711000001:" + day; key != want {
		t.Errorf("dedupe_key %q, chai %q", key, want)
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
	h := auth.NewHandlers(nil, slog.New(slog.DiscardHandler), otpSecret, ts)
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
	phone, code, err := src(ctx, a, otpID, p.Nonce, p.CT)
	if err != nil || phone != "+8801711000001" || !auth.CheckOTP(otpSecret, phone, "login", code, hash) {
		t.Fatalf("decrypt: %q %q %v", phone, code, err)
	}
	if strings.Contains(string(raw), code) || strings.Contains(string(raw), "1711000001") {
		t.Errorf("payload e plain code ba phone: %s", raw)
	}
	if _, _, err := src(ctx, f.shops["b.com"], otpID, p.Nonce, p.CT); err == nil {
		t.Error("onno shop e decrypt holo")
	}
	f.exec(t, `INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, expires_at) VALUES ($1, '+8801711000001', 'login', '\x00', now())`, a)
	if _, _, err := src(ctx, a, otpID+1, p.Nonce, p.CT); err == nil {
		t.Error("onno otp_codes.id (AAD) e decrypt holo")
	}
	if _, _, err := auth.SMSCode(f.d, "onno-secret")(ctx, a, otpID, p.Nonce, p.CT); err == nil {
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
	if len(got) != 1 || !strings.HasPrefix(got[0], strconv.FormatInt(a, 10)+"|+8801711000001|") || !strings.Contains(got[0], code) {
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
