package shop

import (
	"encoding/json"
	"testing"
)

// TestPolicyText (7.2, 9.2, 10.10, v1.13): published settings er policies theke locale er lekha, en khali hole bn;
// block/bn khali hole Bangla placeholder (404 na); ochena kind false; notun shop e placeholder ache.
func TestPolicyText(t *testing.T) {
	pub := []byte(`{"policies":{"terms":{"bn":"শর্ত","en":"Terms"},"refund":{"bn":"ফেরত","en":""},"return":{"bn":"  "}}}`)
	cases := []struct {
		settings     []byte
		kind, locale string
		want         string
		ok           bool
	}{
		{pub, "terms", "bn", "শর্ত", true},
		{pub, "terms", "en", "Terms", true},
		{pub, "refund", "en", "ফেরত", true},
		{pub, "return", "bn", policyPlaceholder, true},
		{pub, "privacy", "bn", policyPlaceholder, true},
		{[]byte(`{}`), "delivery", "en", policyPlaceholder, true},
		{pub, "faq", "bn", "", false},
	}
	for _, c := range cases {
		got, ok, err := PolicyText(c.settings, c.kind, c.locale)
		if err != nil || got != c.want || ok != c.ok {
			t.Errorf("%s/%s: %q %v %v, chai %q %v", c.kind, c.locale, got, ok, err, c.want, c.ok)
		}
	}
	if _, _, err := PolicyText([]byte(`{"policies":{"terms":1}}`), "terms", "bn"); err == nil {
		t.Error("bhul JSON")
	}

	full, _ := DefaultSettings("organic", map[string]string{"bn": "সবুজ"})
	var m struct {
		Policies map[string]map[string]string `json:"policies"`
	}
	_ = json.Unmarshal(full, &m)
	for _, k := range PolicyKinds {
		if m.Policies[k]["bn"] != policyPlaceholder {
			t.Errorf("DefaultSettings e %s er placeholder nai", k)
		}
	}
}
