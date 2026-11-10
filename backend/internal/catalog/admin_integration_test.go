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
		`{"slug":"x","name":{"bn":"x"},"sort_order":3000000000}`:                                   "validation_failed sort_order=invalid",
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
	f.admin(t, a, "DELETE", "/api/admin/categories/"+strconv.FormatInt(f.ids["b-cat"], 10), "", 404)
	if n := f.id(t, `SELECT count(*) FROM categories WHERE id = $1 AND shop_id = $2 AND slug = 'b-cat'`, f.ids["b-cat"], b.ShopID); n != 1 {
		t.Error("onno shop er category bodleche")
	}
	mixed := httpx.Staff{ID: b.ID, ShopID: a.ShopID, Role: "owner"} // session er bug: staff onno shop er
	f.admin(t, mixed, "POST", "/api/admin/categories", `{"slug":"mix","name":{"bn":"x"}}`, 500)
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

func s64(n int64) string { return strconv.FormatInt(n, 10) }

// TestAdminProductIntegration (7.7, 6.1, v1.10): create (house vendor), validation, variable_weight → kg,
// onno shop er category/vendor 400 (14.2 test 13), PATCH (updated_at, is_active), audit.
func TestAdminProductIntegration(t *testing.T) {
	f := newFixture(t)
	a := f.owner(t, "a")
	f.category(t, "a", "chal", "", "চাল", true)
	f.category(t, "b", "b-cat", "", "বি", true)

	p := f.admin(t, a, "POST", "/api/admin/products", `{"slug":"lal-chal","name":{"bn":"লাল চাল"},"category_id":`+
		s64(f.ids["chal"])+`,"unit":"kg","variable_weight":true,"attributes":{"origin":"Dinajpur"},"search_keywords":"lal chal"}`, 201)
	if p["vendor_id"] != float64(f.ids["a.vendor"]) || p["unit"] != "kg" || p["is_active"] != true || p["search_text"] != nil {
		t.Errorf("create: %v", p)
	}
	for body, want := range map[string]string{
		`{"slug":"lal-chal","name":{"bn":"x"}}`:                                     "slug_taken",
		`{"slug":"x","name":{"bn":"x"},"variable_weight":true}`:                     "validation_failed variable_weight=invalid",
		`{"slug":"x","name":{"bn":"x"},"unit":"box"}`:                               "validation_failed unit=invalid",
		`{"slug":"x","name":{"bn":"x"},"attributes":[1]}`:                           "validation_failed attributes=invalid",
		`{"slug":"x","name":{"bn":"x"},"description":{"fr":"x"}}`:                   "validation_failed description=invalid",
		`{"slug":"x","name":{"bn":"x"},"category_id":` + s64(f.ids["b-cat"]) + `}`:  "validation_failed category_id=invalid",
		`{"slug":"x","name":{"bn":"x"},"vendor_id":` + s64(f.ids["b.vendor"]) + `}`: "validation_failed vendor_id=invalid",
		`{"slug":"x"}`: "validation_failed name=invalid",
	} {
		status := 400
		if want == "slug_taken" {
			status = 409
		}
		if got := errCode(f.admin(t, a, "POST", "/api/admin/products", body, status)); got != want {
			t.Errorf("%s: %s, chai %s", body, got, want)
		}
	}
	id := idOf(p)
	up := f.admin(t, a, "PATCH", "/api/admin/products/"+id, `{"is_active":false,"category_id":null,"name":{"bn":"লাল চাল","en":"Red Rice"}}`, 200)
	if up["is_active"] != false || up["category_id"] != nil || up["updated_at"] == p["updated_at"] || up["slug"] != "lal-chal" {
		t.Errorf("patch: %v", up)
	}
	for body, want := range map[string]string{
		`{"unit":"piece"}`: "validation_failed variable_weight=invalid",
		`{"vendor_id":1}`:  "validation_failed vendor_id=read_only",
		`{"category_id":` + s64(f.ids["b-cat"]) + `}`: "validation_failed category_id=invalid",
	} {
		if got := errCode(f.admin(t, a, "PATCH", "/api/admin/products/"+id, body, 400)); got != want {
			t.Errorf("patch %s: %s", body, got)
		}
	}
	got := f.admin(t, a, "GET", "/api/admin/products/"+id, "", 200)
	if got["variants"] == nil || got["images"] == nil || got["name"].(map[string]any)["en"] != "Red Rice" {
		t.Errorf("get: %v", got)
	}
	var n int
	if err := f.d.QueryRow(`SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND entity_type = 'product' AND entity_id = $2`,
		a.ShopID, int64(p["id"].(float64))).Scan(&n); err != nil || n != 2 {
		t.Errorf("audit: %d %v", n, err)
	}
}

