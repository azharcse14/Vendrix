package audit_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"vendrix/backend/internal/audit"
	"vendrix/backend/internal/customer"
	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/shop"
)

type auditFx struct {
	t   *testing.T
	d   *sql.DB
	srv http.Handler
}

func (f *auditFx) id(q string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.d.QueryRow(q, args...).Scan(&id); err != nil {
		f.t.Fatalf("%s: %v", q, err)
	}
	return id
}

func (f *auditFx) call(st httpx.Staff, method, path string, body any) (int, []byte) {
	f.t.Helper()
	var b bytes.Buffer
	if body != nil {
		json.NewEncoder(&b).Encode(body)
	}
	r := httptest.NewRequest(method, path, &b)
	r = r.WithContext(httpx.WithStaff(r.Context(), st))
	w := httptest.NewRecorder()
	f.srv.ServeHTTP(w, r)
	return w.Code, w.Body.Bytes()
}

type auditPage struct {
	Items []map[string]any `json:"items"`
	Total int64            `json:"total"`
}

// list: GET /api/admin/audit-logs + query → action gulo (kram e) ar total.
func (f *auditFx) list(st httpx.Staff, query string) ([]string, int64, auditPage) {
	f.t.Helper()
	code, b := f.call(st, "GET", "/api/admin/audit-logs"+query, nil)
	var p auditPage
	if code != http.StatusOK || json.Unmarshal(b, &p) != nil {
		f.t.Fatalf("list %q: %d %s", query, code, b)
	}
	var acts []string
	for _, it := range p.Items {
		acts = append(acts, it["action"].(string))
	}
	return acts, p.Total, p
}

