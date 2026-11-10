package shop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/dbtest"
)

// TestDefaultSettings (9.2, 9.4, 14.4): preset er rong, font, radius; noindex true; ochena preset e error.
func TestDefaultSettings(t *testing.T) {
	name := map[string]string{"bn": "সবুজ বাজার", "en": "Sobuj Bazar"}
	for preset, want := range map[string][3]string{
		"organic":     {"#1F5A32", "Baloo Da 2", "14"},
		"fashion":     {"#2A3480", "Noto Serif Bengali", "2"},
		"electronics": {"#0A5BB5", "Anek Bangla", "10"},
	} {
		b, err := DefaultSettings(preset, name)
		if err != nil {
			t.Fatal(err)
		}
		var s struct {
			Branding struct{ Name map[string]string }
			Theme    struct {
				Preset string
				Colors map[string]string
				Fonts  map[string]string
				Radius json.Number
			}
			Locales  struct{ Default string }
			Checkout struct {
				PaymentMethods []string `json:"payment_methods"`
			}
			Seo struct{ Noindex bool }
		}
		if err := json.Unmarshal(b, &s); err != nil {
			t.Fatal(err)
		}
		if s.Theme.Preset != preset || s.Theme.Colors["primary"] != want[0] || s.Theme.Fonts["heading"] != want[1] ||
			s.Theme.Radius.String() != want[2] || s.Theme.Fonts["body"] != "Hind Siliguri" || len(s.Theme.Colors) != 5 {
			t.Errorf("%s: %+v", preset, s.Theme)
		}
		if s.Branding.Name["bn"] != "সবুজ বাজার" || s.Locales.Default != "bn" || !s.Seo.Noindex ||
			strings.Join(s.Checkout.PaymentMethods, ",") != "cod" {
			t.Errorf("%s: %s", preset, b)
		}
		for _, block := range []string{"branding", "theme", "locales", "features", "checkout", "inventory", "marketplace", "homepage", "seo", "integrations"} {
			if !strings.Contains(string(b), `"`+block+`":`) {
				t.Errorf("%s: %s block nai", preset, block)
			}
		}
	}
	if _, err := DefaultSettings("nai", name); err == nil {
		t.Error("ochena preset")
	}
}

// TestCreateShopIntegration (3.8, v1.10): ek transaction e shop, domain, house vendor, settings, counter,
// owner; bhul ba duplicate hole kono row na.
func TestCreateShopIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	p := Params{Slug: "sobujbazar", Prefix: "SB", Domain: "SobujBazar.com", NameBn: "সবুজ বাজার", NameEn: "Sobuj Bazar",
		OwnerEmail: "owner@example.com", OwnerName: "Rahim", Preset: "organic"}
	id, pw, err := Create(ctx, d, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(pw) < 20 {
		t.Errorf("temporary password choto: %d", len(pw))
	}
	var n int
	if err := d.QueryRow(`SELECT
		(SELECT count(*) FROM shops WHERE id = $1 AND slug = 'sobujbazar' AND order_prefix = 'SB' AND mode = 'single' AND name->>'bn' = 'সবুজ বাজার')
		+ (SELECT count(*) FROM shop_domains WHERE shop_id = $1 AND domain = 'sobujbazar.com' AND is_primary)
		+ (SELECT count(*) FROM vendors WHERE shop_id = $1 AND is_house AND slug = 'sobujbazar' AND commission_bps = 0)
		+ (SELECT count(*) FROM shop_settings WHERE shop_id = $1 AND draft = published AND version = 1
		     AND published->'theme'->>'preset' = 'organic' AND published->'branding'->'name'->>'en' = 'Sobuj Bazar')
		+ (SELECT count(*) FROM shop_counters WHERE shop_id = $1 AND name = 'order_number' AND value = 0)
		+ (SELECT count(*) FROM staff WHERE shop_id = $1 AND role = 'owner' AND vendor_id IS NULL AND must_change_password
		     AND email = 'owner@example.com' AND name = 'Rahim')`, id).Scan(&n); err != nil || n != 6 {
		t.Fatalf("row: %d/6 %v", n, err)
	}
	var hash string
	if err := d.QueryRow(`SELECT password_hash FROM staff WHERE shop_id = $1`, id).Scan(&hash); err != nil || !auth.CheckPassword(hash, pw) {
		t.Errorf("owner er password mile na: %v", err)
	}

	bad := map[string]func(*Params){
		"duplicate slug":   func(p *Params) { p.Domain = "onno.com" },
		"duplicate domain": func(p *Params) { p.Slug = "onno" },
		"bhul prefix":      func(p *Params) { p.Slug, p.Domain, p.Prefix = "x1", "x1.com", "sb" },
		"bhul slug":        func(p *Params) { p.Slug, p.Domain = "Sobuj Bazar", "x2.com" },
		"www domain":       func(p *Params) { p.Slug, p.Domain = "x3", "www.x3.com" },
		"naam nai":         func(p *Params) { p.Slug, p.Domain, p.NameBn = "x4", "x4.com", "" },
		"owner naam nai":   func(p *Params) { p.Slug, p.Domain, p.OwnerName = "x5", "x5.com", "" },
		"email bhul":       func(p *Params) { p.Slug, p.Domain, p.OwnerEmail = "x6", "x6.com", "owner" },
		"ochena preset":    func(p *Params) { p.Slug, p.Domain, p.Preset = "x7", "x7.com", "nai" },
	}
	for name, change := range bad {
		q := p
		change(&q)
		if _, _, err := Create(ctx, d, q); err == nil {
			t.Errorf("%s: error ashe ni", name)
		}
	}
	if err := d.QueryRow(`SELECT (SELECT count(*) FROM shops) + (SELECT count(*) FROM vendors) + (SELECT count(*) FROM staff)`).Scan(&n); err != nil || n != 3 {
		t.Errorf("bhul create er por baki row: %d (chai 3) %v", n, err)
	}
}