// TestVendorScopeIntegration (Rule 13, 14.2 test 14 er prostuti): vendor X er staff vendor Y er product
// dekhe na (list e nai, GET/PATCH 404), Y er naam e create 404; category route 403. Shop B er staff A er
// product 404 (Rule 3).
func TestVendorScopeIntegration(t *testing.T) {
	f := newFixture(t)
	a, b := f.owner(t, "a"), f.owner(t, "b")
	x := f.id(t, `INSERT INTO vendors (shop_id, slug, name) VALUES ($1, 'x', '{"bn":"x"}') RETURNING id`, f.ids["a"])
	y := f.id(t, `INSERT INTO vendors (shop_id, slug, name) VALUES ($1, 'y', '{"bn":"y"}') RETURNING id`, f.ids["a"])
	vx := httpx.Staff{ID: f.id(t, `INSERT INTO staff (shop_id, vendor_id, email, name, password_hash, role)
		VALUES ($1, $2, 'x@a.com', 'X', 'h', 'vendor_owner') RETURNING id`, f.ids["a"], x), ShopID: f.ids["a"], VendorID: &x, Role: "vendor_owner"}

	px := f.admin(t, vx, "POST", "/api/admin/products", `{"slug":"x-chal","name":{"bn":"x"}}`, 201)
	if px["vendor_id"] != float64(x) {
		t.Errorf("vendor staff er product: %v", px)
	}
	py := f.admin(t, a, "POST", "/api/admin/products", `{"slug":"y-chal","name":{"bn":"y"},"vendor_id":`+s64(y)+`}`, 201)
	f.admin(t, vx, "POST", "/api/admin/products", `{"slug":"z","name":{"bn":"z"},"vendor_id":`+s64(y)+`}`, 404)
	f.admin(t, vx, "GET", "/api/admin/products/"+idOf(py), "", 404)
	f.admin(t, vx, "PATCH", "/api/admin/products/"+idOf(py), `{"is_active":false}`, 404)
	f.admin(t, vx, "GET", "/api/admin/products/"+idOf(px), "", 200)
	f.admin(t, vx, "POST", "/api/admin/categories", `{"slug":"c","name":{"bn":"c"}}`, 403)

	l := f.admin(t, vx, "GET", "/api/admin/products?vendor_id="+s64(y), "", 200)
	if items := l["items"].([]any); len(items) != 1 || items[0].(map[string]any)["slug"] != "x-chal" {
		t.Errorf("vendor x er list: %v", l)
	}
	if l := f.admin(t, a, "GET", "/api/admin/products?vendor_id="+s64(y), "", 200); l["total"] != float64(1) {
		t.Errorf("owner ?vendor_id: %v", l)
	}
	if l := f.admin(t, a, "GET", "/api/admin/products", "", 200); l["total"] != float64(2) {
		t.Errorf("owner list: %v", l)
	}
	f.admin(t, b, "GET", "/api/admin/products/"+idOf(px), "", 404)
	f.admin(t, b, "PATCH", "/api/admin/products/"+idOf(px), `{"is_active":false}`, 404)
	if l := f.admin(t, b, "GET", "/api/admin/products", "", 200); l["total"] != float64(0) {
		t.Errorf("shop b list: %v", l)
	}
}

