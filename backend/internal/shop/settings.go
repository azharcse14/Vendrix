package shop

import (
	"encoding/json"
	"fmt"
)

type preset struct {
	colors  map[string]string
	heading string
	radius  int
}

// presets (9.4): 5 rong, heading font, radius; body font shobar Hind Siliguri.
var presets = map[string]preset{
	"organic":     {map[string]string{"primary": "#1F5A32", "accent": "#F2C230", "tint": "#E3EED3", "text": "#3B2A1A", "background": "#FBFBF6"}, "Baloo Da 2", 14},
	"fashion":     {map[string]string{"primary": "#2A3480", "accent": "#B83A5E", "tint": "#ECEDF7", "text": "#1C1E33", "background": "#FFFFFF"}, "Noto Serif Bengali", 2},
	"electronics": {map[string]string{"primary": "#0A5BB5", "accent": "#0B7F65", "tint": "#E4EEFA", "text": "#14202E", "background": "#F4F7FB"}, "Anek Bangla", 10},
}

// DefaultSettings (9.2): notun shop er settings, preset theke theme. payment shudhu cod (SSLCommerz er
// credential M10 e), features faka (admin chalu kore), seo.noindex true (limited launch, 14.4).
func DefaultSettings(presetName string, name map[string]string) ([]byte, error) {
	p, ok := presets[presetName]
	if !ok {
		return nil, fmt.Errorf("preset %q nai (organic, fashion, electronics)", presetName)
	}
	return json.Marshal(map[string]any{
		"branding": map[string]any{
			"name": name, "tagline": map[string]string{}, "logo_key": nil, "favicon_key": nil,
			"phone": nil, "whatsapp": nil, "email": nil, "address": map[string]string{},
			"social": map[string]any{"facebook": nil, "instagram": nil}, "dbid_number": nil, "bfsa_number": nil,
		},
		"theme": map[string]any{
			"preset": presetName, "colors": p.colors,
			"fonts": map[string]string{"heading": p.heading, "body": "Hind Siliguri"}, "radius": p.radius,
		},
		"locales":  map[string]any{"enabled": []string{"bn", "en"}, "default": "bn", "bangla_digits": true},
		"features": []string{},
		"checkout": map[string]any{
			"payment_methods": []string{"cod"}, "otp_mode": "risky", "cod_confirmation": "risky",
			"auto_cancel_unconfirmed_hours": 24,
			"risk":                          map[string]int{"cod_amount_over": 300000, "new_phone_amount_over": 200000, "max_orders_per_hour": 3},
			"sms_daily_cap":                 500, "return_window_days": 3,
		},
		"inventory":   map[string]int{"expiry_alert_days": 7},
		"marketplace": map[string]any{"delivery_charge_mode": "per_order", "default_commission_bps": 0, "show_vendor_name": false},
		"homepage": map[string]any{
			"announcement": map[string]string{}, "banners": []any{},
			"featured_category_slugs": []string{}, "featured_product_slugs": []string{},
		},
		"seo":          map[string]any{"title": name, "description": map[string]string{}, "og_image_key": nil, "noindex": true},
		"integrations": map[string]any{"facebook_pixel_id": nil, "ga4_id": nil},
		"policies":     defaultPolicies(),
	})
}

// publicKeys (7.2, v1.10): published settings er je ongsho public; notun block ba key default e private.
var publicKeys = map[string][]string{
	"branding": nil, "theme": nil, "locales": nil, "features": nil, "homepage": nil, "seo": nil, "integrations": nil,
	"checkout":    {"payment_methods", "return_window_days"},
	"marketplace": {"show_vendor_name"},
}

// PublicSettings: published JSON theke shudhu publicKeys (nil = puro block).
func PublicSettings(published []byte) (map[string]any, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(published, &all); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for block, keys := range publicKeys {
		raw, ok := all[block]
		if !ok {
			continue
		}
		if keys == nil {
			out[block] = raw
			continue
		}
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(raw, &inner); err != nil {
			return nil, fmt.Errorf("%s: %w", block, err)
		}
		pick := map[string]json.RawMessage{}
		for _, k := range keys {
			if v, ok := inner[k]; ok {
				pick[k] = v
			}
		}
		out[block] = pick
	}
	return out, nil
}
