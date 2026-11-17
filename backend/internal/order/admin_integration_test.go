package order_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"vendrix/backend/internal/httpx"
)

// staffAs: shop a te notun staff (role, vendor 0 = shop er staff); owner = shop create er.
func (f *checkoutFx) staffAs(role string, vendor int64) httpx.Staff {
	f.t.Helper()
	st := httpx.Staff{ShopID: f.shop, Role: role}
	if role == "owner" {
		st.ID = f.id(`SELECT id FROM staff WHERE shop_id = $1 AND role = 'owner'`, f.shop)
		return st
	}
	if vendor != 0 {
		st.VendorID = &vendor
	}
	st.ID = f.id(`INSERT INTO staff (shop_id, vendor_id, email, name, password_hash, role)
		VALUES ($1, NULLIF($2, 0), $3 || '-' || $2 || '@a.com', 'S', 'x', $3) RETURNING id`, f.shop, vendor, role)
	return st
}

// adm: admin request staff context e (route er middleware test e identity; RequireRole handler e).
func (f *checkoutFx) adm(st httpx.Staff, method, path string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	var b bytes.Buffer
	if s, ok := body.(string); ok {
		b.WriteString(s)
	} else if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			f.t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &b)
	r = r.WithContext(httpx.WithStaff(r.Context(), st))
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r)
	return rec
}

func (f *checkoutFx) admJSON(st httpx.Staff, method, path string, body any, status int) map[string]any {
	f.t.Helper()
	rec := f.adm(st, method, path, body)
	var out map[string]any
	dec := json.NewDecoder(rec.Body)
	dec.UseNumber() // taka ar id int64 (Rule 2: order package e float na)
	if rec.Code != status || dec.Decode(&out) != nil {
		f.t.Fatalf("%s %s: chai %d, pelam %d %s", method, path, status, rec.Code, rec.Body.String())
	}
	return out
}

func (f *checkoutFx) number(oid int64) string {
	return f.str(`SELECT order_number FROM orders WHERE id = $1`, oid)
}

// listNumbers: GET list er order_number (kram e) ar total.
func (f *checkoutFx) listNumbers(st httpx.Staff, query string) ([]string, json.Number) {
	f.t.Helper()
	m := f.admJSON(st, "GET", "/api/admin/orders"+query, nil, http.StatusOK)
	var out []string
	for _, it := range m["items"].([]any) {
		out = append(out, it.(map[string]any)["order_number"].(string))
	}
	return out, m["total"].(json.Number)
}

func s(n int64) string { return strconv.FormatInt(n, 10) }

func num(n int64) json.Number { return json.Number(s(n)) }

