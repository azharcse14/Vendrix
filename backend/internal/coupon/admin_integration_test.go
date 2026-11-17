package coupon_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"vendrix/backend/internal/coupon"
	"vendrix/backend/internal/httpx"
)

func (f *fixture) admin(t *testing.T) func(st httpx.Staff, method, path string, body any) (int, map[string]any) {
	mux := http.NewServeMux()
	coupon.NewHandlers(f.d, slog.New(slog.DiscardHandler)).AdminRoutes(mux, func(h http.Handler) http.Handler { return h })
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
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
		}
		return w.Code, out
	}
}

func (f *fixture) staff(t *testing.T, shopID int64, role string) httpx.Staff {
	t.Helper()
	st := httpx.Staff{ShopID: shopID, Role: role}
	q := `SELECT id FROM staff WHERE shop_id = $1 AND role = 'owner'`
	if role != "owner" {
		q = `INSERT INTO staff (shop_id, email, name, password_hash, role) VALUES ($1, $2 || '@x.com', 'S', 'x', $2) RETURNING id`
	}
	args := []any{shopID}
	if role != "owner" {
		args = append(args, role)
	}
	if err := f.d.QueryRow(q, args...).Scan(&st.ID); err != nil {
		t.Fatal(err)
	}
	return st
}

func errOf(out map[string]any) (string, map[string]any) {
	e, _ := out["error"].(map[string]any)
	code, _ := e["code"].(string)
	fields, _ := e["fields"].(map[string]any)
	return code, fields
}

