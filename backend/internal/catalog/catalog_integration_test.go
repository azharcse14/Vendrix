package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/shop"
)

// fixture: duita shop (a.com, b.com), protitir house vendor; id gulo map e.
type fixture struct {
	d   *sql.DB
	srv http.Handler
	ids map[string]int64 // "a", "b" (shop), "a.vendor", "b.vendor", ar insert kora jinish er naam
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	d := dbtest.New(t)
	ctx := context.Background()
	f := &fixture{d: d, ids: map[string]int64{}}
	for _, s := range []string{"a", "b"} {
		id, _, err := shop.Create(ctx, d, shop.Params{Slug: s, Prefix: "S" + string(s[0]-32), Domain: s + ".com",
			NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		f.ids[s] = id
		f.ids[s+".vendor"] = f.id(t, `SELECT id FROM vendors WHERE shop_id = $1 AND is_house`, id)
	}
	h := NewHandlers(d, slog.New(slog.DiscardHandler))
	mux := http.NewServeMux()
	h.Routes(mux, httpx.ShopFromHost(slog.New(slog.DiscardHandler), shop.NewResolver(d).Resolve))
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

// category: shop s e category; parent "" = root.
func (f *fixture) category(t *testing.T, s, slug, parent, bn string, active bool) {
	t.Helper()
	var p any
	if parent != "" {
		p = f.ids[parent]
	}
	f.ids[slug] = f.id(t, `INSERT INTO categories (shop_id, parent_id, slug, name, is_active)
		VALUES ($1, $2, $3, jsonb_build_object('bn', $4::text, 'en', $3::text), $5) RETURNING id`, f.ids[s], p, slug, bn, active)
}

// product: shop s e product ar ekta variant (price, stock); key = s + "/" + slug.
func (f *fixture) product(t *testing.T, s, slug, bn, en, keywords, category string, price int64, stock int, active bool) int64 {
	t.Helper()
	var c any
	if category != "" {
		c = f.ids[category]
	}
	id := f.id(t, `INSERT INTO products (shop_id, vendor_id, category_id, slug, name, search_keywords, is_active)
		VALUES ($1, $2, $3, $4, jsonb_build_object('bn', $5::text, 'en', $6::text), $7, $8) RETURNING id`,
		f.ids[s], f.ids[s+".vendor"], c, slug, bn, en, keywords, active)
	f.ids[s+"/"+slug] = id
	f.variant(t, s, slug, slug+"-1", price, stock, true)
	return id
}

func (f *fixture) variant(t *testing.T, s, product, sku string, price int64, stock int, active bool) {
	t.Helper()
	f.ids[s+"/"+sku] = f.id(t, `INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`, f.ids[s], f.ids[s+"/"+product], f.ids[s+".vendor"], sku, price, stock, active)
}

func (f *fixture) get(t *testing.T, host, path, lang string, want int, out any) {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("X-Shop-Host", host)
	if lang != "" {
		r.Header.Set("Accept-Language", lang)
	}
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r)
	if rec.Code != want {
		t.Fatalf("%s %s: %d %s", host, path, rec.Code, rec.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatal(err)
		}
	}
}

type list struct {
	Items []productCardJSON
	Page  int
	Limit int
	Total int64
}

func (f *fixture) slugs(t *testing.T, host, query string) []string {
	t.Helper()
	var l list
	f.get(t, host, "/api/products?"+query, "", 200, &l)
	var out []string
	for _, p := range l.Items {
		out = append(out, p.Slug)
	}
	return out
}

// TestCategoryTreeIntegration (7.2): active category er tree, sort_order kram, inactive (ar tar child) nai,
// naam Accept-Language e.
func TestCategoryTreeIntegration(t *testing.T) {
	f := newFixture(t)
	f.category(t, "a", "chal", "", "চাল", true)
	f.category(t, "a", "lal-chal", "chal", "লাল চাল", true)
	f.category(t, "a", "lukano", "", "লুকানো", false)
	f.category(t, "a", "lukano-child", "lukano", "লুকানো সন্তান", true)
	f.category(t, "b", "b-only", "", "বি", true)
	var tree []category
	f.get(t, "a.com", "/api/categories", "", 200, &tree)
	if len(tree) != 1 || tree[0].Slug != "chal" || tree[0].Name != "চাল" || len(tree[0].Children) != 1 ||
		tree[0].Children[0].Name != "লাল চাল" {
		t.Errorf("tree: %+v", tree)
	}
	f.get(t, "a.com", "/api/categories", "en", 200, &tree)
	if tree[0].Name != "chal" {
		t.Errorf("en: %+v", tree)
	}
}

// TestListProductsIntegration (7.2, 7.1): filter, sort, paging; inactive product ba variant chhara product nai.
func TestListProductsIntegration(t *testing.T) {
	f := newFixture(t)
	f.category(t, "a", "chal", "", "চাল", true)
	f.product(t, "a", "lal-chal", "লাল চাল", "Red Rice", "", "chal", 9000, 10, true)
	f.product(t, "a", "modhu", "মধু", "Honey", "", "", 50000, 0, true)
	f.variant(t, "a", "modhu", "modhu-boro", 120000, 3, true)
	f.product(t, "a", "ghee", "ঘি", "Ghee", "", "", 80000, 5, true)
	f.product(t, "a", "bondho", "বন্ধ", "Off", "", "", 100, 5, false)
	f.product(t, "a", "variant-bondho", "ভ্যারিয়েন্ট বন্ধ", "No variant", "", "", 100, 5, true)
	if _, err := f.d.Exec(`UPDATE product_variants SET is_active = false WHERE id = $1`, f.ids["a/variant-bondho-1"]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.d.Exec(`UPDATE products SET review_count = 4, rating_sum = 18 WHERE id = $1`, f.ids["a/ghee"]); err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string][]string{
		"":                           {"ghee", "modhu", "lal-chal"},
		"category=chal":              {"lal-chal"},
		"category=nai":               nil,
		"sort=price_asc":             {"lal-chal", "modhu", "ghee"},
		"sort=price_desc":            {"ghee", "modhu", "lal-chal"},
		"sort=popular":               {"ghee", "modhu", "lal-chal"},
		"min_price=100000":           {"modhu"},
		"max_price=60000":            {"modhu", "lal-chal"},
		"in_stock=true":              {"ghee", "modhu", "lal-chal"},
		"vendor=a":                   {"ghee", "modhu", "lal-chal"},
		"vendor=b":                   nil,
		"limit=2&page=2":             {"lal-chal"},
		"min_price=1&max_price=9000": {"lal-chal"},
	} {
		if got := f.slugs(t, "a.com", q); !slices.Equal(got, want) {
			t.Errorf("%q: %v, chai %v", q, got, want)
		}
	}
	var l list
	f.get(t, "a.com", "/api/products?limit=2", "en", 200, &l)
	if l.Total != 3 || l.Limit != 2 || l.Page != 1 || l.Items[0].Name != "Ghee" || l.Items[0].Rating != 4.5 ||
		l.Items[1].Price != 50000 || l.Items[1].InStock != true {
		t.Errorf("list: %+v", l)
	}
	for _, q := range []string{"sort=nai", "min_price=-1", "max_price=x", "in_stock=ha", "limit=101", "page=0"} {
		f.get(t, "a.com", "/api/products?"+q, "", 400, nil)
	}
}

// TestGetProductIntegration (7.2): active variant ar chobi sort_order e, locale e lekha; inactive → 404.
func TestGetProductIntegration(t *testing.T) {
	f := newFixture(t)
	f.category(t, "a", "chal", "", "চাল", true)
	id := f.product(t, "a", "lal-chal", "লাল চাল", "Red Rice", "", "chal", 9000, 0, true)
	f.variant(t, "a", "lal-chal", "lal-chal-5kg", 42000, 2, true)
	f.variant(t, "a", "lal-chal", "lal-chal-off", 1, 2, false)
	f.product(t, "a", "bondho", "বন্ধ", "Off", "", "", 100, 5, false)
	for _, q := range []string{
		`UPDATE products SET description = '{"bn":"বিবরণ","en":"About"}', seo = '{"bn":{"title":"শিরোনাম"},"en":{"title":"Title"}}',
		  attributes = '{"origin":"Dinajpur"}' WHERE id = $1 AND shop_id = $2`,
		`UPDATE product_variants SET options = '{"weight":{"bn":"১ কেজি","en":"1 kg"}}', sort_order = 1
		  WHERE product_id = $1 AND shop_id = $2 AND sku = 'lal-chal-1'`,
		`INSERT INTO product_images (shop_id, product_id, storage_key, alt, width, height, sort_order) VALUES
		  ($2, $1, 'shops/a/2.jpg', '{"bn":"দুই"}', 10, 10, 2), ($2, $1, 'shops/a/1.jpg', '{"bn":"এক"}', 10, 10, 1)`,
	} {
		if _, err := f.d.Exec(q, id, f.ids["a"]); err != nil {
			t.Fatal(err)
		}
	}
	var p productJSON
	f.get(t, "a.com", "/api/products/lal-chal", "en", 200, &p)
	if p.Name != "Red Rice" || p.Description != "About" || string(p.SEO) != `{"title":"Title"}` || p.Category == nil ||
		*p.Category != "chal" || len(p.Variants) != 2 || p.Variants[0].SKU != "lal-chal-5kg" || !p.Variants[0].InStock ||
		p.Variants[1].Options["weight"] != "1 kg" || p.Variants[1].InStock || len(p.Images) != 2 ||
		p.Images[0].StorageKey != "shops/a/1.jpg" || p.Images[0].Alt != "এক" {
		t.Errorf("product: %+v", p)
	}
	f.get(t, "a.com", "/api/products/bondho", "", 404, nil)
	f.get(t, "a.com", "/api/products/nai", "", 404, nil)
}

// TestSearchIntegration (6.4): Bangla, juktakkhor, English, Banglish keyword, NFC, `%`/`_` escape,
// 2 okkhorer kom hole search na; q thakle default relevance.
func TestSearchIntegration(t *testing.T) {
	f := newFixture(t)
	f.product(t, "a", "lal-chal", "লাল চাল", "Red Rice", "lal chal", "", 9000, 1, true)
	f.product(t, "a", "chinigura-chal", "চিনিগুড়া চাল", "Chinigura Rice", "chinigura chal", "", 15000, 1, true)
	f.product(t, "a", "modhu", "অর্গানিক মধু", "Organic Honey", "modhu", "", 50000, 1, true)
	f.product(t, "a", "ghee", "১০০% খাঁটি ঘি", "100% Pure Ghee", "ghi", "", 80000, 1, true)
	f.product(t, "a", "mug", "সোনা মুগ ডাল", "Sona Mug Dal", "", "", 14000, 1, true)
	f.product(t, "b", "b-chal", "চাল", "Rice", "", "", 1, 1, true)
	for q, want := range map[string][]string{
		"চাল":      {"lal-chal", "chinigura-chal"},
		"লাল চাল":  {"lal-chal"},
		"অর্গানিক": {"modhu"},
		"rice":     {"lal-chal", "chinigura-chal"},
		"RICE":     {"lal-chal", "chinigura-chal"},
		"lal chal": {"lal-chal"},
		"modhu":    {"modhu"},
		"সোনা":    {"mug"},
		"0%":       {"ghee"},
		"0_":       nil,
		"%%":       nil,
		"নেই":      nil,
	} {
		got := f.slugs(t, "a.com", "q="+url.QueryEscape(q))
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%q: %v, chai %v", q, got, want)
		}
	}
	if got := f.slugs(t, "a.com", "q="+url.QueryEscape("চ")); len(got) != 5 {
		t.Errorf("1 okkhor e search hoyeche: %v", got)
	}
	if got := f.slugs(t, "a.com", "q=rice&sort=price_desc"); !slices.Equal(got, []string{"chinigura-chal", "lal-chal"}) {
		t.Errorf("q + sort: %v", got)
	}
}

// TestSearchRankingIntegration (6.4 v1.7, M1 acceptance): q thakle default kram relevance: puro shobdo ba
// shobder shuru te mil age, shobder majhe mil pore (Bangla, juktakkhor, English).
func TestSearchRankingIntegration(t *testing.T) {
	f := newFixture(t)
	// agee banano = notun na, tai relevance na thakle created_at DESC e ulto kram ashto
	for _, p := range [][3]string{
		{"modhu", "অর্গানিক মধু", "Organic Honey"},
		{"modhumoti-chal", "মধুমতি চাল", "Modhumoti Rice"},
		{"achar", "আমের আচার", "Mango Pickle"},
		{"lal-chal", "লাল চাল", "Red Rice"},
		{"ice-cream", "আইসক্রিম", "Ice Cream"},
		{"khati-modhu", "খাঁটি মধু", "Pure Honey"},
	} {
		f.product(t, "a", p[0], p[1], p[2], "", "", 100, 1, true)
	}
	for q, want := range map[string][]string{
		"মধু":      {"khati-modhu", "modhu", "modhumoti-chal"},
		"চা":       {"lal-chal", "modhumoti-chal", "achar"},
		"ice":      {"ice-cream", "lal-chal", "modhumoti-chal"},
		"অর্গানিক": {"modhu"},
	} {
		got := f.slugs(t, "a.com", "q="+url.QueryEscape(q))
		// khati-modhu ar modhu duijon-er word_similarity 1; tie e notun age
		if !slices.Equal(got, want) {
			t.Errorf("%q: %v, chai %v", q, got, want)
		}
	}
}

// TestShopIsolationIntegration (Rule 3, 14.2 test 5 er ongsho): ek slug duita shop e; protiti host e
// shudhu nijer category, product; onno shop er slug 404.
func TestShopIsolationIntegration(t *testing.T) {
	f := newFixture(t)
	f.category(t, "a", "a-cat", "", "এ", true)
	f.category(t, "b", "b-cat", "", "বি", true)
	f.product(t, "a", "chal", "এ চাল", "A Rice", "", "a-cat", 100, 1, true)
	f.product(t, "b", "chal", "বি চাল", "B Rice", "", "b-cat", 200, 1, true)
	f.product(t, "b", "b-only", "বি", "B", "", "", 200, 1, true)
	if got := f.slugs(t, "a.com", ""); !slices.Equal(got, []string{"chal"}) {
		t.Errorf("a list: %v", got)
	}
	if got := f.slugs(t, "a.com", "category=b-cat"); got != nil {
		t.Errorf("a te b er category: %v", got)
	}
	var p productJSON
	f.get(t, "a.com", "/api/products/chal", "", 200, &p)
	if p.Name != "এ চাল" || p.Variants[0].Price != 100 {
		t.Errorf("a product: %+v", p)
	}
	f.get(t, "a.com", "/api/products/b-only", "", 404, nil)
	var tree []category
	f.get(t, "a.com", "/api/categories", "", 200, &tree)
	if len(tree) != 1 || tree[0].Slug != "a-cat" {
		t.Errorf("a tree: %+v", tree)
	}
}

// TestTenantForeignKeyIntegration (14.2 test 13): shop A er product e B er category ba vendor, variant e B er
// product → DB foreign key error (23503).
func TestTenantForeignKeyIntegration(t *testing.T) {
	f := newFixture(t)
	f.category(t, "b", "b-cat", "", "বি", true)
	f.product(t, "b", "b-chal", "বি", "B", "", "", 1, 1, true)
	for name, q := range map[string][]any{
		"B er category": {`INSERT INTO products (shop_id, vendor_id, category_id, slug, name) VALUES ($1, $2, $3, 'x', '{"bn":"x"}')`,
			f.ids["a"], f.ids["a.vendor"], f.ids["b-cat"]},
		"B er vendor": {`INSERT INTO products (shop_id, vendor_id, slug, name) VALUES ($1, $2, 'y', '{"bn":"y"}')`,
			f.ids["a"], f.ids["b.vendor"]},
		"B er product": {`INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price) VALUES ($1, $2, $3, 'z', 1)`,
			f.ids["a"], f.ids["b/b-chal"], f.ids["a.vendor"]},
		"B er parent": {`INSERT INTO categories (shop_id, parent_id, slug, name) VALUES ($1, $2, 'p', '{"bn":"p"}')`,
			f.ids["a"], f.ids["b-cat"]},
	} {
		_, err := f.d.Exec(q[0].(string), q[1:]...)
		var pg interface{ SQLState() string }
		if !errors.As(err, &pg) || pg.SQLState() != "23503" {
			t.Errorf("%s: %v", name, err)
		}
	}
}