// TestAdminOrderListIntegration (7.7, 11.7 item 13 vii, ix): filter status = display_status, tab "confirm er
// opekkhay" (COD pending), "deadline kache", risk=1, q = order number ba phone (customer/recipient), from/to shop
// timezone e, placed_at DESC, paging (max 100), shop staff er vendor_id filter; vendor role e nijer fulfillment er
// order, total nijer, payment/customer field nai; GET /fulfillments.
func TestAdminOrderListIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 10000, 50)
	vx, rx := f.vendorVariant("vx", 50)
	vy, ry := f.vendorVariant("vy", 50)
	one := func(v, item int64, status string) seedF {
		return seedF{vendor: v, status: status, items: [][2]int64{{item, 1}}}
	}

	o1, _ := f.seedOrder("cod", "unpaid", "pending", one(f.vnd, rice, "open"))
	o2, _ := f.seedOrder("cod", "unpaid", "confirmed", one(f.vnd, rice, "open"))
	o3, _ := f.seedOrder("sslcommerz", "unpaid", "pending", one(f.vnd, rice, "open"))
	o4, _ := f.seedOrder("cod", "unpaid", "confirmed", one(f.vnd, rice, "packed"))
	o5, _ := f.seedOrder("cod", "unpaid", "confirmed", one(f.vnd, rice, "delivered"))
	o6, f6 := f.seedOrder("cod", "unpaid", "confirmed", one(vx, rx, "open"), seedF{vendor: vy, status: "open", items: [][2]int64{{ry, 2}}})
	for _, x := range []struct {
		id       int64
		placed   string
		deadline string
	}{
		{o1, "2026-01-10T17:30:00Z", ""}, // Dhaka 10 January 23:30
		{o2, "2026-01-10T18:00:00Z", ""}, // Dhaka 11 January 00:00
		{o3, "2026-01-11T01:00:00Z", "48 hours"},
		{o4, "2026-01-11T02:00:00Z", "2 hours"},
		{o5, "2026-01-11T03:00:00Z", "2 hours"}, // delivered: deadline kache te na
		{o6, "2026-01-11T04:00:00Z", ""},
	} {
		f.exec(`UPDATE orders SET placed_at = $2, delivery_deadline = now() + NULLIF($3, '')::interval WHERE id = $1`, x.id, x.placed, x.deadline)
	}
	f.exec(`UPDATE orders SET customer_phone = '+8801811000002', risk_flags = '["velocity"]' WHERE id = $1`, o2)
	f.exec(`UPDATE orders SET recipient_phone = '+8801911000003' WHERE id = $1`, o3)
	n := func(ids ...int64) []string {
		var out []string
		for _, id := range ids {
			out = append(out, f.number(id))
		}
		return out
	}
	owner, staff := f.staffAs("owner", 0), f.staffAs("staff", 0)

	for _, c := range []struct {
		st    httpx.Staff
		query string
		want  []string
	}{
		{owner, "", n(o6, o5, o4, o3, o2, o1)},
		{staff, "?status=pending", n(o3, o1)},
		{staff, "?status=delivered", n(o5)},
		{staff, "?tab=awaiting_confirm", n(o1)},
		{staff, "?tab=deadline_soon", n(o4)},
		{staff, "?risk=1", n(o2)},
		{staff, "?q=" + strings.ToLower(f.number(o2)), n(o2)},
		{staff, "?q=01811000002", n(o2)},
		{staff, "?q=%2B8801911000003", n(o3)},
		{staff, "?q=01700000000", nil},
		{staff, "?from=2026-01-11", n(o6, o5, o4, o3, o2)},
		{staff, "?to=2026-01-10", n(o1)},
		{staff, "?from=2026-01-11&to=2026-01-11&status=confirmed", n(o6, o2)},
		{owner, "?vendor_id=" + s(vx), n(o6)},
		{owner, "?limit=2&page=2", n(o4, o3)},
	} {
		got, _ := f.listNumbers(c.st, c.query)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s %q: %v, chai %v", c.st.Role, c.query, got, c.want)
		}
	}
	if _, total := f.listNumbers(owner, "?limit=2&page=2"); total != "6" {
		t.Errorf("total %v", total)
	}
	for _, q := range []string{"?limit=101", "?status=nai", "?tab=nai", "?from=11-01-2026", "?to=2026-13-01", "?from=0000-01-01", "?risk=2", "?vendor_id=x"} {
		wantErr(t, q, f.adm(owner, "GET", "/api/admin/orders"+q, nil), http.StatusBadRequest, "validation_failed")
	}

	// shop staff er row e customer ar payment
	m := f.admJSON(staff, "GET", "/api/admin/orders?q="+f.number(o2), nil, http.StatusOK)
	row := m["items"].([]any)[0].(map[string]any)
	if row["customer_phone"] != "+8801811000002" || row["payment_method"] != "cod" || row["payment_status"] != "unpaid" ||
		row["total"] != num(10000) || row["display_status"] != "confirmed" || row["id"] != num(o2) {
		t.Errorf("shop staff row: %v", row)
	}

	// vendor role: nijer fulfillment er order, total nijer ongsho, payment/customer na
	vo := f.staffAs("vendor_owner", vy)
	if got, _ := f.listNumbers(vo, ""); !slices.Equal(got, n(o6)) {
		t.Errorf("vendor list: %v", got)
	}
	if got, _ := f.listNumbers(vo, "?q="+f.number(o1)); got != nil {
		t.Errorf("vendor q onno order: %v", got)
	}
	if got, _ := f.listNumbers(vo, "?vendor_id="+s(vx)); !slices.Equal(got, n(o6)) {
		t.Errorf("vendor er vendor_id filter session er vendor e: %v", got)
	}
	m = f.admJSON(vo, "GET", "/api/admin/orders", nil, http.StatusOK)
	row = m["items"].([]any)[0].(map[string]any)
	for _, k := range []string{"customer_phone", "customer_name", "payment_method", "payment_status", "risk_flags", "subtotal"} {
		if _, ok := row[k]; ok {
			t.Errorf("vendor row e %s: %v", k, row)
		}
	}
	if row["total"] != num(20000) || row["recipient_phone"] == nil {
		t.Errorf("vendor row total/recipient: %v", row)
	}

	// GET /fulfillments: status, vendor scope
	fl := f.admJSON(owner, "GET", "/api/admin/fulfillments?status=packed", nil, http.StatusOK)
	if items := fl["items"].([]any); len(items) != 1 || items[0].(map[string]any)["order_number"] != f.number(o4) {
		t.Errorf("fulfillments packed: %v", fl)
	}
	fl = f.admJSON(vo, "GET", "/api/admin/fulfillments", nil, http.StatusOK)
	if items := fl["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != num(f6[1]) {
		t.Errorf("vendor fulfillments: %v", fl)
	}
	fl = f.admJSON(owner, "GET", "/api/admin/fulfillments?date=2026-01-10", nil, http.StatusOK)
	if items := fl["items"].([]any); len(items) != 1 || items[0].(map[string]any)["order_number"] != f.number(o1) {
		t.Errorf("fulfillments date: %v", fl)
	}
	wantErr(t, "fulfillment status", f.adm(owner, "GET", "/api/admin/fulfillments?status=nai", nil), http.StatusBadRequest, "validation_failed")
}

