package auth_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/httpx"
)

// syncBuf: handler er log (goroutine theke o).
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type staffAdminFx struct {
	*staffFixture
	t   *testing.T
	srv http.Handler
	log *syncBuf
}

func (f *staffAdminFx) call(st httpx.Staff, method, path string, body any) (int, map[string]any, *httptest.ResponseRecorder) {
	f.t.Helper()
	var b bytes.Buffer
	if s, ok := body.(string); ok {
		b.WriteString(s)
	} else if body != nil {
		json.NewEncoder(&b).Encode(body)
	}
	r := httptest.NewRequest(method, path, &b)
	r = r.WithContext(httpx.WithStaff(r.Context(), st))
	w := httptest.NewRecorder()
	f.srv.ServeHTTP(w, r)
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		f.t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
	}
	return w.Code, out, w
}

func (f *staffAdminFx) want(st httpx.Staff, method, path string, body any, status int, code string) map[string]any {
	f.t.Helper()
	c, out, w := f.call(st, method, path, body)
	if c != status || (code != "" && errCode(w) != code) {
		f.t.Errorf("%s %s %v: %d %s, chai %d %s", method, path, body, c, w.Body, status, code)
	}
	return out
}

func (f *staffAdminFx) field(st httpx.Staff, method, path string, body any, field string) {
	f.t.Helper()
	_, out, w := f.call(st, method, path, body)
	e, _ := out["error"].(map[string]any)
	fs, _ := e["fields"].(map[string]any)
	if w.Code != http.StatusBadRequest || fs[field] == nil {
		f.t.Errorf("%s %s %v: %d %s, chai 400 fields.%s", method, path, body, w.Code, w.Body, field)
	}
}

// session: staff er active session (DB te sorasori).
func (f *staffAdminFx) session(shopID, staffID int64) {
	f.exec(f.t, `INSERT INTO sessions (shop_id, token_hash, subject_type, staff_id, stage, expires_at)
		VALUES ($1, $2, 'staff', $3, 'active', now() + interval '1 hour')`, shopID, hashOf(strconv.FormatInt(staffID, 10)+"-"+
		strconv.Itoa(f.count(f.t, `SELECT count(*) FROM sessions`))), staffID)
}

func (f *staffAdminFx) live(staffID int64) int {
	return f.count(f.t, `SELECT count(*) FROM sessions WHERE staff_id = $1 AND revoked_at IS NULL`, staffID)
}

// noSecret: temp password audit, log e na (11.7 item 11).
func (f *staffAdminFx) noSecret(pw string) {
	f.t.Helper()
	if n := f.count(f.t, `SELECT count(*) FROM audit_logs WHERE strpos(coalesce(before::text, '') || coalesce(after::text, ''), $1) > 0`, pw); n != 0 {
		f.t.Errorf("audit e temp password (%d row)", n)
	}
	if strings.Contains(f.log.String(), pw) {
		f.t.Error("log e temp password")
	}
}

func path(id int64, rest string) string {
	return "/api/admin/staff/" + strconv.FormatInt(id, 10) + rest
}

