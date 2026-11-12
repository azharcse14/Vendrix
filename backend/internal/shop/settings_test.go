package shop

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
)

// TestPublicSettings (7.2, 9.2, v1.10): whitelist er block ache, vitorer setting (risk, otp_mode,
// sms_daily_cap, inventory, commission) nai; ochena notun block o nai.
func TestPublicSettings(t *testing.T) {
	full, err := DefaultSettings("organic", map[string]string{"bn": "সবুজ"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(full, &m)
	m["notun_block"] = map[string]any{"secret": 1}
	full, _ = json.Marshal(m)

	pub, err := PublicSettings(full)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(pub)
	s := string(b)
	for _, want := range []string{`"branding"`, `"theme"`, `"locales"`, `"features"`, `"homepage"`, `"seo"`, `"integrations"`,
		`"payment_methods"`, `"return_window_days"`, `"show_vendor_name"`} {
		if !strings.Contains(s, want) {
			t.Errorf("%s nai: %s", want, s)
		}
	}
	for _, no := range []string{"risk", "otp_mode", "cod_confirmation", "auto_cancel", "sms_daily_cap", "inventory",
		"delivery_charge_mode", "default_commission_bps", "notun_block"} {
		if strings.Contains(s, no) {
			t.Errorf("%s public e: %s", no, s)
		}
	}
	if _, err := PublicSettings([]byte("{")); err == nil {
		t.Error("bhul JSON")
	}
}

// TestSettingsIsolationIntegration (Rule 3, 7.2): duita shop er host e nijer published settings ar version;
// draft public e na.
func TestSettingsIsolationIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	for _, p := range []Params{
		{Slug: "a", Prefix: "AA", Domain: "a.com", NameBn: "দোকান এ", OwnerEmail: "o@a.com", OwnerName: "A", Preset: "organic"},
		{Slug: "b", Prefix: "BB", Domain: "b.com", NameBn: "দোকান বি", OwnerEmail: "o@b.com", OwnerName: "B", Preset: "fashion"},
	} {
		if _, _, err := Create(ctx, d, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.Exec(`UPDATE shop_settings SET version = 7, draft = jsonb_set(draft, '{branding,name,bn}', '"খসড়া"')
		WHERE shop_id = (SELECT id FROM shops WHERE slug = 'b')`); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(d, slog.New(slog.DiscardHandler))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/settings", h.Settings)
	mux.HandleFunc("GET /api/settings/version", h.SettingsVersion)
	mux.HandleFunc("GET /api/policies/{kind}", h.Policy)
	srv := httpx.ShopFromHost(slog.New(slog.DiscardHandler), NewResolver(d).Resolve)(mux)
	get := func(host, path string) string {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("X-Shop-Host", host)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, r)
		if rec.Code != 200 {
			t.Fatalf("%s %s: %d %s", host, path, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	a, b := get("a.com", "/api/settings"), get("b.com", "/api/settings")
	if !strings.Contains(a, "দোকান এ") || strings.Contains(a, "দোকান বি") || !strings.Contains(a, "#1F5A32") {
		t.Errorf("a: %s", a)
	}
	if !strings.Contains(b, "দোকান বি") || strings.Contains(b, "খসড়া") || !strings.Contains(b, "#2A3480") {
		t.Errorf("b (draft public e na): %s", b)
	}
	if v := strings.TrimSpace(get("b.com", "/api/settings/version")); v != `{"version":7}` {
		t.Errorf("version: %s", v)
	}

	// Policy (v1.13): shop er nijer published lekha; settings payload e na; ochena kind 404.
	if _, err := d.Exec(`UPDATE shop_settings SET published = jsonb_set(published, '{policies,terms,bn}', '"এ এর শর্ত"')
		WHERE shop_id = (SELECT id FROM shops WHERE slug = 'a')`); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(a, "policies") {
		t.Errorf("settings e policies: %s", a)
	}
	if p := get("a.com", "/api/policies/terms"); !strings.Contains(p, `"body":"এ এর শর্ত"`) {
		t.Errorf("a terms: %s", p)
	}
	if p := get("b.com", "/api/policies/terms"); !strings.Contains(p, policyPlaceholder) {
		t.Errorf("b terms: %s", p)
	}
	r := httptest.NewRequest("GET", "/api/policies/faq", nil)
	r.Header.Set("X-Shop-Host", "a.com")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != 404 {
		t.Errorf("faq: %d", rec.Code)
	}
}