// TestAdminVariantIntegration (7.7, 8.12, v1.10, v1.11): dam check, compare_at_price > price, sku ek vendor e
// unique (409 sku_taken), stock field na, vendor product theke; onno vendor/shop 404; audit.
func TestAdminVariantIntegration(t *testing.T) {
	f := newFixture(t)
	a, b := f.owner(t, "a"), f.owner(t, "b")
	p := idOf(f.admin(t, a, "POST", "/api/admin/products", `{"slug":"chal","name":{"bn":"চাল"}}`, 201))
	v := f.admin(t, a, "POST", "/api/admin/products/"+p+"/variants",
		`{"sku":"CHAL-5","price":42000,"compare_at_price":45000,"options":{"weight":{"bn":"৫ কেজি","en":"5 kg"}},"nominal_weight_g":5000}`, 201)
	if v["vendor_id"] != float64(f.ids["a.vendor"]) || v["stock"] != float64(0) || v["price"] != float64(42000) {
		t.Errorf("create: %v", v)
	}
	for body, want := range map[string]string{
		`{"sku":"CHAL-5","price":1}`: "sku_taken",
		`{"sku":"X","price":-1}`:     "validation_failed price=invalid",
		`{"sku":"X"}`:                "validation_failed price=invalid",
		`{"price":1}`:                "validation_failed sku=invalid",
		`{"sku":"X","price":100,"compare_at_price":100}`:        "validation_failed compare_at_price=invalid",
		`{"sku":"X","price":100,"stock":5}`:                     "validation_failed body=invalid",
		`{"sku":"X","price":100,"price_x":1}`:                   "validation_failed body=invalid",
		`{"sku":"X","price":100,"options":{"w":{"en":"x"}}}`:    "validation_failed options=invalid",
		`{"sku":"X","price":100,"nominal_weight_g":0}`:          "validation_failed nominal_weight_g=invalid",
		`{"sku":"X","price":100,"cost_price":-5}`:               "validation_failed cost_price=invalid",
		`{"sku":"X","price":100,"sort_order":3000000000}`:       "validation_failed sort_order=invalid",
		`{"sku":"X","price":100,"nominal_weight_g":3000000000}`: "validation_failed nominal_weight_g=invalid",
	} {
		status := 400
		if want == "sku_taken" {
			status = 409
		}
		if got := errCode(f.admin(t, a, "POST", "/api/admin/products/"+p+"/variants", body, status)); got != want {
			t.Errorf("%s: %s, chai %s", body, got, want)
		}
	}
	up := f.admin(t, a, "PATCH", "/api/admin/variants/"+idOf(v), `{"price":40000,"compare_at_price":null,"is_active":false}`, 200)
	if up["price"] != float64(40000) || up["compare_at_price"] != nil || up["is_active"] != false || up["sku"] != "CHAL-5" {
		t.Errorf("patch: %v", up)
	}
	if got := errCode(f.admin(t, a, "PATCH", "/api/admin/variants/"+idOf(v), `{"stock":9}`, 400)); got != "validation_failed body=invalid" {
		t.Errorf("stock patch: %s", got)
	}
	f.admin(t, b, "POST", "/api/admin/products/"+p+"/variants", `{"sku":"B","price":1}`, 404)
	f.admin(t, b, "PATCH", "/api/admin/variants/"+idOf(v), `{"price":1}`, 404)
	x := f.id(t, `INSERT INTO vendors (shop_id, slug, name) VALUES ($1, 'x', '{"bn":"x"}') RETURNING id`, f.ids["a"])
	vx := httpx.Staff{ID: f.id(t, `INSERT INTO staff (shop_id, vendor_id, email, name, password_hash, role)
		VALUES ($1, $2, 'x@a.com', 'X', 'h', 'vendor_owner') RETURNING id`, f.ids["a"], x), ShopID: f.ids["a"], VendorID: &x, Role: "vendor_owner"}
	f.admin(t, vx, "POST", "/api/admin/products/"+p+"/variants", `{"sku":"X","price":1}`, 404)
	f.admin(t, vx, "PATCH", "/api/admin/variants/"+idOf(v), `{"price":1}`, 404)
	px := idOf(f.admin(t, vx, "POST", "/api/admin/products", `{"slug":"x-chal","name":{"bn":"x"}}`, 201))
	if vv := f.admin(t, vx, "POST", "/api/admin/products/"+px+"/variants", `{"sku":"CHAL-5","price":1}`, 201); vv["vendor_id"] != float64(x) {
		t.Errorf("onno vendor e ek sku cholbe: %v", vv)
	}
	var n int
	if err := f.d.QueryRow(`SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND entity_type = 'variant'`, a.ShopID).Scan(&n); err != nil || n != 3 {
		t.Errorf("audit: %d (chai 2 create + 1 update) %v", n, err)
	}
}
