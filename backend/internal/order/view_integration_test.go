package order_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/order"
)

// do: host er shop e request; body nil hole body chhara.
func (f *checkoutFx) do(method, host, path, bearer string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			f.t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &b)
	r.Header.Set("X-Shop-Host", host)
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r)
	return rec
}

func masked(p string) string { return p[:6] + "****" + p[len(p)-4:] }

// local: +8801XXXXXXXXX → 01XXXXXXXXX (customer je bhabe likhe).
func local(p string) string { return "0" + strings.TrimPrefix(p, "+880") }

// noSecret: body te puro phone, thikana line, token nai.
func noSecret(t *testing.T, name, body string, secrets ...string) {
	t.Helper()
	for _, s := range secrets {
		if strings.Contains(body, s) {
			t.Errorf("%s e %q dekha jay: %s", name, s, body)
		}
	}
}

// guestOrder: notun guest cart theke COD order (recipient alada hole dewa).
func (f *checkoutFx) guestOrder(variant int64, customerPhone, recipientPhone string) placed {
	f.t.Helper()
	ck := f.guestCart([2]int64{variant, 1})
	b := orderBody(customerPhone, f.quoteTotal(ck, "", "", "Dhaka", "Dhanmondi", customerPhone))
	if recipientPhone != "" {
		b["recipient"] = map[string]any{"name": "Karim", "phone": recipientPhone}
	}
	return decodePlaced(f.t, f.place(idemKey(), ck, b), http.StatusCreated)
}

type orderView struct {
	OrderNumber          string     `json:"order_number"`
	DisplayStatus        string     `json:"display_status"`
	PlacedAt             time.Time  `json:"placed_at"`
	CustomerName         string     `json:"customer_name"`
	PhoneMasked          string     `json:"phone_masked"`
	RecipientName        string     `json:"recipient_name"`
	RecipientPhoneMasked string     `json:"recipient_phone_masked"`
	District             string     `json:"district"`
	Area                 string     `json:"area"`
	PaymentMethod        string     `json:"payment_method"`
	Subtotal             int64      `json:"subtotal"`
	Discount             int64      `json:"discount"`
	DeliveryCharge       int64      `json:"delivery_charge"`
	Total                int64      `json:"total"`
	DeliveryDeadline     *time.Time `json:"delivery_deadline"`
	CanCancel            bool       `json:"can_cancel"`
	Items                []struct {
		Name      map[string]string `json:"name"`
		Quantity  int               `json:"quantity"`
		LineTotal int64             `json:"line_total"`
	} `json:"items"`
}

// TestOrderViewIntegration (7.5, 8.19 item 6, Rule 11, 12.2 Data 5): GET /api/orders/{token} shudhu public_token
// diye; order number, confirm token ba ochena token 404; phone mask, thikana shudhu district + area, token/thikana
// line nai; can_cancel cancel er por false; no-store.
func TestOrderViewIntegration(t *testing.T) {
	f := newCheckout(t)
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,cod_confirmation}', '"all"') WHERE shop_id = $1`, f.shop)
	rice := f.variant("rice", 50000, 10)
	cp, rp := phoneN(1), phoneN(2)
	p := f.guestOrder(rice, cp, rp)

	for _, tok := range []string{p.OrderNumber, rand.Text(), order.ConfirmToken(confirmSecret, p.PublicToken)} {
		wantErr(t, "token "+tok, f.do("GET", "a.com", "/api/orders/"+tok, "", nil), http.StatusNotFound, "not_found")
	}

	rec := f.do("GET", "a.com", "/api/orders/"+p.PublicToken, "", nil)
	var v orderView
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &v) != nil {
		t.Fatalf("view: %d %s", rec.Code, rec.Body.String())
	}
	if v.OrderNumber != p.OrderNumber || v.DisplayStatus != "pending" || v.PlacedAt.IsZero() || v.CustomerName != "Rahim" ||
		v.PhoneMasked != masked(cp) || v.RecipientName != "Karim" || v.RecipientPhoneMasked != masked(rp) ||
		v.District != "Dhaka" || v.Area != "Dhanmondi" || v.PaymentMethod != "cod" || v.Subtotal != 50000 || v.Discount != 0 ||
		v.DeliveryCharge != 6000 || v.Total != 56000 || v.DeliveryDeadline == nil || !v.CanCancel ||
		len(v.Items) != 1 || v.Items[0].Quantity != 1 || v.Items[0].LineTotal != 50000 || v.Items[0].Name["en"] != "Item rice" {
		t.Errorf("view: %s", rec.Body.String())
	}
	noSecret(t, "view", rec.Body.String(), cp, rp, areaLine, p.PublicToken, `"public_token"`, order.ConfirmToken(confirmSecret, p.PublicToken))
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control %q", cc)
	}

	// cancel er por: cancelled, can_cancel false
	if rec := f.post("/api/orders/"+p.PublicToken+"/cancel", "", "", "", map[string]string{}); rec.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	rec = f.do("GET", "a.com", "/api/orders/"+p.PublicToken, "", nil)
	v = orderView{}
	if json.Unmarshal(rec.Body.Bytes(), &v) != nil || v.DisplayStatus != "cancelled" || v.CanCancel {
		t.Errorf("cancel er por view: %s", rec.Body.String())
	}
}

