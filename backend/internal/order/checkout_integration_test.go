package order_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	mrand "math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/coupon"
	customeradmin "vendrix/backend/internal/customer"
	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/order"
	"vendrix/backend/internal/pricing"
	"vendrix/backend/internal/shipping"
	"vendrix/backend/internal/shop"
)

// checkoutFx: ek shop "a" (prefix SA, otp_mode never), zone, login customer; shop "b" isolation er jonno.
type checkoutFx struct {
	t         *testing.T
	d         *sql.DB
	ctx       context.Context
	srv       http.Handler
	shop, vnd int64
	user      int64
	tok       string // login customer er Bearer
	sv        *siteverify
	otpIP     *httpx.Limit // login ar checkout er OTP send er ekoi IP limit (router er moto)
	logs      *logHook     // handler er log (alert.batch_short)
}

const (
	otpSecret     = "test-otp-secret-0123456789abcdef"
	confirmSecret = "test-confirm-secret-0123456789ab"
	printSecret   = "test-print-secret-0123456789abcd"
	tsSecret      = "ts-secret"
)

// siteverify: Cloudflare er moto fake; token "tok" ba "tok:<x>" → success, hostname a.com; "b:<x>" → success
// kintu hostname b.com (onno site er token); baki fail. calls gone (otp_code e Turnstile dake na, 14.2 test 23).
type siteverify struct {
	calls atomic.Int64
	last  atomic.Value // shesh form (url.Values)
}

func newSiteverify(t *testing.T) (*siteverify, string) {
	sv := &siteverify{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sv.calls.Add(1)
		if err := r.ParseForm(); err != nil || r.PostForm.Get("secret") != tsSecret {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		sv.last.Store(r.PostForm)
		tok := r.PostForm.Get("response")
		host, ok := "a.com", tok == "tok" || strings.HasPrefix(tok, "tok:")
		if strings.HasPrefix(tok, "b:") {
			host, ok = "b.com", true
		}
		json.NewEncoder(w).Encode(map[string]any{"success": ok, "hostname": host})
	}))
	t.Cleanup(srv.Close)
	return sv, srv.URL
}

const (
	userPhone = "+8801711000001"
	areaLine  = "Road 10, House 5"
)

func newCheckout(t *testing.T) *checkoutFx {
	t.Helper()
	f := &checkoutFx{t: t, d: dbtest.New(t), ctx: context.Background()}
	for _, s := range []string{"a", "b"} {
		id, _, err := shop.Create(f.ctx, f.d, shop.Params{Slug: s, Prefix: "S" + strings.ToUpper(s), Domain: s + ".com",
			NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		if s == "a" {
			f.shop = id
		}
	}
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,otp_mode}', '"never"') WHERE shop_id = $1`, f.shop)
	f.vnd = f.id(`SELECT id FROM vendors WHERE shop_id = $1 AND is_house`, f.shop)
	for _, z := range []struct {
		district string
		area     any
		charge   int64
		min      int64
		free     any
		same     bool
	}{
		{"Dhaka", "Dhanmondi", 6000, 30000, int64(200000), true},
		{"Dhaka", nil, 10000, 0, nil, true},
		{"Chattogram", nil, 15000, 0, nil, false},
	} {
		f.id(`INSERT INTO delivery_zones (shop_id, name, district, area, charge, min_order, free_delivery_over, same_city)
			VALUES ($1, '{"bn":"এলাকা"}', $2, $3, $4, $5, $6, $7) RETURNING id`, f.shop, z.district, z.area, z.charge, z.min, z.free, z.same)
	}
	f.user = f.id(`INSERT INTO users (shop_id, phone) VALUES ($1, $2) RETURNING id`, f.shop, userPhone)
	f.tok = rand.Text()
	sum := sha256.Sum256([]byte(f.tok))
	f.id(`INSERT INTO sessions (shop_id, token_hash, subject_type, user_id, expires_at)
		VALUES ($1, $2, 'customer', $3, now() + interval '1 day') RETURNING id`, f.shop, sum[:], f.user)

	f.logs = &logHook{}
	log := slog.New(f.logs)
	public := httpx.ShopFromHost(log, shop.NewResolver(f.d).Resolve)
	opt := httpx.OptionalCustomer(log, auth.LookupSession(f.d))
	mux := http.NewServeMux()
	var svURL string
	f.sv, svURL = newSiteverify(t)
	ts := auth.Turnstile{URL: svURL, Secret: tsSecret}
	f.otpIP = httpx.NewLimit(10, time.Hour)
	oh := order.NewHandlers(f.d, log, order.Guard{Turnstile: ts, OTPSecret: otpSecret, ConfirmSecret: confirmSecret, PrintSecret: printSecret, OTPIP: f.otpIP})
	oh.Routes(mux, func(h http.Handler) http.Handler { return public(opt(h)) })
	login := httpx.RequireCustomer(log, auth.LookupSession(f.d))
	oh.MeRoutes(mux, func(h http.Handler) http.Handler { return public(login(h)) })
	// admin (M7a): staff context test e request e (httpx.WithStaff); RequireRole handler e
	oh.AdminRoutes(mux, func(h http.Handler) http.Handler { return h })
	// zone, coupon, customer admin (TestAdminShopIsolationIntegration)
	noMW := func(h http.Handler) http.Handler { return h }
	shipping.NewHandlers(f.d, log).AdminRoutes(mux, noMW)
	coupon.NewHandlers(f.d, log).AdminRoutes(mux, noMW)
	customeradmin.NewHandlers(f.d, log).AdminRoutes(mux, noMW)
	// login (14.2 test 11: guest order → OTP login → /api/me/orders)
	ah := auth.NewHandlers(f.d, log, otpSecret, ts, func(ctx context.Context, tx *sql.Tx, shopID, userID int64, key, locale string) (any, error) {
		return cart.Merge(ctx, tx, shopID, userID, key, locale)
	})
	mux.Handle("POST /api/auth/otp/verify", public(http.HandlerFunc(ah.VerifyOTP)))
	mux.Handle("POST /api/auth/otp/send", public(f.otpIP.Middleware(http.HandlerFunc(ah.SendOTP))))
	f.srv = httpx.JSONErrors(mux)
	return f
}

func (f *checkoutFx) id(query string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.d.QueryRowContext(f.ctx, query, args...).Scan(&id); err != nil {
		f.t.Fatalf("%s: %v", query, err)
	}
	return id
}

func (f *checkoutFx) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.d.ExecContext(f.ctx, query, args...); err != nil {
		f.t.Fatalf("%s: %v", query, err)
	}
}

// variant: fixed variant, kena dam = price/2, option "size".
func (f *checkoutFx) variant(sku string, price int64, stock int) int64 {
	f.t.Helper()
	p := f.id(`INSERT INTO products (shop_id, vendor_id, slug, name)
		VALUES ($1, $2, $3, jsonb_build_object('bn', 'পণ্য ' || $3, 'en', 'Item ' || $3)) RETURNING id`, f.shop, f.vnd, sku)
	return f.id(`INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, cost_price, stock, options)
		VALUES ($1, $2, $3, $4, $5, $6, $7, '{"size":{"bn":"১ কেজি","en":"1 kg"}}') RETURNING id`, f.shop, p, f.vnd, sku, price, price/2, stock)
}

// weighted: ojon-variable variant (price = 1 kg er dam).
func (f *checkoutFx) weighted(sku string, perKg int64, nominalG, stock int) int64 {
	f.t.Helper()
	p := f.id(`INSERT INTO products (shop_id, vendor_id, slug, name, variable_weight, unit) VALUES ($1, $2, $3, '{"bn":"মাছ"}', true, 'kg') RETURNING id`,
		f.shop, f.vnd, sku)
	return f.id(`INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock, nominal_weight_g, options)
		VALUES ($1, $2, $3, $4, $5, $6, $7, '{}') RETURNING id`, f.shop, p, f.vnd, sku, perKg, stock, nominalG)
}