// TestAuditListIntegration (7.7, 11.2, 11.7 item 11, v1.46): GET /audit-logs owner/admin (staff, vendor 403),
// notun age, paged, action/staff_id/from/to (shop timezone e din, to shoho) filter, bhul filter 400; onno shop er
// row na (onno shop er staff_id e khali); CLI row (staff NULL) e staff_name null; row e puro phone na (block er
// audit masked).
func TestAuditListIntegration(t *testing.T) {
	f := &auditFx{t: t, d: dbtest.New(t)}
	ctx := context.Background()
	var shops, owners [2]int64
	for i, s := range []string{"a", "b"} {
		id, _, err := shop.Create(ctx, f.d, shop.Params{Slug: s, Prefix: "AL" + strings.ToUpper(s), Domain: s + ".com",
			NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "Owner " + s, Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		shops[i] = id
		owners[i] = f.id(`SELECT id FROM staff WHERE shop_id = $1`, id)
	}
	a := shops[0]
	f.id(`UPDATE shops SET timezone = 'Pacific/Kiritimati' WHERE id = $1 RETURNING id`, a)
	staffID := f.id(`INSERT INTO staff (shop_id, email, name, password_hash, role) VALUES ($1, 's@a.com', 'Staff S', 'x', 'staff') RETURNING id`, a)
	// shop er ajker shuru (local midnight) theke minute
	row := func(shopID int64, staff any, action string, min int) {
		f.id(`INSERT INTO audit_logs (shop_id, staff_id, action, entity_type, entity_id, after, ip, created_at)
			VALUES ($1, $2, $3, 'x', 7, '{"k":1}', '203.0.113.9',
			(now() AT TIME ZONE 'Pacific/Kiritimati')::date::timestamp AT TIME ZONE 'Pacific/Kiritimati' + make_interval(mins => $4))
			RETURNING id`, shopID, staff, action, min)
	}
	row(a, owners[0], "category.update", 2)
	row(a, staffID, "batch.create", 1)
	row(a, nil, "staff.reset_2fa", -1) // kal (shop er din), CLI
	row(shops[1], owners[1], "category.update", 3)

	mux := http.NewServeMux()
	noMW := func(h http.Handler) http.Handler { return h }
	log := slog.New(slog.DiscardHandler)
	audit.NewHandlers(f.d, log).AdminRoutes(mux, noMW)
	customer.NewHandlers(f.d, log).AdminRoutes(mux, noMW)
	f.srv = httpx.JSONErrors(mux)

	owner := httpx.Staff{ID: owners[0], ShopID: a, Role: "owner"}
	admin := httpx.Staff{ID: f.id(`INSERT INTO staff (shop_id, email, name, password_hash, role) VALUES ($1, 'ad@a.com', 'A', 'x', 'admin') RETURNING id`, a),
		ShopID: a, Role: "admin"}
	// block er audit (masked phone): list e puro phone na
	if code, b := f.call(owner, "POST", "/api/admin/blocked-phones", map[string]string{"phone": "01711000001", "reason": "x"}); code != http.StatusCreated {
		t.Fatalf("block: %d %s", code, b)
	}
	// kram thik rakhte block er row shop er din er +3 minute e (test DB user superuser)
	f.id(`UPDATE audit_logs SET created_at = (now() AT TIME ZONE 'Pacific/Kiritimati')::date::timestamp AT TIME ZONE 'Pacific/Kiritimati'
		+ interval '3 minutes' WHERE action = 'customer.block' RETURNING id`)
	if code, b := f.call(owner, "GET", "/api/admin/audit-logs", nil); code != http.StatusOK || bytes.Contains(b, []byte("1711000001")) {
		t.Errorf("list e phone: %d %s", code, b)
	}

	for _, st := range []httpx.Staff{owner, admin} {
		acts, total, p := f.list(st, "")
		if want := []string{"customer.block", "category.update", "batch.create", "staff.reset_2fa"}; !slices.Equal(acts, want) || total != 4 {
			t.Errorf("%s list: %v %d", st.Role, acts, total)
		}
		keys := make([]string, 0)
		for k := range p.Items[1] {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		if want := []string{"action", "after", "before", "created_at", "entity_id", "entity_type", "id", "ip", "staff_id", "staff_name"}; !slices.Equal(keys, want) {
			t.Errorf("row key: %v", keys)
		}
		if it := p.Items[1]; it["staff_name"] != "Owner a" || it["ip"] != "203.0.113.9" || it["entity_id"] != float64(7) ||
			it["after"].(map[string]any)["k"] != float64(1) || it["before"] != nil {
			t.Errorf("row: %v", it)
		}
		if it := p.Items[3]; it["staff_id"] != nil || it["staff_name"] != nil {
			t.Errorf("CLI row: %v", it)
		}
	}

	for q, want := range map[string][]string{
		"?action=category.update":                       {"category.update"},
		"?action=category":                              nil,
		"?staff_id=" + itoa(staffID):                    {"batch.create"},
		"?staff_id=" + itoa(owners[1]):                  nil,
		"?from=" + today(f) + "&to=" + today(f):         {"customer.block", "category.update", "batch.create"},
		"?to=" + yesterday(f):                           {"staff.reset_2fa"},
		"?from=" + yesterday(f) + "&to=" + yesterday(f): {"staff.reset_2fa"},
		"?limit=2&page=2":                               {"batch.create", "staff.reset_2fa"},
	} {
		if acts, _, _ := f.list(owner, q); !slices.Equal(acts, want) {
			t.Errorf("%s: %v, chai %v", q, acts, want)
		}
	}
	if _, total, _ := f.list(owner, "?limit=2&page=2"); total != 4 {
		t.Errorf("page total %d", total)
	}
	for q, field := range map[string]string{"?from=2026-13-01": "from", "?to=x": "to", "?staff_id=x": "staff_id",
		"?staff_id=0": "staff_id", "?action=" + strings.Repeat("a", 65): "action", "?limit=101": "limit"} {
		code, b := f.call(owner, "GET", "/api/admin/audit-logs"+q, nil)
		if code != http.StatusBadRequest || !strings.Contains(string(b), `"`+field+`":"invalid"`) {
			t.Errorf("%s: %d %s", q, code, b)
		}
	}

	// shop b er owner shudhu nijer
	if acts, total, _ := f.list(httpx.Staff{ID: owners[1], ShopID: shops[1], Role: "owner"}, ""); !slices.Equal(acts, []string{"category.update"}) || total != 1 {
		t.Errorf("shop b: %v %d", acts, total)
	}
	vnd := f.id(`SELECT id FROM vendors WHERE shop_id = $1`, a)
	for _, st := range []httpx.Staff{{ID: staffID, ShopID: a, Role: "staff"}, {ID: staffID, ShopID: a, Role: "vendor_owner", VendorID: &vnd},
		{ID: staffID, ShopID: a, Role: "vendor_staff", VendorID: &vnd}} {
		if code, b := f.call(st, "GET", "/api/admin/audit-logs", nil); code != http.StatusForbidden {
			t.Errorf("%s: %d %s", st.Role, code, b)
		}
	}
}

func itoa(n int64) string {
	return json.Number(strings.TrimSpace(string(must(json.Marshal(n))))).String()
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

func shopDate(f *auditFx, days int) string {
	f.t.Helper()
	var s string
	if err := f.d.QueryRow(`SELECT to_char((now() AT TIME ZONE 'Pacific/Kiritimati')::date + $1, 'YYYY-MM-DD')`, days).Scan(&s); err != nil {
		f.t.Fatal(err)
	}
	return s
}

func today(f *auditFx) string     { return shopDate(f, 0) }
func yesterday(f *auditFx) string { return shopDate(f, -1) }
