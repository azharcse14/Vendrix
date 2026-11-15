package cart_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/db"
	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/jobs"
	"vendrix/backend/internal/shop"
)

// fixture: duita shop (a.com, b.com), house vendor; cart route router er moto (public + OptionalCustomer).
type fixture struct {
	d   *sql.DB
	srv http.Handler
	ids map[string]int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	d := dbtest.New(t)
	f := &fixture{d: d, ids: map[string]int64{}}
	for _, s := range []string{"a", "b"} {
		id, _, err := shop.Create(context.Background(), d, shop.Params{Slug: s, Prefix: "S" + string(s[0]-32),
			Domain: s + ".com", NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		f.ids[s] = id
		f.ids[s+".vendor"] = f.id(t, `SELECT id FROM vendors WHERE shop_id = $1 AND is_house`, id)
	}
	log := slog.New(slog.DiscardHandler)
	public := httpx.ShopFromHost(log, shop.NewResolver(d).Resolve)
	opt := httpx.OptionalCustomer(log, auth.LookupSession(d))
	mux := http.NewServeMux()
	cart.NewHandlers(d, log).Routes(mux, func(h http.Handler) http.Handler { return public(opt(h)) })
	f.srv = httpx.JSONErrors(mux)
	return f
}

func (f *fixture) id(t *testing.T, q string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := f.d.QueryRow(q, args...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return id
}

func (f *fixture) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := f.d.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// variant: shop s e product (naam bn/en = sku) ar ekta variant; id f.ids[s+"/"+sku].
func (f *fixture) variant(t *testing.T, s, sku string, price int64, stock int) int64 {
	t.Helper()
	p := f.id(t, `INSERT INTO products (shop_id, vendor_id, slug, name)
		VALUES ($1, $2, $3, jsonb_build_object('bn', 'পণ্য ' || $3::text, 'en', 'Item ' || $3::text)) RETURNING id`,
		f.ids[s], f.ids[s+".vendor"], sku)
	f.ids[s+"/"+sku+".product"] = p
	v := f.id(t, `INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock, options)
		VALUES ($1, $2, $3, $4, $5, $6, '{"weight":{"bn":"১ কেজি","en":"1 kg"}}') RETURNING id`,
		f.ids[s], p, f.ids[s+".vendor"], sku, price, stock)
	f.ids[s+"/"+sku] = v
	return v
}

// customer: shop s e user ar active session; Bearer token ferot.
func (f *fixture) customer(t *testing.T, s, phone string) string {
	t.Helper()
	u := f.id(t, `INSERT INTO users (shop_id, phone) VALUES ($1, $2) RETURNING id`, f.ids[s], phone)
	tok := rand.Text()
	sum := sha256.Sum256([]byte(tok))
	f.exec(t, `INSERT INTO sessions (shop_id, token_hash, subject_type, user_id, expires_at)
		VALUES ($1, $2, 'customer', $3, now() + interval '1 day')`, f.ids[s], sum[:], u)
	return tok
}

type item struct {
	VariantID      int64   `json:"variant_id"`
	ProductSlug    string  `json:"product_slug"`
	Name           string  `json:"name"`
	VariantLabel   string  `json:"variant_label"`
	Image          *string `json:"image"`
	UnitPrice      int64   `json:"unit_price"`
	CompareAtPrice *int64  `json:"compare_at_price"`
	VariableWeight bool    `json:"variable_weight"`
	Quantity       int     `json:"quantity"`
	MaxQuantity    int     `json:"max_quantity"`
	InStock        bool    `json:"in_stock"`
	LineTotal      int64   `json:"line_total"`
}

type notice struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type cartJSON struct {
	Items    []item   `json:"items"`
	Count    int      `json:"count"`
	Subtotal int64    `json:"subtotal"`
	Notices  []notice `json:"notices"`
	CartKey  string   `json:"cart_key"`
}

type errJSON struct {
	Error struct {
		Code    string `json:"code"`
		Details struct {
			VariantID   int64 `json:"variant_id"`
			MaxQuantity int   `json:"max_quantity"`
		} `json:"details"`
	} `json:"error"`
}

// req: host, Bearer token ("" = na), X-Cart-Key ("" = na); body string hole hubohu, na hole JSON.
type req struct {
	method, path, host, token, key, lang string
	body                                 any
}

func (f *fixture) do(t *testing.T, r req) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	switch v := r.body.(type) {
	case nil:
	case string:
		b.WriteString(v)
	default:
		if err := json.NewEncoder(&b).Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	hr := httptest.NewRequest(r.method, r.path, &b)
	hr.Header.Set("X-Shop-Host", r.host)
	if r.token != "" {
		hr.Header.Set("Authorization", "Bearer "+r.token)
	}
	if r.key != "" {
		hr.Header.Set("X-Cart-Key", r.key)
	}
	if r.lang != "" {
		hr.Header.Set("Accept-Language", r.lang)
	}
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, hr)
	return rec
}

// ok: 200 ar cart; Cache-Control no-store (7.4).
func (f *fixture) ok(t *testing.T, r req) cartJSON {
	t.Helper()
	rec := f.do(t, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s: %d %s", r.method, r.path, rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("%s %s: Cache-Control %q", r.method, r.path, cc)
	}
	var c cartJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func (f *fixture) fail(t *testing.T, r req, status int, code string) errJSON {
	t.Helper()
	rec := f.do(t, r)
	var e errJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if rec.Code != status || e.Error.Code != code {
		t.Fatalf("%s %s: chai %d %s, pelam %d %s", r.method, r.path, status, code, rec.Code, rec.Body.String())
	}
	return e
}

func add(host, token, key string, variant int64, qty int) req {
	return req{method: "POST", path: "/api/cart/items", host: host, token: token, key: key,
		body: map[string]any{"variant_id": variant, "quantity": qty}}
}

func patch(host, token, key string, variant int64, qty int) req {
	return req{method: "PATCH", path: fmt.Sprintf("/api/cart/items/%d", variant), host: host, token: token, key: key,
		body: map[string]any{"quantity": qty}}
}

func get(host, token, key string) req {
	return req{method: "GET", path: "/api/cart", host: host, token: token, key: key}
}

func find(c cartJSON, variant int64) *item {
	for i := range c.Items {
		if c.Items[i].VariantID == variant {
			return &c.Items[i]
		}
	}
	return nil
}

func wantOutOfStock(t *testing.T, e errJSON, variant int64, max int) {
	t.Helper()
	if d := e.Error.Details; d.VariantID != variant || d.MaxQuantity != max {
		t.Errorf("details %v, chai variant_id %d max_quantity %d", e.Error.Details, variant, max)
	}
}

// TestCartQuantityIntegration (8.4 v1.28 item 3, 4, 5, 7): 1..min(stock, 50), 409 out_of_stock + max_quantity,
// stock 0 e add 409, ekoi variant abar add e qty jog, GET e normalize (quantity_reduced, stock 0 thake),
// 50 variant e cart_full, ojon-variable er line total, DELETE idempotent.
func TestCartQuantityIntegration(t *testing.T) {
	f := newFixture(t)
	tok := f.customer(t, "a", "+8801711000001")
	v10 := f.variant(t, "a", "v10", 1000, 10)
	v100 := f.variant(t, "a", "v100", 500, 100)
	v0 := f.variant(t, "a", "v0", 700, 0)

	for _, q := range []int{0, -1} {
		f.fail(t, add("a.com", tok, "", v10, q), 400, "validation_failed")
	}
	wantOutOfStock(t, f.fail(t, add("a.com", tok, "", v10, 11), 409, "out_of_stock"), v10, 10)
	wantOutOfStock(t, f.fail(t, add("a.com", tok, "", v100, 51), 409, "out_of_stock"), v100, 50)
	wantOutOfStock(t, f.fail(t, add("a.com", tok, "", v0, 1), 409, "out_of_stock"), v0, 0)

	c := f.ok(t, add("a.com", tok, "", v10, 3))
	if it := find(c, v10); it == nil || it.Quantity != 3 || it.MaxQuantity != 10 || it.LineTotal != 3000 || !it.InStock {
		t.Fatalf("add er por %+v", c)
	}
	// ekoi variant abar add: qty jog; jog min(stock, 50) er beshi hole 409, chupchap komay na
	c = f.ok(t, add("a.com", tok, "", v10, 2))
	if it := find(c, v10); it == nil || it.Quantity != 5 || c.Count != 1 {
		t.Fatalf("abar add e qty 5 chai: %+v", c)
	}
	wantOutOfStock(t, f.fail(t, add("a.com", tok, "", v10, 6), 409, "out_of_stock"), v10, 10)

	c = f.ok(t, patch("a.com", tok, "", v10, 10))
	if it := find(c, v10); it == nil || it.Quantity != 10 {
		t.Fatalf("patch 10: %+v", c)
	}
	f.fail(t, patch("a.com", tok, "", v10, 0), 400, "validation_failed")
	wantOutOfStock(t, f.fail(t, patch("a.com", tok, "", v10, 11), 409, "out_of_stock"), v10, 10)
	f.fail(t, patch("a.com", tok, "", v100, 1), 404, "not_found") // cart e nai

	// normalize: stock komle qty komay (DB te o), notice; stock 0 hole item thake, subtotal e na
	f.ok(t, add("a.com", tok, "", v100, 2))
	f.exec(t, `UPDATE product_variants SET stock = 4 WHERE id = $1`, v10)
	c = f.ok(t, get("a.com", tok, ""))
	if it := find(c, v10); it == nil || it.Quantity != 4 || it.MaxQuantity != 4 {
		t.Fatalf("normalize: %+v", c)
	}
	if len(c.Notices) != 1 || c.Notices[0].Code != "quantity_reduced" || c.Notices[0].Name == "" {
		t.Errorf("quantity_reduced notice: %+v", c.Notices)
	}
	if c.Subtotal != 4*1000+2*500 {
		t.Errorf("subtotal %d", c.Subtotal)
	}
	if n := f.id(t, `SELECT quantity FROM cart_items WHERE variant_id = $1`, v10); n != 4 {
		t.Errorf("DB qty %d, chai 4", n)
	}
	if c = f.ok(t, get("a.com", tok, "")); len(c.Notices) != 0 {
		t.Errorf("notice ekbar-i: %+v", c.Notices)
	}
	f.exec(t, `UPDATE product_variants SET stock = 0 WHERE id = $1`, v10)
	c = f.ok(t, get("a.com", tok, ""))
	if it := find(c, v10); it == nil || it.InStock || it.Quantity != 4 || it.MaxQuantity != 0 {
		t.Fatalf("stock 0 er item thakbe, in_stock false: %+v", c)
	}
	if c.Subtotal != 2*500 || c.Count != 2 {
		t.Errorf("stock 0 line subtotal e na: subtotal %d count %d", c.Subtotal, c.Count)
	}

	// DELETE: ache ba nai, duitai 200
	c = f.ok(t, req{method: "DELETE", path: fmt.Sprintf("/api/cart/items/%d", v10), host: "a.com", token: tok})
	if find(c, v10) != nil || c.Count != 1 {
		t.Fatalf("delete: %+v", c)
	}
	f.ok(t, req{method: "DELETE", path: fmt.Sprintf("/api/cart/items/%d", v10), host: "a.com", token: tok})
	f.ok(t, req{method: "DELETE", path: "/api/cart/items/999999", host: "a.com", token: tok})
	f.fail(t, req{method: "DELETE", path: "/api/cart/items/abc", host: "a.com", token: tok}, 400, "validation_failed")

	// ojon-variable: line = 1 kg er dam × nominal_weight_g × qty / 1000; nominal_weight_g NULL e 409
	vw := f.variant(t, "a", "vw", 10000, 20)
	f.exec(t, `UPDATE products SET unit = 'kg', variable_weight = true WHERE id = $1`, f.ids["a/vw.product"])
	f.fail(t, add("a.com", tok, "", vw, 1), 409, "out_of_stock")
	f.exec(t, `UPDATE product_variants SET nominal_weight_g = 500 WHERE id = $1`, vw)
	c = f.ok(t, add("a.com", tok, "", vw, 3))
	if it := find(c, vw); it == nil || !it.VariableWeight || it.LineTotal != 15000 {
		t.Fatalf("ojon-variable: %+v", it)
	}

	// cart_full: 50 ta alada variant er por notun variant 409; cart e thaka variant er qty bodol chole
	cartID := f.id(t, `SELECT cart_id FROM cart_items WHERE variant_id = $1`, vw)
	f.exec(t, `WITH p AS (INSERT INTO products (shop_id, vendor_id, slug, name)
		SELECT $1, $2, 'bulk-' || g, '{"bn":"বাল্ক"}' FROM generate_series(1, 49) g RETURNING id, slug)
		INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock)
		SELECT $1, id, $2, slug, 100, 5 FROM p`, f.ids["a"], f.ids["a.vendor"])
	f.exec(t, `INSERT INTO cart_items (shop_id, cart_id, variant_id, quantity)
		SELECT shop_id, $2, id, 1 FROM product_variants WHERE shop_id = $1 AND sku LIKE 'bulk-%'
		ORDER BY id LIMIT 48`, f.ids["a"], cartID)
	if c = f.ok(t, get("a.com", tok, "")); c.Count != 50 {
		t.Fatalf("count %d, chai 50", c.Count)
	}
	last := f.id(t, `SELECT id FROM product_variants WHERE shop_id = $1 AND sku LIKE 'bulk-%' ORDER BY id DESC LIMIT 1`, f.ids["a"])
	f.fail(t, add("a.com", tok, "", last, 1), 409, "cart_full")
	f.ok(t, add("a.com", tok, "", v100, 1))
}

// TestCartFreshPriceIntegration (8.4 Dam, v1.28 item 5, Rule 1): cart e dam save hoy na; dam bodlale porer
// GET e notun unit_price, compare_at_price, line_total ar subtotal.
func TestCartFreshPriceIntegration(t *testing.T) {
	f := newFixture(t)
	v := f.variant(t, "a", "rice", 12000, 30)
	c := f.ok(t, add("a.com", "", "", v, 2))
	key := c.CartKey
	if c.Subtotal != 24000 {
		t.Fatalf("subtotal %d", c.Subtotal)
	}
	f.exec(t, `UPDATE product_variants SET price = 9000, compare_at_price = 12000 WHERE id = $1`, v)
	c = f.ok(t, get("a.com", "", key))
	it := find(c, v)
	if it == nil || it.UnitPrice != 9000 || it.CompareAtPrice == nil || *it.CompareAtPrice != 12000 ||
		it.LineTotal != 18000 || c.Subtotal != 18000 {
		t.Fatalf("notun dam chai: %+v %+v", it, c)
	}
	if it.ProductSlug != "rice" || it.Name != "পণ্য rice" || it.VariantLabel != "১ কেজি" {
		t.Errorf("naam/label (bn default): %+v", it)
	}
	c = f.ok(t, req{method: "GET", path: "/api/cart", host: "a.com", key: key, lang: "en"})
	if it := find(c, v); it == nil || it.Name != "Item rice" || it.VariantLabel != "1 kg" {
		t.Errorf("en: %+v", it)
	}
	if len(c.Notices) != 0 {
		t.Errorf("dam bodle notice na (v1.28 item 5): %+v", c.Notices)
	}
}

// TestCartRejectsPriceField (8.4, 7.4, Rule 1): body te dam/total er field nai; DisallowUnknownFields e 400.
func TestCartRejectsPriceField(t *testing.T) {
	f := newFixture(t)
	v := f.variant(t, "a", "oil", 50000, 10)
	for _, body := range []string{
		fmt.Sprintf(`{"variant_id": %d, "quantity": 1, "price": 1}`, v),
		fmt.Sprintf(`{"variant_id": %d, "quantity": 1, "unit_price": 1}`, v),
		fmt.Sprintf(`{"variant_id": %d, "quantity": 1, "line_total": 1}`, v),
		`{"quantity": 1}`,
		`not json`,
	} {
		f.fail(t, req{method: "POST", path: "/api/cart/items", host: "a.com", body: body}, 400, "validation_failed")
	}
	c := f.ok(t, add("a.com", "", "", v, 1))
	for _, body := range []string{`{"quantity": 1, "subtotal": 1}`, `{"quantity": 1, "price": 1}`} {
		f.fail(t, req{method: "PATCH", path: fmt.Sprintf("/api/cart/items/%d", v), host: "a.com", key: c.CartKey,
			body: body}, 400, "validation_failed")
	}
	if c = f.ok(t, get("a.com", "", c.CartKey)); c.Subtotal != 50000 {
		t.Errorf("dam DB theke: %d", c.Subtotal)
	}
}

// TestCartInactiveRemovedIntegration (8.4 v1.28 item 4, Rule 13): bondho product, bondho variant ar active na
// vendor er item GET e muche + item_removed (naam bortoman bhashay); notice ekbar; abar chalu holeo ferot na;
// POST e bondho/ochena variant 404.
func TestCartInactiveRemovedIntegration(t *testing.T) {
	f := newFixture(t)
	tok := f.customer(t, "a", "+8801711000002")
	keep := f.variant(t, "a", "keep", 100, 5)
	offP := f.variant(t, "a", "offp", 100, 5)
	offV := f.variant(t, "a", "offv", 100, 5)
	// marketplace vendor (house na)
	f.ids["a.vendor"] = f.id(t, `INSERT INTO vendors (shop_id, slug, name) VALUES ($1, 'mv', '{"bn":"ভেন্ডর"}') RETURNING id`, f.ids["a"])
	offVendor := f.variant(t, "a", "offvendor", 100, 5)
	for _, v := range []int64{keep, offP, offV, offVendor} {
		f.ok(t, add("a.com", tok, "", v, 1))
	}
	f.exec(t, `UPDATE products SET is_active = false WHERE id = $1`, f.ids["a/offp.product"])
	f.exec(t, `UPDATE product_variants SET is_active = false WHERE id = $1`, offV)
	f.exec(t, `UPDATE vendors SET status = 'suspended' WHERE id = $1`, f.ids["a.vendor"])

	c := f.ok(t, req{method: "GET", path: "/api/cart", host: "a.com", token: tok, lang: "en"})
	if c.Count != 1 || find(c, keep) == nil {
		t.Fatalf("shudhu keep thakbe: %+v", c)
	}
	names := map[string]bool{}
	for _, n := range c.Notices {
		if n.Code != "item_removed" {
			t.Errorf("notice %+v", n)
		}
		names[n.Name] = true
	}
	if len(c.Notices) != 3 || !names["Item offp"] || !names["Item offv"] || !names["Item offvendor"] {
		t.Errorf("item_removed (en naam): %+v", c.Notices)
	}
	if n := f.id(t, `SELECT count(*) FROM cart_items WHERE shop_id = $1`, f.ids["a"]); n != 1 {
		t.Errorf("DB te %d row, chai 1", n)
	}
	f.exec(t, `UPDATE products SET is_active = true WHERE id = $1`, f.ids["a/offp.product"])
	f.exec(t, `UPDATE product_variants SET is_active = true WHERE id = $1`, offV)
	f.exec(t, `UPDATE vendors SET status = 'active' WHERE id = $1`, f.ids["a.vendor"])
	if c = f.ok(t, get("a.com", tok, "")); c.Count != 1 || len(c.Notices) != 0 {
		t.Errorf("abar chalu holeo ferot na, notice ekbar: %+v", c)
	}

	f.exec(t, `UPDATE product_variants SET is_active = false WHERE id = $1`, offV)
	f.fail(t, add("a.com", tok, "", offV, 1), 404, "not_found")
	f.exec(t, `UPDATE products SET is_active = false WHERE id = $1`, f.ids["a/offp.product"])
	f.fail(t, add("a.com", tok, "", offP, 1), 404, "not_found")
	f.exec(t, `UPDATE vendors SET status = 'suspended' WHERE id = $1`, f.ids["a.vendor"])
	f.fail(t, add("a.com", tok, "", offVendor, 1), 404, "not_found")
	f.fail(t, add("a.com", tok, "", 999999, 1), 404, "not_found")
	f.fail(t, patch("a.com", tok, "", offV, 1), 404, "not_found")
}

var keyRe = regexp.MustCompile(`^[A-Z2-7]{26}$`)

// TestCartKeyIntegration (8.4 v1.28 item 1, 2): Go key banay (notun guest cart e cart_key), DB te shudhu
// sha256; GET row banay na; bhul format ba ochena key e notun key (client er key na); Bearer thakle account
// cart, key ignore; bhul/expire Bearer 401, guest e fall back na.
func TestCartKeyIntegration(t *testing.T) {
	f := newFixture(t)
	v := f.variant(t, "a", "egg", 1500, 40)
	carts := func() int64 { return f.id(t, `SELECT count(*) FROM carts`) }

	if c := f.ok(t, get("a.com", "", "")); c.Count != 0 || c.Items == nil || c.CartKey != "" {
		t.Fatalf("khali cart (items [] ): %+v", c)
	}
	if carts() != 0 {
		t.Fatal("GET row banalo")
	}
	c := f.ok(t, add("a.com", "", "", v, 1))
	key := c.CartKey
	if !keyRe.MatchString(key) {
		t.Fatalf("cart_key %q", key)
	}
	sum := sha256.Sum256([]byte(key))
	if n := f.id(t, `SELECT count(*) FROM carts WHERE session_key = $1 AND user_id IS NULL`, sum[:]); n != 1 {
		t.Fatal("DB te sha256 chai")
	}
	if n := f.id(t, `SELECT count(*) FROM carts WHERE position(convert_to($1, 'UTF8') in session_key) > 0`, key); n != 0 {
		t.Fatal("plain key DB te")
	}
	c = f.ok(t, add("a.com", "", key, v, 2))
	if c.CartKey != "" || find(c, v) == nil || find(c, v).Quantity != 3 {
		t.Fatalf("ekoi cart, cart_key abar na: %+v", c)
	}
	if c = f.ok(t, get("a.com", "", key)); find(c, v) == nil {
		t.Fatal("key er cart")
	}

	// bhul format: "nai"; GET khali, POST notun key
	for _, bad := range []string{"short", key[:25] + "1", key + "A", "abcdefghijklmnopqrstuvwxyz"} {
		if c := f.ok(t, get("a.com", "", bad)); c.Count != 0 {
			t.Errorf("bhul key %q e cart pelo", bad)
		}
	}
	c = f.ok(t, add("a.com", "", "short", v, 1))
	if !keyRe.MatchString(c.CartKey) || c.CartKey == key || c.Count != 1 {
		t.Fatalf("bhul key e notun cart: %+v", c)
	}
	// thik format kintu ochena key: client er key diye cart na, notun key
	unknown := rand.Text()
	if c = f.ok(t, get("a.com", "", unknown)); c.Count != 0 {
		t.Fatal("ochena key e cart")
	}
	c = f.ok(t, add("a.com", "", unknown, v, 1))
	if c.CartKey == "" || c.CartKey == unknown {
		t.Fatalf("ochena key e Go er notun key chai: %q", c.CartKey)
	}
	un := sha256.Sum256([]byte(unknown))
	if n := f.id(t, `SELECT count(*) FROM carts WHERE session_key = $1`, un[:]); n != 0 {
		t.Fatal("client er key diye cart holo")
	}

	// Bearer: account cart, key ignore; key er cart bodlay na
	tok := f.customer(t, "a", "+8801711000003")
	if c = f.ok(t, get("a.com", tok, key)); c.Count != 0 {
		t.Fatalf("login e key er cart dekhalo: %+v", c)
	}
	c = f.ok(t, add("a.com", tok, key, v, 5))
	if c.CartKey != "" || find(c, v).Quantity != 5 {
		t.Fatalf("account cart: %+v", c)
	}
	if c = f.ok(t, get("a.com", "", key)); find(c, v).Quantity != 3 {
		t.Fatalf("key er cart bodlalo: %+v", c)
	}
	if n := f.id(t, `SELECT count(*) FROM carts WHERE user_id IS NOT NULL AND session_key IS NULL`); n != 1 {
		t.Errorf("account cart %d", n)
	}

	// bhul ba expire Bearer: 401, guest e fall back na
	f.fail(t, get("a.com", "bhul-token", key), 401, "unauthenticated")
	f.fail(t, add("a.com", "bhul-token", key, v, 1), 401, "unauthenticated")
	ts := sha256.Sum256([]byte(tok))
	f.exec(t, `UPDATE sessions SET expires_at = now() - interval '1 second' WHERE token_hash = $1`, ts[:])
	f.fail(t, get("a.com", tok, key), 401, "session_expired")
}

// TestCartShopIsolationIntegration (14.2 test 5 er cart ongsho, Rule 3): shop A er key/session shop B te
// A er cart pay na; shop B er variant shop A er cart e 404.
func TestCartShopIsolationIntegration(t *testing.T) {
	f := newFixture(t)
	va := f.variant(t, "a", "a1", 100, 10)
	vb := f.variant(t, "b", "b1", 200, 10)
	key := f.ok(t, add("a.com", "", "", va, 1)).CartKey
	tok := f.customer(t, "a", "+8801711000004")
	f.ok(t, add("a.com", tok, "", va, 2))

	if c := f.ok(t, get("b.com", "", key)); c.Count != 0 {
		t.Fatalf("A er key B te: %+v", c)
	}
	c := f.ok(t, add("b.com", "", key, vb, 1))
	if c.CartKey == "" || c.CartKey == key || c.Count != 1 {
		t.Fatalf("B te A er key e notun cart chai: %+v", c)
	}
	if c = f.ok(t, get("a.com", "", key)); c.Count != 1 || find(c, va) == nil {
		t.Fatalf("A er cart bodlalo: %+v", c)
	}
	f.fail(t, get("b.com", tok, ""), 401, "unauthenticated")

	f.fail(t, add("a.com", "", key, vb, 1), 404, "not_found")
	f.fail(t, add("a.com", tok, "", vb, 1), 404, "not_found")
	f.fail(t, patch("a.com", tok, "", vb, 1), 404, "not_found")
	if n := f.id(t, `SELECT count(*) FROM cart_items ci JOIN carts c ON c.id = ci.cart_id WHERE c.shop_id <> ci.shop_id`); n != 0 {
		t.Fatal("shop mile na")
	}
}

// userID: Bearer token er user.
func (f *fixture) userID(t *testing.T, tok string) int64 {
	t.Helper()
	sum := sha256.Sum256([]byte(tok))
	return f.id(t, `SELECT user_id FROM sessions WHERE token_hash = $1`, sum[:])
}

// merge: verify er transaction er moto ek tx e cart.Merge.
func (f *fixture) merge(t *testing.T, s string, user int64, key, locale string) ([]cart.Notice, error) {
	t.Helper()
	var ns []cart.Notice
	err := db.InTx(context.Background(), f.d, func(tx *sql.Tx) error {
		var err error
		ns, err = cart.Merge(context.Background(), tx, f.ids[s], user, key, locale)
		return err
	})
	return ns, err
}

// variants: shop s e n ta product+variant (slug/sku prefix+i, naam "Item <prefix><i>"), stock 10; id kram e.
func (f *fixture) variants(t *testing.T, s, prefix string, n int) []int64 {
	t.Helper()
	var ids []int64
	for i := range n {
		ids = append(ids, f.variant(t, s, fmt.Sprintf("%s%02d", prefix, i), 100, 10))
	}
	return ids
}

// guestKeyCount: key er guest cart row ache kina.
func (f *fixture) guestKeyCount(t *testing.T, key string) int64 {
	t.Helper()
	sum := sha256.Sum256([]byte(key))
	return f.id(t, `SELECT count(*) FROM carts WHERE session_key = $1`, sum[:])
}

// TestCartMergeIntegration (8.4 item 6, v1.29, v1.30; 8.2 step 6): ekoi variant e qty jog, min(jog, 50, stock);
// stock 0 e item thake min(jog, 50); bondho bad (notice na); guest cart muche; 50 variant er beshi hole account
// er item thake, guest er notun 50 porjonto, baki item_dropped; parallel e ekbar; onno shop er key ar bhul
// format key = key nai.
func TestCartMergeIntegration(t *testing.T) {
	f := newFixture(t)
	v1 := f.variant(t, "a", "v1", 100, 100)
	v2 := f.variant(t, "a", "v2", 100, 10)
	v3 := f.variant(t, "a", "v3", 100, 100)
	v4 := f.variant(t, "a", "v4", 100, 100)
	v5 := f.variant(t, "a", "v5", 100, 100)
	key := f.ok(t, add("a.com", "", "", v1, 30)).CartKey
	for _, a := range [][2]int64{{v2, 8}, {v3, 40}, {v4, 2}, {v5, 3}} {
		f.ok(t, add("a.com", "", key, a[0], int(a[1])))
	}
	tok := f.customer(t, "a", "+8801711000001")
	for _, a := range [][2]int64{{v1, 30}, {v2, 5}, {v3, 20}} {
		f.ok(t, add("a.com", tok, "", a[0], int(a[1])))
	}
	f.exec(t, `UPDATE product_variants SET stock = 0 WHERE id = $1`, v3)
	f.exec(t, `UPDATE products SET is_active = false WHERE id = $1`, f.ids["a/v4.product"])

	ns, err := f.merge(t, "a", f.userID(t, tok), key, "en")
	if err != nil || len(ns) != 0 {
		t.Fatalf("merge: %v %+v", err, ns)
	}
	c := f.ok(t, get("a.com", tok, ""))
	want := map[int64]int{v1: 50, v2: 10, v3: 50, v5: 3}
	if c.Count != len(want) || find(c, v4) != nil {
		t.Fatalf("merge er por: %+v", c)
	}
	for v, q := range want {
		if it := find(c, v); it == nil || it.Quantity != q {
			t.Errorf("variant %d: %+v, chai qty %d", v, it, q)
		}
	}
	if it := find(c, v3); it.InStock {
		t.Error("stock 0 er item in_stock")
	}
	if n := f.guestKeyCount(t, key); n != 0 {
		t.Error("guest cart muche nai")
	}
	if c = f.ok(t, get("a.com", "", key)); c.Count != 0 {
		t.Errorf("purono key e cart: %+v", c)
	}

	// 50 er limit: account e 48, guest e (bondho g, account er a00, notun n00..n03) → a00 jog, n00 n01 thake,
	// n02 n03 item_dropped (guest er kram e); bondho slot ney na
	acc := f.variants(t, "a", "a", 48)
	gone := f.variant(t, "a", "gone", 100, 10)
	news := f.variants(t, "a", "n", 4)
	tok2 := f.customer(t, "a", "+8801711000002")
	for _, v := range acc {
		f.ok(t, add("a.com", tok2, "", v, 1))
	}
	key2 := f.ok(t, add("a.com", "", "", gone, 1)).CartKey
	for _, v := range append([]int64{acc[0]}, news...) {
		f.ok(t, add("a.com", "", key2, v, 2))
	}
	f.exec(t, `UPDATE product_variants SET is_active = false WHERE id = $1`, gone)
	ns, err = f.merge(t, "a", f.userID(t, tok2), key2, "en")
	if err != nil {
		t.Fatal(err)
	}
	wantNs := []cart.Notice{{Code: "item_dropped", Name: "Item n02"}, {Code: "item_dropped", Name: "Item n03"}}
	if fmt.Sprint(ns) != fmt.Sprint(wantNs) {
		t.Errorf("notice %+v, chai %+v", ns, wantNs)
	}
	c = f.ok(t, get("a.com", tok2, ""))
	if c.Count != 50 || find(c, acc[0]).Quantity != 3 || find(c, news[1]) == nil || find(c, news[2]) != nil {
		t.Errorf("limit e cart: count %d", c.Count)
	}
	if f.guestKeyCount(t, key2) != 0 {
		t.Error("limit e guest cart muche nai")
	}

	// parallel merge (ekoi key, ekoi user, account cart nai): qty ekbar-i jog
	key3 := f.ok(t, add("a.com", "", "", v1, 2)).CartKey
	tok3 := f.customer(t, "a", "+8801711000003")
	u3 := f.userID(t, tok3)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if _, err := f.merge(t, "a", u3, key3, "en"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if c = f.ok(t, get("a.com", tok3, "")); c.Count != 1 || find(c, v1).Quantity != 2 {
		t.Errorf("parallel merge: %+v", c)
	}

	// onno shop er key, bhul format, key nai: kichu na
	vb := f.variant(t, "b", "b1", 100, 10)
	keyB := f.ok(t, add("b.com", "", "", vb, 1)).CartKey
	tok4 := f.customer(t, "a", "+8801711000004")
	for _, k := range []string{keyB, "short", strings.ToLower(keyB), ""} {
		if ns, err := f.merge(t, "a", f.userID(t, tok4), k, "en"); err != nil || len(ns) != 0 {
			t.Errorf("key %q: %v %+v", k, err, ns)
		}
	}
	if c = f.ok(t, get("a.com", tok4, "")); c.Count != 0 {
		t.Errorf("onno key theke merge: %+v", c)
	}
	if c = f.ok(t, get("b.com", "", keyB)); c.Count != 1 {
		t.Errorf("shop b er cart bodlalo: %+v", c)
	}
}

// TestCartCleanupIntegration (8.17 v1.28, 8.4 item 7, M5 Q11 vi): cart.cleanup 30 din er purono guest cart ar
// tar item muche (duita shop-i); login cart (purono holeo) ar notun guest cart thake; duibar chalale-o thik.
func TestCartCleanupIntegration(t *testing.T) {
	f := newFixture(t)
	va := f.variant(t, "a", "rice", 9000, 10)
	vb := f.variant(t, "b", "honey", 50000, 10)
	oldA := f.ok(t, add("a.com", "", "", va, 1)).CartKey
	newA := f.ok(t, add("a.com", "", "", va, 2)).CartKey
	oldB := f.ok(t, add("b.com", "", "", vb, 1)).CartKey
	newB := f.ok(t, add("b.com", "", "", vb, 1)).CartKey
	tok := f.customer(t, "a", "+8801711000001")
	f.ok(t, add("a.com", tok, "", va, 3))
	user := f.userID(t, tok)

	age := func(key string, days int) {
		sum := sha256.Sum256([]byte(key))
		f.exec(t, `UPDATE carts SET updated_at = now() - make_interval(days => $2) WHERE session_key = $1`, sum[:], days)
	}
	age(oldA, 31)
	age(oldB, 31)
	age(newA, 29)
	age(newB, 29)
	f.exec(t, `UPDATE carts SET updated_at = now() - interval '400 days' WHERE user_id = $1`, user)

	run := cart.CleanupJob(f.d)
	for i := range 2 {
		if err := run(context.Background(), jobs.Job{}); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		for key, want := range map[string]int64{oldA: 0, oldB: 0, newA: 1, newB: 1} {
			if n := f.guestKeyCount(t, key); n != want {
				t.Errorf("run %d: key er cart %d, chai %d", i, n, want)
			}
		}
		if n := f.id(t, `SELECT count(*) FROM carts WHERE user_id = $1`, user); n != 1 {
			t.Errorf("run %d: login cart muche gelo", i)
		}
		// muche jaoa cart er item CASCADE e jay; baki 3 cart e ekta kore item
		if n := f.id(t, `SELECT count(*) FROM cart_items`); n != 3 {
			t.Errorf("run %d: cart_items %d, chai 3", i, n)
		}
	}
	if c := f.ok(t, get("a.com", "", oldA)); c.Count != 0 {
		t.Errorf("muche jaoa key e cart: %+v", c)
	}
	if c := f.ok(t, get("a.com", "", newA)); find(c, va) == nil || find(c, va).Quantity != 2 {
		t.Errorf("notun guest cart: %+v", c)
	}
}