// guestCart: notun guest cart (item gulo dewa kram e add), X-Cart-Key ferot.
func (f *checkoutFx) guestCart(items ...[2]int64) string {
	f.t.Helper()
	key, hash := cart.NewKey()
	f.fill(f.id(`INSERT INTO carts (shop_id, session_key) VALUES ($1, $2) RETURNING id`, f.shop, hash), items...)
	return key
}

func (f *checkoutFx) fill(cartID int64, items ...[2]int64) {
	f.t.Helper()
	for i, it := range items {
		f.exec(`INSERT INTO cart_items (shop_id, cart_id, variant_id, quantity, added_at)
			VALUES ($1, $2, $3, $4, now() + make_interval(secs => $5))`, f.shop, cartID, it[0], it[1], i)
	}
}

// userCart: login customer er cart notun kore bhore.
func (f *checkoutFx) userCart(items ...[2]int64) {
	f.t.Helper()
	f.exec(`DELETE FROM carts WHERE shop_id = $1 AND user_id = $2`, f.shop, f.user)
	f.fill(f.id(`INSERT INTO carts (shop_id, user_id) VALUES ($1, $2) RETURNING id`, f.shop, f.user), items...)
}

func (f *checkoutFx) post(path, idemKey, cartKey, bearer string, body any, hdr ...string) *httptest.ResponseRecorder {
	f.t.Helper()
	var b bytes.Buffer
	if s, ok := body.(string); ok {
		b.WriteString(s)
	} else if err := json.NewEncoder(&b).Encode(body); err != nil {
		f.t.Fatal(err)
	}
	r := httptest.NewRequest("POST", path, &b)
	r.Header.Set("X-Shop-Host", "a.com")
	if idemKey != "" {
		r.Header.Set("Idempotency-Key", idemKey)
	}
	if cartKey != "" {
		r.Header.Set("X-Cart-Key", cartKey)
	}
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r)
	return rec
}

func (f *checkoutFx) place(idemKey, cartKey string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.post("/api/orders", idemKey, cartKey, "", body)
}

// quoteTotal: ekoi cart er quote (customer je total dekhe = expected_total).
func (f *checkoutFx) quoteTotal(cartKey, bearer, coupon, district, area, phone string) int64 {
	f.t.Helper()
	rec := f.post("/api/checkout/quote", "", cartKey, bearer, map[string]string{"coupon_code": coupon, "district": district,
		"area": area, "payment_method": "cod", "phone": phone})
	var q quoteJSON
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &q) != nil {
		f.t.Fatalf("quote: %d %s", rec.Code, rec.Body.String())
	}
	return q.Total
}

func (f *checkoutFx) count(query string, args ...any) int64 {
	f.t.Helper()
	return f.id(query, args...)
}

func idemKey() string { return rand.Text() } // 26 okkhor base32

// orderBody: 7.5 er body; recipient nil = customer er copy.
func orderBody(phone string, total int64) map[string]any {
	return map[string]any{
		"customer":        map[string]any{"name": "Rahim", "phone": phone, "email": nil},
		"recipient":       nil,
		"address":         map[string]any{"district": "Dhaka", "area": "Dhanmondi", "line": areaLine},
		"note":            "",
		"coupon_code":     "",
		"payment_method":  "cod",
		"expected_total":  total,
		"consent":         true,
		"otp_code":        nil,
		"turnstile_token": "tok",
		"website":         "",
	}
}

type placed struct {
	OrderNumber string  `json:"order_number"`
	PublicToken string  `json:"public_token"`
	Status      string  `json:"status"`
	PaymentURL  *string `json:"payment_url"`
}

func decodePlaced(t *testing.T, rec *httptest.ResponseRecorder, status int) placed {
	t.Helper()
	var p placed
	var raw map[string]any
	if rec.Code != status || json.Unmarshal(rec.Body.Bytes(), &p) != nil || json.Unmarshal(rec.Body.Bytes(), &raw) != nil {
		t.Fatalf("chai %d, pelam %d %s", status, rec.Code, rec.Body.String())
	}
	if v, ok := raw["payment_url"]; !ok || v != nil || p.OrderNumber == "" || p.PublicToken == "" {
		t.Errorf("response %s: payment_url null ar number/token chai", rec.Body.String())
	}
	return p
}

func errOf(rec *httptest.ResponseRecorder) httpx.ErrorBody {
	var e struct{ Error httpx.ErrorBody }
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	return e.Error
}

func wantErr(t *testing.T, name string, rec *httptest.ResponseRecorder, status int, code string) httpx.ErrorBody {
	t.Helper()
	e := errOf(rec)
	if rec.Code != status || e.Code != code {
		t.Errorf("%s: chai %d %s, pelam %d %s", name, status, code, rec.Code, rec.Body.String())
	}
	return e
}

// parallel: n ta fn ek shathe shuru (barrier), result index e.
func parallel[T any](n int, fn func(i int) T) []T {
	out := make([]T, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			out[i] = fn(i)
		}()
	}
	close(start)
	wg.Wait()
	return out
}

func phoneN(i int) string { return fmt.Sprintf("+88018%08d", 10000000+i) }

// TestCheckoutIdempotencyIntegration (14.2 test 4, 8.6 step 1, 2, 9, 15; 8.19 item 12 i–iii).
func TestCheckoutIdempotencyIntegration(t *testing.T) {
	f := newCheckout(t)
	last := f.variant("last", 50000, 1) // shesh stock
	ck := f.guestCart([2]int64{last, 1})
	body := orderBody(phoneN(1), 56000)
	key := idemKey()

	// ekoi key duibar parallel: ek order, ek bar stock; duplicate 200 ager order (409 na)
	recs := parallel(2, func(int) *httptest.ResponseRecorder { return f.place(key, ck, body) })
	codes := []int{recs[0].Code, recs[1].Code}
	first, dup := recs[0], recs[1]
	if first.Code != http.StatusCreated {
		first, dup = dup, first
	}
	p1 := decodePlaced(t, first, http.StatusCreated)
	p2 := decodePlaced(t, dup, http.StatusOK)
	if p1 != p2 {
		t.Errorf("duplicate alada order: %+v vs %+v (codes %v)", p1, p2, codes)
	}
	if n := f.count(`SELECT count(*) FROM orders`); n != 1 {
		t.Errorf("order %d, chai 1", n)
	}
	if s := f.count(`SELECT stock FROM product_variants WHERE id = $1`, last); s != 0 {
		t.Errorf("stock %d, chai 0", s)
	}
	if n := f.count(`SELECT count(*) FROM inventory_movements WHERE variant_id = $1`, last); n != 1 {
		t.Errorf("movement %d, chai 1", n)
	}
	// commit er por cart khali, response harano retry (ekoi key, ekoi body) → 200 ager order
	p3 := decodePlaced(t, f.place(key, ck, body), http.StatusOK)
	if p3 != p1 {
		t.Errorf("retry: %+v, chai %+v", p3, p1)
	}
	// Turnstile/otp_code/website hash e na: retry te notun Turnstile token o 200
	retry := orderBody(phoneN(1), 56000)
	retry["turnstile_token"] = "notun"
	decodePlaced(t, f.place(key, ck, retry), http.StatusOK)
	// onno guest (onno cart) ekoi key + body → 200 ager response (8.19 item 12 iii)
	other := f.guestCart([2]int64{f.variant("other", 1000, 5), 1})
	if p := decodePlaced(t, f.place(key, other, body), http.StatusOK); p != p1 {
		t.Errorf("onno guest: %+v", p)
	}
	// alada body → 422
	changed := orderBody(phoneN(1), 56000)
	changed["note"] = "bikel e"
	wantErr(t, "alada body", f.place(key, ck, changed), http.StatusUnprocessableEntity, "idempotency_key_reused")
	// key nai ba bhul format → 400 missing_idempotency_key
	for name, k := range map[string]string{"key nai": "", "choto": "abc123", "bhul okkhor": "abcdefghijklmnop+", "65 okkhor": strings.Repeat("a", 65)} {
		wantErr(t, name, f.place(k, other, orderBody(phoneN(2), 7000)), http.StatusBadRequest, "missing_idempotency_key")
	}
	// expected_total nai → 400 validation_failed
	noTotal := orderBody(phoneN(2), 0)
	delete(noTotal, "expected_total")
	wantErr(t, "expected_total nai", f.place(idemKey(), other, noTotal), http.StatusBadRequest, "validation_failed")
	if n := f.count(`SELECT count(*) FROM orders`); n != 1 {
		t.Errorf("shesh e order %d, chai 1", n)
	}
}