type trackJSON struct {
	OrderNumber   string    `json:"order_number"`
	DisplayStatus string    `json:"display_status"`
	PlacedAt      time.Time `json:"placed_at"`
	History       []struct {
		Status string    `json:"status"`
		At     time.Time `json:"at"`
	} `json:"history"`
}

// TestTrackIntegration (7.5, 7.8, 8.19 item 6): POST /api/orders/track {number, phone}: customer ba recipient
// phone (local ba +880, number er case jai hok) mille status ar history, token/phone na, tracking null; phone ba
// number na mille 404 not_found (kon ta bhul bola na); bhul phone format 400; minute e 10 er por 429 + Retry-After.
func TestTrackIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 50000, 10)
	cp, rp := phoneN(1), phoneN(2)
	p := f.guestOrder(rice, cp, rp)
	track := func(number, phone string) *httptest.ResponseRecorder {
		return f.do("POST", "a.com", "/api/orders/track", "", map[string]string{"number": number, "phone": phone})
	}

	for _, c := range []struct{ name, number, phone string }{
		{"customer phone", p.OrderNumber, cp},
		{"recipient phone local", strings.ToLower(p.OrderNumber), local(rp)},
	} {
		rec := track(c.number, c.phone)
		var tr trackJSON
		var raw map[string]any
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &tr) != nil || json.Unmarshal(rec.Body.Bytes(), &raw) != nil {
			t.Fatalf("%s: %d %s", c.name, rec.Code, rec.Body.String())
		}
		if v, ok := raw["tracking"]; !ok || v != nil || tr.OrderNumber != p.OrderNumber || tr.DisplayStatus != p.Status ||
			tr.PlacedAt.IsZero() || len(tr.History) != 1 || tr.History[0].Status != p.Status || tr.History[0].At.IsZero() {
			t.Errorf("%s: %s", c.name, rec.Body.String())
		}
		noSecret(t, c.name, rec.Body.String(), cp, rp, areaLine, p.PublicToken, "Rahim")
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%s: Cache-Control %q", c.name, cc)
		}
	}

	wantErr(t, "onno phone", track(p.OrderNumber, phoneN(3)), http.StatusNotFound, "not_found")
	wantErr(t, "ochena number", track("SA-999999", cp), http.StatusNotFound, "not_found")
	wantErr(t, "phone format", track(p.OrderNumber, "123"), http.StatusBadRequest, "validation_failed")

	// 21 ta request e (minute bodlaleo) kom-poksha ekta 429; thik number + phone eo
	var limited *httptest.ResponseRecorder
	for range 21 {
		if rec := track(p.OrderNumber, cp); rec.Code == http.StatusTooManyRequests {
			limited = rec
			break
		}
	}
	if limited == nil {
		t.Fatal("track e 429 nai")
	}
	wantErr(t, "track limit", limited, http.StatusTooManyRequests, "rate_limited")
	if limited.Header().Get("Retry-After") == "" {
		t.Error("429 e Retry-After nai")
	}
}

