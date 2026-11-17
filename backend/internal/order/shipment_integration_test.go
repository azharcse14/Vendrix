package order_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"vendrix/backend/internal/notify"
	"vendrix/backend/internal/order"
	"vendrix/backend/internal/pricing"
)

func shipPath(fid int64) string    { return "/api/admin/fulfillments/" + s(fid) + "/shipments" }
func shipmentPath(id int64) string { return "/api/admin/shipments/" + s(id) }

func returnBody(note string, items ...int64) map[string]any {
	ids := []int64{}
	ids = append(ids, items...)
	return map[string]any{"to": "returned", "note": note, "restock_item_ids": ids}
}

// shipment: status/cod_amount/tracking_code (- = NULL).
func (f *checkoutFx) shipment(id int64) string {
	return f.str(`SELECT status || '/' || cod_amount || '/' || coalesce(tracking_code, '-') FROM shipments WHERE id = $1`, id)
}

// seedShipment: DB te sorasori shipment (seedOrder er shipped/delivered fulfillment er jonno).
func (f *checkoutFx) seedShipment(oid, fid, vendor int64, status string, cod int64) int64 {
	f.t.Helper()
	return f.id(`INSERT INTO shipments (shop_id, order_id, fulfillment_id, vendor_id, carrier, status, cod_amount)
		VALUES ($1, $2, $3, $4, 'own_rider', $5, $6) RETURNING id`, f.shop, oid, fid, vendor, status, cod)
}

func (f *checkoutFx) shippedJobs(oid, sid int64) int64 {
	return f.count(`SELECT count(*) FROM jobs WHERE kind = 'sms.shipped' AND shop_id = $3
		AND payload = jsonb_build_object('order_id', $1::bigint, 'shipment_id', $2::bigint)`, oid, sid, f.shop)
}