// TestCheckoutStockRaceIntegration (14.2 test 1): stock 1, ek shathe 10 checkout → thik 1 order.
func TestCheckoutStockRaceIntegration(t *testing.T) {
	f := newCheckout(t)
	v := f.variant("one", 50000, 1)
	keys := make([]string, 10)
	for i := range keys {
		keys[i] = f.guestCart([2]int64{v, 1})
	}
	recs := parallel(10, func(i int) *httptest.ResponseRecorder {
		return f.place(idemKey(), keys[i], orderBody(phoneN(i), 56000))
	})
	ok := 0
	for i, rec := range recs {
		if rec.Code == http.StatusCreated {
			ok++
			continue
		}
		e := wantErr(t, fmt.Sprint("checkout ", i), rec, http.StatusConflict, "out_of_stock")
		if fmt.Sprint(e.Details["variant_id"]) != fmt.Sprint(v) {
			t.Errorf("checkout %d: details %v, variant_id %d chai", i, e.Details, v)
		}
	}
	if ok != 1 {
		t.Errorf("%d ta order, chai 1", ok)
	}
	if s := f.count(`SELECT stock FROM product_variants WHERE id = $1`, v); s != 0 {
		t.Errorf("stock %d, chai 0", s)
	}
	if n := f.count(`SELECT count(*) FROM orders`); n != 1 {
		t.Errorf("DB te order %d", n)
	}
}