// TestStaffAdminIntegration (7.7, 8.3, 11.2, 11.7 item 11, 12.2, v1.46): GET, POST /staff, PATCH /staff/{id},
// POST /staff/{id}/reset shudhu owner (admin, staff, vendor 403); temp password crypto/rand, response e ekbar
// (no-store), audit/log e na, tai diye login e setup_pending; email/role/name validation, duplicate 409
// staff_exists; role/is_active bodlale oi staff er shob session revoke (naam e na); nijeke deactivate/role bodol
// 403; duijon owner ek shathe ekjon arek jon ke namale-o ek active owner thake; reset e must_change_password,
// TOTP ar recovery code muche, lock khole, session revoke, nijeke 403; onno shop er staff 404.
func TestStaffAdminIntegration(t *testing.T) {
	sf := newStaffFixture(t)
	f := &staffAdminFx{staffFixture: sf, t: t, log: &syncBuf{}}
	mux := http.NewServeMux()
	auth.NewStaffAdmin(f.d, slog.New(slog.NewTextHandler(f.log, nil))).AdminRoutes(mux, func(h http.Handler) http.Handler { return h })
	f.srv = httpx.JSONErrors(mux)
	a, b := f.shops["a.com"], f.shops["b.com"]
	owner := httpx.Staff{ID: f.owner["a"], ShopID: a, Role: "owner"}
	ownerB := httpx.Staff{ID: f.owner["b"], ShopID: b, Role: "owner"}
	var adminID int64
	if err := f.d.QueryRow(`INSERT INTO staff (shop_id, email, name, password_hash, role) VALUES ($1, 'ad@a.com', 'Ad', 'x', 'admin')
		RETURNING id`, a).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	admin := httpx.Staff{ID: adminID, ShopID: a, Role: "admin"}

	// create
	_, out, w := f.call(owner, "POST", "/api/admin/staff", map[string]string{"email": " Rahim@A.com ", "name": " Rahim ", "role": "staff"})
	if w.Code != http.StatusCreated || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create: %d %q %s", w.Code, w.Header().Get("Cache-Control"), w.Body)
	}
	pw, _ := out["temp_password"].(string)
	if len(pw) != 26 || out["email"] != "Rahim@A.com" || out["name"] != "Rahim" || out["role"] != "staff" || out["is_active"] != true ||
		out["must_change_password"] != true || out["totp_enabled"] != false || out["vendor_id"] != nil || out["locked_until"] != nil {
		t.Errorf("create row: %v", out)
	}
	for _, k := range []string{"password_hash", "totp_secret_enc", "failed_logins"} {
		if _, ok := out[k]; ok {
			t.Errorf("create e %s", k)
		}
	}
	sid := int64(out["id"].(float64))
	if n := f.count(t, `SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND staff_id = $2 AND action = 'staff.create'
		AND entity_type = 'staff' AND entity_id = $3 AND after = '{"email":"Rahim@A.com","name":"Rahim","role":"staff"}'`, a, owner.ID, sid); n != 1 {
		t.Errorf("staff.create audit %d", n)
	}
	f.noSecret(pw)
	if rec := f.login("a", "rahim@a.com", pw); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"setup_required":true}` {
		t.Errorf("temp password e login: %d %s", rec.Code, rec.Body)
	}

	// validation, duplicate
	f.want(owner, "POST", "/api/admin/staff", map[string]string{"email": "RAHIM@a.com", "name": "R2", "role": "staff"}, http.StatusConflict, "staff_exists")
	for _, c := range []struct {
		body  any
		field string
	}{
		{map[string]string{"email": "x@a.com", "name": "X", "role": "vendor_staff"}, "role"},
		{map[string]string{"email": "x@a.com", "name": "X", "role": "boss"}, "role"},
		{map[string]string{"email": "nai", "name": "X", "role": "staff"}, "email"},
		{map[string]string{"email": "X <x@a.com>", "name": "X", "role": "staff"}, "email"},
		{map[string]string{"email": strings.Repeat("x", 250) + "@a.com", "name": "X", "role": "staff"}, "email"},
		{map[string]string{"email": "x@a.com", "name": "  ", "role": "staff"}, "name"},
		{map[string]string{"email": "x@a.com", "name": strings.Repeat("ন", 101), "role": "staff"}, "name"},
		{map[string]any{"email": "x@a.com", "name": "X", "role": "staff", "vendor_id": 1}, "body"},
	} {
		f.field(owner, "POST", "/api/admin/staff", c.body, c.field)
	}
	f.want(ownerB, "POST", "/api/admin/staff", map[string]string{"email": "rahim@a.com", "name": "B", "role": "admin"}, http.StatusCreated, "")

	// list: shudhu nijer shop, secret na
	_, l, w := f.call(owner, "GET", "/api/admin/staff", nil)
	items, _ := l["items"].([]any)
	if w.Code != http.StatusOK || l["total"] != float64(3) || len(items) != 3 || strings.Contains(w.Body.String(), "password") ||
		strings.Contains(w.Body.String(), "totp_secret") || items[0].(map[string]any)["totp_enabled"] != true {
		t.Errorf("list: %d %s", w.Code, w.Body)
	}

	// shudhu owner
	for _, st := range []httpx.Staff{admin, {ID: sid, ShopID: a, Role: "staff"}} {
		f.want(st, "GET", "/api/admin/staff", nil, http.StatusForbidden, "forbidden")
		f.want(st, "POST", "/api/admin/staff", map[string]string{"email": "y@a.com", "name": "Y", "role": "staff"}, http.StatusForbidden, "forbidden")
		f.want(st, "PATCH", path(sid, ""), map[string]string{"name": "Y"}, http.StatusForbidden, "forbidden")
		f.want(st, "POST", path(sid, "/reset"), nil, http.StatusForbidden, "forbidden")
	}
	// onno shop er staff
	f.want(ownerB, "PATCH", path(sid, ""), map[string]string{"name": "Y"}, http.StatusNotFound, "not_found")
	f.want(ownerB, "POST", path(sid, "/reset"), nil, http.StatusNotFound, "not_found")
	f.want(owner, "PATCH", path(999999, ""), map[string]string{"name": "Y"}, http.StatusNotFound, "not_found")

	// patch: naam e session thake; role/is_active e revoke
	f.session(a, sid)
	f.session(a, sid)
	if p := f.want(owner, "PATCH", path(sid, ""), map[string]string{"name": "Rahim U"}, http.StatusOK, ""); p["name"] != "Rahim U" || f.live(sid) != 2 {
		t.Errorf("name patch: %v, session %d", p, f.live(sid))
	}
	if p := f.want(owner, "PATCH", path(sid, ""), map[string]string{"role": "admin"}, http.StatusOK, ""); p["role"] != "admin" || f.live(sid) != 0 {
		t.Errorf("role patch: %v, session %d", p, f.live(sid))
	}
	if n := f.count(t, `SELECT count(*) FROM audit_logs WHERE action = 'staff.update' AND entity_id = $1 AND
		((before = '{"name":"Rahim"}' AND after = '{"name":"Rahim U"}') OR (before = '{"role":"staff"}' AND after = '{"role":"admin"}'))`, sid); n != 2 {
		t.Errorf("staff.update audit %d", n)
	}
	f.session(a, sid)
	f.want(owner, "PATCH", path(sid, ""), map[string]any{"role": "admin", "name": "Rahim U"}, http.StatusOK, "") // bodol nai
	if f.live(sid) != 1 {
		t.Error("ekoi role e revoke")
	}
	if p := f.want(owner, "PATCH", path(sid, ""), map[string]bool{"is_active": false}, http.StatusOK, ""); p["is_active"] != false || f.live(sid) != 0 {
		t.Errorf("deactivate: %v, session %d", p, f.live(sid))
	}
	if rec := f.login("a", "rahim@a.com", pw); rec.Code != http.StatusUnauthorized {
		t.Errorf("inactive login: %d", rec.Code)
	}
	f.want(owner, "PATCH", path(sid, ""), map[string]bool{"is_active": true}, http.StatusOK, "")
	f.field(owner, "PATCH", path(sid, ""), map[string]string{"role": "vendor_owner"}, "role")
	f.field(owner, "PATCH", path(sid, ""), map[string]string{"name": ""}, "name")
	f.field(owner, "PATCH", path(sid, ""), map[string]string{"email": "z@a.com"}, "body")

	// nijeke
	f.session(a, owner.ID)
	f.want(owner, "PATCH", path(owner.ID, ""), map[string]string{"role": "admin"}, http.StatusForbidden, "forbidden")
	f.want(owner, "PATCH", path(owner.ID, ""), map[string]bool{"is_active": false}, http.StatusForbidden, "forbidden")
	f.want(owner, "PATCH", path(owner.ID, ""), map[string]any{"role": "owner", "is_active": true, "name": "O2"}, http.StatusOK, "")
	f.want(owner, "POST", path(owner.ID, "/reset"), nil, http.StatusForbidden, "forbidden")
	if f.live(owner.ID) != 1 || f.count(t, `SELECT count(*) FROM staff WHERE id = $1 AND role = 'owner' AND is_active AND name = 'O2'`, owner.ID) != 1 {
		t.Error("nijer patch e owner bodleche ba session gelo")
	}

	// duijon owner ek shathe ekjon arek jon ke namay: ek jon pare, active owner ekjon thake
	for i := 0; i < 5; i++ {
		_, o2, _ := f.call(owner, "POST", "/api/admin/staff", map[string]string{"email": "co" + strconv.Itoa(i) + "@a.com", "name": "Co", "role": "owner"})
		co := httpx.Staff{ID: int64(o2["id"].(float64)), ShopID: a, Role: "owner"}
		var x any = map[string]string{"role": "staff"}
		if i%2 == 1 {
			x = map[string]bool{"is_active": false}
		}
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for j, c := range [][2]httpx.Staff{{owner, co}, {co, owner}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[j], _, _ = f.call(c[0], "PATCH", path(c[1].ID, ""), x)
			}()
		}
		wg.Wait()
		n := f.count(t, `SELECT count(*) FROM staff WHERE shop_id = $1 AND role = 'owner' AND is_active`, a)
		if n != 1 || codes[0]+codes[1] != http.StatusOK+http.StatusForbidden {
			t.Fatalf("race %d: active owner %d, codes %v", i, n, codes)
		}
		if codes[0] == http.StatusForbidden { // co jitlo: owner ke abar owner koro (DB te), porer round er jonno
			f.exec(t, `UPDATE staff SET role = 'owner', is_active = true WHERE id = $1`, owner.ID)
			f.exec(t, `UPDATE staff SET is_active = false WHERE id = $1`, co.ID)
		}
	}

	// reset: TOTP, recovery, session, lock muche; must_change_password; notun temp password e login
	f.setTOTP(t, a, sid)
	f.exec(t, `INSERT INTO staff_recovery_codes (shop_id, staff_id, code_hash) VALUES ($1, $2, '\x01')`, a, sid)
	f.exec(t, `UPDATE staff SET must_change_password = false, failed_logins = 7, failed_mfa = 3, locked_until = now() + interval '1 hour'
		WHERE id = $1`, sid)
	f.session(a, sid)
	if _, l, _ := f.call(owner, "GET", "/api/admin/staff?limit=100", nil); !strings.Contains(mustJSON(l), `"locked_until":"`) {
		t.Errorf("list e locked_until nai: %v", l)
	}
	_, r, w := f.call(owner, "POST", path(sid, "/reset"), nil)
	pw2, _ := r["temp_password"].(string)
	if w.Code != http.StatusOK || len(r) != 1 || len(pw2) != 26 || pw2 == pw || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("reset: %d %s", w.Code, w.Body)
	}
	if n := f.count(t, `SELECT count(*) FROM staff WHERE id = $1 AND must_change_password AND totp_secret_enc IS NULL AND totp_pending_enc IS NULL
		AND failed_logins = 0 AND failed_mfa = 0 AND locked_until IS NULL`, sid); n != 1 {
		t.Error("reset er por staff row")
	}
	if f.live(sid) != 0 || f.count(t, `SELECT count(*) FROM staff_recovery_codes WHERE staff_id = $1`, sid) != 0 {
		t.Error("reset e session/recovery baki")
	}
	if n := f.count(t, `SELECT count(*) FROM audit_logs WHERE action = 'staff.reset' AND staff_id = $1 AND entity_id = $2 AND before IS NULL
		AND after = '{"must_change_password":true,"totp_cleared":true}'`, owner.ID, sid); n != 1 {
		t.Errorf("staff.reset audit %d", n)
	}
	f.noSecret(pw2)
	if rec := f.login("a", "rahim@a.com", pw); rec.Code != http.StatusUnauthorized {
		t.Errorf("purono password: %d", rec.Code)
	}
	if rec := f.login("a", "rahim@a.com", pw2); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"setup_required":true}` {
		t.Errorf("reset er por login: %d %s", rec.Code, rec.Body)
	}
	// inactive staff eo reset chole
	f.want(owner, "PATCH", path(sid, ""), map[string]bool{"is_active": false}, http.StatusOK, "")
	f.want(owner, "POST", path(sid, "/reset"), nil, http.StatusOK, "")
	f.noSecret(pw2)
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