// TestShipmentIntegration (8.8 "packed → shipped", 11.7 item 5 v1.43, 7.7): POST shipments ek tx e shipment +
// packed → shipped (shipped_at, history, display_status, audit shipment.create), COD e cod_amount = fulfillment total,
// prepaid e 0; sms.shipped tx e (payload {order_id, shipment_id}), lekha te tracking ar order link. Ek fulfillment e
// ek active shipment (abar POST 409); shipment returned tarpor notun shipment (fulfillment shipped e thake, notun
// history na, notun SMS); duplicate tracking 400 taken; PATCH tracking_code; bondho shipment PATCH 409; open
// fulfillment 409; /transition shipped ar delivered 409; body validation 400; onno vendor 404, nijer vendor 201.
func TestShipmentIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 10000, 50)
	st := f.staffAs("staff", 0)
	oid, fs := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "packed", items: [][2]int64{{rice, 2}}})
	fid := fs[0]

	out := f.admJSON(st, "POST", shipPath(fid), map[string]any{"carrier": "steadfast", "tracking_code": " TR1 ", "carrier_fee": 6000},
		http.StatusCreated)
	sid := f.id(`SELECT id FROM shipments WHERE fulfillment_id = $1`, fid)
	if out["id"] != num(sid) || out["status"] != "booked" || out["cod_amount"] != num(20000) || out["display_status"] != "shipped" {
		t.Errorf("response %v", out)
	}
	for _, c := range []struct {
		name      string
		got, want int64
	}{
		{"shipment", f.count(`SELECT count(*) FROM shipments WHERE id = $1 AND status = 'booked' AND cod_amount = 20000
			AND tracking_code = 'TR1' AND carrier = 'steadfast' AND carrier_fee = 6000 AND vendor_id = $2 AND order_id = $3`, sid, f.vnd, oid), 1},
		{"shipped_at", f.count(`SELECT count(*) FROM order_fulfillments WHERE id = $1 AND status = 'shipped' AND shipped_at IS NOT NULL`, fid), 1},
		{"history", f.count(`SELECT count(*) FROM order_status_history WHERE fulfillment_id = $1 AND from_status = 'packed'
			AND to_status = 'shipped' AND actor_type = 'staff' AND actor_id = $2`, fid, st.ID), 1},
		{"audit", f.count(`SELECT count(*) FROM audit_logs WHERE action = 'shipment.create' AND entity_type = 'shipment'
			AND entity_id = $1 AND staff_id = $2`, sid, st.ID), 1},
		{"sms.shipped", f.shippedJobs(oid, sid), 1},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, chai %d", c.name, c.got, c.want)
		}
	}
	if s := f.orderStatus(oid); s != "confirmed/shipped" {
		t.Errorf("order %s", s)
	}

	// SMS: tracking ar /order/<public_token>
	sms := &smsLog{}
	h := order.JobHandlers(f.d, sms, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), confirmSecret)
	if err := f.runJob(h, "sms.shipped", oid); err != nil {
		t.Fatal(err)
	}
	tok := f.str(`SELECT public_token FROM orders WHERE id = $1`, oid)
	want := "+8801711000009|" + notify.ShippedText("bn", nil, "দোকান", f.number(oid), "https://a.com/order/"+tok, "TR1")
	if got := sms.take(); len(got) != 1 || got[0] != want || !strings.Contains(got[0], "TR1") {
		t.Errorf("SMS %q, chai %q", got, want)
	}

	// ek active shipment
	wantErr(t, "duita active", f.adm(st, "POST", shipPath(fid), map[string]any{"carrier": "own_rider"}), http.StatusConflict, "invalid_transition")
	// tracking bodol
	out = f.admJSON(st, "PATCH", shipmentPath(sid), map[string]any{"tracking_code": "TR1B"}, http.StatusOK)
	if out["status"] != "booked" || f.shipment(sid) != "booked/20000/TR1B" {
		t.Errorf("tracking bodol: %v %s", out, f.shipment(sid))
	}
	// courier parcel ferot: shudhu shipment, fulfillment shipped e
	out = f.admJSON(st, "PATCH", shipmentPath(sid), map[string]any{"status": "returned"}, http.StatusOK)
	if out["status"] != "returned" || f.fStatuses(oid) != "shipped" {
		t.Errorf("shipment returned: %v, fulfillment %s", out, f.fStatuses(oid))
	}
	wantErr(t, "bondho shipment", f.adm(st, "PATCH", shipmentPath(sid), map[string]any{"status": "cancelled"}), http.StatusConflict, "invalid_transition")
	// abar pathano: duplicate tracking 400, notun tracking 201
	e := wantErr(t, "duplicate tracking", f.adm(st, "POST", shipPath(fid), map[string]any{"carrier": "steadfast", "tracking_code": "TR1B"}),
		http.StatusBadRequest, "validation_failed")
	if e.Fields["tracking_code"] != "taken" {
		t.Errorf("duplicate tracking fields %v", e.Fields)
	}
	hist := f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1`, oid)
	f.admJSON(st, "POST", shipPath(fid), map[string]any{"carrier": "steadfast", "tracking_code": "TR2"}, http.StatusCreated)
	sid2 := f.id(`SELECT id FROM shipments WHERE fulfillment_id = $1 AND tracking_code = 'TR2'`, fid)
	if f.fStatuses(oid) != "shipped" || f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1`, oid) != hist ||
		f.shippedJobs(oid, sid2) != 1 || f.shipment(sid2) != "booked/20000/TR2" {
		t.Errorf("abar pathano: %s, shipment %s", f.fStatuses(oid), f.shipment(sid2))
	}

	// prepaid: cod_amount 0
	poid, pf := f.seedOrder("sslcommerz", "paid", "confirmed", seedF{vendor: f.vnd, status: "packed", items: [][2]int64{{rice, 1}}})
	out = f.admJSON(st, "POST", shipPath(pf[0]), map[string]any{"carrier": "pathao"}, http.StatusCreated)
	if out["cod_amount"] != num(0) || f.orderStatus(poid) != "confirmed/shipped" {
		t.Errorf("prepaid %v %s", out, f.orderStatus(poid))
	}

	// open fulfillment, /transition shipped/delivered: 409; kichu likha na
	ooid, of := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}})
	wantErr(t, "open", f.adm(st, "POST", shipPath(of[0]), map[string]any{"carrier": "own_rider"}), http.StatusConflict, "invalid_transition")
	_, kf := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "packed", items: [][2]int64{{rice, 1}}})
	wantErr(t, "/transition shipped", f.adm(st, "POST", transitionPath(kf[0]), map[string]any{"to": "shipped"}), http.StatusConflict, "invalid_transition")
	wantErr(t, "/transition delivered", f.adm(st, "POST", transitionPath(fid), map[string]any{"to": "delivered"}), http.StatusConflict, "invalid_transition")
	if f.count(`SELECT count(*) FROM shipments WHERE order_id = $1`, ooid) != 0 || f.fStatuses(ooid) != "open" {
		t.Error("409 e kichu likha hoyeche")
	}

	// validation
	inactive := f.staffAs("admin", 0)
	f.exec(`UPDATE staff SET is_active = false WHERE id = $1`, inactive.ID)
	otherShop := f.id(`SELECT id FROM staff WHERE shop_id = (SELECT id FROM shops WHERE slug = 'b') AND role = 'owner'`)
	for _, c := range []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"carrier", map[string]any{"carrier": "dhl"}, "carrier"},
		{"lomba tracking", map[string]any{"carrier": "redx", "tracking_code": strings.Repeat("A", 41)}, "tracking_code"},
		{"non-ascii tracking", map[string]any{"carrier": "redx", "tracking_code": "ট্র্যাক"}, "tracking_code"},
		{"fee negative", map[string]any{"carrier": "redx", "carrier_fee": -1}, "carrier_fee"},
		{"inactive rider", map[string]any{"carrier": "own_rider", "rider_staff_id": inactive.ID}, "rider_staff_id"},
		{"onno shop er rider", map[string]any{"carrier": "own_rider", "rider_staff_id": otherShop}, "rider_staff_id"},
	} {
		e := wantErr(t, c.name, f.adm(st, "POST", shipPath(kf[0]), c.body), http.StatusBadRequest, "validation_failed")
		if e.Fields[c.field] == "" {
			t.Errorf("%s: fields %v", c.name, e.Fields)
		}
	}
	for _, c := range []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"status", map[string]any{"status": "picked"}, "status"},
		{"khali", map[string]any{}, "body"},
	} {
		e := wantErr(t, c.name, f.adm(st, "PATCH", shipmentPath(sid2), c.body), http.StatusBadRequest, "validation_failed")
		if e.Fields[c.field] == "" {
			t.Errorf("%s: fields %v", c.name, e.Fields)
		}
	}
	// rider: active staff
	f.admJSON(st, "POST", shipPath(kf[0]), map[string]any{"carrier": "own_rider", "rider_staff_id": st.ID}, http.StatusCreated)

	// vendor: onno vendor 404, nijer 201; onno shop er shipment 404
	vb, vr := f.vendorVariant("vb", 5)
	_, vf := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: vb, status: "packed", items: [][2]int64{{vr, 1}}})
	vs := f.staffAs("vendor_staff", vb)
	wantErr(t, "onno vendor", f.adm(vs, "POST", shipPath(pf[0]), map[string]any{"carrier": "own_rider"}), http.StatusNotFound, "not_found")
	wantErr(t, "onno vendor shipment", f.adm(vs, "PATCH", shipmentPath(sid2), map[string]any{"tracking_code": "X"}), http.StatusNotFound, "not_found")
	f.admJSON(vs, "POST", shipPath(vf[0]), map[string]any{"carrier": "own_rider"}, http.StatusCreated)
	bst := st
	bst.ShopID = f.id(`SELECT id FROM shops WHERE slug = 'b'`)
	wantErr(t, "onno shop", f.adm(bst, "PATCH", shipmentPath(sid2), map[string]any{"tracking_code": "X"}), http.StatusNotFound, "not_found")
}