type myOrders struct {
	Items []map[string]any `json:"items"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
	Total int64            `json:"total"`
}

func (f *checkoutFx) myOrders(host, bearer, query string) myOrders {
	f.t.Helper()
	rec := f.do("GET", host, "/api/me/orders"+query, bearer, nil)
	var m myOrders
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &m) != nil {
		f.t.Fatalf("me/orders: %d %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		f.t.Errorf("me/orders Cache-Control %q", cc)
	}
	return m
}

func numbers(m myOrders) []string {
	var out []string
	for _, it := range m.Items {
		out = append(out, it["order_number"].(string))
	}
	return out
}

// TestMyOrdersIntegration (7.3, Rule 16, 8.19 item 12 x): GET /api/me/orders shudhu nijer user_id er order
// (onno user, ekoi phone er guest order na), notun age, paged, card e shudhu 5 ta field; login chhara 401.
func TestMyOrdersIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 50000, 10)
	other := f.id(`INSERT INTO users (shop_id, phone) VALUES ($1, '+8801711000002') RETURNING id`, f.shop)
	seed := func(user any, ago string) string {
		oid, _ := f.seedOrder("cod", "pending", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}})
		f.exec(`UPDATE orders SET user_id = $2, customer_phone = $3, placed_at = now() - $4::interval WHERE id = $1`, oid, user, userPhone, ago)
		return f.str(`SELECT order_number FROM orders WHERE id = $1`, oid)
	}
	older, newer := seed(f.user, "2 hours"), seed(f.user, "1 hour")
	seed(other, "30 minutes")
	seed(nil, "10 minutes") // ekoi phone er guest order: login er por OTP verify chhara jog hoy na

	m := f.myOrders("a.com", f.tok, "")
	if got := numbers(m); !slices.Equal(got, []string{newer, older}) || m.Total != 2 || m.Page != 1 || m.Limit != 24 {
		t.Fatalf("me/orders: %v total %d page %d limit %d", got, m.Total, m.Page, m.Limit)
	}
	keys := make([]string, 0, 5)
	for k := range m.Items[0] {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"display_status", "order_number", "placed_at", "public_token", "total"}) {
		t.Errorf("card er field %v", keys)
	}
	if m.Items[0]["public_token"] != newer || m.Items[0]["display_status"] != "confirmed" || fmt.Sprint(m.Items[0]["total"]) != "10000" {
		t.Errorf("card %v", m.Items[0])
	}
	if p := f.myOrders("a.com", f.tok, "?page=2&limit=1"); !slices.Equal(numbers(p), []string{older}) || p.Total != 2 {
		t.Errorf("page 2: %v total %d", numbers(p), p.Total)
	}
	wantErr(t, "login chhara", f.do("GET", "a.com", "/api/me/orders", "", nil), http.StatusUnauthorized, "unauthenticated")
	wantErr(t, "bhul limit", f.do("GET", "a.com", "/api/me/orders?limit=101", f.tok, nil), http.StatusBadRequest, "validation_failed")
}

// TestOrderShopIsolationIntegration (14.2 test 5, order ongsho; Rule 3): shop a er public_token, order number +
// phone, confirm token ar Bearer shop b er host e 404/401; shop b er coupon ar zone shop a te chole na.
func TestOrderShopIsolationIntegration(t *testing.T) {
	f := newCheckout(t)
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,cod_confirmation}', '"all"') WHERE shop_id = $1`, f.shop)
	rice := f.variant("rice", 50000, 10)
	cp := phoneN(1)
	p := f.guestOrder(rice, cp, "")

	wantErr(t, "b te view", f.do("GET", "b.com", "/api/orders/"+p.PublicToken, "", nil), http.StatusNotFound, "not_found")
	wantErr(t, "b te cancel", f.do("POST", "b.com", "/api/orders/"+p.PublicToken+"/cancel", "", map[string]string{}),
		http.StatusNotFound, "not_found")
	wantErr(t, "b te track", f.do("POST", "b.com", "/api/orders/track", "", map[string]string{"number": p.OrderNumber, "phone": cp}),
		http.StatusNotFound, "not_found")
	wantErr(t, "b te confirm", f.do("POST", "b.com", "/api/confirm/"+order.ConfirmToken(confirmSecret, p.PublicToken), "", nil),
		http.StatusNotFound, "not_found")
	wantErr(t, "b te a er Bearer", f.do("GET", "b.com", "/api/me/orders", f.tok, nil), http.StatusUnauthorized, "unauthenticated")
	if s := f.str(`SELECT status FROM orders WHERE public_token = $1`, p.PublicToken); s != "pending" {
		t.Errorf("b er request er por status %s", s)
	}

	// shop b er coupon ar zone shop a er quote e na
	b := f.id(`SELECT id FROM shops WHERE slug = 'b'`)
	f.id(`INSERT INTO coupons (shop_id, code, kind, value) VALUES ($1, 'BONLY', 'fixed', 5000) RETURNING id`, b)
	f.id(`INSERT INTO delivery_zones (shop_id, name, district, charge, same_city) VALUES ($1, '{"bn":"এ"}', 'Sylhet', 1000, false) RETURNING id`, b)
	ck := f.guestCart([2]int64{rice, 1})
	rec := f.post("/api/checkout/quote", "", ck, "", map[string]string{"coupon_code": "BONLY", "district": "Dhaka",
		"area": "Dhanmondi", "payment_method": "cod", "phone": cp})
	var q quoteJSON
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &q) != nil || q.Discount != 0 || !slices.Equal(q.Problems, []string{"coupon_invalid"}) {
		t.Errorf("b er coupon a te: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, "b er zone a te", f.post("/api/checkout/quote", "", ck, "", map[string]string{"district": "Sylhet", "area": "Zindabazar",
		"payment_method": "cod", "phone": cp}), http.StatusConflict, "area_not_served")
}