// TestCheckoutLockOrderIntegration (14.2 test 1, 8.6 step 10): ulto kram e add kora duita variant er 20
// checkout ek shathe: deadlock er 500 nai, shob order, stock thik.
func TestCheckoutLockOrderIntegration(t *testing.T) {
	f := newCheckout(t)
	a, b := f.variant("a", 20000, 100), f.variant("b", 20000, 100)
	keys := make([]string, 20)
	for i := range keys {
		if i%2 == 0 {
			keys[i] = f.guestCart([2]int64{a, 1}, [2]int64{b, 1})
		} else {
			keys[i] = f.guestCart([2]int64{b, 1}, [2]int64{a, 1})
		}
	}
	recs := parallel(20, func(i int) *httptest.ResponseRecorder {
		return f.place(idemKey(), keys[i], orderBody(phoneN(i), 46000))
	})
	for i, rec := range recs {
		if rec.Code != http.StatusCreated {
			t.Errorf("checkout %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if sa, sb := f.count(`SELECT stock FROM product_variants WHERE id = $1`, a), f.count(`SELECT stock FROM product_variants WHERE id = $1`, b); sa != 80 || sb != 80 {
		t.Errorf("stock %d %d, chai 80 80", sa, sb)
	}
}

// TestCheckoutCouponLimitRaceIntegration (14.2 test 2): usage_limit 1 e 5 parallel → 1 ta discount.
func TestCheckoutCouponLimitRaceIntegration(t *testing.T) {
	f := newCheckout(t)
	v := f.variant("rice", 50000, 100)
	f.id(`INSERT INTO coupons (shop_id, code, kind, value, usage_limit) VALUES ($1, 'ONCE', 'fixed', 5000, 1) RETURNING id`, f.shop)
	keys := make([]string, 5)
	for i := range keys {
		keys[i] = f.guestCart([2]int64{v, 2})
	}
	recs := parallel(5, func(i int) *httptest.ResponseRecorder {
		b := orderBody(phoneN(i), 101000) // 100000 - 5000 + 6000
		b["coupon_code"] = "once"
		return f.place(idemKey(), keys[i], b)
	})
	ok := 0
	for i, rec := range recs {
		if rec.Code == http.StatusCreated {
			ok++
			continue
		}
		wantErr(t, fmt.Sprint("checkout ", i), rec, http.StatusConflict, "coupon_limit_reached")
	}
	if ok != 1 {
		t.Errorf("%d ta discount order, chai 1", ok)
	}
	if u, r := f.count(`SELECT used_count FROM coupons WHERE code = 'ONCE'`), f.count(`SELECT count(*) FROM coupon_redemptions`); u != 1 || r != 1 {
		t.Errorf("used_count %d redemption %d, chai 1 1", u, r)
	}
	if s := f.count(`SELECT stock FROM product_variants WHERE id = $1`, v); s != 98 {
		t.Errorf("stock %d, chai 98 (fail e ROLLBACK)", s)
	}
}

// TestCheckoutCouponPhoneLimitIntegration (14.2 test 3): ek phone e per_phone_limit er beshi na; ROLLBACK e
// used_count ferot.
func TestCheckoutCouponPhoneLimitIntegration(t *testing.T) {
	f := newCheckout(t)
	v := f.variant("rice", 50000, 100)
	f.id(`INSERT INTO coupons (shop_id, code, kind, value, per_phone_limit) VALUES ($1, 'PHONE', 'percent', 10, 1) RETURNING id`, f.shop)
	const same = "+8801911000009"
	keys := make([]string, 4)
	for i := range keys {
		keys[i] = f.guestCart([2]int64{v, 2})
	}
	body := func() map[string]any {
		b := orderBody(same, 96000) // 100000 - 10000 + 6000
		b["coupon_code"] = "PHONE"
		return b
	}
	recs := parallel(3, func(i int) *httptest.ResponseRecorder { return f.place(idemKey(), keys[i], body()) })
	ok := 0
	for i, rec := range recs {
		if rec.Code == http.StatusCreated {
			ok++
			continue
		}
		wantErr(t, fmt.Sprint("checkout ", i), rec, http.StatusConflict, "coupon_limit_reached")
	}
	if ok != 1 {
		t.Errorf("ek phone e %d ta, chai 1", ok)
	}
	if u := f.count(`SELECT used_count FROM coupons WHERE code = 'PHONE'`); u != 1 {
		t.Errorf("used_count %d, chai 1 (ROLLBACK e ferot)", u)
	}
	wantErr(t, "porer bar ekoi phone", f.place(idemKey(), keys[3], body()), http.StatusConflict, "coupon_limit_reached")
	b := body()
	b["customer"] = map[string]any{"name": "Karim", "phone": phoneN(7)}
	decodePlaced(t, f.place(idemKey(), keys[3], b), http.StatusCreated)
	if u, r := f.count(`SELECT used_count FROM coupons WHERE code = 'PHONE'`), f.count(`SELECT count(*) FROM coupon_redemptions`); u != 2 || r != 2 {
		t.Errorf("used_count %d redemption %d, chai 2 2", u, r)
	}
}

// TestCheckoutCouponNotApplicableIntegration (14.2 test 39, 8.5): min_order er kom ba eligible 0 →
// 409 coupon_invalid, used_count/redemption bodlay na.
func TestCheckoutCouponNotApplicableIntegration(t *testing.T) {
	f := newCheckout(t)
	v := f.variant("rice", 50000, 100)
	other := f.id(`INSERT INTO vendors (shop_id, slug, name) VALUES ($1, 'v2', '{"bn":"বিক্রেতা"}') RETURNING id`, f.shop)
	f.id(`INSERT INTO coupons (shop_id, code, kind, value, min_order) VALUES ($1, 'BIG', 'fixed', 5000, 500000) RETURNING id`, f.shop)
	f.id(`INSERT INTO coupons (shop_id, code, kind, value, vendor_id, funded_by) VALUES ($1, 'V2', 'percent', 10, $2, 'vendor') RETURNING id`,
		f.shop, other)
	ck := f.guestCart([2]int64{v, 2})
	for _, code := range []string{"BIG", "V2"} {
		b := orderBody(phoneN(1), 106000)
		b["coupon_code"] = code
		wantErr(t, code, f.place(idemKey(), ck, b), http.StatusConflict, "coupon_invalid")
	}
	if u, r, o := f.count(`SELECT sum(used_count) FROM coupons`), f.count(`SELECT count(*) FROM coupon_redemptions`),
		f.count(`SELECT count(*) FROM orders`); u != 0 || r != 0 || o != 0 {
		t.Errorf("used_count %d redemption %d order %d, shob 0 chai", u, r, o)
	}
	if s := f.count(`SELECT stock FROM product_variants WHERE id = $1`, v); s != 100 {
		t.Errorf("stock %d, chai 100", s)
	}
}

// TestCheckoutTotalsSavedIntegration (14.2 test 15): random cart/coupon/zone e checkout er por DB te
// sum(discount_share) = discount, line er jog = subtotal, fulfillment er jog = order er taka, total = quote.
func TestCheckoutTotalsSavedIntegration(t *testing.T) {
	f := newCheckout(t)
	rng := mrand.New(mrand.NewPCG(15, 39))
	var vs []int64
	for i := range 6 {
		vs = append(vs, f.variant(fmt.Sprint("f", i), int64(rng.IntN(90000)+1), 1000))
	}
	vs = append(vs, f.weighted("fish", 80000, 750, 1000), f.weighted("meat", 99999, 333, 1000))
	f.id(`INSERT INTO coupons (shop_id, code, kind, value, max_discount) VALUES ($1, 'P', 'percent', 17, 7000) RETURNING id`, f.shop)
	f.id(`INSERT INTO coupons (shop_id, code, kind, value) VALUES ($1, 'F', 'fixed', 3333) RETURNING id`, f.shop)
	zones := [][2]string{{"Dhaka", "Dhanmondi"}, {"Dhaka", "Uttara"}, {"Chattogram", "Agrabad"}}
	for i := range 20 {
		var items [][2]int64
		for _, j := range rng.Perm(len(vs))[:rng.IntN(len(vs))+1] {
			items = append(items, [2]int64{vs[j], int64(rng.IntN(5) + 1)})
		}
		ck := f.guestCart(items...)
		coupon := []string{"", "P", "F"}[rng.IntN(3)]
		z := zones[rng.IntN(len(zones))]
		ph := phoneN(100 + i)
		total := f.quoteTotal(ck, "", coupon, z[0], z[1], ph)
		b := orderBody(ph, total)
		b["coupon_code"] = coupon
		b["address"] = map[string]any{"district": z[0], "area": z[1], "line": areaLine}
		rec := f.place(idemKey(), ck, b)
		if rec.Code == http.StatusConflict && errOf(rec).Code == "below_min_order" {
			continue
		}
		p := decodePlaced(t, rec, http.StatusCreated)
		var bad int64
		if err := f.d.QueryRowContext(f.ctx, `SELECT
			(SELECT count(*) FROM orders o WHERE o.public_token = $1 AND (o.total <> $2
			  OR o.discount <> (SELECT sum(discount_share) FROM order_items i WHERE i.order_id = o.id)
			  OR o.subtotal <> (SELECT sum(line_total) FROM order_items i WHERE i.order_id = o.id)
			  OR (o.subtotal, o.discount, o.delivery_charge, o.total) IS DISTINCT FROM
			     (SELECT (sum(subtotal)::bigint, sum(discount)::bigint, sum(delivery_charge)::bigint, sum(total)::bigint) FROM order_fulfillments x WHERE x.order_id = o.id)))`,
			p.PublicToken, total).Scan(&bad); err != nil {
			t.Fatal(err)
		}
		if bad != 0 {
			t.Errorf("order %d (%s): taka DB te mile na", i, p.OrderNumber)
		}
	}
}

// TestCouponSnapshotIntegration (14.2 test 33): order er por coupon edit/bondho → DB row theke LineFromItem +
// orders.coupon_* diye Reprice ager result.
func TestCouponSnapshotIntegration(t *testing.T) {
	f := newCheckout(t)
	fish := f.weighted("fish", 100000, 1000, 10)
	rice := f.variant("rice", 30000, 10)
	f.id(`INSERT INTO coupons (shop_id, code, kind, value, max_discount) VALUES ($1, 'SNAP', 'percent', 10, 20000) RETURNING id`, f.shop)
	ck := f.guestCart([2]int64{fish, 2}, [2]int64{rice, 1})
	total := f.quoteTotal(ck, "", "SNAP", "Dhaka", "Dhanmondi", phoneN(1))
	b := orderBody(phoneN(1), total)
	b["coupon_code"] = "SNAP"
	p := decodePlaced(t, f.place(idemKey(), ck, b), http.StatusCreated)
	f.exec(`UPDATE coupons SET value = 50, max_discount = NULL, is_active = false WHERE code = 'SNAP'`)

	var o pricing.Original
	var orderID int64
	var c pricing.Coupon
	if err := f.d.QueryRowContext(f.ctx, `SELECT id, subtotal, discount, delivery_charge, total, coupon_kind, coupon_value,
		COALESCE(coupon_max_discount, 0), COALESCE(coupon_vendor_id, 0) FROM orders WHERE public_token = $1`, p.PublicToken).
		Scan(&orderID, &o.Result.Subtotal, &o.Result.Discount, &o.Result.DeliveryCharge, &o.Result.Total,
			&c.Kind, &c.Value, &c.MaxDiscount, &c.VendorID); err != nil {
		t.Fatal(err)
	}
	o.Coupon = &c
	rows, err := f.d.QueryContext(f.ctx, `SELECT vendor_id, unit_price, quantity, variable_weight, COALESCE(ordered_weight_g, 0),
		COALESCE(packed_weight_g, 0), status = 'removed', line_total, discount_share FROM order_items WHERE order_id = $1 ORDER BY id`, orderID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var it pricing.Item
		if err := rows.Scan(&it.VendorID, &it.UnitPrice, &it.Quantity, &it.VariableWeight, &it.OrderedWeightG, &it.PackedWeightG,
			&it.Removed, &it.LineTotal, &it.DiscountShare); err != nil {
			t.Fatal(err)
		}
		l, lr := pricing.LineFromItem(it)
		o.Lines, o.Result.Lines = append(o.Lines, l), append(o.Result.Lines, lr)
	}
	rows.Close()
	var g pricing.Group
	if err := f.d.QueryRowContext(f.ctx, `SELECT vendor_id, subtotal, discount, delivery_charge, total FROM order_fulfillments WHERE order_id = $1`,
		orderID).Scan(&g.VendorID, &g.Subtotal, &g.Discount, &g.DeliveryCharge, &g.Total); err != nil {
		t.Fatal(err)
	}
	o.Result.Groups = []pricing.Group{g}
	if c.Kind != "percent" || c.Value != 10 || c.MaxDiscount != 20000 || o.Result.Discount == 0 {
		t.Fatalf("snapshot %+v discount %d", c, o.Result.Discount)
	}
	r, _, err := pricing.Reprice(o, o.Lines)
	if err != nil || r.Totals != o.Result.Totals {
		t.Errorf("Reprice: %+v err %v, chai %+v", r.Totals, err, o.Result.Totals)
	}
}

// TestCheckoutValidationIntegration (8.6 step 3–7, 12; 8.19 item 10): bhul e order na, kichu bodlay na.
func TestCheckoutValidationIntegration(t *testing.T) {
	f := newCheckout(t)
	rice, oil := f.variant("rice", 50000, 10), f.variant("oil", 20000, 10)
	off := f.variant("off", 10000, 10)
	f.exec(`UPDATE products SET is_active = false WHERE id = (SELECT product_id FROM product_variants WHERE id = $1)`, off)
	f.exec(`INSERT INTO blocked_phones (shop_id, phone) VALUES ($1, '+8801311000003')`, f.shop)
	ck := f.guestCart([2]int64{rice, 2}) // 100000 + 6000
	const total = 106000
	set := func(k string, v any) map[string]any {
		b := orderBody(phoneN(1), total)
		b[k] = v
		return b
	}
	cust := func(name, phone string, email any) map[string]any {
		return set("customer", map[string]any{"name": name, "phone": phone, "email": email})
	}
	addr := func(district, area, line string) map[string]any {
		return set("address", map[string]any{"district": district, "area": area, "line": line})
	}
	bad := map[string]map[string]any{
		"honeypot":           set("website", "http://spam"),
		"consent false":      set("consent", false),
		"naam khali":         cust("  ", phoneN(1), nil),
		"naam 101":           cust(strings.Repeat("ক", 101), phoneN(1), nil),
		"phone bhul":         cust("Rahim", "12345", nil),
		"email bhul":         cust("Rahim", phoneN(1), "not-an-email"),
		"recipient phone":    set("recipient", map[string]any{"name": "Karim", "phone": "999"}),
		"recipient naam":     set("recipient", map[string]any{"name": "", "phone": phoneN(2)}),
		"thikana 4":          addr("Dhaka", "Dhanmondi", "Road"),
		"thikana 501":        addr("Dhaka", "Dhanmondi", strings.Repeat("a", 501)),
		"district khali":     addr("", "Dhanmondi", areaLine),
		"note 501":           set("note", strings.Repeat("n", 501)),
		"coupon 33":          set("coupon_code", strings.Repeat("C", 33)),
		"payment bondho":     set("payment_method", "sslcommerz"),
		"client er dam":      set("total", 1),
		"client er subtotal": set("subtotal", 1),
	}
	noConsent := orderBody(phoneN(1), total)
	delete(noConsent, "consent")
	bad["consent nai"] = noConsent
	for name, b := range bad {
		wantErr(t, name, f.place(idemKey(), ck, b), http.StatusBadRequest, "validation_failed")
	}
	wantErr(t, "login e onno phone", f.post("/api/orders", idemKey(), "", f.tok, orderBody(phoneN(1), total)),
		http.StatusBadRequest, "validation_failed")
	wantErr(t, "area nai", f.place(idemKey(), ck, addr("Sylhet", "Zindabazar", areaLine)), http.StatusConflict, "area_not_served")
	wantErr(t, "blocked customer", f.place(idemKey(), ck, cust("Rahim", "01311000003", nil)), http.StatusForbidden, "phone_blocked")
	wantErr(t, "blocked recipient", f.place(idemKey(), ck, set("recipient", map[string]any{"name": "K", "phone": "+8801311000003"})),
		http.StatusForbidden, "phone_blocked")
	e := wantErr(t, "dam bodleche", f.place(idemKey(), ck, set("expected_total", 100000)), http.StatusConflict, "price_changed")
	want := map[string]any{"total": 106000, "subtotal": 100000, "discount": 0, "delivery_charge": 6000} // JSON number, Sprint e ekoi
	if fmt.Sprint(e.Details) != fmt.Sprint(want) {
		t.Errorf("price_changed details %v, chai %v", e.Details, want)
	}
	minCart := f.guestCart([2]int64{oil, 1}) // 20000 < min_order 30000
	wantErr(t, "below_min_order", f.place(idemKey(), minCart, orderBody(phoneN(1), 26000)), http.StatusConflict, "below_min_order")
	offCart := f.guestCart([2]int64{rice, 1}, [2]int64{off, 1})
	e = wantErr(t, "bondho product", f.place(idemKey(), offCart, orderBody(phoneN(1), 66000)), http.StatusConflict, "out_of_stock")
	if fmt.Sprint(e.Details["variant_id"]) != fmt.Sprint(off) {
		t.Errorf("out_of_stock details %v, variant_id %d chai", e.Details, off)
	}
	wantErr(t, "cart nai", f.place(idemKey(), "", orderBody(phoneN(1), total)), http.StatusBadRequest, "cart_empty")
	wantErr(t, "cart khali", f.place(idemKey(), f.guestCart(), orderBody(phoneN(1), total)), http.StatusBadRequest, "cart_empty")

	if o, s, m := f.count(`SELECT count(*) FROM orders`), f.count(`SELECT sum(stock) FROM product_variants`),
		f.count(`SELECT count(*) FROM inventory_movements`); o != 0 || s != 30 || m != 0 {
		t.Errorf("bhul er por order %d stock %d movement %d (0, 30, 0 chai)", o, s, m)
	}
	if n := f.count(`SELECT count(*) FROM cart_items`); n != 4 {
		t.Errorf("cart item %d, chai 4 (bhul e cart thake)", n)
	}
}

// TestCheckoutSnapshotIntegration (8.6 step 13–17, Rule 8): order er row, snapshot, number, deadline,
// status, history, movement, redemption, cart khali; login order e user_id ar otp_verified.
func TestCheckoutSnapshotIntegration(t *testing.T) {
	f := newCheckout(t)
	// risk flag na (new_phone_high_amount er shima upore), tai cod_confirmation risky e confirmed
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,risk,new_phone_amount_over}', '1000000') WHERE shop_id = $1`, f.shop)
	rice := f.variant("rice", 50000, 10)
	fish := f.weighted("fish", 80000, 750, 10)
	f.id(`INSERT INTO coupons (shop_id, code, kind, value, max_discount) VALUES ($1, 'EID', 'percent', 10, 5000) RETURNING id`, f.shop)
	ck := f.guestCart([2]int64{rice, 2}, [2]int64{fish, 2}) // 100000 + 120000 = 220000, discount 5000 → 215000 free delivery
	b := orderBody("01811000002", 215000)
	b["customer"] = map[string]any{"name": " Rahim ", "phone": "01811000002", "email": "r@example.com"}
	b["recipient"] = map[string]any{"name": "Karim", "phone": "01911000003"}
	b["note"] = "Bikel e"
	b["coupon_code"] = "eid"
	before := time.Now()
	rec := f.post("/api/orders", idemKey(), ck, "", b, "Accept-Language", "en-US,en;q=0.9")
	p := decodePlaced(t, rec, http.StatusCreated)
	if p.OrderNumber != "SA-100001" || p.Status != "confirmed" || len(p.PublicToken) != 26 {
		t.Errorf("response %+v", p)
	}
	var (
		cname, cphone, rname, rphone, district, area, line, note, locale, status, display, code, kind, pay, payStatus string
		email                                                                                                         sql.NullString
		sub, disc, del, tot, value, maxd                                                                              int64
		userID, zoneID                                                                                                sql.NullInt64
		flags                                                                                                         string
		otp                                                                                                           bool
		deadline                                                                                                      time.Time
		confirmHash                                                                                                   []byte
	)
	if err := f.d.QueryRowContext(f.ctx, `SELECT customer_name, customer_phone, customer_email, recipient_name, recipient_phone,
		ship_district, ship_area, ship_address, note, locale, status, display_status, coupon_code, coupon_kind, coupon_value,
		coupon_max_discount, payment_method, payment_status, subtotal, discount, delivery_charge, total, user_id, delivery_zone_id,
		risk_flags::text, otp_verified, delivery_deadline, confirm_token_hash
		FROM orders WHERE public_token = $1`, p.PublicToken).Scan(&cname, &cphone, &email, &rname, &rphone, &district, &area, &line,
		&note, &locale, &status, &display, &code, &kind, &value, &maxd, &pay, &payStatus, &sub, &disc, &del, &tot, &userID, &zoneID,
		&flags, &otp, &deadline, &confirmHash); err != nil {
		t.Fatal(err)
	}
	if cname != "Rahim" || cphone != "+8801811000002" || email.String != "r@example.com" || rname != "Karim" || rphone != "+8801911000003" ||
		district != "Dhaka" || area != "Dhanmondi" || line != areaLine || note != "Bikel e" || locale != "en" {
		t.Errorf("customer/thikana: %q %q %q %q %q %q %q %q %q %q", cname, cphone, email.String, rname, rphone, district, area, line, note, locale)
	}
	if status != "confirmed" || display != "confirmed" || pay != "cod" || payStatus != "unpaid" || userID.Valid || otp || !zoneID.Valid ||
		flags != "[]" || confirmHash != nil {
		t.Errorf("status %s/%s pay %s/%s user %v otp %v zone %v flags %s", status, display, pay, payStatus, userID, otp, zoneID, flags)
	}
	if code != "EID" || kind != "percent" || value != 10 || maxd != 5000 || sub != 220000 || disc != 5000 || del != 0 || tot != 215000 {
		t.Errorf("coupon %s %s %d %d taka %d %d %d %d", code, kind, value, maxd, sub, disc, del, tot)
	}
	if d := deadline.Sub(before); d < 5*24*time.Hour-time.Minute || d > 5*24*time.Hour+time.Minute {
		t.Errorf("same_city deadline %v, 5 din chai", d)
	}
	// item snapshot: naam, label, sku, dam, kena dam, ojon
	rows, err := f.d.QueryContext(f.ctx, `SELECT variant_id, product_name::text, variant_label::text, sku, unit_price, cost_price,
		quantity, variable_weight, ordered_weight_g, line_total, i.status, f.seq, f.status
		FROM order_items i JOIN order_fulfillments f ON f.id = i.fulfillment_id
		JOIN orders o ON o.id = i.order_id WHERE o.public_token = $1 ORDER BY variant_id`, p.PublicToken)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var vid, price, lt int64
		var cost, weight sql.NullInt64
		var name, label, sku, st, fst string
		var qty, seq int
		var vw bool
		if err := rows.Scan(&vid, &name, &label, &sku, &price, &cost, &qty, &vw, &weight, &lt, &st, &seq, &fst); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s|%s|%s|%d|%v|%d|%v|%v|%d|%s|%d|%s", name, label, sku, price, cost, qty, vw, weight, lt, st, seq, fst))
	}
	rows.Close()
	want := []string{
		`{"bn": "পণ্য rice", "en": "Item rice"}|{"bn": "১ কেজি", "en": "1 kg"}|rice|50000|{25000 true}|2|false|{0 false}|100000|active|1|open`,
		`{"bn": "মাছ"}|{}|fish|80000|{0 false}|2|true|{1500 true}|120000|active|1|open`,
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("item:\n got %q\nwant %q", got, want)
	}
	var fulfillments, history, moves, redemptions, cartLeft int64
	if err := f.d.QueryRowContext(f.ctx, `SELECT
		(SELECT count(*) FROM order_fulfillments x JOIN orders o ON o.id = x.order_id WHERE o.public_token = $1),
		(SELECT count(*) FROM order_status_history h JOIN orders o ON o.id = h.order_id
		  WHERE o.public_token = $1 AND h.from_status IS NULL AND h.to_status = 'confirmed' AND h.fulfillment_id IS NULL AND h.actor_type = 'customer'),
		(SELECT count(*) FROM inventory_movements m JOIN orders o ON o.id = m.order_id
		  WHERE o.public_token = $1 AND m.reason = 'order' AND m.change = -2),
		(SELECT count(*) FROM coupon_redemptions r JOIN orders o ON o.id = r.order_id AND o.coupon_id = r.coupon_id
		  WHERE o.public_token = $1 AND r.phone = '+8801811000002'),
		(SELECT count(*) FROM cart_items)`, p.PublicToken).Scan(&fulfillments, &history, &moves, &redemptions, &cartLeft); err != nil {
		t.Fatal(err)
	}
	if fulfillments != 1 || history != 1 || moves != 2 || redemptions != 1 || cartLeft != 0 {
		t.Errorf("fulfillment %d history %d movement %d redemption %d cart item %d (1 1 2 1 0 chai)", fulfillments, history, moves, redemptions, cartLeft)
	}
	if s := f.count(`SELECT stock FROM product_variants WHERE id = $1`, rice); s != 8 {
		t.Errorf("stock %d, chai 8", s)
	}

	// COD confirmation "all": pending; onno zone (same_city na) e 10 din; porer number
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,cod_confirmation}', '"all"') WHERE shop_id = $1`, f.shop)
	ck = f.guestCart([2]int64{rice, 1})
	b = orderBody(phoneN(5), 65000)
	b["address"] = map[string]any{"district": "Chattogram", "area": "Agrabad", "line": areaLine}
	p = decodePlaced(t, f.place(idemKey(), ck, b), http.StatusCreated)
	if p.OrderNumber != "SA-100002" || p.Status != "pending" {
		t.Errorf("pending order %+v", p)
	}
	if err := f.d.QueryRowContext(f.ctx, `SELECT display_status, delivery_deadline, locale FROM orders WHERE public_token = $1`,
		p.PublicToken).Scan(&display, &deadline, &locale); err != nil {
		t.Fatal(err)
	}
	if d := time.Until(deadline); display != "pending" || locale != "bn" || d < 10*24*time.Hour-2*time.Minute || d > 10*24*time.Hour+time.Minute {
		t.Errorf("display %s locale %s deadline %v", display, locale, d)
	}

	// login customer: user_id, otp_verified, account cart khali, guest cart e hat na
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,cod_confirmation}', '"risky"') WHERE shop_id = $1`, f.shop)
	f.userCart([2]int64{rice, 1})
	f.guestCart([2]int64{rice, 1})
	b = orderBody("01711000001", 56000)
	p = decodePlaced(t, f.post("/api/orders", idemKey(), "", f.tok, b), http.StatusCreated)
	if err := f.d.QueryRowContext(f.ctx, `SELECT user_id, otp_verified FROM orders WHERE public_token = $1`, p.PublicToken).Scan(&userID, &otp); err != nil {
		t.Fatal(err)
	}
	if userID.Int64 != f.user || !otp {
		t.Errorf("login order: user %v otp %v", userID, otp)
	}
	if n := f.count(`SELECT count(*) FROM cart_items ci JOIN carts c ON c.id = ci.cart_id WHERE c.user_id = $1`, f.user); n != 0 {
		t.Errorf("account cart e %d item", n)
	}
	if n := f.count(`SELECT count(*) FROM cart_items`); n != 1 {
		t.Errorf("guest cart o khali holo: %d item", n)
	}
}