// TestAdminCouponIntegration (7.7, 8.13, 11.7 item 13 v, v1.45): code coupon.Normalize + format, duplicate (case
// chhara) 409 coupon_exists, field validation, use hoye gele code/kind/value lock (ekoi value chole), usage_limit
// ≥ used_count (below_used), PATCH e null muche; owner/admin shudhu; onno shop 404; audit; delete na.
func TestAdminCouponIntegration(t *testing.T) {
	f := newFixture(t)
	call := f.admin(t)
	owner, admin := f.staff(t, f.shopA, "owner"), f.staff(t, f.shopA, "admin")
	ownerB := f.staff(t, f.shopB, "owner")

	code, c := call(owner, "POST", "/api/admin/coupons", map[string]any{"code": "  eid-20 ", "kind": "percent", "value": 20,
		"max_discount": 10000, "starts_at": "2026-12-01T00:00:00Z", "expires_at": "2026-12-31T18:00:00Z"})
	if code != http.StatusCreated || c["code"] != "EID-20" || c["per_phone_limit"] != 1.0 || c["is_active"] != true ||
		c["used_count"] != 0.0 || c["usage_limit"] != nil || c["min_order"] != 0.0 || c["shop_id"] != nil || c["funded_by"] != "shop" {
		t.Fatalf("create: %d %v", code, c)
	}
	// Use/Peek notun code e mile (normalize kora)
	if _, err := coupon.Peek(t.Context(), f.d, f.shopA, "Eid-20"); err != coupon.ErrInvalid { // starts_at bhobishyot
		t.Errorf("peek: %v", err)
	}
	for _, dup := range []string{"eid-20", "Save10"} {
		code, out := call(admin, "POST", "/api/admin/coupons", map[string]any{"code": dup, "kind": "fixed", "value": 100})
		if e, _ := errOf(out); code != http.StatusConflict || e != "coupon_exists" {
			t.Errorf("duplicate %s: %d %v", dup, code, out)
		}
	}
	// onno shop er code (BONLY) a te nite pare
	if code, out := call(owner, "POST", "/api/admin/coupons", map[string]any{"code": "bonly", "kind": "fixed", "value": 100}); code != http.StatusCreated {
		t.Errorf("b er code a te: %d %v", code, out)
	}

	base := func(k string, v any) map[string]any {
		m := map[string]any{"code": "NEW1", "kind": "fixed", "value": 500}
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	for _, tc := range []struct {
		body  any
		field string
	}{
		{base("code", nil), "code"},
		{base("code", "  "), "code"},
		{base("code", "EID 20"), "code"},
		{base("code", "ঈদ"), "code"},
		{base("code", strings.Repeat("A", 33)), "code"},
		{base("kind", "bogo"), "kind"},
		{base("kind", nil), "kind"},
		{base("value", 0), "value"},
		{base("value", nil), "value"},
		{map[string]any{"code": "P", "kind": "percent", "value": 101}, "value"},
		{base("min_order", -1), "min_order"},
		{base("max_discount", 0), "max_discount"},
		{base("usage_limit", 0), "usage_limit"},
		{base("per_phone_limit", 0), "per_phone_limit"},
		{base("expires_at", "kal"), "expires_at"},
		{map[string]any{"code": "D", "kind": "fixed", "value": 1, "starts_at": "2026-12-02T00:00:00Z", "expires_at": "2026-12-01T00:00:00Z"}, "expires_at"},
		{base("vendor_id", 1), "body"},
		{base("funded_by", "vendor"), "body"},
		{base("used_count", 0), "body"},
	} {
		code, out := call(owner, "POST", "/api/admin/coupons", tc.body)
		if e, fl := errOf(out); code != http.StatusBadRequest || e != "validation_failed" || fl[tc.field] == nil {
			t.Errorf("%v: %d %v, chai fields.%s", tc.body, code, out, tc.field)
		}
	}

	id := func(code string) string { return strconv.FormatInt(f.coupons[code], 10) }
	// used_count 0: shob bodlano jay
	code, up := call(owner, "PATCH", "/api/admin/coupons/"+id("SAVE10"), map[string]any{"code": "save11", "value": 15, "max_discount": nil})
	if code != http.StatusOK || up["code"] != "SAVE11" || up["value"] != 15.0 || up["max_discount"] != nil || up["min_order"] != 50000.0 {
		t.Errorf("patch unused: %d %v", code, up)
	}
	if code, out := call(owner, "PATCH", "/api/admin/coupons/"+id("SAVE10"), map[string]any{"code": "pp1"}); code != http.StatusConflict {
		t.Errorf("patch duplicate: %d %v", code, out)
	}
	// Flat50: used 4, limit 5 → code/kind/value lock (ekoi value chole)
	for _, b := range []map[string]any{{"code": "FLAT60"}, {"kind": "percent"}, {"value": 6000}} {
		code, out := call(owner, "PATCH", "/api/admin/coupons/"+id("Flat50"), b)
		e, fl := errOf(out)
		for k := range b {
			if code != http.StatusBadRequest || e != "validation_failed" || fl[k] != "locked" {
				t.Errorf("lock %v: %d %v", b, code, out)
			}
		}
	}
	if code, out := call(owner, "PATCH", "/api/admin/coupons/"+id("Flat50"), map[string]any{"code": " flat50", "kind": "fixed", "value": 5000, "min_order": 100}); code != http.StatusOK || out["min_order"] != 100.0 {
		t.Errorf("ekoi value: %d %v", code, out)
	}
	if code, out := call(owner, "PATCH", "/api/admin/coupons/"+id("Flat50"), map[string]any{"usage_limit": 3}); code != http.StatusBadRequest {
		t.Errorf("limit < used: %d %v", code, out)
	} else if _, fl := errOf(out); fl["usage_limit"] != "below_used" {
		t.Errorf("below_used: %v", out)
	}
	if code, out := call(owner, "PATCH", "/api/admin/coupons/"+id("Flat50"), map[string]any{"usage_limit": 4}); code != http.StatusOK || out["usage_limit"] != 4.0 {
		t.Errorf("limit = used: %d %v", code, out)
	}
	if code, out := call(owner, "PATCH", "/api/admin/coupons/"+id("Flat50"), map[string]any{"usage_limit": nil, "is_active": false, "expires_at": nil}); code != http.StatusOK ||
		out["usage_limit"] != nil || out["is_active"] != false || out["expires_at"] != nil {
		t.Errorf("null: %d %v", code, out)
	}
	if code, out := call(owner, "PATCH", "/api/admin/coupons/"+id("Flat50"), map[string]any{"starts_at": "2030-01-01T00:00:00Z", "expires_at": "2029-01-01T00:00:00Z"}); code != http.StatusBadRequest {
		t.Errorf("patch tarikh: %d %v", code, out)
	}
	if code, out := call(owner, "PATCH", "/api/admin/coupons/"+id("Flat50"), map[string]any{"used_count": 0}); code != http.StatusBadRequest {
		t.Errorf("used_count: %d %v", code, out)
	}
	if n := f.used(t, "Flat50"); n != 4 {
		t.Errorf("used_count bodleche: %d", n)
	}

	// list: paged, notun age, shudhu a er
	code, list := call(owner, "GET", "/api/admin/coupons?limit=2", nil)
	if code != http.StatusOK || list["total"] != 11.0 || len(list["items"].([]any)) != 2 || list["items"].([]any)[0].(map[string]any)["code"] != "BONLY" {
		t.Errorf("list: %d %v", code, list)
	}
	if code, l := call(ownerB, "GET", "/api/admin/coupons", nil); code != http.StatusOK || l["total"] != 1.0 {
		t.Errorf("b list: %d %v", code, l)
	}
	// onno shop 404, kichu bodlay na
	if code, _ := call(ownerB, "PATCH", "/api/admin/coupons/"+id("PP1"), map[string]any{"is_active": false}); code != http.StatusNotFound {
		t.Errorf("onno shop: %d", code)
	}
	if code, _ := call(owner, "DELETE", "/api/admin/coupons/"+id("PP1"), nil); code != http.StatusMethodNotAllowed {
		t.Errorf("delete: %d", code)
	}
	for _, role := range []string{"staff"} {
		st := f.staff(t, f.shopA, role)
		if code, _ := call(st, "GET", "/api/admin/coupons", nil); code != http.StatusForbidden {
			t.Errorf("%s: %d", role, code)
		}
		if code, _ := call(st, "PATCH", "/api/admin/coupons/"+id("PP1"), map[string]any{"is_active": false}); code != http.StatusForbidden {
			t.Errorf("%s patch: %d", role, code)
		}
	}
	var n int
	f.d.QueryRow(`SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND entity_type = 'coupon' AND action IN ('coupon.create', 'coupon.update')`, f.shopA).Scan(&n)
	if n != 6 { // 2 create + 4 sofol patch
		t.Errorf("audit %d", n)
	}
}