// TestDeliverCODPaidIntegration (8.8 "shipped → delivered" v1.43, 11.7 item 5): PATCH shipments delivered →
// fulfillment delivered (delivered_at, history), shipment delivered + delivered_at; marketplace COD e ekta delivered
// hole payment_status unpaid (onno shipped), shob hole paid. Ager fulfillment returned (delivery chhara) ar baki
// delivered hole paid. Vendor role er delivered 409 (state machine), kichu bodlay na.
func TestDeliverCODPaidIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 10000, 50)
	vb, vr := f.vendorVariant("vb", 5)
	st := f.staffAs("staff", 0)
	oid, fs := f.seedOrder("cod", "unpaid", "confirmed",
		seedF{vendor: f.vnd, status: "shipped", items: [][2]int64{{rice, 1}}}, seedF{vendor: vb, status: "shipped", items: [][2]int64{{vr, 2}}})
	s1, s2 := f.seedShipment(oid, fs[0], f.vnd, "in_transit", 10000), f.seedShipment(oid, fs[1], vb, "booked", 20000)

	wantErr(t, "vendor delivered", f.adm(f.staffAs("vendor_owner", vb), "PATCH", shipmentPath(s2), map[string]any{"status": "delivered"}),
		http.StatusConflict, "invalid_transition")
	out := f.admJSON(st, "PATCH", shipmentPath(s1), map[string]any{"status": "delivered"}, http.StatusOK)
	if out["status"] != "delivered" || out["display_status"] != "partial" {
		t.Errorf("response %v", out)
	}
	pay := func(o int64) string { return f.str(`SELECT payment_status FROM orders WHERE id = $1`, o) }
	if pay(oid) != "unpaid" || f.fStatuses(oid) != "delivered,shipped" {
		t.Errorf("ekta delivered: %s %s", pay(oid), f.fStatuses(oid))
	}
	for _, c := range []struct {
		name      string
		got, want int64
	}{
		{"shipment delivered_at", f.count(`SELECT count(*) FROM shipments WHERE id = $1 AND status = 'delivered' AND delivered_at IS NOT NULL`, s1), 1},
		{"fulfillment delivered_at", f.count(`SELECT count(*) FROM order_fulfillments WHERE id = $1 AND delivered_at IS NOT NULL`, fs[0]), 1},
		{"history", f.count(`SELECT count(*) FROM order_status_history WHERE fulfillment_id = $1 AND from_status = 'shipped'
			AND to_status = 'delivered' AND actor_id = $2`, fs[0], st.ID), 1},
		{"audit", f.count(`SELECT count(*) FROM audit_logs WHERE action = 'shipment.update' AND entity_id = $1
			AND before->>'status' = 'in_transit' AND after->>'status' = 'delivered'`, s1), 1},
		{"vendor 409 e s2 ekoi", f.count(`SELECT count(*) FROM shipments WHERE id = $1 AND status = 'booked'`, s2), 1},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, chai %d", c.name, c.got, c.want)
		}
	}
	wantErr(t, "abar delivered", f.adm(st, "PATCH", shipmentPath(s1), map[string]any{"status": "delivered"}), http.StatusConflict, "invalid_transition")
	f.admJSON(st, "PATCH", shipmentPath(s2), map[string]any{"status": "delivered"}, http.StatusOK)
	if pay(oid) != "paid" || f.orderStatus(oid) != "confirmed/delivered" {
		t.Errorf("shob delivered: %s %s", pay(oid), f.orderStatus(oid))
	}

	// ek fulfillment delivery chhara returned, baki delivered → paid
	roid, rf := f.seedOrder("cod", "unpaid", "confirmed",
		seedF{vendor: f.vnd, status: "shipped", items: [][2]int64{{rice, 1}}}, seedF{vendor: vb, status: "shipped", items: [][2]int64{{vr, 1}}})
	f.seedShipment(roid, rf[0], f.vnd, "returned", 10000)
	rs := f.seedShipment(roid, rf[1], vb, "booked", 10000)
	f.admJSON(st, "POST", transitionPath(rf[0]), returnBody("customer nei"), http.StatusOK)
	if pay(roid) != "unpaid" {
		t.Errorf("returned + shipped: %s", pay(roid))
	}
	f.admJSON(st, "PATCH", shipmentPath(rs), map[string]any{"status": "delivered"}, http.StatusOK)
	if pay(roid) != "paid" || f.refunds(roid) != "" {
		t.Errorf("returned + delivered: %s, refund %q", pay(roid), f.refunds(roid))
	}

	// shob returned (delivery chhara): paid na
	aoid, af := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "shipped", items: [][2]int64{{rice, 1}}})
	f.seedShipment(aoid, af[0], f.vnd, "returned", 10000)
	f.admJSON(st, "POST", transitionPath(af[0]), returnBody("ferot"), http.StatusOK)
	if pay(aoid) != "unpaid" || f.orderStatus(aoid) != "confirmed/returned" {
		t.Errorf("shob returned: %s %s", pay(aoid), f.orderStatus(aoid))
	}
}

