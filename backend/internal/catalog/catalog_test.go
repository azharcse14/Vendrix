package catalog

import "testing"

// TestLocalizedText (7.1, 14.2 test 12 er ongsho): locale er lekha; khali ba nai hole bn.
func TestLocalizedText(t *testing.T) {
	full := Text{"bn": "লাল চাল", "en": "Red Rice"}
	for _, c := range []struct {
		t      Text
		locale string
		want   string
	}{
		{full, "en", "Red Rice"},
		{full, "bn", "লাল চাল"},
		{full, "fr", "লাল চাল"},
		{Text{"bn": "মধু"}, "en", "মধু"},
		{Text{"bn": "মধু", "en": ""}, "en", "মধু"},
		{nil, "en", ""},
	} {
		if got := c.t.In(c.locale); got != c.want {
			t.Errorf("%v.In(%s) = %q, chai %q", c.t, c.locale, got, c.want)
		}
	}
	if !full.valid() || (Text{"en": "x"}).valid() || (Text{"bn": " "}).valid() || (Text{"bn": "x", "fr": "y"}).valid() {
		t.Error("valid: bn lagbe, shudhu bn/en")
	}
}

// TestCategoryCycle (7.7): nijer bongshodhor ke parent korle cycle; onno jaygay na.
func TestCategoryCycle(t *testing.T) {
	// 1 → 2 → 3 (3 er parent 2, 2 er parent 1); 4 root
	parents := map[int64]int64{1: 0, 2: 1, 3: 2, 4: 0}
	for _, c := range []struct {
		id, parent int64
		cycle      bool
	}{
		{1, 3, true}, {1, 2, true}, {2, 3, true}, {1, 1, true},
		{3, 4, false}, {4, 3, false}, {2, 0, false}, {3, 1, false},
	} {
		if got := makesCycle(parents, c.id, c.parent); got != c.cycle {
			t.Errorf("%d er parent %d: cycle %v, chai %v", c.id, c.parent, got, c.cycle)
		}
	}
}

// TestSearchTerm (6.4): 2 okkhorer kom hole search na; `\ % _` escape; asol lekha alada.
func TestSearchTerm(t *testing.T) {
	if _, _, ok := searchTerm(" চ "); ok {
		t.Error("1 okkhor e search")
	}
	like, raw, ok := searchTerm(` 10%_\ `)
	if !ok || like != `10\%\_\\` || raw != `10%_\` {
		t.Errorf("like %q raw %q", like, raw)
	}
	if _, raw, ok := searchTerm("চাল"); !ok || raw != "চাল" {
		t.Error("Bangla 3 okkhor")
	}
}

func TestValidSlug(t *testing.T) {
	for s, want := range map[string]bool{"lal-chal": true, "a1": true, "Lal": false, "lal--chal": false,
		"-lal": false, "লাল": false, "": false, "a b": false} {
		if validSlug(s) != want {
			t.Errorf("%q", s)
		}
	}
}
