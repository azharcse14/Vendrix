package shop

import (
	"encoding/json"
	"slices"
	"strings"
)

// PolicyKinds (10.1): `/policy/{kind}` er kind.
var PolicyKinds = []string{"terms", "return", "refund", "delivery", "privacy"}

// policyPlaceholder: shop policy na likhle (M8 er editor er age) Bangla lekha; Bangla version shob shomoy thake (9.8).
const policyPlaceholder = "এই নীতিমালা শীঘ্রই যোগ করা হবে। কোনো প্রশ্ন থাকলে দোকানের সাথে যোগাযোগ করুন।"

// PolicyText (7.2, v1.13): published settings er `policies[kind]` theke locale er lekha, khali hole bn, ta-o khali
// hole placeholder. Ochena kind e ok false (404).
func PolicyText(published []byte, kind, locale string) (string, bool, error) {
	if !slices.Contains(PolicyKinds, kind) {
		return "", false, nil
	}
	var s struct {
		Policies map[string]map[string]string `json:"policies"`
	}
	if err := json.Unmarshal(published, &s); err != nil {
		return "", false, err
	}
	p := s.Policies[kind]
	for _, l := range []string{locale, "bn"} {
		if t := p[l]; strings.TrimSpace(t) != "" {
			return t, true, nil
		}
	}
	return policyPlaceholder, true, nil
}

// defaultPolicies: notun shop e protiti kind e Bangla placeholder (9.2).
func defaultPolicies() map[string]map[string]string {
	m := map[string]map[string]string{}
	for _, k := range PolicyKinds {
		m[k] = map[string]string{"bn": policyPlaceholder, "en": ""}
	}
	return m
}