// TestReturnIntegration (8.8 "→ returned" v1.43, 11.7 item 6, 14.2 test 35): restock_item_ids er item stock ar pack
// er batch e ferot (movement return, batch_id shoho), baki item stock e na; returned_at, history note, audit
// {status, restock_item_ids}. COD delivered (taka newa) e refund kind return, method cash, fulfillment_id, pending,
// poriman fulfillment total: owner/admin pare, staff 403 (kichu likha na). COD shipped → returned (taka na) e refund
// na, staff 200. Prepaid shipped → returned e refund (payment er method). Guard: khali note 400, bhul/duplicate/onno
// fulfillment er item 400, returned chhara restock_item_ids 400, active shipment 409, vendor 409.
func TestReturnIntegration(t *testing.T) {
	f := newCheckout(t)
	f.batchExpiry(true)
	rice, dal := f.variant("rice", 10000, 10), f.variant("dal", 10000, 10)
	b1 := f.batchAt(rice, 3, nil, 0)
	oid, fs := f.seedOrder("cod", "paid", "confirmed", seedF{vendor: f.vnd, status: "delivered", items: [][2]int64{{rice, 2}, {dal, 1}}})
	fid := fs[0]
	f.seedShipment(oid, fid, f.vnd, "delivered", 30000)
	it := func(o, v int64) int64 {
		return f.id(`SELECT id FROM order_items WHERE order_id = $1 AND variant_id = $2`, o, v)
	}
	ri, di := it(oid, rice), it(oid, dal)
	f.exec(`INSERT INTO order_item_batches (shop_id, order_item_id, batch_id, variant_id, quantity) VALUES ($1, $2, $3, $4, 2)`,
		f.shop, ri, b1, rice)
	snap := func() string { return f.orderSnap(oid) + "|" + s(f.rem(b1)) }

	staffSt, owner := f.staffAs("staff", 0), f.staffAs("owner", 0)
	before := snap()
	wantErr(t, "staff paid", f.adm(staffSt, "POST", transitionPath(fid), returnBody("nosto", ri)), http.StatusForbidden, "forbidden")
	for _, c := range []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"khali note", returnBody("  ", ri), "note"},
		{"duplicate", returnBody("x", ri, ri), "restock_item_ids"},
		{"ochena item", returnBody("x", 999999), "restock_item_ids"},
		{"returned chhara", map[string]any{"to": "cancelled", "note": "x", "restock_item_ids": []int64{ri}}, "restock_item_ids"},
	} {
		e := wantErr(t, c.name, f.adm(owner, "POST", transitionPath(fid), c.body), http.StatusBadRequest, "validation_failed")
		if e.Fields[c.field] == "" {
			t.Errorf("%s: fields %v", c.name, e.Fields)
		}
	}
	wantErr(t, "vendor", f.adm(f.staffAs("vendor_owner", f.vnd), "POST", transitionPath(fid), returnBody("x", ri)), http.StatusConflict, "invalid_transition")
	if got := snap(); got != before {
		t.Fatalf("403/400/409 e likha: %s, age %s", got, before)
	}

	out := f.admJSON(owner, "POST", transitionPath(fid), returnBody(" pocha ", ri), http.StatusOK)
	if out["status"] != "returned" {
		t.Errorf("response %v", out)
	}
	if got := f.refunds(oid); got != "30000/return/pending/"+s(fid) {
		t.Errorf("refund %q", got)
	}
	for _, c := range []struct {
		name      string
		got, want int64
	}{
		{"refund cash, payment nai", f.count(`SELECT count(*) FROM refunds WHERE order_id = $1 AND method = 'cash' AND payment_id IS NULL
			AND reason = 'pocha'`, oid), 1},
		{"rice stock +2", f.count(`SELECT stock FROM product_variants WHERE id = $1`, rice), 12},
		{"dal stock ekoi", f.count(`SELECT stock FROM product_variants WHERE id = $1`, dal), 10},
		{"batch +2", f.rem(b1), 5},
		{"movement return", f.count(`SELECT count(*) FROM inventory_movements WHERE order_id = $1 AND reason = 'return' AND change = 2
			AND batch_id = $2 AND variant_id = $3`, oid, b1, rice), 1},
		{"movement shudhu ekta", f.count(`SELECT count(*) FROM inventory_movements WHERE order_id = $1`, oid), 1},
		{"returned_at", f.count(`SELECT count(*) FROM order_fulfillments WHERE id = $1 AND status = 'returned' AND returned_at IS NOT NULL`, fid), 1},
		{"history", f.count(`SELECT count(*) FROM order_status_history WHERE fulfillment_id = $1 AND from_status = 'delivered'
			AND to_status = 'returned' AND note = 'pocha' AND actor_id = $2`, fid, owner.ID), 1},
		{"audit", f.count(`SELECT count(*) FROM audit_logs WHERE action = 'fulfillment.transition' AND entity_id = $1
			AND after = jsonb_build_object('status', 'returned', 'restock_item_ids', jsonb_build_array($2::bigint))`, fid, ri), 1},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, chai %d", c.name, c.got, c.want)
		}
	}
	if s := f.orderStatus(oid); s != "confirmed/returned" {
		t.Errorf("order %s", s)
	}
	_ = di

	// COD shipped → returned (taka newa hoyni): refund na, staff pare, khali list e stock na
	soid, sf := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "shipped", items: [][2]int64{{dal, 1}}})
	ship := f.seedShipment(soid, sf[0], f.vnd, "booked", 10000)
	wantErr(t, "active shipment", f.adm(staffSt, "POST", transitionPath(sf[0]), returnBody("x")), http.StatusConflict, "invalid_transition")
	f.exec(`UPDATE shipments SET status = 'returned' WHERE id = $1`, ship)
	f.admJSON(staffSt, "POST", transitionPath(sf[0]), returnBody("customer nei"), http.StatusOK)
	if f.refunds(soid) != "" || f.fStatuses(soid) != "returned" || f.count(`SELECT stock FROM product_variants WHERE id = $1`, dal) != 10 {
		t.Errorf("COD shipped return: refund %q, %s", f.refunds(soid), f.fStatuses(soid))
	}

	// prepaid shipped → returned: refund payment er method e (admin)
	poid, pf := f.seedOrder("sslcommerz", "paid", "confirmed", seedF{vendor: f.vnd, status: "shipped", items: [][2]int64{{dal, 2}}})
	f.seedShipment(poid, pf[0], f.vnd, "returned", 0)
	wantErr(t, "staff prepaid", f.adm(staffSt, "POST", transitionPath(pf[0]), returnBody("x")), http.StatusForbidden, "forbidden")
	f.admJSON(f.staffAs("admin", 0), "POST", transitionPath(pf[0]), returnBody("ferot"), http.StatusOK)
	if got := f.refunds(poid); got != "20000/return/pending/"+s(pf[0]) ||
		f.count(`SELECT count(*) FROM refunds WHERE order_id = $1 AND method = 'sslcommerz' AND payment_id IS NOT NULL`, poid) != 1 {
		t.Errorf("prepaid refund %q", got)
	}
}

