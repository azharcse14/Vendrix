package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var b struct{ Error ErrorBody }
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("JSON na: %q", rec.Body.String())
	}
	return b.Error.Code
}

func testMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/products/{slug}", func(w http.ResponseWriter, r *http.Request) { WriteJSON(w, 200, "ok") })
	return mux
}

// TestNotFoundJSON (7.1): ochena path e ServeMux er text na, JSON `not_found`.
func TestNotFoundJSON(t *testing.T) {
	h := JSONErrors(testMux())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/nai", nil))
	if rec.Code != 404 || errCode(t, rec) != "not_found" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Errorf("%d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/products/chal", nil))
	if rec.Code != 200 {
		t.Errorf("route ache, tobu %d", rec.Code)
	}
}

// TestMethodNotAllowedJSON (7.1): bhul method e JSON `method_not_allowed`, Allow header thake.
func TestMethodNotAllowedJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	JSONErrors(testMux()).ServeHTTP(rec, httptest.NewRequest("POST", "/api/products/chal", nil))
	if rec.Code != 405 || errCode(t, rec) != "method_not_allowed" || !strings.Contains(rec.Header().Get("Allow"), "GET") {
		t.Errorf("%d %q Allow=%q", rec.Code, rec.Body.String(), rec.Header().Get("Allow"))
	}
}

// TestPage (7.1): default 1/24, max 100; bhul hole 400 validation_failed fields shoho.
func TestPage(t *testing.T) {
	for q, want := range map[string][2]int{"": {1, 24}, "?page=3&limit=100": {3, 100}, "?limit=1": {1, 1}} {
		rec := httptest.NewRecorder()
		page, limit, ok := Page(rec, httptest.NewRequest("GET", "/x"+q, nil))
		if !ok || page != want[0] || limit != want[1] {
			t.Errorf("%q: %d %d %v", q, page, limit, ok)
		}
	}
	for q, field := range map[string]string{"?page=0": "page", "?page=x": "page", "?limit=101": "limit", "?limit=0": "limit", "?page=-1": "page"} {
		rec := httptest.NewRecorder()
		if _, _, ok := Page(rec, httptest.NewRequest("GET", "/x"+q, nil)); ok {
			t.Errorf("%q: ok", q)
			continue
		}
		var b struct{ Error ErrorBody }
		_ = json.Unmarshal(rec.Body.Bytes(), &b)
		if rec.Code != 400 || b.Error.Code != "validation_failed" || b.Error.Fields[field] == "" {
			t.Errorf("%q: %d %q", q, rec.Code, rec.Body.String())
		}
	}
	got, _ := json.Marshal(NewPaged[string](nil, 2, 24, 30))
	if string(got) != `{"items":[],"page":2,"limit":24,"total":30}` {
		t.Errorf("Paged: %s", got)
	}
}
