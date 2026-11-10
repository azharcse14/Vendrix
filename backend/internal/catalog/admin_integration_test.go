package catalog

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"vendrix/backend/internal/httpx"
)

// owner: shop s er owner staff (shop create er).
func (f *fixture) owner(t *testing.T, s string) httpx.Staff {
	t.Helper()
	return httpx.Staff{ID: f.id(t, `SELECT id FROM staff WHERE shop_id = $1 AND role = 'owner'`, f.ids[s]), ShopID: f.ids[s], Role: "owner"}
}

// admin: staff context e request (st.ID 0 = staff nai); response er code ar body.
func (f *fixture) admin(t *testing.T, st httpx.Staff, method, path, body string, want int) map[string]any {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if st.ID != 0 {
		r = r.WithContext(httpx.WithStaff(r.Context(), st))
	}
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r)
	if rec.Code != want {
		t.Fatalf("%s %s %s: %d %s", method, path, body, rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func errCode(m map[string]any) string {
	e, _ := m["error"].(map[string]any)
	c, _ := e["code"].(string)
	if fs, ok := e["fields"].(map[string]any); ok {
		for k, v := range fs {
			c += " " + k + "=" + v.(string)
		}
	}
	return c
}

func idOf(m map[string]any) string { return strconv.FormatInt(int64(m["id"].(float64)), 10) }

// TestAdminCategoryIntegration (7.7, 11.2, v1.10): fail-closed, role, validation, slug_taken, onno shop er
// parent, cycle, khali hole delete, audit.
func TestAdminCategoryIntegration(t *testing.T) {
	f := newFixture(t)
	a, b := f.owner(t, "a"), f.owner(t, "b")
	f.category(t, "b", "b-cat", "", "বি", true)

	f.admin(t, httpx.Staff{}, "GET", "/api/admin/categories", "", 401)
	vendor := a
	vendor.Role = "vendor_owner"
	f.admin(t, vendor, "POST", "/api/admin/categories", `{}`, 403)

	root := f.admin(t, a, "POST", "/api/admin/categories", `{"slug":"chal","name":{"bn":"চাল","en":"Rice"},"image_key":"k.jpg"}`, 201)
	if root["slug"] != "chal" || root["is_active"] != true || root["shop_id"] != nil {
		t.Errorf("create: %v", root)
	}
	child := f.admin(t, a, "POST", "/api/admin/categories", `{"slug":"lal-chal","name":{"bn":"লাল চাল"},"parent_id":`+idOf(root)+`}`, 201)
	grand := f.admin(t, a, "POST", "/api/admin/categories", `{"slug":"lal-chal-2","name":{"bn":"২"},"parent_id":`+idOf(child)+`}`, 201)

	for body, want := range map[string]string{
		`{"slug":"chal","name":{"bn":"আবার"}}`:                                                     "slug_taken",
		`{"slug":"Chal","name":{"bn":"x"}}`:                                                        "validation_failed slug=invalid",
		`{"slug":"x","name":{"en":"x"}}`:                                                           "validation_failed name=invalid",
		`{"slug":"x","name":{"bn":"x"},"nai":1}`:                                                   "validation_failed body=invalid",
		`{"slug":"x","name":{"bn":"x"},"parent_id":` + strconv.FormatInt(f.ids["b-cat"], 10) + `}`: "validation_failed parent_id=invalid",
	} {
		status := 400
		if want == "slug_taken" {
			status = 409
		}
		if got := errCode(f.admin(t, a, "POST", "/api/admin/categories", body, status)); got != want {
			t.Errorf("%s: %s, chai %s", body, got, want)
		}
	}

	if got := errCode(f.admin(t, a, "PATCH", "/api/admin/categories/"+idOf(root), `{"parent_id":`+idOf(grand)+`}`, 400)); got != "validation_failed parent_id=cycle" {
		t.Errorf("cycle: %s", got)
	}
	f.admin(t, a, "PATCH", "/api/admin/categories/"+strconv.FormatInt(f.ids["b-cat"], 10), `{"slug":"churi"}`, 404)
	f.admin(t, b, "PATCH", "/api/admin/categories/"+idOf(root), `{"parent_id":`+strconv.FormatInt(f.ids["b-cat"], 10)+`}`, 404)
	up := f.admin(t, a, "PATCH", "/api/admin/categories/"+idOf(grand), `{"parent_id":null,"image_key":null,"is_active":false,"name":{"bn":"নতুন"}}`, 200)
	if up["parent_id"] != nil || up["is_active"] != false || up["slug"] != "lal-chal-2" || up["name"].(map[string]any)["bn"] != "নতুন" {
		t.Errorf("patch: %v", up)
	}
	if got := errCode(f.admin(t, a, "DELETE", "/api/admin/categories/"+idOf(root), "", 409)); got != "category_not_empty" {
		t.Errorf("delete: %s", got)
	}
	f.admin(t, a, "DELETE", "/api/admin/categories/"+idOf(grand), "", 204)
	f.admin(t, a, "DELETE", "/api/admin/categories/"+idOf(grand), "", 404)

	var list []categoryRow
	r := httptest.NewRequest("GET", "/api/admin/categories", nil)
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r.WithContext(httpx.WithStaff(r.Context(), a)))
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 2 {
		t.Errorf("list: %s", rec.Body.String())
	}
	var n int
	if err := f.d.QueryRow(`SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND staff_id = $2 AND entity_type = 'category'`,
		a.ShopID, a.ID).Scan(&n); err != nil || n != 5 {
		t.Errorf("audit: %d (chai 3 create + 1 update + 1 delete) %v", n, err)
	}
}