// TestCheckoutGuestThenLoginIntegration (14.2 test 11, 8.2 step 5): guest checkout → OTP login → /api/me/orders e
// oi order; onno phone er guest order na; login er age oi phone er account chhilo na.
func TestCheckoutGuestThenLoginIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 50000, 10)
	mine, theirs := phoneN(1), phoneN(2)
	p := f.guestOrder(rice, mine, "")
	f.guestOrder(rice, theirs, mine) // onno customer, recipient amar phone: jog hoy na (customer_phone diye)
	if n := f.count(`SELECT count(*) FROM users WHERE shop_id = $1 AND phone = $2`, f.shop, mine); n != 0 {
		t.Fatalf("login er age user %d", n)
	}

	f.exec(`INSERT INTO otp_codes (shop_id, phone, purpose, code_hash, expires_at) VALUES ($1, $2, 'login', $3, now() + interval '5 minutes')`,
		f.shop, mine, auth.HashOTP(otpSecret, mine, "login", "123456"))
	rec := f.do("POST", "a.com", "/api/auth/otp/verify", "", map[string]string{"phone": local(mine), "code": "123456"})
	var login struct {
		Token string `json:"token"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &login) != nil || login.Token == "" {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	m := f.myOrders("a.com", login.Token, "")
	if got := numbers(m); !slices.Equal(got, []string{p.OrderNumber}) || m.Items[0]["public_token"] != p.PublicToken {
		t.Errorf("login er por me/orders %v", m.Items)
	}
	// ager login customer (onno phone) e na
	if got := numbers(f.myOrders("a.com", f.tok, "")); len(got) != 0 {
		t.Errorf("onno user er me/orders %v", got)
	}
}
