package httpx

import (
	"encoding/json"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestErrorMessageLocale (7.1, v1.10): message Accept-Language onujayi (bn default); ochena code e-o
// faka na; caller er nijer message thakle sheta.
func TestErrorMessageLocale(t *testing.T) {
	msg := func(lang, code, own string) string {
		r := httptest.NewRequest("GET", "/", nil)
		if lang != "" {
			r.Header.Set("Accept-Language", lang)
		}
		rec := httptest.NewRecorder()
		WriteError(rec, r, 404, ErrorBody{Code: code, Message: own})
		var b struct{ Error ErrorBody }
		if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
			t.Fatal(err)
		}
		return b.Error.Message
	}
	bn, en := msg("", "not_found", ""), msg("en-US,en;q=0.9", "not_found", "")
	if bn == "" || en == "" || bn == en {
		t.Errorf("bn %q, en %q", bn, en)
	}
	if got := msg("bn", "not_found", ""); got != bn {
		t.Errorf("bn: %q", got)
	}
	if got := msg("fr", "not_found", ""); got != bn {
		t.Errorf("ochena bhasha e bn chai: %q", got)
	}
	if got := msg("en", "notun_code", ""); got == "" {
		t.Error("ochena code e message faka")
	}
	if got := msg("en", "not_found", "nijer"); got != "nijer" {
		t.Errorf("caller er message: %q", got)
	}
	for code, m := range messages {
		if m[0] == "" || m[1] == "" {
			t.Errorf("%s: bn ba en nai", code)
		}
	}
}

// TestErrorCodesHaveMessage (7.1): backend er protiti ErrorBody{Code: "..."} er messages e lekha ache,
// na hole client "সার্ভারে সমস্যা হয়েছে" dekhe (M7a step 15 e weight_over_limit e dhora).
func TestErrorCodesHaveMessage(t *testing.T) {
	re := regexp.MustCompile(`ErrorBody\{Code: +"([a-z_]+)"`)
	n := 0
	err := filepath.WalkDir("../..", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range re.FindAllSubmatch(src, -1) {
			n++
			if _, ok := messages[string(m[1])]; !ok {
				t.Errorf("%s: %s er message nai", p, m[1])
			}
		}
		return nil
	})
	if err != nil || n < 20 {
		t.Fatalf("source pora: %v, %d code", err, n)
	}
}
