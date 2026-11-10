package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestShopFromHost (3.2, v1.10): X-Shop-Host (lowercase, port bad) → shop context e; nai, ochena ba
// closed → 404 shop_not_found; suspended → 423 shop_suspended; resolver er error → 500.
func TestShopFromHost(t *testing.T) {
	shops := map[string]Shop{
		"sobujbazar.com": {ID: 1, Slug: "sobujbazar", Status: "active"},
		"bondho.com":     {ID: 2, Status: "suspended"},
		"shesh.com":      {ID: 3, Status: "closed"},
	}
	var asked string
	resolve := func(_ context.Context, domain string) (Shop, bool, error) {
		asked = domain
		if domain == "error.com" {
			return Shop{}, false, errors.New("db down")
		}
		s, ok := shops[domain]
		return s, ok, nil
	}
	var got Shop
	h := ShopFromHost(slog.New(slog.DiscardHandler), resolve)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = ShopFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	cases := []struct {
		host   string
		status int
		code   string
	}{
		{"SobujBazar.com:443", 204, ""},
		{"sobujbazar.com.", 204, ""},
		{"", 404, "shop_not_found"},
		{"nai.com", 404, "shop_not_found"},
		{"shesh.com", 404, "shop_not_found"},
		{"bondho.com", 423, "shop_suspended"},
		{"error.com", 500, "internal"},
	}
	for _, c := range cases {
		got, asked = Shop{}, ""
		r := httptest.NewRequest("GET", "/api/products", nil)
		if c.host != "" {
			r.Header.Set("X-Shop-Host", c.host)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != c.status || (c.code != "" && errCode(t, rec) != c.code) {
			t.Errorf("%q: %d %s", c.host, rec.Code, rec.Body.String())
		}
		if c.status == 204 && (got.ID != 1 || asked != "sobujbazar.com") {
			t.Errorf("%q: shop %+v, resolver e %q", c.host, got, asked)
		}
	}
	if _, ok := ShopFrom(context.Background()); ok {
		t.Error("middleware chhara shop ache")
	}
}
