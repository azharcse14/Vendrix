package order_test

import (
	"math/rand"
	"net/http"
	"strings"
	"testing"

	"vendrix/backend/internal/pricing"
)

func editPath(oid int64) string { return "/api/admin/orders/" + s(oid) + "/items" }

// editBody: {items: [{item_id, quantity}...], note}; iq = item, qty, item, qty ...
func editBody(note string, iq ...int64) map[string]any {
	items := []any{}
	for i := 0; i+1 < len(iq); i += 2 {
		items = append(items, map[string]any{"item_id": iq[i], "quantity": iq[i+1]})
	}
	return map[string]any{"items": items, "note": note}
}

// itemRow: quantity/status/ordered_weight_g/line_total/discount_share (- = NULL).
func (f *checkoutFx) itemRow(id int64) string {
	return f.str(`SELECT quantity || '/' || status || '/' || coalesce(ordered_weight_g::text, '-') || '/' || line_total || '/' || discount_share
		FROM order_items WHERE id = $1`, id)
}

// refunds: amount/kind/status/fulfillment_id (- = NULL), id kram e, koma diye.
func (f *checkoutFx) refunds(oid int64) string {
	return f.str(`SELECT string_agg(amount || '/' || kind || '/' || status || '/' || coalesce(fulfillment_id::text, '-'), ',' ORDER BY id)
		FROM refunds WHERE order_id = $1`, oid)
}