// TestReturnRefundCapIntegration (11.7 item 6, Q6): prepaid e pack er reprice refund ar manual refund er por return refund
// = min(fulfillment total, paid − ager refund (failed bad)); mot refund paid er beshi na.
func TestReturnRefundCapIntegration(t *testing.T) {
	f := newCheckout(t)
	fish := f.weighted("fish", 80000, 1000, 20)
	oid, fs, it := f.seedPriced(true, nil, 6000, pl{f.vnd, fish, 80000, 1, 1000})
	fid := fs[0]
	owner := f.staffAs("owner", 0)
	f.admJSON(owner, "POST", packPath(fid), packBody("", it[0], 900), http.StatusOK)
	if got := f.refunds(oid); got != "8000/reprice/pending/"+s(fid) {
		t.Fatalf("pack refund %q", got)
	}
	f.exec(`INSERT INTO refunds (shop_id, order_id, amount, method, kind, reason) VALUES ($1, $2, 5000, 'sslcommerz', 'manual', 'x')`, f.shop, oid)
	f.exec(`INSERT INTO refunds (shop_id, order_id, amount, method, kind, reason, status) VALUES ($1, $2, 99999, 'sslcommerz', 'manual', 'x', 'failed')`,
		f.shop, oid)
	f.admJSON(owner, "POST", shipPath(fid), map[string]any{"carrier": "pathao"}, http.StatusCreated)
	sid := f.id(`SELECT id FROM shipments WHERE fulfillment_id = $1`, fid)
	f.admJSON(owner, "PATCH", shipmentPath(sid), map[string]any{"status": "delivered"}, http.StatusOK)
	f.admJSON(owner, "POST", transitionPath(fid), returnBody("bhalo na", it[0]), http.StatusOK)
	// paid 86000 (80000 + 6000), reprice 8000, manual 5000 → baki 73000; fulfillment total 78000
	if got := f.refunds(oid); got != "8000/reprice/pending/"+s(fid)+",5000/manual/pending/-,99999/manual/failed/-,73000/return/pending/"+s(fid) {
		t.Errorf("refund %q", got)
	}
	if paid, ref := f.count(`SELECT sum(amount) FROM payments WHERE order_id = $1`, oid),
		f.count(`SELECT sum(amount) FROM refunds WHERE order_id = $1 AND status <> 'failed'`, oid); ref != paid {
		t.Errorf("mot refund %d, paid %d", ref, paid)
	}
}