// otpMode: published checkout.otp_mode bodlay.
func (f *checkoutFx) otpMode(mode string) {
	f.t.Helper()
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,otp_mode}', to_jsonb($2::text)) WHERE shop_id = $1`, f.shop, mode)
}

// otpCode: ei phone er shesh checkout OTP er code, sms.send job er payload decrypt kore (SMS je code pathabe).
func (f *checkoutFx) otpCode(phone string) string {
	f.t.Helper()
	var id int64
	var raw []byte
	if err := f.d.QueryRowContext(f.ctx, `SELECT o.id, j.payload FROM otp_codes o
		JOIN jobs j ON j.kind = 'sms.send' AND (j.payload->>'otp_id')::bigint = o.id
		WHERE o.shop_id = $1 AND o.phone = $2 AND o.purpose = 'checkout' ORDER BY o.id DESC LIMIT 1`, f.shop, phone).Scan(&id, &raw); err != nil {
		f.t.Fatalf("checkout OTP nai: %v", err)
	}
	var p struct {
		Nonce []byte `json:"nonce"`
		CT    []byte `json:"ct"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		f.t.Fatal(err)
	}
	o, err := auth.SMSCode(f.d, otpSecret)(f.ctx, f.shop, id, p.Nonce, p.CT)
	if err != nil {
		f.t.Fatal(err)
	}
	return o.Code
}

// withOTP: body te otp_code, Turnstile token chhara (OTP resubmit, 8.6 step 3).
func withOTP(b map[string]any, code string) map[string]any {
	b["otp_code"], b["turnstile_token"] = code, ""
	return b
}

func otherCode(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}

// TestCheckoutTurnstileIntegration (14.2 test 23, 8.6 step 2, 3, 8): otp_code na thakle Turnstile (hostname
// shoho); otp_code thakle Turnstile dake na kintu code shob shomoy verify (OTP na lagleo), bhul 400
// otp_invalid; retry (ekoi key, notun Turnstile token) ager order.
func TestCheckoutTurnstileIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 50000, 10)
	ck := f.guestCart([2]int64{rice, 1})
	body := func(tok string) map[string]any {
		b := orderBody(phoneN(1), 56000)
		b["turnstile_token"] = tok
		return b
	}
	for name, tok := range map[string]string{"bhul": "nope", "khali": ""} {
		wantErr(t, name, f.place(idemKey(), ck, body(tok)), http.StatusBadRequest, "turnstile_failed")
	}
	wantErr(t, "onno site", f.place(idemKey(), ck, body("b:x")), http.StatusBadRequest, "turnstile_failed")
	if form, _ := f.sv.last.Load().(url.Values); form.Get("response") != "b:x" || form.Get("secret") != tsSecret {
		t.Errorf("siteverify form %v", form)
	}
	// otp_mode never: OTP lage na, tobu bhul otp_code 400 (Turnstile thik holeo), siteverify dake na
	calls := f.sv.calls.Load()
	wantErr(t, "otp_code, Turnstile chhara", f.place(idemKey(), ck, withOTP(body(""), "123456")), http.StatusBadRequest, "otp_invalid")
	b := body("tok")
	b["otp_code"] = "123456"
	wantErr(t, "otp_code + Turnstile", f.place(idemKey(), ck, b), http.StatusBadRequest, "otp_invalid")
	if n := f.sv.calls.Load(); n != calls {
		t.Errorf("otp_code e siteverify %d bar dakche", n-calls)
	}
	if n := f.count(`SELECT count(*) FROM orders`); n != 0 {
		t.Fatalf("bhul e order %d", n)
	}

	// risky: guest e flag → 409 otp_required (Turnstile ekbar), tarpor code diye Turnstile chhara 201
	f.otpMode("risky")
	big := f.variant("big", 250000, 10) // new_phone_high_amount
	bigCart := f.guestCart([2]int64{big, 1})
	key := idemKey()
	e := wantErr(t, "otp lagbe", f.place(key, bigCart, orderBody(phoneN(2), 250000)), http.StatusConflict, "otp_required")
	if e.Details["phone_masked"] != "+88018****0002" {
		t.Errorf("details %v, phone_masked +88018****0002 chai", e.Details)
	}
	if n := f.sv.calls.Load(); n != calls+1 {
		t.Errorf("siteverify %d bar, chai 1", n-calls)
	}
	code := f.otpCode(phoneN(2))
	wantErr(t, "bhul code", f.place(key, bigCart, withOTP(orderBody(phoneN(2), 250000), otherCode(code))), http.StatusBadRequest, "otp_invalid")
	p := decodePlaced(t, f.place(key, bigCart, withOTP(orderBody(phoneN(2), 250000), code)), http.StatusCreated)
	if n := f.sv.calls.Load(); n != calls+1 {
		t.Errorf("OTP resubmit e siteverify dakche (%d)", n-calls-1)
	}
	// response harano retry: ekoi key, notun Turnstile token, otp_code chhara → 200 ager order (step 2 Turnstile er age)
	retry := orderBody(phoneN(2), 250000)
	retry["turnstile_token"] = "tok:notun"
	if p2 := decodePlaced(t, f.place(key, bigCart, retry), http.StatusOK); p2 != p {
		t.Errorf("retry %+v, chai %+v", p2, p)
	}
	if n := f.count(`SELECT count(*) FROM orders`); n != 1 {
		t.Errorf("order %d, chai 1", n)
	}
}

// TestCheckoutOTPRetryIntegration (14.2 test 40, 8.6 step 8, 9): OTP thik kintu price_changed/out_of_stock →
// ROLLBACK e OTP thake, ekoi code e order; order er por ekoi code onno key e 400 otp_invalid.
func TestCheckoutOTPRetryIntegration(t *testing.T) {
	f := newCheckout(t)
	f.otpMode("risky")
	big := f.variant("big", 250000, 10)
	ck := f.guestCart([2]int64{big, 1})
	ph := phoneN(3)
	wantErr(t, "otp lagbe", f.place(idemKey(), ck, orderBody(ph, 250000)), http.StatusConflict, "otp_required")
	code := f.otpCode(ph)

	wantErr(t, "dam bodleche", f.place(idemKey(), ck, withOTP(orderBody(ph, 240000), code)), http.StatusConflict, "price_changed")
	f.exec(`UPDATE product_variants SET stock = 0 WHERE id = $1`, big)
	wantErr(t, "stock nai", f.place(idemKey(), ck, withOTP(orderBody(ph, 250000), code)), http.StatusConflict, "out_of_stock")
	f.exec(`UPDATE product_variants SET stock = 10 WHERE id = $1`, big)
	if n := f.count(`SELECT count(*) FROM otp_codes WHERE consumed_at IS NOT NULL`); n != 0 {
		t.Fatalf("fail er por OTP consumed %d", n)
	}

	key := idemKey()
	body := withOTP(orderBody(ph, 250000), code)
	p := decodePlaced(t, f.place(key, ck, body), http.StatusCreated)
	var verified bool
	var status string
	if err := f.d.QueryRowContext(f.ctx, `SELECT otp_verified, status FROM orders WHERE public_token = $1`, p.PublicToken).Scan(&verified, &status); err != nil {
		t.Fatal(err)
	}
	if !verified || status != "confirmed" { // risky: OTP e phone proman, confirm SMS lage na (8.19 item 4)
		t.Errorf("otp_verified %v status %s, chai true confirmed", verified, status)
	}
	if n := f.count(`SELECT count(*) FROM otp_codes WHERE consumed_at IS NOT NULL AND purpose = 'checkout'`); n != 1 {
		t.Errorf("consumed %d, chai 1", n)
	}
	// ekoi code, onno key (notun cart) → 400 otp_invalid
	wantErr(t, "use kora code", f.place(idemKey(), f.guestCart([2]int64{big, 1}), withOTP(orderBody(ph, 250000), code)),
		http.StatusBadRequest, "otp_invalid")
	// ekoi key retry → 200 ager order
	if p2 := decodePlaced(t, f.place(key, ck, body), http.StatusOK); p2 != p {
		t.Errorf("retry %+v", p2)
	}
	if n := f.count(`SELECT count(*) FROM orders`); n != 1 {
		t.Errorf("order %d, chai 1", n)
	}
}

// TestCheckoutRiskIntegration (8.6 step 4, 8; 8.7; 8.19 item 3, 4, 9; 7.8): flag orders.risk_flags e, login e
// OTP na, guest e 409 otp_required + checkout OTP (login er send limit ar SMS cap e gona), rate limit 429.
func TestCheckoutRiskIntegration(t *testing.T) {
	f := newCheckout(t)
	f.srv = httpx.ClientIP("")(f.srv) // orders.ip = httptest er 192.0.2.1
	f.otpMode("risky")
	big, rice := f.variant("big", 250000, 50), f.variant("rice", 50000, 100)
	row := func(token string) (flags string, verified bool, status, ip string) {
		t.Helper()
		if err := f.d.QueryRowContext(f.ctx, `SELECT risk_flags::text, otp_verified, status, coalesce(ip, '') FROM orders WHERE public_token = $1`,
			token).Scan(&flags, &verified, &status, &ip); err != nil {
			t.Fatal(err)
		}
		return
	}

	// login customer: flag thakleo OTP na, otp_verified true
	f.userCart([2]int64{big, 1})
	p := decodePlaced(t, f.post("/api/orders", idemKey(), "", f.tok, orderBody(userPhone, 250000)), http.StatusCreated)
	if fl, v, st, ip := row(p.PublicToken); fl != `["new_phone_high_amount"]` || !v || st != "confirmed" || ip != "192.0.2.1" {
		t.Errorf("login order: flags %s verified %v status %s ip %s", fl, v, st, ip)
	}
	if n := f.count(`SELECT count(*) FROM otp_codes`); n != 0 {
		t.Errorf("login e OTP %d", n)
	}

	// guest risky: 409 otp_required + phone_masked, checkout OTP ar sms.send job, order na
	e := wantErr(t, "guest risky", f.place(idemKey(), f.guestCart([2]int64{big, 1}), orderBody(phoneN(4), 250000)),
		http.StatusConflict, "otp_required")
	if e.Details["phone_masked"] != "+88018****0004" {
		t.Errorf("details %v", e.Details)
	}
	if o, j := f.count(`SELECT count(*) FROM otp_codes WHERE phone = $1 AND purpose = 'checkout'`, phoneN(4)),
		f.count(`SELECT count(*) FROM jobs WHERE kind = 'sms.send'`); o != 1 || j != 1 {
		t.Errorf("otp %d job %d, chai 1, 1", o, j)
	}
	// send limit login er sathe (7.8 shob purpose): 2 ta login OTP + 1 checkout = 3, tarpor 429
	for range 2 {
		f.exec(`INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, expires_at) VALUES ($1, $2, 'login', '\x00', now() + interval '5 minutes')`,
			f.shop, phoneN(5))
	}
	ck := f.guestCart([2]int64{big, 1})
	wantErr(t, "3 tomo send", f.place(idemKey(), ck, orderBody(phoneN(5), 250000)), http.StatusConflict, "otp_required")
	wantRetry(t, "4 tomo send", f.place(idemKey(), ck, orderBody(phoneN(5), 250000)))
	// shop er sms_daily_cap
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,sms_daily_cap}', '0') WHERE shop_id = $1`, f.shop)
	wantRetry(t, "sms cap", f.place(idemKey(), ck, orderBody(phoneN(6), 250000)))
	if n := f.count(`SELECT count(*) FROM orders`); n != 1 {
		t.Fatalf("OTP chhara order %d", n)
	}

	// otp_mode never: flag save, otp_verified false, risky confirm e pending
	f.otpMode("never")
	p = decodePlaced(t, f.place(idemKey(), f.guestCart([2]int64{big, 1}), orderBody(phoneN(7), 250000)), http.StatusCreated)
	if fl, v, st, _ := row(p.PublicToken); !strings.Contains(fl, "new_phone_high_amount") || v || st != "pending" {
		t.Errorf("guest order: flags %s verified %v status %s", fl, v, st)
	}

	// rate limit (8.19 item 9): ek phone e ghontay 5 ta hoya order; price_changed gone na; idempotent retry 200
	ph := phoneN(8)
	first, firstCart := idemKey(), f.guestCart([2]int64{rice, 1})
	decodePlaced(t, f.place(first, firstCart, orderBody(ph, 56000)), http.StatusCreated)
	for range 3 {
		decodePlaced(t, f.place(idemKey(), f.guestCart([2]int64{rice, 1}), orderBody(ph, 56000)), http.StatusCreated)
	}
	for range 3 {
		wantErr(t, "price_changed", f.place(idemKey(), f.guestCart([2]int64{rice, 1}), orderBody(ph, 1)), http.StatusConflict, "price_changed")
	}
	decodePlaced(t, f.place(idemKey(), f.guestCart([2]int64{rice, 1}), orderBody(ph, 56000)), http.StatusCreated)
	rec := f.place(idemKey(), f.guestCart([2]int64{rice, 1}), orderBody(ph, 56000))
	wantRetry(t, "phone 6 tomo", rec)
	if s, _ := strconv.Atoi(rec.Header().Get("Retry-After")); s > 3600 {
		t.Errorf("order limit Retry-After %d > 1 ghonta", s)
	}
	decodePlaced(t, f.place(first, firstCart, orderBody(ph, 56000)), http.StatusOK)
	// ek IP te ghontay 10
	for i := f.count(`SELECT count(*) FROM orders WHERE ip = '192.0.2.1'`); i < 10; i++ {
		decodePlaced(t, f.place(idemKey(), f.guestCart([2]int64{rice, 1}), orderBody(phoneN(20+int(i)), 56000)), http.StatusCreated)
	}
	wantRetry(t, "IP 11 tomo", f.place(idemKey(), f.guestCart([2]int64{rice, 1}), orderBody(phoneN(40), 56000)))
	if n := f.count(`SELECT count(*) FROM orders`); n != 10 {
		t.Errorf("order %d, chai 10", n)
	}
}

