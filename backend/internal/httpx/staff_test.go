package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func ok204(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }

// TestRequireStaff (3.2, v1.10): staff context na thakle shob shomoy 401 unauthenticated (fail-closed; login M4 e).
func TestRequireStaff(t *testing.T) {
	h := RequireStaff(http.HandlerFunc(ok204))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/admin/products", nil))
	if rec.Code != 401 || errCode(t, rec) != "unauthenticated" {
		t.Errorf("staff chhara: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/admin/products", nil)
	h.ServeHTTP(rec, r.WithContext(WithStaff(r.Context(), Staff{ID: 1, ShopID: 1, Role: "owner"})))
	if rec.Code != 204 {
		t.Errorf("staff shoho: %d", rec.Code)
	}
	if _, ok := StaffFrom(context.Background()); ok {
		t.Error("context chhara staff ache")
	}
}

// TestRequireRole (11.2): role list e na thakle 403 forbidden; staff na thakle 401; vendor staff er vendor context e.
func TestRequireRole(t *testing.T) {
	h := RequireRole("owner", "admin")(http.HandlerFunc(ok204))
	vendor := int64(7)
	for _, c := range []struct {
		staff  *Staff
		status int
	}{
		{nil, 401},
		{&Staff{ID: 1, ShopID: 1, Role: "owner"}, 204},
		{&Staff{ID: 2, ShopID: 1, Role: "admin"}, 204},
		{&Staff{ID: 3, ShopID: 1, Role: "staff"}, 403},
		{&Staff{ID: 4, ShopID: 1, Role: "vendor_owner", VendorID: &vendor}, 403},
	} {
		r := httptest.NewRequest("POST", "/api/admin/categories", nil)
		if c.staff != nil {
			r = r.WithContext(WithStaff(r.Context(), *c.staff))
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != c.status {
			t.Errorf("%+v: %d, chai %d", c.staff, rec.Code, c.status)
		}
		if c.status == 403 && errCode(t, rec) != "forbidden" {
			t.Errorf("%+v: %s", c.staff, rec.Body.String())
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	if s, _ := StaffFrom(WithStaff(r.Context(), Staff{VendorID: &vendor})); s.VendorID == nil || *s.VendorID != 7 {
		t.Error("vendor_id context e nai")
	}
}
