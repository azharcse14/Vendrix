package shipping_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/shipping"
)

// zoneAdmin: admin route er server; staff context request e (RequireRole handler e dekhe).
func zoneAdmin(t *testing.T, d *sql.DB) func(st httpx.Staff, method, path string, body any) (int, map[string]any) {
	mux := http.NewServeMux()
	shipping.NewHandlers(d, slog.New(slog.DiscardHandler)).AdminRoutes(mux, func(h http.Handler) http.Handler { return h })
	srv := httpx.JSONErrors(mux)
	return func(st httpx.Staff, method, path string, body any) (int, map[string]any) {
		t.Helper()
		var b bytes.Buffer
		if s, ok := body.(string); ok {
			b.WriteString(s)
		} else if body != nil {
			json.NewEncoder(&b).Encode(body)
		}
		r := httptest.NewRequest(method, path, &b)
		r = r.WithContext(httpx.WithStaff(r.Context(), st))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		var out map[string]any
		if w.Code != http.StatusNoContent {
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
			}
		}
		return w.Code, out
	}
}

func staffOf(t *testing.T, d *sql.DB, shopID int64, role string) httpx.Staff {
	t.Helper()
	st := httpx.Staff{ShopID: shopID, Role: role}
	if err := d.QueryRow(`SELECT id FROM staff WHERE shop_id = $1 AND role = 'owner'`, shopID).Scan(&st.ID); err != nil {
		t.Fatal(err)
	}
	if role != "owner" {
		var vendor any
		if strings.HasPrefix(role, "vendor_") {
			var v int64
			d.QueryRow(`SELECT id FROM vendors WHERE shop_id = $1 AND is_house`, shopID).Scan(&v)
			vendor, st.VendorID = v, &v
		}
		if err := d.QueryRow(`INSERT INTO staff (shop_id, vendor_id, email, name, password_hash, role)
			VALUES ($1, $2, $3 || '@x.com', 'S', 'x', $3) RETURNING id`, shopID, vendor, role).Scan(&st.ID); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

func errOf(out map[string]any) (string, map[string]any) {
	e, _ := out["error"].(map[string]any)
	code, _ := e["code"].(string)
	fields, _ := e["fields"].(map[string]any)
	return code, fields
}

// TestAdminZoneIntegration (7.7, 11.7 item 13 vi, v1.45): CRUD, district/area trim, area khali = NULL, duplicate
// (NULLS NOT DISTINCT shoho) 409 zone_exists, validation, Phase 2 field 400, delete na; owner/admin shudhu;
// onno shop er zone 404; audit.
func TestAdminZoneIntegration(t *testing.T) {
	d, ids := zoneFixture(t)
	call := zoneAdmin(t, d)
	owner, admin := staffOf(t, d, ids["a"], "owner"), staffOf(t, d, ids["a"], "admin")
	ownerB := staffOf(t, d, ids["b"], "owner")

	code, z := call(owner, "POST", "/api/admin/delivery-zones", map[string]any{
		"name": map[string]string{"bn": "বোয়ালিয়া", "en": "Boalia"}, "district": "  Rajshahi ", "area": " Boalia ",
		"charge": 9000, "same_city": false})
	if code != http.StatusCreated || z["district"] != "Rajshahi" || z["area"] != "Boalia" || z["charge"] != 9000.0 ||
		z["min_order"] != 0.0 || z["free_delivery_over"] != nil || z["is_active"] != true || z["shop_id"] != nil {
		t.Fatalf("create: %d %v", code, z)
	}
	zid := strconv.FormatInt(int64(z["id"].(float64)), 10)
	// area khali = puro district; duibar = 409 (NULLS NOT DISTINCT)
	code, dw := call(admin, "POST", "/api/admin/delivery-zones", map[string]any{
		"name": map[string]string{"bn": "রাজশাহী"}, "district": "Rajshahi", "area": "  ", "charge": 12000, "same_city": false})
	if code != http.StatusCreated || dw["area"] != nil {
		t.Fatalf("district-wide: %d %v", code, dw)
	}
	for _, area := range []any{nil, "Boalia"} {
		code, out := call(owner, "POST", "/api/admin/delivery-zones", map[string]any{
			"name": map[string]string{"bn": "x"}, "district": "Rajshahi", "area": area, "charge": 1, "same_city": false})
		if c, _ := errOf(out); code != http.StatusConflict || c != "zone_exists" {
			t.Errorf("duplicate %v: %d %v", area, code, out)
		}
	}
	// lookup e notun zone
	if got, err := shipping.Lookup(t.Context(), d, ids["a"], "Rajshahi", "Shaheb Bazar"); err != nil || got.Charge != 12000 {
		t.Errorf("lookup: %+v %v", got, err)
	}

	long := strings.Repeat("ক", 101)
	base := func(k string, v any) map[string]any {
		m := map[string]any{"name": map[string]string{"bn": "নাম"}, "district": "Bogura", "area": "Sadar", "charge": 100, "same_city": false}
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	for _, c := range []struct {
		body  any
		field string
	}{
		{base("name", map[string]string{"en": "x"}), "name"},
		{base("name", map[string]string{"bn": " ", "en": "x"}), "name"},
		{base("name", map[string]string{"bn": "x", "fr": "x"}), "name"},
		{base("district", " "), "district"},
		{base("district", long), "district"},
		{base("area", long), "area"},
		{base("charge", -1), "charge"},
		{base("charge", nil), "charge"},
		{base("min_order", -1), "min_order"},
		{base("free_delivery_over", 0), "free_delivery_over"},
		{base("same_city", nil), "same_city"},
		{base("own_rider", true), "body"},
		{base("same_day_cutoff", "12:00"), "body"},
		{`{"name":`, "body"},
	} {
		code, out := call(owner, "POST", "/api/admin/delivery-zones", c.body)
		if e, f := errOf(out); code != http.StatusBadRequest || e != "validation_failed" || f[c.field] == nil {
			t.Errorf("%v: %d %v, chai fields.%s", c.body, code, out, c.field)
		}
	}

	// list: paged, inactive o, shudhu ei shop er, district → area NULLS FIRST
	code, list := call(owner, "GET", "/api/admin/delivery-zones?limit=100", nil)
	if code != http.StatusOK || list["total"] != 8.0 {
		t.Fatalf("list: %d %v", code, list)
	}
	var keys []string
	for _, it := range list["items"].([]any) {
		m := it.(map[string]any)
		a, _ := m["area"].(string)
		keys = append(keys, m["district"].(string)+"/"+a)
		if m["shop_id"] != nil || m["name"].(map[string]any)["bn"] == nil {
			t.Errorf("row: %v", m)
		}
	}
	if got := strings.Join(keys, ","); got != "Chattogram/Agrabad,Dhaka/,Dhaka/Banani,Dhaka/Dhanmondi,Dhaka/Mirpur,Rajshahi/,Rajshahi/Boalia,Sylhet/" {
		t.Errorf("list kram: %s", got)
	}

	// PATCH: je field ache shudhu sheta; null = muche (free_delivery_over)
	code, up := call(owner, "PATCH", "/api/admin/delivery-zones/"+zid, map[string]any{"charge": 9500, "is_active": false, "free_delivery_over": 300000})
	if code != http.StatusOK || up["charge"] != 9500.0 || up["is_active"] != false || up["free_delivery_over"] != 300000.0 || up["district"] != "Rajshahi" {
		t.Errorf("patch: %d %v", code, up)
	}
	if code, up = call(owner, "PATCH", "/api/admin/delivery-zones/"+zid, map[string]any{"free_delivery_over": nil, "area": " Upashahar "}); code != http.StatusOK ||
		up["free_delivery_over"] != nil || up["area"] != "Upashahar" {
		t.Errorf("patch null: %d %v", code, up)
	}
	if code, out := call(owner, "PATCH", "/api/admin/delivery-zones/"+zid, map[string]any{"area": nil}); code != http.StatusConflict {
		t.Errorf("patch duplicate: %d %v", code, out)
	} else if c, _ := errOf(out); c != "zone_exists" {
		t.Errorf("patch duplicate code: %v", out)
	}
	if code, out := call(owner, "PATCH", "/api/admin/delivery-zones/"+zid, map[string]any{"charge": -5}); code != http.StatusBadRequest {
		t.Errorf("patch charge: %d %v", code, out)
	}
	if code, out := call(owner, "PATCH", "/api/admin/delivery-zones/"+zid, map[string]any{"name": map[string]string{"en": "x"}}); code != http.StatusBadRequest {
		t.Errorf("patch name bn chhara: %d %v", code, out)
	}

	// onno shop: 404, kichu bodlay na; list e a er zone nai
	if code, _ := call(ownerB, "PATCH", "/api/admin/delivery-zones/"+zid, map[string]any{"charge": 1}); code != http.StatusNotFound {
		t.Errorf("onno shop patch: %d", code)
	}
	if code, l := call(ownerB, "GET", "/api/admin/delivery-zones", nil); code != http.StatusOK || l["total"] != 2.0 {
		t.Errorf("b list: %d %v", code, l)
	}
	if code, _ := call(owner, "PATCH", "/api/admin/delivery-zones/999999", map[string]any{"charge": 1}); code != http.StatusNotFound {
		t.Errorf("nai: %d", code)
	}
	if code, _ := call(owner, "DELETE", "/api/admin/delivery-zones/"+zid, nil); code != http.StatusMethodNotAllowed {
		t.Errorf("delete: %d", code)
	}

	// role: staff/vendor 403
	for _, role := range []string{"staff", "vendor_owner", "vendor_staff"} {
		st := staffOf(t, d, ids["a"], role)
		if code, _ := call(st, "GET", "/api/admin/delivery-zones", nil); code != http.StatusForbidden {
			t.Errorf("%s GET: %d", role, code)
		}
		if code, _ := call(st, "POST", "/api/admin/delivery-zones", base("district", "Pabna")); code != http.StatusForbidden {
			t.Errorf("%s POST: %d", role, code)
		}
	}

	var n int
	d.QueryRow(`SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND entity_type = 'zone' AND action IN ('zone.create', 'zone.update')`, ids["a"]).Scan(&n)
	if n != 4 { // 2 create + 2 sofol patch
		t.Errorf("audit %d", n)
	}
}
