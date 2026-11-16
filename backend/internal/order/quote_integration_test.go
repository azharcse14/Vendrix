package order_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/order"
	"vendrix/backend/internal/shop"
)

type quoteJSON struct {
	Subtotal       int64    `json:"subtotal"`
	Discount       int64    `json:"discount"`
	DeliveryCharge int64    `json:"delivery_charge"`
	Total          int64    `json:"total"`
	OTPRequired    bool     `json:"otp_required"`
	Problems       []string `json:"problems"`
}

// TestQuoteIntegration (7.5, 8.5, 8.19 item 2, 5; 14.2 test 39): taka pricing theke, free delivery >=,
// zone min_order problem, area na thakle 409, coupon na khatle coupon chhara total + problem, otp_required,
// client er dam field 400, kichu save na.
func TestQuoteIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	q := func(query string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := d.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return id
	}
	shops := map[string]int64{}
	for _, s := range []string{"a", "b"} {
		id, _, err := shop.Create(ctx, d, shop.Params{Slug: s, Prefix: "S" + string(s[0]-32), Domain: s + ".com",
			NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		shops[s] = id
	}
	a := shops["a"]
	vendor := q(`SELECT id FROM vendors WHERE shop_id = $1 AND is_house`, a)
	variant := func(sku string, price int64, stock int) int64 {
		p := q(`INSERT INTO products (shop_id, vendor_id, slug, name) VALUES ($1, $2, $3, '{"bn":"পণ্য"}') RETURNING id`, a, vendor, sku)
		return q(`INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock, options)
			VALUES ($1, $2, $3, $4, $5, $6, '{}') RETURNING id`, a, p, vendor, sku, price, stock)
	}
	rice, oil := variant("rice", 50000, 10), variant("oil", 20000, 5)
	for _, z := range []struct {
		shop, district string
		area           any
		charge, min    int64
		free           any
	}{
		{"a", "Dhaka", "Dhanmondi", 6000, 30000, int64(200000)},
		{"a", "Dhaka", nil, 10000, 0, nil},
		{"b", "Khulna", nil, 8000, 0, nil},
	} {
		q(`INSERT INTO delivery_zones (shop_id, name, district, area, charge, min_order, free_delivery_over, same_city)
			VALUES ($1, '{"bn":"এলাকা"}', $2, $3, $4, $5, $6, true) RETURNING id`, shops[z.shop], z.district, z.area, z.charge, z.min, z.free)
	}
	for _, c := range []struct {
		code, kind  string
		value, min  int64
		limit, used any
	}{
		{"SAVE10", "percent", 10, 0, nil, 0},
		{"BIG", "fixed", 5000, 500000, nil, 0}, // min_order cart er beshi: pricing ErrCouponNotApplicable
		{"ONCE", "fixed", 5000, 0, int64(1), 1},
	} {
		q(`INSERT INTO coupons (shop_id, code, kind, value, min_order, max_discount, usage_limit, used_count, per_phone_limit)
			VALUES ($1, $2, $3, $4, $5, 10000, $6, $7, 1) RETURNING id`, a, c.code, c.kind, c.value, c.min, c.limit, c.used)
	}

	// guest cart (X-Cart-Key) ar login customer er cart
	key, hash := cart.NewKey()
	guestCart := q(`INSERT INTO carts (shop_id, session_key) VALUES ($1, $2) RETURNING id`, a, hash)
	const custPhone, guestPhone = "+8801711000001", "+8801811000002"
	user := q(`INSERT INTO users (shop_id, phone) VALUES ($1, $2) RETURNING id`, a, custPhone)
	userCart := q(`INSERT INTO carts (shop_id, user_id) VALUES ($1, $2) RETURNING id`, a, user)
	tok := rand.Text()
	sum := sha256.Sum256([]byte(tok))
	q(`INSERT INTO sessions (shop_id, token_hash, subject_type, user_id, expires_at)
		VALUES ($1, $2, 'customer', $3, now() + interval '1 day') RETURNING id`, a, sum[:], user)
	setCart := func(cartID int64, items map[int64]int) {
		t.Helper()
		if _, err := d.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, cartID); err != nil {
			t.Fatal(err)
		}
		for v, n := range items {
			if _, err := d.ExecContext(ctx, `INSERT INTO cart_items (shop_id, cart_id, variant_id, quantity) VALUES ($1, $2, $3, $4)`,
				a, cartID, v, n); err != nil {
				t.Fatal(err)
			}
		}
	}

	log := slog.New(slog.DiscardHandler)
	public := httpx.ShopFromHost(log, shop.NewResolver(d).Resolve)
	opt := httpx.OptionalCustomer(log, auth.LookupSession(d))
	mux := http.NewServeMux()
	order.NewHandlers(d, log, order.Guard{}).Routes(mux, func(h http.Handler) http.Handler { return public(opt(h)) })
	srv := httpx.JSONErrors(mux)
	do := func(token string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var b bytes.Buffer
		if s, ok := body.(string); ok {
			b.WriteString(s)
		} else if err := json.NewEncoder(&b).Encode(body); err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/api/checkout/quote", &b)
		r.Header.Set("X-Shop-Host", "a.com")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		} else {
			r.Header.Set("X-Cart-Key", key)
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, r)
		return rec
	}
	body := func(coupon, district, area, phone string) map[string]string {
		return map[string]string{"coupon_code": coupon, "district": district, "area": area, "payment_method": "cod", "phone": phone}
	}
	ok := func(name, token string, b any) quoteJSON {
		t.Helper()
		rec := do(token, b)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%s: Cache-Control %q", name, cc)
		}
		var out quoteJSON
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.Problems == nil {
			t.Errorf("%s: problems null, [] chai", name)
		}
		return out
	}
	fail := func(name, token string, b any, status int, code string) {
		t.Helper()
		rec := do(token, b)
		var e struct{ Error httpx.ErrorBody }
		_ = json.Unmarshal(rec.Body.Bytes(), &e)
		if rec.Code != status || e.Error.Code != code {
			t.Errorf("%s: chai %d %s, pelam %d %s", name, status, code, rec.Code, rec.Body.String())
		}
	}
	type want struct {
		sub, disc, del, total int64
		otp                   bool
		problems              []string
	}
	check := func(name string, got quoteJSON, w want) {
		t.Helper()
		if got.Subtotal != w.sub || got.Discount != w.disc || got.DeliveryCharge != w.del || got.Total != w.total ||
			got.OTPRequired != w.otp || !slices.Equal(got.Problems, append([]string{}, w.problems...)) {
			t.Errorf("%s: got %+v want %+v", name, got, w)
		}
	}

	var before [4]int64
	counts := func() [4]int64 {
		var c [4]int64
		if err := d.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM orders), (SELECT sum(used_count) FROM coupons),
			(SELECT count(*) FROM coupon_redemptions) + (SELECT count(*) FROM otp_codes), (SELECT count(*) FROM jobs)`).
			Scan(&c[0], &c[1], &c[2], &c[3]); err != nil {
			t.Fatal(err)
		}
		return c
	}
	before = counts()

	setCart(guestCart, map[int64]int{rice: 2}) // 100000
	check("coupon chhara", ok("coupon chhara", "", body("", "Dhaka", "Dhanmondi", guestPhone)),
		want{100000, 0, 6000, 106000, false, nil})
	check("percent coupon, code trim/upper", ok("percent", "", body(" save10 ", "Dhaka", "Dhanmondi", guestPhone)),
		want{100000, 10000, 6000, 96000, false, nil})
	check("district-wide zone", ok("district", "", body("", "Dhaka", "Uttara", guestPhone)),
		want{100000, 0, 10000, 110000, false, nil})
	// 14.2 test 39: coupon na khatle coupon chhara total + problem
	check("min_order er kom coupon", ok("big", "", body("BIG", "Dhaka", "Dhanmondi", guestPhone)),
		want{100000, 0, 6000, 106000, false, []string{"coupon_invalid"}})
	check("ochena coupon", ok("ochena", "", body("NOPE", "Dhaka", "Dhanmondi", guestPhone)),
		want{100000, 0, 6000, 106000, false, []string{"coupon_invalid"}})
	check("limit shesh coupon", ok("once", "", body("ONCE", "Dhaka", "Dhanmondi", guestPhone)),
		want{100000, 0, 6000, 106000, false, []string{"coupon_limit_reached"}})

	setCart(guestCart, map[int64]int{rice: 4}) // 200000 = free_delivery_over
	check("free delivery >=", ok("free", "", body("", "Dhaka", "Dhanmondi", guestPhone)),
		want{200000, 0, 0, 200000, false, nil})
	check("discount er por shimar niche", ok("free coupon", "", body("SAVE10", "Dhaka", "Dhanmondi", guestPhone)),
		want{200000, 10000, 6000, 196000, false, nil})

	setCart(guestCart, map[int64]int{oil: 1}) // 20000 < zone min_order 30000
	check("below_min_order", ok("min", "", body("", "Dhaka", "Dhanmondi", guestPhone)),
		want{20000, 0, 6000, 26000, false, []string{"below_min_order"}})

	setCart(guestCart, map[int64]int{rice: 1, oil: 6}) // oil stock 5: out_of_stock, total e na
	check("stock er beshi", ok("stock", "", body("", "Dhaka", "Dhanmondi", guestPhone)),
		want{50000, 0, 6000, 56000, false, []string{"out_of_stock"}})

	setCart(guestCart, map[int64]int{rice: 7}) // 350000 > cod_amount_over 300000: flag, guest e OTP
	check("guest risky", ok("risky", "", body("", "Dhaka", "Dhanmondi", guestPhone)),
		want{350000, 0, 0, 350000, true, nil})
	setCart(userCart, map[int64]int{rice: 7})
	check("login e OTP na", ok("login", tok, body("", "Dhaka", "Dhanmondi", custPhone)),
		want{350000, 0, 0, 350000, false, nil})

	fail("area nai", "", body("", "Sylhet", "Zindabazar", guestPhone), http.StatusConflict, "area_not_served")
	fail("onno shop er zone", "", body("", "Khulna", "Sonadanga", guestPhone), http.StatusConflict, "area_not_served")
	fail("client er total (Rule 1)", "", `{"district":"Dhaka","area":"Dhanmondi","payment_method":"cod","phone":"`+guestPhone+`","total":1}`,
		http.StatusBadRequest, "validation_failed")
	fail("client er expected_total", "", `{"district":"Dhaka","area":"Dhanmondi","payment_method":"cod","phone":"`+guestPhone+`","expected_total":1}`,
		http.StatusBadRequest, "validation_failed")
	fail("phone bhul", "", body("", "Dhaka", "Dhanmondi", "12345"), http.StatusBadRequest, "validation_failed")
	fail("login e onno phone", tok, body("", "Dhaka", "Dhanmondi", guestPhone), http.StatusBadRequest, "validation_failed")
	fail("district nai", "", body("", "", "Dhanmondi", guestPhone), http.StatusBadRequest, "validation_failed")
	bad := body("", "Dhaka", "Dhanmondi", guestPhone)
	bad["payment_method"] = "sslcommerz" // settings e shudhu cod
	fail("payment method bondho", "", bad, http.StatusBadRequest, "validation_failed")
	fail("coupon 33 okkhor", "", body("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456", "Dhaka", "Dhanmondi", guestPhone),
		http.StatusBadRequest, "validation_failed")
	setCart(guestCart, nil)
	fail("cart khali", "", body("", "Dhaka", "Dhanmondi", guestPhone), http.StatusBadRequest, "cart_empty")

	if after := counts(); after != before {
		t.Errorf("quote kichu save korlo: age %v, pore %v", before, after)
	}
	var n int
	if err := d.QueryRowContext(ctx, `SELECT count(*) FROM cart_items WHERE cart_id = $1`, userCart).Scan(&n); err != nil || n != 1 {
		t.Errorf("login cart bodlalo: %d %v", n, err)
	}
}

// TestQuotePrivacyIntegration (7.5, 7.8, 8.19 item 13 b): quote e guest phone er history ar coupon er per-phone
// use dekha jay na (otp_required history chhara, per_phone_limit order e); order e asol niyom; route e minute e 30.
// Ager niyom (v1.32) e quote egulo dekhato: v1.34 e ichchha kore bodol (privacy), durbol na.
func TestQuotePrivacyIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 50000, 100)
	f.exec(`INSERT INTO coupons (shop_id, code, kind, value, per_phone_limit) VALUES ($1, 'ONEPP', 'fixed', 5000, 1)`, f.shop)
	ph := phoneN(1)
	b := orderBody(ph, 51000)
	b["coupon_code"] = "ONEPP"
	decodePlaced(t, f.place(idemKey(), f.guestCart([2]int64{rice, 1}), b), http.StatusCreated)
	decodePlaced(t, f.place(idemKey(), f.guestCart([2]int64{rice, 1}), orderBody(ph, 56000)), http.StatusCreated)
	f.otpMode("risky")

	ck := f.guestCart([2]int64{rice, 1})
	quote := func() *httptest.ResponseRecorder {
		return f.post("/api/checkout/quote", "", ck, "", map[string]string{"coupon_code": "ONEPP", "district": "Dhaka",
			"area": "Dhanmondi", "payment_method": "cod", "phone": ph})
	}
	rec := quote()
	var q quoteJSON
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &q) != nil {
		t.Fatalf("quote: %d %s", rec.Code, rec.Body.String())
	}
	if q.OTPRequired || len(q.Problems) != 0 || q.Discount != 5000 {
		t.Errorf("quote e history/coupon use dekha jay: %+v", q)
	}
	// order e asol niyom: many_orders → OTP; coupon per-phone step 11 e (OTP er por)
	wantErr(t, "order e many_orders", f.place(idemKey(), ck, orderBody(ph, 56000)), http.StatusConflict, "otp_required")

	for i := 2; i <= 30; i++ {
		if rec := quote(); rec.Code != http.StatusOK {
			t.Fatalf("quote %d: %d", i, rec.Code)
		}
	}
	wantRetry(t, "quote 31 tomo", quote())
}