// orderSnap: order er shob taka, item ar status (kichu likha hoyni dekhte).
func (f *checkoutFx) orderSnap(oid int64) string {
	return f.oMoney(oid) + "|" + f.orderStatus(oid) + "|" + f.fStatuses(oid) + "|" + f.str(`SELECT string_agg(quantity || '/' || status
		|| '/' || line_total || '/' || discount_share || '/' || coalesce(ordered_weight_g::text, '-'), ',' ORDER BY id) FROM order_items
		WHERE order_id = $1`, oid) + "|" + f.refunds(oid) + "|" + s(f.count(`SELECT count(*) FROM inventory_movements WHERE order_id = $1`, oid)) +
		"|" + s(f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1`, oid))
}

// TestOrderEditIntegration (8.11 item 4, 8.5 Reprice Edit=true, 11.7 item 7, 7.7): COD order e qty kom (ojon-variable e
// EditWeightG) ar item bad (status removed, quantity ager ta, taka 0); Reprice(Edit=true) er notun item/fulfillment/order
// taka; stock ferot, movement cancel note edit (staff_id shoho); history te note; audit order.edit; SMS na; refund na.
// Prepaid e ekta refund row (reprice, fulfillment_id NULL, pending). Guard: packed fulfillment 409 invalid_transition,
// cancelled order 409, qty barano/ekoi, removed/onno order er item, duplicate, khali list, negative 400 fields.items, khali
// note 400 fields.note, nai emon order 404, online pending unpaid 409 payment_required; 409/400 e kichu likha na. Pending COD order eo edit hoy.
func TestOrderEditIntegration(t *testing.T) {
	f := newCheckout(t)
	fish, rice := f.weighted("fish", 80000, 1000, 20), f.variant("rice", 10000, 20)
	pc := &pricing.Coupon{Kind: "percent", Value: 10}
	oid, fids, it := f.seedPriced(false, pc, 6000, pl{f.vnd, fish, 80000, 2, 2000}, pl{f.vnd, rice, 10000, 3, 0})
	st := f.staffAs("staff", 0)

	out := f.admJSON(st, "PATCH", editPath(oid), editBody(" customer 1 ta chay ", it[0], 1, it[1], 0), http.StatusOK)
	if out["status"] != "confirmed" || out["display_status"] != "confirmed" {
		t.Errorf("response %v", out)
	}
	type row struct{ name, got, want string }
	for _, r := range []row{
		{"fish", f.itemRow(it[0]), "1/active/1000/80000/8000"},
		{"rice", f.itemRow(it[1]), "3/removed/-/0/0"},
		{"fulfillment", f.fMoney(fids[0]), "80000/8000/6000/78000"},
		{"order", f.oMoney(oid), "80000/8000/6000/78000"},
		{"status", f.orderStatus(oid) + "|" + f.fStatuses(oid), "confirmed/confirmed|open"},
		{"stock", s(f.count(`SELECT stock FROM product_variants WHERE id = $1`, fish)) + "/" +
			s(f.count(`SELECT stock FROM product_variants WHERE id = $1`, rice)), "21/23"},
		{"movement", f.str(`SELECT string_agg(variant_id || '/' || change || '/' || reason || '/' || note || '/' || (staff_id = $2) ||
			'/' || (batch_id IS NULL), ',' ORDER BY variant_id) FROM inventory_movements WHERE order_id = $1`, oid, st.ID),
			s(fish) + "/1/cancel/edit/true/true," + s(rice) + "/3/cancel/edit/true/true"},
		{"history", f.str(`SELECT string_agg(coalesce(from_status, '-') || '/' || to_status || '/' || actor_type || '/' || (actor_id = $2)
			|| '/' || (fulfillment_id IS NULL) || '/' || note, ',') FROM order_status_history WHERE order_id = $1`, oid, st.ID),
			"confirmed/confirmed/staff/true/true/customer 1 ta chay"},
		{"audit", s(f.count(`SELECT count(*) FROM audit_logs WHERE action = 'order.edit' AND entity_type = 'order' AND entity_id = $1
			AND staff_id = $2 AND before = jsonb_build_object('items', jsonb_build_array(jsonb_build_object('item_id', $3::bigint, 'quantity', 2),
			jsonb_build_object('item_id', $4::bigint, 'quantity', 3)), 'total', 177000) AND after = jsonb_build_object('items',
			jsonb_build_array(jsonb_build_object('item_id', $3::bigint, 'quantity', 1), jsonb_build_object('item_id', $4::bigint, 'quantity', 0)),
			'total', 78000)`, oid, st.ID, it[0], it[1])), "1"},
		{"refund", f.refunds(oid), ""},
		{"sms", s(f.jobCount("sms.total_changed", oid)), "0"},
	} {
		if r.got != r.want {
			t.Errorf("%s: %s, chai %s", r.name, r.got, r.want)
		}
	}

	// prepaid: refund ekta, fulfillment_id NULL
	owner := f.staffAs("owner", 0)
	poid, pf, pit := f.seedPriced(true, pc, 6000, pl{f.vnd, fish, 80000, 2, 2000}, pl{f.vnd, rice, 10000, 3, 0})
	f.admJSON(owner, "PATCH", editPath(poid), editBody("ekta kom", pit[1], 1), http.StatusOK)
	if got := f.itemRow(pit[1]) + "|" + f.fMoney(pf[0]) + "|" + f.refunds(poid); got != "1/active/-/10000/1000|170000/17000/6000/159000|18000/reprice/pending/-" {
		t.Errorf("prepaid: %s", got)
	}

	// guard: kichu likha na
	_, _, other := f.seedPriced(false, nil, 0, pl{f.vnd, rice, 10000, 1, 0})
	goid, _, git := f.seedPriced(false, nil, 6000, pl{f.vnd, fish, 80000, 2, 2000}, pl{f.vnd, rice, 10000, 3, 0}, pl{f.vnd, rice, 10000, 1, 0})
	f.admJSON(owner, "PATCH", editPath(goid), editBody("age bad", git[2], 0), http.StatusOK)
	snap := f.orderSnap(goid)
	for _, c := range []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"qty barano", editBody("x", git[1], 4), "items"},
		{"qty ekoi", editBody("x", git[1], 3), "items"},
		{"negative", editBody("x", git[1], -1), "items"},
		{"removed item", editBody("x", git[2], 0), "items"},
		{"onno order er item", editBody("x", other[0], 0), "items"},
		{"duplicate", editBody("x", git[1], 1, git[1], 2), "items"},
		{"khali list", editBody("x"), "items"},
		{"khali note", editBody("   ", git[1], 1), "note"},
		{"note 501", editBody(strings.Repeat("ক", 501), git[1], 1), "note"},
	} {
		e := wantErr(t, c.name, f.adm(owner, "PATCH", editPath(goid), c.body), http.StatusBadRequest, "validation_failed")
		if e.Fields[c.field] == "" {
			t.Errorf("%s: fields %v", c.name, e.Fields)
		}
	}
	wantErr(t, "nai", f.adm(owner, "PATCH", editPath(999999999), editBody("x", git[1], 1)), http.StatusNotFound, "not_found")
	if got := f.orderSnap(goid); got != snap {
		t.Errorf("400 er por bodleche: %s → %s", snap, got)
	}

	koid, kf := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "packed", items: [][2]int64{{rice, 2}}})
	kit := f.id(`SELECT id FROM order_items WHERE order_id = $1`, koid)
	ksnap := f.orderSnap(koid)
	wantErr(t, "packed", f.adm(owner, "PATCH", editPath(koid), editBody("x", kit, 1)), http.StatusConflict, "invalid_transition")
	if got := f.orderSnap(koid); got != ksnap || f.fStatuses(koid) != "packed" {
		t.Errorf("packed: %s → %s (%v)", ksnap, got, kf)
	}
	coid, _ := f.seedOrder("cod", "unpaid", "cancelled", seedF{vendor: f.vnd, status: "cancelled", items: [][2]int64{{rice, 2}}})
	wantErr(t, "cancelled order", f.adm(owner, "PATCH", editPath(coid),
		editBody("x", f.id(`SELECT id FROM order_items WHERE order_id = $1`, coid), 1)), http.StatusConflict, "invalid_transition")

	// online pending unpaid: customer hoyto gateway e ager taka dicche (confirm er moto payment_required)
	uoid, _ := f.seedOrder("sslcommerz", "unpaid", "pending", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 2}}})
	usnap := f.orderSnap(uoid)
	wantErr(t, "online unpaid", f.adm(owner, "PATCH", editPath(uoid),
		editBody("x", f.id(`SELECT id FROM order_items WHERE order_id = $1`, uoid), 1)), http.StatusConflict, "payment_required")
	if got := f.orderSnap(uoid); got != usnap {
		t.Errorf("online unpaid: %s → %s", usnap, got)
	}

	// pending COD
	noid, _ := f.seedOrder("cod", "unpaid", "pending", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 2}}})
	nit := f.id(`SELECT id FROM order_items WHERE order_id = $1`, noid)
	f.admJSON(st, "PATCH", editPath(noid), editBody("pending", nit, 1), http.StatusOK)
	if got := f.itemRow(nit) + "|" + f.oMoney(noid) + "|" + f.orderStatus(noid); got != "1/active/-/10000/0|10000/0/0/10000|pending/pending" {
		t.Errorf("pending: %s", got)
	}
}

// TestEditVendorAllRemovedIntegration (8.11 item 4 v1.5, 14.2 test 34): prepaid marketplace e B er shob item bad (B ≤ A):
// refund ekbar-i (kind reprice, fulfillment_id NULL), B fulfillment cancelled, cancel er 0 taka refund row na, mot refund
// = paid − notun total ≤ paid; A er taka hubohu. Single store e shob bad → fulfillment ar order cancelled, mot refund =
// paid (ekta row). COD e shob bad → order cancelled, refund na, stock ferot.
func TestEditVendorAllRemovedIntegration(t *testing.T) {
	f := newCheckout(t)
	va, ra := f.vendorVariant("va", 50)
	vb, rb := f.vendorVariant("vb", 50)
	owner := f.staffAs("owner", 0)
	paid := func(oid int64) int64 {
		return f.count(`SELECT sum(amount) FROM payments WHERE order_id = $1 AND status = 'succeeded'`, oid)
	}
	refunded := func(oid int64) int64 {
		return f.count(`SELECT coalesce(sum(amount), 0) FROM refunds WHERE order_id = $1`, oid)
	}

	oid, fids, it := f.seedPriced(true, nil, 9000, pl{va, ra, 50000, 1, 0}, pl{vb, rb, 20000, 2, 0})
	a0 := f.fMoney(fids[0]) + "|" + f.itemMoney(it[0])
	f.admJSON(owner, "PATCH", editPath(oid), editBody("B nai", it[1], 0), http.StatusOK)
	total := f.count(`SELECT total FROM orders WHERE id = $1`, oid)
	for _, c := range []struct{ name, got, want string }{
		{"fulfillment", f.fStatuses(oid), "open,cancelled"},
		{"order", f.str(`SELECT status FROM orders WHERE id = $1`, oid), "confirmed"},
		{"A freeze", f.fMoney(fids[0]) + "|" + f.itemMoney(it[0]), a0},
		{"B taka 0", f.fMoney(fids[1]), "0/0/0/0"},
		{"refund ekta", s(f.count(`SELECT count(*) FROM refunds WHERE order_id = $1`, oid)) + "/" +
			f.str(`SELECT kind || '/' || coalesce(fulfillment_id::text, '-') FROM refunds WHERE order_id = $1`, oid), "1/reprice/-"},
		{"mot refund", s(refunded(oid)), s(paid(oid) - total)},
		{"B stock", s(f.count(`SELECT stock FROM product_variants WHERE id = $1`, rb)), "52"},
		{"B history", s(f.count(`SELECT count(*) FROM order_status_history WHERE fulfillment_id = $1 AND to_status = 'cancelled'`, fids[1])), "1"},
	} {
		if c.got != c.want {
			t.Errorf("marketplace %s: %s, chai %s", c.name, c.got, c.want)
		}
	}

	// single store, shob bad
	soid, sf, sit := f.seedPriced(true, &pricing.Coupon{Kind: "fixed", Value: 5000}, 6000, pl{f.vnd, ra, 30000, 1, 0}, pl{f.vnd, ra, 10000, 2, 0})
	f.admJSON(owner, "PATCH", editPath(soid), editBody("shob bad", sit[0], 0, sit[1], 0), http.StatusOK)
	if got := f.orderStatus(soid) + "|" + f.fStatuses(soid) + "|" + f.fMoney(sf[0]) + "|" + f.oMoney(soid); got != "cancelled/cancelled|cancelled|0/0/0/0|0/0/0/0" {
		t.Errorf("single store: %s", got)
	}
	if got := f.refunds(soid); got != s(paid(soid))+"/reprice/pending/-" {
		t.Errorf("single store refund %s, paid %d", got, paid(soid))
	}

	// COD, shob bad
	coid, _, cit := f.seedPriced(false, nil, 6000, pl{f.vnd, ra, 30000, 2, 0})
	before := f.count(`SELECT stock FROM product_variants WHERE id = $1`, ra)
	f.admJSON(owner, "PATCH", editPath(coid), editBody("customer chay na", cit[0], 0), http.StatusOK)
	if got := f.orderStatus(coid) + "|" + f.refunds(coid) + "|" + s(f.count(`SELECT stock FROM product_variants WHERE id = $1`, ra)-before); got != "cancelled/cancelled||2" {
		t.Errorf("COD: %s", got)
	}
}

// TestEditRoleIntegration (11.2, 11.7 item 7, 8.11 item 4 v1.5, 14.2 test 35): staff role e COD qty kom 200; kono vendor
// er shob item bad ba prepaid e refund toiri kora edit 403 forbidden, kichu likha na; owner/admin pare; vendor_owner/
// vendor_staff route e 403.
func TestEditRoleIntegration(t *testing.T) {
	f := newCheckout(t)
	va, ra := f.vendorVariant("va", 50)
	vb, rb := f.vendorVariant("vb", 50)
	staff := f.staffAs("staff", 0)

	oid, _, it := f.seedPriced(false, nil, 6000, pl{va, ra, 10000, 3, 0}, pl{vb, rb, 10000, 1, 0})
	f.admJSON(staff, "PATCH", editPath(oid), editBody("kom", it[0], 2), http.StatusOK)

	snap := f.orderSnap(oid)
	wantErr(t, "staff vendor shob bad", f.adm(staff, "PATCH", editPath(oid), editBody("x", it[1], 0)), http.StatusForbidden, "forbidden")
	poid, _, pit := f.seedPriced(true, nil, 6000, pl{va, ra, 10000, 3, 0})
	psnap := f.orderSnap(poid)
	wantErr(t, "staff prepaid refund", f.adm(staff, "PATCH", editPath(poid), editBody("x", pit[0], 2)), http.StatusForbidden, "forbidden")
	if a, b := f.orderSnap(oid), f.orderSnap(poid); a != snap || b != psnap {
		t.Errorf("403 er por bodleche: %s → %s, %s → %s", snap, a, psnap, b)
	}
	for _, v := range []struct {
		role string
		vnd  int64
	}{{"vendor_owner", va}, {"vendor_staff", va}} {
		wantErr(t, v.role, f.adm(f.staffAs(v.role, v.vnd), "PATCH", editPath(oid), editBody("x", it[0], 1)), http.StatusForbidden, "forbidden")
	}
	f.admJSON(f.staffAs("admin", 0), "PATCH", editPath(oid), editBody("admin", it[1], 0), http.StatusOK)
	f.admJSON(f.staffAs("owner", 0), "PATCH", editPath(poid), editBody("owner", pit[0], 2), http.StatusOK)
	if f.fStatuses(oid) != "open,cancelled" || f.count(`SELECT count(*) FROM refunds WHERE order_id = $1`, poid) != 1 {
		t.Errorf("owner/admin: %s, refund %d", f.fStatuses(oid), f.count(`SELECT count(*) FROM refunds WHERE order_id = $1`, poid))
	}
}

// TestEditThenPackIntegration (8.5 v1.5, 14.2 test 36): edit e bad deya item packing e abar bill e ashe na (taka 0,
// removed), qty komano ojon-variable line er notun ordered ojon diye pack (+10% shima o notun ojon e), batch shudhu
// active item er; order = fulfillment = active line er jog.
func TestEditThenPackIntegration(t *testing.T) {
	f := newCheckout(t)
	f.batchExpiry(true)
	fish, rice := f.weighted("fish", 80000, 1000, 20), f.variant("rice", 10000, 20)
	fb, rbt := f.batchAt(fish, 10, nil, 0), f.batchAt(rice, 10, nil, 0)
	oid, fids, it := f.seedPriced(false, &pricing.Coupon{Kind: "fixed", Value: 10000}, 6000, pl{f.vnd, fish, 80000, 3, 3000}, pl{f.vnd, rice, 10000, 2, 0})
	st := f.staffAs("staff", 0)
	f.admJSON(st, "PATCH", editPath(oid), editBody("rice nai", it[0], 2, it[1], 0), http.StatusOK)
	if got := f.itemRow(it[0]); !strings.HasPrefix(got, "2/active/2000/160000/") {
		t.Errorf("edit fish %s", got)
	}
	// notun ordered 2000 g: 2201 g +10% er beshi
	wantErr(t, "+10%", f.adm(st, "POST", packPath(fids[0]), packBody("", it[0], 2201)), http.StatusConflict, "weight_over_limit")
	wantErr(t, "removed item body te", f.adm(st, "POST", packPath(fids[0]), packBody("", it[0], 1900, it[1], 1000)), http.StatusBadRequest, "validation_failed")
	f.admJSON(st, "POST", packPath(fids[0]), packBody("", it[0], 1900), http.StatusOK)
	for _, c := range []struct{ name, got, want string }{
		{"rice", f.itemRow(it[1]), "2/removed/-/0/0"},
		// fixed coupon edit e eligible er onupate (10000 × 160000/260000 = 6153), pack e ekoi thake (8.5)
		{"fish", f.itemMoney(it[0]), "152000/6153/1900"},
		{"order", f.oMoney(oid), "152000/6153/6000/151847"},
		{"fulfillment", f.fMoney(fids[0]), "152000/6153/6000/151847"},
		{"batch", s(f.rem(fb)) + "/" + s(f.rem(rbt)), "8/10"},
	} {
		if c.got != c.want {
			t.Errorf("%s: %s, chai %s", c.name, c.got, c.want)
		}
	}
}

// TestEditThenPackRandomIntegration (14.2 test 41, 8.5 v1.7 TestRepriceEditThenPackRandom er DB rup): 200 random order
// (1–3 vendor, fixed/ojon-variable, coupon nai/percent/fixed, COD/prepaid; seed 41) e owner er random edit, tarpor
// vendor onujayi pack (asol ojon 70–130%, note shoho). Protiti dhap er por DB theke: order = fulfillment er jog =
// active line er jog, removed item e taka 0, edit e ojon EditWeightG, pack e onno fulfillment hubohu (freeze), prepaid e
// mot refund = paid − total (COD e refund na), stock ferot = komano qty.
func TestEditThenPackRandomIntegration(t *testing.T) {
	f := newCheckout(t)
	rng := rand.New(rand.NewSource(41))
	vb, rb := f.vendorVariant("vb", 0)
	vc, rc := f.vendorVariant("vc", 0)
	vendors := [][2]int64{{f.vnd, f.variant("h", 10000, 0)}, {vb, rb}, {vc, rc}}
	owner := f.staffAs("owner", 0)

	type item struct {
		id, variant int64
		qty, grams  int
		vendorIdx   int
		removed     bool
		newQty      int
		newGrams    int
		touched     bool
	}
	check := func(k int, step string, oid int64, prepaid bool) {
		t.Helper()
		bad := f.str(`SELECT concat_ws(',',
			CASE WHEN o.subtotal <> x.s OR o.discount <> x.d OR o.delivery_charge <> x.c OR o.total <> x.t THEN 'order≠fulfillment' END,
			(SELECT 'fulfillment≠line' FROM order_fulfillments g WHERE g.order_id = o.id AND (g.subtotal, g.discount) <>
				(SELECT coalesce(sum(line_total), 0), coalesce(sum(discount_share), 0) FROM order_items i WHERE i.fulfillment_id = g.id) LIMIT 1),
			(SELECT 'total' FROM order_fulfillments g WHERE g.order_id = o.id AND g.total <> g.subtotal - g.discount + g.delivery_charge LIMIT 1),
			(SELECT 'removed taka' FROM order_items i WHERE i.order_id = o.id AND i.status = 'removed' AND (i.line_total, i.discount_share) <> (0, 0) LIMIT 1))
			FROM orders o, LATERAL (SELECT sum(subtotal) s, sum(discount) d, sum(delivery_charge) c, sum(total) t
				FROM order_fulfillments WHERE order_id = o.id) x WHERE o.id = $1`, oid)
		if bad != "" {
			t.Fatalf("order %d %s: %s", k, step, bad)
		}
		ref := f.count(`SELECT coalesce(sum(amount), 0) FROM refunds WHERE order_id = $1`, oid)
		want := int64(0)
		if prepaid {
			want = f.count(`SELECT (SELECT sum(amount) FROM payments WHERE order_id = $1) - total FROM orders WHERE id = $1`, oid)
		}
		if ref != want {
			t.Fatalf("order %d %s: refund %d, chai %d", k, step, ref, want)
		}
	}
	stock := func() int64 {
		return f.count(`SELECT sum(stock) FROM product_variants WHERE id IN ($1, $2, $3)`, vendors[0][1], rb, rc)
	}

	for k := 0; k < 200; k++ {
		var ls []pl
		var items []*item
		nv := 1 + rng.Intn(3)
		for i := 0; i < 1+rng.Intn(5); i++ {
			vi := rng.Intn(nv)
			x := &item{variant: vendors[vi][1], qty: 1 + rng.Intn(4), vendorIdx: vi}
			if rng.Intn(3) == 0 {
				x.grams = x.qty * (200 + rng.Intn(1800))
			}
			items = append(items, x)
			ls = append(ls, pl{vendors[vi][0], x.variant, 1 + int64(rng.Intn(300000)), x.qty, x.grams})
		}
		var c *pricing.Coupon
		switch rng.Intn(3) {
		case 1:
			c = &pricing.Coupon{Kind: "fixed", Value: 1 + int64(rng.Intn(100000))}
		case 2:
			c = &pricing.Coupon{Kind: "percent", Value: 1 + int64(rng.Intn(50))}
		}
		prepaid := rng.Intn(2) == 0
		oid, fids, ids := f.seedPriced(prepaid, c, int64(rng.Intn(15000)), ls...)
		for i, id := range ids {
			items[i].id = id
		}
		check(k, "seed", oid, prepaid)

		var body []int64
		for _, x := range items {
			switch rng.Intn(3) {
			case 1:
				x.touched, x.newQty = true, 0
			case 2:
				if x.qty > 1 {
					x.touched, x.newQty = true, 1+rng.Intn(x.qty-1)
				}
			}
		}
		if !items[0].touched {
			items[0].touched, items[0].newQty = true, 0
		}
		var back int64
		for _, x := range items {
			if !x.touched {
				continue
			}
			body = append(body, x.id, int64(x.newQty))
			back += int64(x.qty - x.newQty)
			if x.newQty == 0 {
				x.removed = true
			} else if x.grams > 0 {
				x.newGrams = pricing.EditWeightG(x.grams, x.qty, x.newQty)
			}
		}
		s0 := stock()
		if rec := f.adm(owner, "PATCH", editPath(oid), editBody("random", body...)); rec.Code != http.StatusOK {
			t.Fatalf("order %d edit: %d %s", k, rec.Code, rec.Body.String())
		}
		if got := stock() - s0; got != back {
			t.Fatalf("order %d: stock ferot %d, chai %d", k, got, back)
		}
		for _, x := range items {
			if x.newGrams > 0 {
				if g := f.count(`SELECT ordered_weight_g FROM order_items WHERE id = $1`, x.id); g != int64(x.newGrams) {
					t.Fatalf("order %d item %d: ojon %d, chai %d", k, x.id, g, x.newGrams)
				}
			}
		}
		check(k, "edit", oid, prepaid)

		for fi, fid := range fids {
			if f.str(`SELECT status FROM order_fulfillments WHERE id = $1`, fid) != "open" {
				continue
			}
			var w []int64
			for _, x := range items {
				if x.removed || x.grams == 0 || f.id(`SELECT fulfillment_id FROM order_items WHERE id = $1`, x.id) != fid {
					continue
				}
				g := f.count(`SELECT ordered_weight_g FROM order_items WHERE id = $1`, x.id)
				w = append(w, x.id, max(1, g*int64(70+rng.Intn(61))/100))
			}
			others := f.str(`SELECT string_agg(subtotal || '/' || discount || '/' || delivery_charge || '/' || total, ',' ORDER BY id)
				FROM order_fulfillments WHERE order_id = $1 AND id <> $2`, oid, fid) + f.str(`SELECT string_agg(line_total || '/' ||
				discount_share, ',' ORDER BY id) FROM order_items WHERE order_id = $1 AND fulfillment_id <> $2`, oid, fid)
			if rec := f.adm(owner, "POST", packPath(fid), packBody("customer OK", w...)); rec.Code != http.StatusOK {
				t.Fatalf("order %d pack %d: %d %s", k, fi, rec.Code, rec.Body.String())
			}
			if got := f.str(`SELECT string_agg(subtotal || '/' || discount || '/' || delivery_charge || '/' || total, ',' ORDER BY id)
				FROM order_fulfillments WHERE order_id = $1 AND id <> $2`, oid, fid) + f.str(`SELECT string_agg(line_total || '/' ||
				discount_share, ',' ORDER BY id) FROM order_items WHERE order_id = $1 AND fulfillment_id <> $2`, oid, fid); got != others {
				t.Fatalf("order %d pack %d: onno vendor bodleche %s → %s", k, fi, others, got)
			}
			check(k, "pack", oid, prepaid)
		}
	}
}