// wantRetry: 429 rate_limited + Retry-After 1..86400 (SMS cap shop er rat 12 ta porjonto, baki ghontay).
func wantRetry(t *testing.T, name string, rec *httptest.ResponseRecorder) {
	t.Helper()
	wantErr(t, name, rec, http.StatusTooManyRequests, "rate_limited")
	if s, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil || s < 1 || s > 86400 {
		t.Errorf("%s: Retry-After %q", name, rec.Header().Get("Retry-After"))
	}
}

// Checkout e shoja confirmed (risk na, cod_confirmation risky) order er confirmed_at thakbe (8.8), na hole
// pick list (confirmed_at diye) e kokhono ashe na (M7a step 15 e dhora).
func TestCheckoutAutoConfirmedAtIntegration(t *testing.T) {
	f := newCheckout(t)
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,risk,new_phone_amount_over}', '1000000') WHERE shop_id = $1`, f.shop)
	rice := f.variant("rice", 50000, 10)
	p := decodePlaced(t, f.place(idemKey(), f.guestCart([2]int64{rice, 1}), orderBody("01811000002", 56000)), http.StatusCreated)
	if p.Status != "confirmed" {
		t.Fatalf("status %s, confirmed chai", p.Status)
	}
	var id int64
	var at sql.NullTime
	if err := f.d.QueryRowContext(f.ctx, `SELECT id, confirmed_at FROM orders WHERE public_token = $1`, p.PublicToken).Scan(&id, &at); err != nil {
		t.Fatal(err)
	}
	if !at.Valid {
		t.Error("confirmed_at NULL")
	}
	pl, err := order.LoadPickList(f.ctx, f.d, f.shop, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Fulfillments) != 1 || pl.Fulfillments[0].OrderID != id {
		t.Errorf("pick list e order nai: %+v", pl.Fulfillments)
	}
}
