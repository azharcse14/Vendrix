package shop

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
)

// TestTranslationsIntegration (7.2, 9.8): shop er override {key: value}, shudhu oi shop ar locale er;
// override na thakle {}; locale bn/en chhara 400.
func TestTranslationsIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	ids := map[string]int64{}
	for _, slug := range []string{"a", "b"} {
		id, _, err := Create(ctx, d, Params{Slug: slug, Prefix: strings.ToUpper(slug + slug), Domain: slug + ".com", NameBn: "ক",
			OwnerEmail: "o@" + slug + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		ids[slug] = id
	}
	if _, err := d.Exec(`INSERT INTO translations (shop_id, locale, key, value) VALUES
		($1, 'bn', 'cart.add', 'ব্যাগে নিন'), ($1, 'en', 'cart.add', 'Add to bag'), ($2, 'bn', 'cart.add', 'অন্য দোকান')`,
		ids["a"], ids["b"]); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/translations", NewHandlers(d, slog.New(slog.DiscardHandler)).Translations)
	srv := httpx.ShopFromHost(slog.New(slog.DiscardHandler), NewResolver(d).Resolve)(mux)
	get := func(host, q string) (int, string) {
		r := httptest.NewRequest("GET", "/api/translations"+q, nil)
		r.Header.Set("X-Shop-Host", host)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, r)
		return rec.Code, strings.TrimSpace(rec.Body.String())
	}
	for _, c := range []struct{ host, q, want string }{
		{"a.com", "?locale=bn", `{"cart.add":"ব্যাগে নিন"}`},
		{"a.com", "?locale=en", `{"cart.add":"Add to bag"}`},
		{"b.com", "?locale=bn", `{"cart.add":"অন্য দোকান"}`},
		{"b.com", "?locale=en", `{}`},
	} {
		if code, body := get(c.host, c.q); code != 200 || body != c.want {
			t.Errorf("%s%s: %d %s", c.host, c.q, code, body)
		}
	}
	for _, q := range []string{"", "?locale=fr"} {
		if code, body := get("a.com", q); code != 400 || !strings.Contains(body, `"locale"`) {
			t.Errorf("%q: %d %s", q, code, body)
		}
	}
}
