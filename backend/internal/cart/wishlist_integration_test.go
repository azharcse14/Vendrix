package cart_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"testing"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/shop"
)

// wishlistFixture: cart er fixture, route router er moto (public + RequireCustomer).
func wishlistFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	log := slog.New(slog.DiscardHandler)
	public := httpx.ShopFromHost(log, shop.NewResolver(f.d).Resolve)
	customer := httpx.RequireCustomer(log, auth.LookupSession(f.d))
	mux := http.NewServeMux()
	cart.NewHandlers(f.d, log).WishlistRoutes(mux, func(h http.Handler) http.Handler { return public(customer(h)) })
	f.srv = httpx.JSONErrors(mux)
	return f
}

type wishCard struct {
	ID             int64  `json:"id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Price          int64  `json:"price"`
	CompareAtPrice *int64 `json:"compare_at_price"`
	InStock        bool   `json:"in_stock"`
}

type wishJSON struct {
	Items []wishCard `json:"items"`
}

func (w wishJSON) has(id int64) bool {
	for _, c := range w.Items {
		if c.ID == id {
			return true
		}
	}
	return false
}

// wish: 200, no-store, card list.
func (f *fixture) wish(t *testing.T, r req) wishJSON {
	t.Helper()
	rec := f.do(t, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s: %d %s", r.method, r.path, rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("%s %s: Cache-Control %q", r.method, r.path, cc)
	}
	var w wishJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &w); err != nil {
		t.Fatal(err)
	}
	return w
}

func wishPath(id any) string { return fmt.Sprintf("/api/wishlist/%v", id) }

func (f *fixture) wishRows(t *testing.T, s string, user int64) int64 {
	return f.id(t, `SELECT count(*) FROM wishlists WHERE shop_id = $1 AND user_id = $2`, f.ids[s], user)
}

// fillWishlist: user er wishlist e n ta (variant chhara, tai GET e lukano) product er row.
func (f *fixture) fillWishlist(t *testing.T, s string, user int64, prefix string, n int) {
	t.Helper()
	f.exec(t, `WITH p AS (INSERT INTO products (shop_id, vendor_id, slug, name)
		  SELECT $1, $2, $3 || g, '{"bn":"x","en":"x"}' FROM generate_series(1, $4::int) g RETURNING id)
		INSERT INTO wishlists (shop_id, user_id, product_id) SELECT $1, $5, id FROM p`,
		f.ids[s], f.ids[s+".vendor"], prefix, n, user)
}

// TestWishlistIntegration (7.4, 8.4 item 8, Rule 16): login chhara 401; protiti query user_id + shop_id
// (onno user/shop er wishlist ba product na); duibar add e ek row; bondho product GET e lukano (row thake);
// 200 e notun add 409 wishlist_full, ager product e 200; parallel add e o 200 er beshi na.
func TestWishlistIntegration(t *testing.T) {
	f := wishlistFixture(t)
	tokA := f.customer(t, "a", "+8801711000001")
	tokB := f.customer(t, "a", "+8801711000002")
	tokOther := f.customer(t, "b", "+8801711000001")
	userA, userB := f.userID(t, tokA), f.userID(t, tokB)
	f.variant(t, "a", "p1", 1000, 5)
	f.variant(t, "a", "p2", 500, 0)
	f.variant(t, "b", "x1", 900, 5)
	p1, p2 := f.ids["a/p1.product"], f.ids["a/p2.product"]
	x1 := f.ids["b/x1.product"]

	for _, r := range []req{
		{method: "GET", path: "/api/wishlist", host: "a.com"},
		{method: "POST", path: wishPath(p1), host: "a.com"},
		{method: "DELETE", path: wishPath(p1), host: "a.com"},
		{method: "POST", path: "/api/wishlist/merge", host: "a.com", body: map[string]any{"product_ids": []int64{p1}}},
		{method: "GET", path: "/api/wishlist", host: "a.com", token: "BHUL"},
		// onno shop er session ei shop e chole na (3.2)
		{method: "POST", path: wishPath(p1), host: "a.com", token: tokOther},
	} {
		f.fail(t, r, 401, "unauthenticated")
	}

	// duibar add: ek row (ON CONFLICT DO NOTHING)
	f.wish(t, req{method: "POST", path: wishPath(p1), host: "a.com", token: tokA})
	w := f.wish(t, req{method: "POST", path: wishPath(p1), host: "a.com", token: tokA, lang: "en"})
	if n := f.wishRows(t, "a", userA); n != 1 || !w.has(p1) {
		t.Fatalf("duibar add: %d row, %+v", n, w)
	}
	w = f.wish(t, req{method: "POST", path: wishPath(p2), host: "a.com", token: tokA, lang: "en"})
	if len(w.Items) != 2 || w.Items[0].ID != p2 || w.Items[1].Name != "Item p1" || w.Items[1].Price != 1000 ||
		!w.Items[1].InStock || w.Items[0].InStock {
		t.Fatalf("GET card (notun age, in_stock): %+v", w)
	}

	// onno user: A er wishlist dekhe na, muchte pare na
	if w := f.wish(t, req{method: "GET", path: "/api/wishlist", host: "a.com", token: tokB}); len(w.Items) != 0 {
		t.Fatalf("B A er wishlist dekhe: %+v", w)
	}
	f.wish(t, req{method: "DELETE", path: wishPath(p1), host: "a.com", token: tokB})
	if n := f.wishRows(t, "a", userA); n != 2 {
		t.Fatalf("B er DELETE e A er row gelo: %d", n)
	}
	// onno shop er product (IDOR): 404, row na
	f.fail(t, req{method: "POST", path: wishPath(x1), host: "a.com", token: tokA}, 404, "not_found")
	if n := f.id(t, `SELECT count(*) FROM wishlists WHERE product_id = $1`, x1); n != 0 {
		t.Fatalf("onno shop er product wishlist e: %d", n)
	}
	for _, bad := range []string{"abc", "0", "-3"} {
		f.fail(t, req{method: "POST", path: wishPath(bad), host: "a.com", token: tokA}, 400, "validation_failed")
	}

	// bondho product: GET e lukano, row thake, chalu hole ferot; notun add 404
	f.exec(t, `UPDATE products SET is_active = false WHERE id = $1`, p1)
	if w := f.wish(t, req{method: "GET", path: "/api/wishlist", host: "a.com", token: tokA}); w.has(p1) || !w.has(p2) {
		t.Fatalf("bondho product dekhay: %+v", w)
	}
	f.fail(t, req{method: "POST", path: wishPath(p1), host: "a.com", token: tokB}, 404, "not_found")
	f.exec(t, `UPDATE products SET is_active = true WHERE id = $1`, p1)
	if w := f.wish(t, req{method: "GET", path: "/api/wishlist", host: "a.com", token: tokA}); !w.has(p1) {
		t.Fatalf("chalu product ferot ashe na: %+v", w)
	}

	// DELETE idempotent
	f.wish(t, req{method: "DELETE", path: wishPath(p2), host: "a.com", token: tokA})
	if w := f.wish(t, req{method: "DELETE", path: wishPath(p2), host: "a.com", token: tokA}); w.has(p2) || !w.has(p1) {
		t.Fatalf("DELETE: %+v", w)
	}

	// 200: notun add 409 wishlist_full, ager product 200
	f.fillWishlist(t, "a", userA, "full", 199)
	f.fail(t, req{method: "POST", path: wishPath(p2), host: "a.com", token: tokA}, 409, "wishlist_full")
	f.wish(t, req{method: "POST", path: wishPath(p1), host: "a.com", token: tokA})
	if n := f.wishRows(t, "a", userA); n != 200 {
		t.Fatalf("200 er por %d row", n)
	}

	// parallel: 198 row e 5 ta notun add, mot 200 er beshi na
	f.fillWishlist(t, "a", userB, "race", 198)
	var ps []int64
	for i := range 5 {
		f.variant(t, "a", fmt.Sprintf("r%d", i), 100, 1)
		ps = append(ps, f.ids[fmt.Sprintf("a/r%d.product", i)])
	}
	var wg sync.WaitGroup
	for _, p := range ps {
		wg.Go(func() { f.do(t, req{method: "POST", path: wishPath(p), host: "a.com", token: tokB}) })
	}
	wg.Wait()
	if n := f.wishRows(t, "a", userB); n != 200 {
		t.Fatalf("parallel add er por %d row, chai 200", n)
	}
}

func wishMerge(token string, body any) req {
	return req{method: "POST", path: "/api/wishlist/merge", host: "a.com", token: token, body: body}
}

// TestWishlistMergeIntegration (7.4, 8.4 item 8, 10.12): guest cookie er ID e shudhu oi shop er active product;
// ochena/bhul ID bad; prothom 50 ID; wishlist e mot 200; body bhul hole 400.
func TestWishlistMergeIntegration(t *testing.T) {
	f := wishlistFixture(t)
	tokA := f.customer(t, "a", "+8801711000001")
	tokC := f.customer(t, "a", "+8801711000003")
	tokD := f.customer(t, "a", "+8801711000004")
	userA, userC, userD := f.userID(t, tokA), f.userID(t, tokC), f.userID(t, tokD)
	f.variant(t, "a", "p1", 1000, 5)
	f.variant(t, "a", "p2", 500, 5)
	f.variant(t, "a", "off", 500, 5)
	f.variant(t, "b", "x1", 900, 5)
	p1, p2, off, x1 := f.ids["a/p1.product"], f.ids["a/p2.product"], f.ids["a/off.product"], f.ids["b/x1.product"]
	f.exec(t, `UPDATE products SET is_active = false WHERE id = $1`, off)
	// variant chhara product: card nai, tai active na
	novar := f.id(t, `INSERT INTO products (shop_id, vendor_id, slug, name) VALUES ($1, $2, 'novar', '{"bn":"x"}')
		RETURNING id`, f.ids["a"], f.ids["a.vendor"])

	f.wish(t, req{method: "POST", path: wishPath(p1), host: "a.com", token: tokA})
	w := f.wish(t, wishMerge(tokA, map[string]any{"product_ids": []int64{p1, p2, p2, x1, off, novar, 999999999, -1, 0}}))
	if n := f.wishRows(t, "a", userA); n != 2 || !w.has(p1) || !w.has(p2) || len(w.Items) != 2 {
		t.Fatalf("merge: %d row, %+v", n, w)
	}
	if n := f.id(t, `SELECT count(*) FROM wishlists WHERE product_id = ANY($1)`, fmt.Sprintf("{%d,%d,%d}", x1, off, novar)); n != 0 {
		t.Fatalf("onno shop/bondho product merge holo: %d", n)
	}
	// khali list thik
	f.wish(t, wishMerge(tokA, map[string]any{"product_ids": []int64{}}))

	// 51 ID: prothom 50
	var ids []int64
	for i := range 51 {
		f.variant(t, "a", fmt.Sprintf("m%d", i), 100, 1)
		ids = append(ids, f.ids[fmt.Sprintf("a/m%d.product", i)])
	}
	f.wish(t, wishMerge(tokC, map[string]any{"product_ids": ids}))
	if n := f.wishRows(t, "a", userC); n != 50 {
		t.Fatalf("51 ID merge e %d row, chai 50", n)
	}
	if n := f.id(t, `SELECT count(*) FROM wishlists WHERE user_id = $1 AND product_id = $2`, userC, ids[50]); n != 0 {
		t.Fatal("51 tomo ID merge holo")
	}

	// mot 200: 190 thakle 20 er moddhe 10
	f.fillWishlist(t, "a", userD, "d", 190)
	f.wish(t, wishMerge(tokD, map[string]any{"product_ids": ids[:20]}))
	if n := f.wishRows(t, "a", userD); n != 200 {
		t.Fatalf("mot %d row, chai 200", n)
	}

	for _, body := range []any{`{"product_ids":["1"]}`, `{"product_ids":[1],"x":1}`, `nope`, `{}`} {
		f.fail(t, wishMerge(tokA, body), 400, "validation_failed")
	}
}