// TestAdminOrderTransitionIntegration (7.7, 8.8, 11.2, 11.7 item 13 i, x): COD confirm staff 200 (history e staff
// actor), online unpaid 409 payment_required, cancel owner/admin, staff 403, abar confirm 409, bhul to 400; note;
// detail (fulfillment, item, history, note); protiti write e audit, phone/thikana chhara.
func TestAdminOrderTransitionIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 10000, 50)
	open := seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}}
	owner, admin, staff := f.staffAs("owner", 0), f.staffAs("admin", 0), f.staffAs("staff", 0)
	cod, _ := f.seedOrder("cod", "unpaid", "pending", open)
	online, _ := f.seedOrder("sslcommerz", "unpaid", "pending", open)
	tr := func(id int64) string { return "/api/admin/orders/" + s(id) + "/transition" }

	got := f.admJSON(staff, "POST", tr(cod), map[string]string{"to": "confirmed", "note": "phone e confirm"}, http.StatusOK)
	if got["status"] != "confirmed" || got["display_status"] != "confirmed" || f.orderStatus(cod) != "confirmed/confirmed" {
		t.Errorf("staff confirm: %v %s", got, f.orderStatus(cod))
	}
	if n := f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'confirmed'
		AND actor_type = 'staff' AND actor_id = $2 AND note = 'phone e confirm'`, cod, staff.ID); n != 1 {
		t.Errorf("confirm history %d", n)
	}
	wantErr(t, "abar confirm", f.adm(staff, "POST", tr(cod), map[string]string{"to": "confirmed"}), http.StatusConflict, "invalid_transition")
	wantErr(t, "online unpaid", f.adm(owner, "POST", tr(online), map[string]string{"to": "confirmed"}), http.StatusConflict, "payment_required")
	wantErr(t, "staff cancel", f.adm(staff, "POST", tr(cod), map[string]string{"to": "cancelled"}), http.StatusForbidden, "forbidden")
	if s := f.orderStatus(cod); s != "confirmed/confirmed" {
		t.Errorf("staff cancel er por %s", s)
	}
	for _, b := range []any{map[string]string{"to": "packed"}, map[string]string{}, `{"to":"cancelled","x":1}`,
		map[string]string{"to": "cancelled", "note": strings.Repeat("ন", 501)}} {
		wantErr(t, "bhul body", f.adm(admin, "POST", tr(cod), b), http.StatusBadRequest, "validation_failed")
	}
	vo := f.staffAs("vendor_owner", f.vnd)
	wantErr(t, "vendor transition", f.adm(vo, "POST", tr(cod), map[string]string{"to": "cancelled"}), http.StatusForbidden, "forbidden")
	f.admJSON(admin, "POST", tr(cod), map[string]string{"to": "cancelled", "note": "customer chay na"}, http.StatusOK)
	if s := f.str(`SELECT status || '/' || cancel_reason FROM orders WHERE id = $1`, cod); s != "cancelled/customer chay na" {
		t.Errorf("admin cancel: %s", s)
	}
	f.admJSON(owner, "POST", tr(online), map[string]string{"to": "cancelled"}, http.StatusOK)
	wantErr(t, "nai", f.adm(owner, "POST", tr(online+1000), map[string]string{"to": "cancelled"}), http.StatusNotFound, "not_found")

	// note
	nu := "/api/admin/orders/" + s(online) + "/notes"
	note := f.admJSON(staff, "POST", nu, map[string]string{"kind": "call", "note": " customer phone dhoreni "}, http.StatusCreated)
	if note["kind"] != "call" || note["note"] != "customer phone dhoreni" || note["staff_id"] != num(staff.ID) {
		t.Errorf("note: %v", note)
	}
	f.admJSON(owner, "POST", nu, map[string]string{"note": "abar"}, http.StatusCreated)
	for _, b := range []any{map[string]string{"note": " "}, map[string]string{"kind": "sms", "note": "x"},
		map[string]string{"note": strings.Repeat("x", 1001)}} {
		wantErr(t, "bhul note", f.adm(staff, "POST", nu, b), http.StatusBadRequest, "validation_failed")
	}
	wantErr(t, "vendor note", f.adm(vo, "POST", nu, map[string]string{"note": "x"}), http.StatusForbidden, "forbidden")

	// detail
	d := f.admJSON(staff, "GET", "/api/admin/orders/"+s(online), nil, http.StatusOK)
	fs := d["fulfillments"].([]any)
	items := fs[0].(map[string]any)["items"].([]any)
	if d["order_number"] != f.number(online) || d["payment_method"] != "sslcommerz" || d["address"] != "x" ||
		len(fs) != 1 || fs[0].(map[string]any)["status"] != "cancelled" || len(items) != 1 ||
		items[0].(map[string]any)["sku"] != "s" || len(d["notes"].([]any)) != 2 || len(d["history"].([]any)) < 2 {
		t.Errorf("detail: %v", d)
	}
	if _, ok := items[0].(map[string]any)["cost_price"]; ok {
		t.Errorf("item e cost_price: %v", items[0])
	}

	// audit: 2 cancel + 1 confirm + 2 note, phone/thikana nai
	rows, err := f.d.Query(`SELECT action, entity_type, COALESCE(before::text, ''), COALESCE(after::text, '') FROM audit_logs
		WHERE shop_id = $1 ORDER BY id`, f.shop)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var a, e, b, af string
		if err := rows.Scan(&a, &e, &b, &af); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, a+":"+e)
		noSecret(t, "audit "+a, b+af, "+8801711000009", "Dhanmondi", "phone dhoreni")
	}
	if want := []string{"order.transition:order", "order.transition:order", "order.transition:order", "order.note:order", "order.note:order"}; !slices.Equal(actions, want) {
		t.Errorf("audit: %v", actions)
	}
}

// TestFulfillmentRoleIntegration (14.2 test 35, 11.2 v1.7): fulfillment cancel staff 403, vendor_staff 403 (status
// bodlay na); vendor_owner nijer open 200, packed 409; admin open, owner packed 200; audit.
func TestFulfillmentRoleIntegration(t *testing.T) {
	f := newCheckout(t)
	vx, rx := f.vendorVariant("vx", 50)
	vy, ry := f.vendorVariant("vy", 50)
	fo := func(status string) int64 { // vx er fulfillment + vy er open (order cancel cascade e na jay)
		_, fs := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: vx, status: status, items: [][2]int64{{rx, 1}}},
			seedF{vendor: vy, status: "open", items: [][2]int64{{ry, 1}}})
		return fs[0]
	}
	tr := func(fid int64) string { return "/api/admin/fulfillments/" + s(fid) + "/transition" }
	st := func(fid int64) string { return f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fid) }
	cancel := map[string]string{"to": "cancelled", "note": "stock nai"}
	staff, vs := f.staffAs("staff", 0), f.staffAs("vendor_staff", vx)
	vo, admin, owner := f.staffAs("vendor_owner", vx), f.staffAs("admin", 0), f.staffAs("owner", 0)

	a, b, c, d := fo("open"), fo("open"), fo("packed"), fo("packed")
	wantErr(t, "staff open cancel", f.adm(staff, "POST", tr(a), cancel), http.StatusForbidden, "forbidden")
	wantErr(t, "staff packed cancel", f.adm(staff, "POST", tr(c), cancel), http.StatusForbidden, "forbidden")
	wantErr(t, "vendor_staff cancel", f.adm(vs, "POST", tr(a), cancel), http.StatusForbidden, "forbidden")
	if st(a) != "open" || st(c) != "packed" {
		t.Errorf("403 er por status bodleche: %s %s", st(a), st(c))
	}
	wantErr(t, "vendor_owner packed", f.adm(vo, "POST", tr(c), cancel), http.StatusConflict, "invalid_transition")
	wantErr(t, "bhul to", f.adm(owner, "POST", tr(a), map[string]string{"to": "nai"}), http.StatusBadRequest, "validation_failed")
	if got := f.admJSON(vo, "POST", tr(a), cancel, http.StatusOK); got["status"] != "cancelled" || st(a) != "cancelled" {
		t.Errorf("vendor_owner open cancel: %v", got)
	}
	f.admJSON(admin, "POST", tr(b), cancel, http.StatusOK)
	f.admJSON(owner, "POST", tr(d), cancel, http.StatusOK)
	if st(b) != "cancelled" || st(d) != "cancelled" {
		t.Errorf("admin/owner cancel: %s %s", st(b), st(d))
	}
	if n := f.count(`SELECT count(*) FROM order_status_history WHERE fulfillment_id = $1 AND actor_type = 'vendor' AND actor_id = $2`, a, vo.ID); n != 1 {
		t.Errorf("vendor history %d", n)
	}
	if n := f.count(`SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND action = 'fulfillment.transition'
		AND entity_type = 'fulfillment' AND before->>'status' IS NOT NULL AND after->>'status' = 'cancelled'`, f.shop); n != 3 {
		t.Errorf("audit %d", n)
	}
}

// TestAdminVendorIsolationIntegration (14.2 test 14 admin ongsho, Rule 13): vendor X er staff vendor Y er order
// (detail, fulfillment transition) 404, list ar q e nai; mixed order e shudhu nijer fulfillment/item/history, payment
// ar customer field nai.
func TestAdminVendorIsolationIntegration(t *testing.T) {
	f := newCheckout(t)
	vx, rx := f.vendorVariant("vx", 50)
	vy, ry := f.vendorVariant("vy", 50)
	oy, fy := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: vy, status: "open", items: [][2]int64{{ry, 1}}})
	om, fm := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: vx, status: "open", items: [][2]int64{{rx, 1}}},
		seedF{vendor: vy, status: "open", items: [][2]int64{{ry, 3}}})
	f.exec(`UPDATE orders SET customer_phone = '+8801811000077' WHERE id = $1`, oy)
	vo, vs := f.staffAs("vendor_owner", vx), f.staffAs("vendor_staff", vx)

	for _, st := range []httpx.Staff{vo, vs} {
		wantErr(t, st.Role+" Y detail", f.adm(st, "GET", "/api/admin/orders/"+s(oy), nil), http.StatusNotFound, "not_found")
		for q, want := range map[string][]string{"": {f.number(om)}, "?status=confirmed": {f.number(om)},
			"?q=" + f.number(oy): nil, "?q=01811000077": nil} {
			if got, _ := f.listNumbers(st, q); !slices.Equal(got, want) {
				t.Errorf("%s list %q: %v, chai %v", st.Role, q, got, want)
			}
		}
	}
	wantErr(t, "Y fulfillment cancel", f.adm(vo, "POST", "/api/admin/fulfillments/"+s(fy[0])+"/transition",
		map[string]string{"to": "cancelled"}), http.StatusNotFound, "not_found")
	wantErr(t, "mixed Y fulfillment", f.adm(vo, "POST", "/api/admin/fulfillments/"+s(fm[1])+"/transition",
		map[string]string{"to": "cancelled"}), http.StatusNotFound, "not_found")
	if s := f.fStatuses(oy) + "|" + f.fStatuses(om); s != "open|open,open" {
		t.Errorf("Y er fulfillment bodleche: %s", s)
	}
	fl := f.admJSON(vs, "GET", "/api/admin/fulfillments", nil, http.StatusOK)
	if items := fl["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != num(fm[0]) {
		t.Errorf("vendor fulfillments: %v", fl)
	}

	// pick list ar print data (11.7 item 4, 8): shudhu nijer fulfillment; Y er order er slip 404
	f.exec(`UPDATE orders SET confirmed_at = now() WHERE id IN ($1, $2)`, oy, om)
	if pl := f.pickList(vs, ""); !slices.Equal(pl.fids(), []int64{fm[0]}) || pv(pl.Variants) != s(rx)+":1" {
		t.Errorf("vendor pick list: %v %s", pl.fids(), pv(pl.Variants))
	}
	if p := f.printData("packing_slip", f.printLink(vs, map[string]any{"kind": "packing_slip", "order_ids": []int64{om, oy}})); len(p.Slips) != 1 ||
		p.Slips[0].VendorID != vx || len(p.Slips[0].Items) != 1 || p.Slips[0].Items[0].Quantity != 1 {
		t.Errorf("vendor slip: %+v", p.Slips)
	}
	wantErr(t, "Y er slip", f.printGet("a.com", "packing_slip", f.printLink(vo, map[string]any{"kind": "packing_slip",
		"order_ids": []int64{oy}})), http.StatusNotFound, "not_found")
	pt := f.printData("pick_list", f.printLink(vo, map[string]any{"kind": "pick_list"}))
	if pt.PickList == nil || !slices.Equal(pt.PickList.fids(), []int64{fm[0]}) {
		t.Errorf("vendor print pick list: %+v", pt.PickList)
	}

	// dashboard (11.7 item 13 viii): nijer fulfillment er order ar bikri, nijer variant er low stock
	f.exec(`UPDATE product_variants SET stock = 2 WHERE id = $1`, rx)
	f.exec(`UPDATE product_variants SET stock = 0 WHERE id = $1`, ry)
	for _, st := range []httpx.Staff{vo, vs} {
		_, got := f.dashboard(st)
		wantCounts(t, st.Role+" dashboard", got, map[string]int64{"orders": 1, "sales": 10000, "pending": 0, "to_pack": 1,
			"low_stock": 1, "expiring": 0, "deadline_soon": 0})
	}
	_, got := f.dashboard(f.staffAs("owner", 0))
	wantCounts(t, "owner dashboard", got, map[string]int64{"orders": 2, "sales": 50000, "pending": 0, "to_pack": 3,
		"low_stock": 2, "expiring": 0, "deadline_soon": 0})

	d := f.admJSON(vs, "GET", "/api/admin/orders/"+s(om), nil, http.StatusOK)
	fs := d["fulfillments"].([]any)
	if len(fs) != 1 || fs[0].(map[string]any)["id"] != num(fm[0]) || len(fs[0].(map[string]any)["items"].([]any)) != 1 ||
		d["total"] != num(10000) {
		t.Errorf("mixed detail: %v", d)
	}
	for _, k := range []string{"customer_phone", "customer_name", "payment_method", "payment_status", "subtotal", "notes", "risk_flags"} {
		if _, ok := d[k]; ok {
			t.Errorf("vendor detail e %s", k)
		}
	}
	for _, h := range d["history"].([]any) {
		if fid := h.(map[string]any)["fulfillment_id"]; fid != nil && fid != num(fm[0]) {
			t.Errorf("onno vendor er history: %v", h)
		}
	}
}

// TestAdminShopIsolationIntegration (14.2 test 5 admin ongsho, Rule 3): shop b er owner shop a er order detail,
// transition, note, fulfillment transition 404, list e nai; kichu bodlay na.
func TestAdminShopIsolationIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 10000, 50)
	oid, fs := f.seedOrder("cod", "unpaid", "pending", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}})
	b := f.id(`SELECT id FROM shops WHERE slug = 'b'`)
	ob := httpx.Staff{ID: f.id(`SELECT id FROM staff WHERE shop_id = $1 AND role = 'owner'`, b), ShopID: b, Role: "owner"}

	wantErr(t, "detail", f.adm(ob, "GET", "/api/admin/orders/"+s(oid), nil), http.StatusNotFound, "not_found")
	wantErr(t, "transition", f.adm(ob, "POST", "/api/admin/orders/"+s(oid)+"/transition", map[string]string{"to": "cancelled"}),
		http.StatusNotFound, "not_found")
	wantErr(t, "note", f.adm(ob, "POST", "/api/admin/orders/"+s(oid)+"/notes", map[string]string{"note": "x"}), http.StatusNotFound, "not_found")
	wantErr(t, "fulfillment", f.adm(ob, "POST", "/api/admin/fulfillments/"+s(fs[0])+"/transition", map[string]string{"to": "cancelled"}),
		http.StatusNotFound, "not_found")
	if got, total := f.listNumbers(ob, ""); got != nil || total != "0" {
		t.Errorf("b er list: %v", got)
	}
	if got, _ := f.listNumbers(ob, "?q="+f.number(oid)); got != nil {
		t.Errorf("b er q: %v", got)
	}
	fl := f.admJSON(ob, "GET", "/api/admin/fulfillments", nil, http.StatusOK)
	if len(fl["items"].([]any)) != 0 {
		t.Errorf("b er fulfillments: %v", fl)
	}
	if s := f.orderStatus(oid) + "|" + f.fStatuses(oid); s != "pending/pending|open" {
		t.Errorf("b er request er por %s", s)
	}
	// zone, coupon, customer (step 11): a er row b er owner er kache 404, list e nai, block muchte pare na
	zone := s(f.id(`SELECT id FROM delivery_zones WHERE shop_id = $1 LIMIT 1`, f.shop))
	cp := s(f.id(`INSERT INTO coupons (shop_id, code, kind, value) VALUES ($1, 'ISO', 'fixed', 100) RETURNING id`, f.shop))
	f.exec(`INSERT INTO blocked_phones (shop_id, phone) VALUES ($1, $2)`, f.shop, userPhone)
	wantErr(t, "zone", f.adm(ob, "PATCH", "/api/admin/delivery-zones/"+zone, map[string]any{"charge": 1}), http.StatusNotFound, "not_found")
	wantErr(t, "coupon", f.adm(ob, "PATCH", "/api/admin/coupons/"+cp, map[string]any{"is_active": false}), http.StatusNotFound, "not_found")
	wantErr(t, "customer", f.adm(ob, "GET", "/api/admin/customers/"+s(f.user), nil), http.StatusNotFound, "not_found")
	for _, p := range []string{"/api/admin/delivery-zones", "/api/admin/coupons", "/api/admin/customers", "/api/admin/blocked-phones"} {
		if l := f.admJSON(ob, "GET", p, nil, http.StatusOK); l["total"].(json.Number).String() != "0" {
			t.Errorf("b er %s: %v", p, l)
		}
	}
	if rec := f.adm(ob, "DELETE", "/api/admin/blocked-phones", map[string]string{"phone": userPhone}); rec.Code != http.StatusNoContent ||
		f.count(`SELECT count(*) FROM blocked_phones`) != 1 {
		t.Errorf("b unblock: %d", rec.Code)
	}
	if n := f.count(`SELECT count(*) FROM delivery_zones WHERE charge = 1`) + f.count(`SELECT count(*) FROM coupons WHERE NOT is_active`); n != 0 {
		t.Errorf("b bodleche %d", n)
	}
	if n := f.count(`SELECT count(*) FROM order_notes`) + f.count(`SELECT count(*) FROM audit_logs`); n != 0 {
		t.Errorf("note/audit %d", n)
	}
}