// TestEditThenReturnRefundIntegration (11.7 item 6, 7; Q6): prepaid e edit er reprice refund, tarpor pack, ship, deliver,
// return: return refund = notun fulfillment total, edit refund + return refund = paid (beshi na).
func TestEditThenReturnRefundIntegration(t *testing.T) {
	f := newCheckout(t)
	rice, dal := f.variant("rice", 10000, 20), f.variant("dal", 15000, 20)
	pc := &pricing.Coupon{Kind: "percent", Value: 10}
	oid, fs, it := f.seedPriced(true, pc, 6000, pl{f.vnd, rice, 10000, 3, 0}, pl{f.vnd, dal, 15000, 1, 0})
	fid := fs[0]
	owner := f.staffAs("owner", 0)
	f.admJSON(owner, "PATCH", editPath(oid), editBody("customer 1 ta chay", it[0], 1), http.StatusOK)
	edit := f.count(`SELECT sum(amount) FROM refunds WHERE order_id = $1 AND kind = 'reprice'`, oid)
	if edit <= 0 {
		t.Fatalf("edit refund %d", edit)
	}
	f.admJSON(owner, "POST", packPath(fid), noItems(), http.StatusOK)
	f.admJSON(owner, "POST", shipPath(fid), map[string]any{"carrier": "redx", "tracking_code": "R-1"}, http.StatusCreated)
	sid := f.id(`SELECT id FROM shipments WHERE fulfillment_id = $1`, fid)
	f.admJSON(owner, "PATCH", shipmentPath(sid), map[string]any{"status": "delivered"}, http.StatusOK)
	f.admJSON(owner, "POST", transitionPath(fid), returnBody("ferot", it[0], it[1]), http.StatusOK)
	total := f.count(`SELECT total FROM order_fulfillments WHERE id = $1`, fid)
	ret := f.count(`SELECT coalesce(sum(amount), 0) FROM refunds WHERE order_id = $1 AND kind = 'return'`, oid)
	paid := f.count(`SELECT sum(amount) FROM payments WHERE order_id = $1`, oid)
	if ret != total || edit+ret != paid {
		t.Errorf("return refund %d (total %d), edit %d, paid %d", ret, total, edit, paid)
	}
}
