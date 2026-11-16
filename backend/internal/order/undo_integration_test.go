package order_test

import (
	"net/http"
	"testing"

	"vendrix/backend/internal/pricing"
)

func transitionPath(fid int64) string { return "/api/admin/fulfillments/" + s(fid) + "/transition" }

func undoBody(note string) map[string]any { return map[string]any{"to": "open", "note": note} }

// TestPackUndoIntegration (8.8 v1.37 "Fulfillment packed → open", 8.11 item 6): COD packed fulfillment undo → batch
// hubohu ferot (duita FIFO batch), order_item_batches khali, packed_weight_g NULL, item/fulfillment/order taka pack er
// ager shoman, packed_at NULL, history te note, display_status, SMS na, movement na, variant.stock bodlay na, audit;
// abar pack e FIFO. Guard: prepaid 409, active shipment 409 (cancelled shipment chole), shipped → open 409, khali note
// 400, onno vendor 404; vendor role nijer fulfillment undo kore.
func TestPackUndoIntegration(t *testing.T) {
	f := newCheckout(t)
	f.batchExpiry(true)
	fish, rice := f.weighted("fish", 80000, 1000, 20), f.variant("rice", 10000, 20)
	b1, b2 := f.batchAt(fish, 1, 5, 2), f.batchAt(fish, 5, 10, 0) // FIFO: b1 age
	rb := f.batchAt(rice, 5, nil, 0)
	pc := &pricing.Coupon{Kind: "percent", Value: 10}
	oid, fids, it := f.seedPriced(false, pc, 6000, pl{f.vnd, fish, 80000, 2, 2000}, pl{f.vnd, rice, 10000, 2, 0})
	fid := fids[0]
	st := f.staffAs("staff", 0)
	money := func() string {
		return f.itemMoney(it[0]) + "|" + f.itemMoney(it[1]) + "|" + f.fMoney(fid) + "|" + f.oMoney(oid)
	}
	before := money()

	f.admJSON(st, "POST", packPath(fid), packBody("", it[0], 1800), http.StatusOK)
	if money() == before || f.rem(b1) != 0 || f.rem(b2) != 4 || f.rem(rb) != 3 {
		t.Fatalf("pack hoyni: %s, batch %d/%d/%d", money(), f.rem(b1), f.rem(b2), f.rem(rb))
	}
	sms := f.jobCount("sms.total_changed", oid)

	out := f.admJSON(st, "POST", transitionPath(fid), undoBody(" bhul ojon "), http.StatusOK)
	if out["status"] != "open" {
		t.Errorf("response %v", out)
	}
	if got := money(); got != before {
		t.Errorf("taka %s, pack er age %s", got, before)
	}
	for _, c := range []struct {
		name      string
		got, want int64
	}{
		{"b1", f.rem(b1), 1},
		{"b2", f.rem(b2), 5},
		{"rice batch", f.rem(rb), 5},
		{"order_item_batches", f.count(`SELECT count(*) FROM order_item_batches ob JOIN order_items i ON i.id = ob.order_item_id
			WHERE i.order_id = $1`, oid), 0},
		{"packed_at NULL", f.count(`SELECT count(*) FROM order_fulfillments WHERE id = $1 AND status = 'open' AND packed_at IS NULL`, fid), 1},
		{"history", f.count(`SELECT count(*) FROM order_status_history WHERE fulfillment_id = $1 AND from_status = 'packed'
			AND to_status = 'open' AND actor_type = 'staff' AND actor_id = $2 AND note = 'bhul ojon'`, fid, st.ID), 1},
		{"audit", f.count(`SELECT count(*) FROM audit_logs WHERE action = 'fulfillment.transition' AND entity_id = $1
			AND before->>'status' = 'packed' AND after->>'status' = 'open'`, fid), 1},
		{"SMS na", f.jobCount("sms.total_changed", oid), sms},
		{"movement na", f.count(`SELECT count(*) FROM inventory_movements WHERE order_id = $1`, oid), 0},
		{"stock", f.count(`SELECT sum(stock) FROM product_variants WHERE id IN ($1, $2)`, fish, rice), 40},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, chai %d", c.name, c.got, c.want)
		}
	}
	if s := f.orderStatus(oid); s != "confirmed/confirmed" {
		t.Errorf("order %s", s)
	}

	// abar pack: FIFO abar b1 theke
	f.admJSON(st, "POST", packPath(fid), packBody("", it[0], 2000), http.StatusOK)
	if f.rem(b1) != 0 || f.rem(b2) != 4 || f.count(`SELECT quantity FROM order_item_batches WHERE batch_id = $1`, b1) != 1 {
		t.Errorf("abar pack: b1 %d, b2 %d", f.rem(b1), f.rem(b2))
	}
	if got := f.itemMoney(it[0]); got != "160000/16000/2000" {
		t.Errorf("abar pack taka %s", got)
	}

	// guard
	owner := f.staffAs("owner", 0)
	poid, pf, pit := f.seedPriced(true, nil, 0, pl{f.vnd, fish, 80000, 1, 1000})
	f.admJSON(owner, "POST", packPath(pf[0]), packBody("", pit[0], 900), http.StatusOK)
	wantErr(t, "prepaid", f.adm(owner, "POST", transitionPath(pf[0]), undoBody("x")), http.StatusConflict, "invalid_transition")
	if s := f.fStatuses(poid); s != "packed" {
		t.Errorf("prepaid fulfillment %s", s)
	}

	soid, sf := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "packed", items: [][2]int64{{rice, 1}}})
	ship := f.id(`INSERT INTO shipments (shop_id, order_id, fulfillment_id, vendor_id, carrier) VALUES ($1, $2, $3, $4, 'own_rider')
		RETURNING id`, f.shop, soid, sf[0], f.vnd)
	wantErr(t, "active shipment", f.adm(owner, "POST", transitionPath(sf[0]), undoBody("x")), http.StatusConflict, "invalid_transition")
	e := wantErr(t, "khali note", f.adm(owner, "POST", transitionPath(sf[0]), undoBody("   ")), http.StatusBadRequest, "validation_failed")
	if e.Fields["note"] == "" {
		t.Errorf("khali note fields %v", e.Fields)
	}
	f.exec(`UPDATE shipments SET status = 'cancelled' WHERE id = $1`, ship)
	f.admJSON(owner, "POST", transitionPath(sf[0]), undoBody("parcel bad"), http.StatusOK)

	hoid, hf := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "shipped", items: [][2]int64{{rice, 1}}})
	wantErr(t, "shipped → open", f.adm(owner, "POST", transitionPath(hf[0]), undoBody("x")), http.StatusConflict, "invalid_transition")
	if s := f.fStatuses(hoid); s != "shipped" {
		t.Errorf("shipped fulfillment %s", s)
	}

	// vendor: onno vendor 404, nijer vendor_staff 200
	vb, vr := f.vendorVariant("vb", 5)
	_, vf := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: vb, status: "packed", items: [][2]int64{{vr, 1}}})
	wantErr(t, "onno vendor", f.adm(f.staffAs("vendor_owner", vb), "POST", transitionPath(sf[0]), undoBody("x")), http.StatusNotFound, "not_found")
	f.admJSON(f.staffAs("vendor_staff", vb), "POST", transitionPath(vf[0]), undoBody("vendor"), http.StatusOK)
}
