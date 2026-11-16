package order_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"vendrix/backend/internal/inventory"
	"vendrix/backend/internal/jobs"
	"vendrix/backend/internal/order"
	"vendrix/backend/internal/pricing"
)

// logHook: fixture er slog handler; fn thakle protiti record e dake (alert.batch_short dhorte).
type logHook struct {
	mu sync.Mutex
	fn func(slog.Record)
}

func (h *logHook) Enabled(context.Context, slog.Level) bool { return true }
func (h *logHook) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *logHook) WithGroup(string) slog.Handler            { return h }
func (h *logHook) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fn != nil {
		h.fn(r)
	}
	return nil
}

func (f *checkoutFx) batchExpiry(on bool) {
	f.t.Helper()
	v := `[]`
	if on {
		v = `["batch_expiry"]`
	}
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{features}', $2::jsonb) WHERE shop_id = $1`, f.shop, v)
}

// batchAt: variant e batch; expiry = shop er aj + days (nil = expiry nai), received_at = now − ago din.
func (f *checkoutFx) batchAt(v int64, rem int, days any, ago int) int64 {
	f.t.Helper()
	return f.id(`INSERT INTO inventory_batches (shop_id, variant_id, quantity_received, quantity_remaining, expiry_date, received_at)
		VALUES ($1, $2, $3, $3, (SELECT (now() AT TIME ZONE timezone)::date + $4::int FROM shops WHERE id = $1),
			now() - make_interval(days => $5)) RETURNING id`, f.shop, v, rem, days, ago)
}

func (f *checkoutFx) rem(b int64) int64 {
	return f.count(`SELECT quantity_remaining FROM inventory_batches WHERE id = $1`, b)
}

func packPath(fid int64) string { return "/api/admin/fulfillments/" + s(fid) + "/pack" }

func noItems() map[string]any { return map[string]any{"items": []any{}} }

// TestPackFIFOIntegration (8.11 item 2, 6.4, 14.2 test 21): batch_expiry on e pack protiti item er qty FIFO te
// batch theke komay: expiry age (ajker expiry chole), ekoi expiry te received_at age, expiry nai (NULL) shesh e;
// expire batch (job er age-o) bad; order_item_batches e variant shoho; variant.stock bodlay na (checkout e kome
// geche); fulfillment packed + packed_at, history, display_status, audit. batch_expiry off e FIFO ar
// order_item_batches na.
func TestPackFIFOIntegration(t *testing.T) {
	f := newCheckout(t)
	f.batchExpiry(true)
	fish, honey := f.variant("fish", 10000, 20), f.variant("honey", 10000, 20)
	exp := f.batchAt(fish, 5, -1, 9)    // kal expire, job chaleni
	null := f.batchAt(fish, 5, nil, 30) // sobcheye purono, kintu expiry nai: shesh e
	late := f.batchAt(fish, 2, 10, 1)
	soon2 := f.batchAt(fish, 1, 5, 0)
	soon1 := f.batchAt(fish, 2, 5, 2) // soon2 er ekoi expiry, age elo
	today := f.batchAt(fish, 1, 0, 0) // aj expiry: chole
	hb := f.batchAt(honey, 4, nil, 0)
	oid, fids := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{fish, 8}, {honey, 3}}})
	st := f.staffAs("staff", 0)

	out := f.admJSON(st, "POST", packPath(fids[0]), noItems(), http.StatusOK)
	if out["status"] != "packed" {
		t.Errorf("response %v", out)
	}
	for _, c := range []struct {
		name      string
		got, want int64
	}{
		{"expire batch", f.rem(exp), 5},
		{"aj expiry", f.rem(today), 0},
		{"soon1 (age elo)", f.rem(soon1), 0},
		{"soon2", f.rem(soon2), 0},
		{"late", f.rem(late), 0},
		{"null (shesh e, 2 newa)", f.rem(null), 3},
		{"honey", f.rem(hb), 1},
		{"fish stock bodlay na", f.count(`SELECT stock FROM product_variants WHERE id = $1`, fish), 20},
		{"order_item_batches row", f.count(`SELECT count(*) FROM order_item_batches ob JOIN order_items i ON i.id = ob.order_item_id
			WHERE i.order_id = $1 AND ob.variant_id = i.variant_id`, oid), 6},
		{"fish null batch e 2", f.count(`SELECT quantity FROM order_item_batches WHERE batch_id = $1`, null), 2},
		{"packed_at", f.count(`SELECT count(*) FROM order_fulfillments WHERE id = $1 AND status = 'packed' AND packed_at IS NOT NULL`, fids[0]), 1},
		{"history", f.count(`SELECT count(*) FROM order_status_history WHERE fulfillment_id = $1 AND from_status = 'open'
			AND to_status = 'packed' AND actor_type = 'staff' AND actor_id = $2`, fids[0], st.ID), 1},
		{"audit", f.count(`SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND staff_id = $2 AND action = 'fulfillment.pack'
			AND entity_type = 'fulfillment' AND entity_id = $3`, f.shop, st.ID, fids[0]), 1},
		{"pack e movement na", f.count(`SELECT count(*) FROM inventory_movements WHERE order_id = $1`, oid), 0},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, chai %d", c.name, c.got, c.want)
		}
	}
	if s := f.orderStatus(oid); s != "confirmed/packed" {
		t.Errorf("order %s", s)
	}

	// batch_expiry off: batch thakleo FIFO na
	f.batchExpiry(false)
	oid2, f2 := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{honey, 1}}})
	f.admJSON(st, "POST", packPath(f2[0]), noItems(), http.StatusOK)
	if r, n := f.rem(hb), f.count(`SELECT count(*) FROM order_item_batches ob JOIN order_items i ON i.id = ob.order_item_id
		WHERE i.order_id = $1`, oid2); r != 1 || n != 0 {
		t.Errorf("off: batch %d, order_item_batches %d", r, n)
	}
	if s := f.fStatuses(oid2); s != "packed" {
		t.Errorf("off: fulfillment %s", s)
	}
}

// TestPackBatchShortIntegration (8.11 item 2, 11.7 item 13 iii, 14.2 test 21): kono item er jonno FIFO batch e qty
// na hole 409 out_of_stock details {item_id, variant_id, available}, kichu likha na (ager item er batch o ferot);
// alert.batch_short slog rollback er por (tokhon variant er lock nei).
func TestPackBatchShortIntegration(t *testing.T) {
	f := newCheckout(t)
	f.batchExpiry(true)
	rice, fish := f.variant("rice", 10000, 20), f.variant("fish", 10000, 20)
	rb := f.batchAt(rice, 5, nil, 0)
	fb, fexp := f.batchAt(fish, 1, 3, 0), f.batchAt(fish, 10, -1, 0) // expire batch gona hoy na
	oid, fids := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 2}, {fish, 3}}})
	fishItem := f.id(`SELECT id FROM order_items WHERE order_id = $1 AND variant_id = $2`, oid, fish)

	var alerts []map[string]any
	f.logs.fn = func(r slog.Record) {
		if r.Message != "alert.batch_short" {
			return
		}
		a := map[string]any{}
		r.Attrs(func(x slog.Attr) bool { a[x.Key] = x.Value.Any(); return true })
		// tx er baire: pack er variant lock chhere diyeche, rice er batch o ferot
		var one int
		a["nowait"] = f.d.QueryRowContext(f.ctx, `SELECT 1 FROM product_variants WHERE id = $1 FOR UPDATE NOWAIT`, rice).Scan(&one)
		a["rice_rem"] = f.rem(rb)
		alerts = append(alerts, a)
	}
	rec := f.adm(f.staffAs("staff", 0), "POST", packPath(fids[0]), noItems())
	e := wantErr(t, "batch kom", rec, http.StatusConflict, "out_of_stock")
	want := map[string]any{"item_id": fishItem, "variant_id": fish, "available": 1}
	if b, _ := json.Marshal(e.Details); string(b) != mustJSON(want) {
		t.Errorf("details %s, chai %s", b, mustJSON(want))
	}
	if len(alerts) != 1 {
		t.Fatalf("alert.batch_short %d ta", len(alerts))
	}
	a := alerts[0]
	if a["nowait"] != nil || a["rice_rem"] != int64(5) || a["item_id"] != fishItem || a["variant_id"] != fish ||
		a["available"] != int64(1) || a["fulfillment_id"] != fids[0] {
		t.Errorf("alert %v", a)
	}
	for _, c := range []struct {
		name      string
		got, want int64
	}{
		{"rice batch", f.rem(rb), 5},
		{"fish batch", f.rem(fb), 1},
		{"fish expire batch", f.rem(fexp), 10},
		{"order_item_batches", f.count(`SELECT count(*) FROM order_item_batches ob JOIN order_items i ON i.id = ob.order_item_id
			WHERE i.order_id = $1`, oid), 0},
		{"history", f.count(`SELECT count(*) FROM order_status_history WHERE fulfillment_id = $1`, fids[0]), 0},
		{"audit", f.count(`SELECT count(*) FROM audit_logs WHERE action = 'fulfillment.pack'`), 0},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, chai %d", c.name, c.got, c.want)
		}
	}
	if s := f.fStatuses(oid); s != "open" {
		t.Errorf("fulfillment %s", s)
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// TestPackValidationIntegration (11.7 item 13 ii, 7.7, 8.8): body items e shudhu active ojon-variable item, protitir
// packed_weight_g 1–1,000,000; fixed/onno/removed/duplicate item ba ojon-variable baki thakle 400 validation_failed
// fields.items; order confirmed na ba fulfillment open na hole 409 invalid_transition; onno vendor/shop er 404;
// vendor role nijer fulfillment pack kore; TransitionFulfillment(open → packed) pending order e ErrInvalidTransition.
func TestPackValidationIntegration(t *testing.T) {
	f := newCheckout(t)
	rice, fish := f.variant("rice", 10000, 20), f.weighted("fish", 80000, 1000, 20)
	oid, fids := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}, {fish, 1}}})
	f.exec(`UPDATE order_items SET variable_weight = true, ordered_weight_g = 1000 WHERE order_id = $1 AND variant_id = $2`, oid, fish)
	riceItem := f.id(`SELECT id FROM order_items WHERE order_id = $1 AND variant_id = $2`, oid, rice)
	fishItem := f.id(`SELECT id FROM order_items WHERE order_id = $1 AND variant_id = $2`, oid, fish)
	otherOid, _ := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{fish, 1}}})
	otherItem := f.id(`SELECT id FROM order_items WHERE order_id = $1`, otherOid)
	st := f.staffAs("staff", 0)
	w := func(id int64, g int) map[string]any { return map[string]any{"item_id": id, "packed_weight_g": g} }
	body := func(it ...map[string]any) map[string]any { return map[string]any{"items": it} }

	for _, c := range []struct {
		name  string
		body  any
		field string
	}{
		{"json na", "{", "body"},
		{"ochena field", `{"items":[],"x":1}`, "body"},
		{"ojon baki", noItems(), "items"},
		{"ojon 0", body(w(fishItem, 0)), "items"},
		{"ojon 1,000,001", body(w(fishItem, 1000001)), "items"},
		{"fixed item", body(w(fishItem, 900), w(riceItem, 1000)), "items"},
		{"onno order er item", body(w(fishItem, 900), w(otherItem, 900)), "items"},
		{"duplicate", body(w(fishItem, 900), w(fishItem, 900)), "items"},
	} {
		e := wantErr(t, c.name, f.adm(st, "POST", packPath(fids[0]), c.body), http.StatusBadRequest, "validation_failed")
		if e.Fields[c.field] == "" {
			t.Errorf("%s: fields %v, %s chai", c.name, e.Fields, c.field)
		}
	}
	if s := f.fStatuses(oid); s != "open" {
		t.Fatalf("bhul body te fulfillment %s", s)
	}
	// removed ojon item body te na (active na)
	rm, rf := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}})
	f.exec(`INSERT INTO order_items (shop_id, order_id, fulfillment_id, vendor_id, variant_id, product_name, sku, unit_price,
		quantity, variable_weight, ordered_weight_g, line_total, status)
		VALUES ($1, $2, $3, $4, $5, '{"bn":"মাছ"}', 's', 80000, 1, true, 1000, 0, 'removed')`, f.shop, rm, rf[0], f.vnd, fish)
	removed := f.id(`SELECT id FROM order_items WHERE order_id = $1 AND status = 'removed'`, rm)
	wantErr(t, "removed item", f.adm(st, "POST", packPath(rf[0]), body(w(removed, 900))), http.StatusBadRequest, "validation_failed")
	f.admJSON(st, "POST", packPath(rf[0]), noItems(), http.StatusOK)

	// 1,000,000 g COD e +10% er beshi: customer_ok_note lage (v1.37, step 5b)
	f.admJSON(st, "POST", packPath(fids[0]), map[string]any{"items": []any{w(fishItem, 1000000)}, "customer_ok_note": "ok"}, http.StatusOK)
	wantErr(t, "abar pack", f.adm(st, "POST", packPath(fids[0]), body(w(fishItem, 900))), http.StatusConflict, "invalid_transition")

	_, pf := f.seedOrder("cod", "unpaid", "pending", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}})
	wantErr(t, "pending order", f.adm(st, "POST", packPath(pf[0]), noItems()), http.StatusConflict, "invalid_transition")
	if err := f.txFulfillment(pf[0], order.Packed, staff, ""); !errors.Is(err, order.ErrInvalidTransition) {
		t.Errorf("TransitionFulfillment pending order: %v", err)
	}
	_, cf := f.seedOrder("cod", "unpaid", "cancelled", seedF{vendor: f.vnd, status: "cancelled", items: [][2]int64{{rice, 1}}})
	wantErr(t, "cancelled", f.adm(st, "POST", packPath(cf[0]), noItems()), http.StatusConflict, "invalid_transition")
	wantErr(t, "nai", f.adm(st, "POST", packPath(999999999), noItems()), http.StatusNotFound, "not_found")
	wantErr(t, "bhul id", f.adm(st, "POST", "/api/admin/fulfillments/x/pack", noItems()), http.StatusNotFound, "not_found")

	// vendor: nijer ta pack, onno vendor er 404
	vx, rx := f.vendorVariant("vx", 50)
	vy, ry := f.vendorVariant("vy", 50)
	mo, mf := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: vx, status: "open", items: [][2]int64{{rx, 1}}},
		seedF{vendor: vy, status: "open", items: [][2]int64{{ry, 1}}})
	vs := f.staffAs("vendor_staff", vx)
	wantErr(t, "onno vendor", f.adm(vs, "POST", packPath(mf[1]), noItems()), http.StatusNotFound, "not_found")
	f.admJSON(vs, "POST", packPath(mf[0]), noItems(), http.StatusOK)
	if s := f.fStatuses(mo) + "|" + f.orderStatus(mo); s != "packed,open|confirmed/partial" {
		t.Errorf("marketplace %s", s)
	}
}

// TestPackParallelIntegration (8.8): ekoi fulfillment e ekshathe pack: ekta 200, baki 409 invalid_transition,
// batch ekbar-i kome.
func TestPackParallelIntegration(t *testing.T) {
	f := newCheckout(t)
	f.batchExpiry(true)
	rice := f.variant("rice", 10000, 20)
	b := f.batchAt(rice, 10, nil, 0)
	_, fids := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 2}}})
	st := f.staffAs("staff", 0)
	out := parallel(6, func(int) *httptest.ResponseRecorder { return f.adm(st, "POST", packPath(fids[0]), noItems()) })
	ok := 0
	for _, r := range out {
		if r.Code == http.StatusOK {
			ok++
		} else {
			wantErr(t, "parallel", r, http.StatusConflict, "invalid_transition")
		}
	}
	if ok != 1 || f.rem(b) != 8 {
		t.Errorf("200 %d ta, batch %d", ok, f.rem(b))
	}
}

// TestPackExpireRaceIntegration (11.7 item 2, 3; M7a acceptance): pack ar inventory.expire ekshathe, duita variant
// (item er kram ulta): deadlock/error na, sob pack hoy (expire batch kokhono newa hoy na), batch ≥ 0, expire batch 0,
// expired movement variant e ekta.
func TestPackExpireRaceIntegration(t *testing.T) {
	f := newCheckout(t)
	f.batchExpiry(true)
	a, b := f.variant("a", 10000, 30), f.variant("b", 10000, 30)
	const n = 8
	var good, old []int64
	for _, v := range []int64{a, b} {
		old = append(old, f.batchAt(v, 3, -1, 5))
		good = append(good, f.batchAt(v, n/2, 3, 2), f.batchAt(v, n/2, 4, 1))
	}
	fids := make([]int64, n)
	for i := range fids {
		items := [][2]int64{{a, 1}, {b, 1}}
		if i%2 == 1 {
			items = [][2]int64{{b, 1}, {a, 1}}
		}
		_, fs := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: items})
		fids[i] = fs[0]
	}
	st := f.staffAs("staff", 0)
	run := inventory.ExpireJob(f.d)
	errs := parallel(n+3, func(i int) error {
		if i >= n {
			return run(f.ctx, jobs.Job{ShopID: f.shop, Kind: "inventory.expire"})
		}
		if r := f.adm(st, "POST", packPath(fids[i]), noItems()); r.Code != http.StatusOK {
			return errors.New(r.Body.String())
		}
		return nil
	})
	for i, err := range errs {
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}
	}
	for _, x := range append(good, old...) {
		if r := f.rem(x); r != 0 {
			t.Errorf("batch %d remaining %d, 0 chai", x, r)
		}
	}
	for _, c := range []struct {
		name      string
		got, want int64
	}{
		{"expire batch order e na", f.count(`SELECT count(*) FROM order_item_batches WHERE batch_id = ANY($1)`, old), 0},
		{"expired movement", f.count(`SELECT count(*) FROM inventory_movements WHERE reason = 'expired' AND variant_id IN ($1, $2)`, a, b), 2},
		{"stock", f.count(`SELECT sum(stock) FROM product_variants WHERE id IN ($1, $2)`, a, b), 54},
		{"packed", f.count(`SELECT count(*) FROM order_fulfillments WHERE id = ANY($1) AND status = 'packed'`, fids), n},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, chai %d", c.name, c.got, c.want)
		}
	}
}

// pl: seedPriced er line; grams 0 = fixed (price × qty), na hole ojon-variable (price 1 kg er, grams puro line er).
type pl struct {
	vendor, variant, price int64
	qty, grams             int
}

// seedPriced: confirmed order pricing.Calculate diye (coupon snapshot, per_order delivery), tai Reprice er Original
// mile. prepaid = sslcommerz paid + succeeded payment. Fulfillment vendor er prothom dekha kram e; item line er kram e.
func (f *checkoutFx) seedPriced(prepaid bool, c *pricing.Coupon, delivery int64, ls ...pl) (int64, []int64, []int64) {
	f.t.Helper()
	lines := make([]pricing.Line, len(ls))
	for i, l := range ls {
		lines[i] = pricing.Line{VendorID: l.vendor, UnitPrice: l.price, Qty: l.qty, VariableWeight: l.grams > 0, WeightG: l.grams}
	}
	r, err := pricing.Calculate(lines, c, pricing.Zone{Charge: delivery}, pricing.PerOrder)
	if err != nil {
		f.t.Fatal(err)
	}
	method, pay := "cod", "unpaid"
	if prepaid {
		method, pay = "sslcommerz", "paid"
	}
	var cid, ckind, cval any
	if c != nil {
		cid = f.id(`INSERT INTO coupons (shop_id, code, kind, value) VALUES ($1, $2, $3, $4) RETURNING id`, f.shop, rand.Text(), c.Kind, c.Value)
		ckind, cval = c.Kind, c.Value
	}
	tok := rand.Text()
	oid := f.id(`INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash, customer_name,
		customer_phone, recipient_name, recipient_phone, ship_district, ship_area, ship_address, subtotal, discount,
		delivery_charge, total, coupon_id, coupon_code, coupon_kind, coupon_value, coupon_funded_by,
		payment_method, payment_status, status, display_status)
		VALUES ($1, $2, $2, $2, '\x00', 'R', '+8801711000009', 'R', '+8801711000009', 'Dhaka', 'Dhanmondi', 'x', $3, $4, $5, $6,
			$7, CASE WHEN $7::bigint IS NULL THEN NULL ELSE 'C' END, $8, $9,
			CASE WHEN $7::bigint IS NULL THEN NULL ELSE 'shop' END, $10, $11, 'confirmed', 'confirmed')
		RETURNING id`, f.shop, tok, r.Subtotal, r.Discount, r.DeliveryCharge, r.Total, cid, ckind, cval, method, pay)
	if prepaid {
		f.id(`INSERT INTO payments (shop_id, order_id, provider, tran_id, amount, status) VALUES ($1, $2, $3, $4, $5, 'succeeded')
			RETURNING id`, f.shop, oid, method, rand.Text(), r.Total)
	}
	fid := map[int64]int64{}
	var fids []int64
	for i, g := range r.Groups {
		fid[g.VendorID] = f.id(`INSERT INTO order_fulfillments (shop_id, order_id, vendor_id, seq, subtotal, discount, delivery_charge, total)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`, f.shop, oid, g.VendorID, i+1, g.Subtotal, g.Discount, g.DeliveryCharge, g.Total)
		fids = append(fids, fid[g.VendorID])
	}
	var items []int64
	for i, l := range ls {
		items = append(items, f.id(`INSERT INTO order_items (shop_id, order_id, fulfillment_id, vendor_id, variant_id, product_name, sku,
			unit_price, quantity, variable_weight, ordered_weight_g, line_total, discount_share)
			VALUES ($1, $2, $3, $4, $5, '{"bn":"প"}', 's', $6, $7, $8, NULLIF($9, 0), $10, $11) RETURNING id`,
			f.shop, oid, fid[l.vendor], l.vendor, l.variant, l.price, l.qty, l.grams > 0, l.grams, r.Lines[i].LineTotal, r.Lines[i].DiscountShare))
	}
	return oid, fids, items
}

// packBody: {items: [{item_id, packed_weight_g}...], customer_ok_note?}; iw = item, gram, item, gram ...
func packBody(note string, iw ...int64) map[string]any {
	items := []any{}
	for i := 0; i+1 < len(iw); i += 2 {
		items = append(items, map[string]any{"item_id": iw[i], "packed_weight_g": iw[i+1]})
	}
	b := map[string]any{"items": items}
	if note != "" {
		b["customer_ok_note"] = note
	}
	return b
}

// itemMoney: line_total/discount_share/packed_weight_g (- = NULL).
func (f *checkoutFx) itemMoney(id int64) string {
	return f.str(`SELECT line_total || '/' || discount_share || '/' || coalesce(packed_weight_g::text, '-') FROM order_items WHERE id = $1`, id)
}

// fMoney ar oMoney: subtotal/discount/delivery_charge/total.
func (f *checkoutFx) fMoney(id int64) string {
	return f.str(`SELECT subtotal || '/' || discount || '/' || delivery_charge || '/' || total FROM order_fulfillments WHERE id = $1`, id)
}

func (f *checkoutFx) oMoney(id int64) string {
	return f.str(`SELECT subtotal || '/' || discount || '/' || delivery_charge || '/' || total FROM orders WHERE id = $1`, id)
}

// TestPackWeightIntegration (8.11 item 3, 8.5 Reprice Edit=false, 14.2 test 9, 11.7 item 13 iv): pack e packed_weight_g
// save, pricing.Reprice er notun line/fulfillment/order taka. Prepaid: beshi ojon e line ager total e cap (taka bare na,
// refund na); kom ojon e parthokko refunds kind reprice pending (fulfillment_id, payment er method), SMS na. COD: notun
// total save, total bodlale sms.total_changed job tx e (payload shudhu order_id, lekha te notun total); ekoi ojon e
// taka ekoi, SMS na.
func TestPackWeightIntegration(t *testing.T) {
	f := newCheckout(t)
	fish, rice := f.weighted("fish", 80000, 1000, 20), f.variant("rice", 10000, 20)
	pc := &pricing.Coupon{Kind: "percent", Value: 10}
	seed := func(prepaid bool) (int64, int64, []int64) {
		oid, fids, items := f.seedPriced(prepaid, pc, 6000, pl{f.vnd, fish, 80000, 1, 1000}, pl{f.vnd, rice, 10000, 2, 0})
		return oid, fids[0], items
	}
	st := f.staffAs("staff", 0)
	type row struct{ name, got, want string }
	check := func(name string, rows ...row) {
		t.Helper()
		for _, r := range rows {
			if r.got != r.want {
				t.Errorf("%s %s: %s, chai %s", name, r.name, r.got, r.want)
			}
		}
	}
	n := func(v int64) string { return s(v) }

	// prepaid, beshi ojon: 88,000 er line 80,000 e cap
	oid, fid, it := seed(true)
	f.admJSON(st, "POST", packPath(fid), packBody("", it[0], 1200), http.StatusOK)
	check("prepaid beshi", row{"fish", f.itemMoney(it[0]), "80000/8000/1200"}, row{"rice", f.itemMoney(it[1]), "20000/2000/-"},
		row{"fulfillment", f.fMoney(fid), "100000/10000/6000/96000"}, row{"order", f.oMoney(oid), "100000/10000/6000/96000"},
		row{"refund", n(f.count(`SELECT count(*) FROM refunds WHERE order_id = $1`, oid)), "0"},
		row{"sms", n(f.jobCount("sms.total_changed", oid)), "0"})

	// prepaid, kom ojon: 72,000; percent discount 9,200; refund 96,000 − 88,800
	oid, fid, it = seed(true)
	f.admJSON(st, "POST", packPath(fid), packBody("", it[0], 900), http.StatusOK)
	check("prepaid kom", row{"fish", f.itemMoney(it[0]), "72000/7200/900"}, row{"rice", f.itemMoney(it[1]), "20000/2000/-"},
		row{"fulfillment", f.fMoney(fid), "92000/9200/6000/88800"}, row{"order", f.oMoney(oid), "92000/9200/6000/88800"},
		row{"refund", f.str(`SELECT string_agg(amount || '/' || kind || '/' || status || '/' || fulfillment_id || '/' || method
			|| '/' || (payment_id IS NOT NULL), ',') FROM refunds WHERE order_id = $1`, oid), "7200/reprice/pending/" + n(fid) + "/sslcommerz/true"},
		row{"sms", n(f.jobCount("sms.total_changed", oid)), "0"})

	// COD, kom ojon: notun total ar SMS
	oid, fid, it = seed(false)
	f.admJSON(st, "POST", packPath(fid), packBody("", it[0], 900), http.StatusOK)
	check("cod kom", row{"fish", f.itemMoney(it[0]), "72000/7200/900"}, row{"fulfillment", f.fMoney(fid), "92000/9200/6000/88800"},
		row{"order", f.oMoney(oid), "92000/9200/6000/88800"},
		row{"refund", n(f.count(`SELECT count(*) FROM refunds WHERE order_id = $1`, oid)), "0"},
		row{"sms", n(f.jobCount("sms.total_changed", oid)), "1"})
	sms := &smsLog{}
	if err := f.runJob(order.JobHandlers(f.d, sms, slog.New(slog.DiscardHandler), confirmSecret), "sms.total_changed", oid); err != nil {
		t.Fatal(err)
	}
	if sent := sms.take(); len(sent) != 1 || !strings.HasPrefix(sent[0], "+8801711000009|") ||
		!strings.Contains(sent[0], f.number(oid)) || !strings.Contains(sent[0], "৳888") {
		t.Errorf("sms %q", sent)
	}

	// COD, ekoi ojon: taka ekoi, SMS na
	oid, fid, it = seed(false)
	f.admJSON(st, "POST", packPath(fid), packBody("", it[0], 1000), http.StatusOK)
	check("cod ekoi", row{"fish", f.itemMoney(it[0]), "80000/8000/1000"}, row{"order", f.oMoney(oid), "100000/10000/6000/96000"},
		row{"sms", n(f.jobCount("sms.total_changed", oid)), "0"})
}

// TestPackOverweightIntegration (8.11 item 3 v1.37, 7.7): COD e ojon-variable line e packed*10 > ordered*11 hole 409
// weight_over_limit details {items: [{item_id, ordered_weight_g, max_weight_g}]} (shudhu beshi item, id kram e), kichu
// likha na; thik +10% chole; customer_ok_note (khali na) thakle pack, note history te; halka ojon e 409 na; prepaid e
// 409 na, total bare na; note 500 rune er beshi 400.
func TestPackOverweightIntegration(t *testing.T) {
	f := newCheckout(t)
	fish, honey := f.weighted("fish", 80000, 1000, 20), f.weighted("honey", 60000, 500, 20)
	seed := func(prepaid bool) (int64, int64, []int64) {
		oid, fids, items := f.seedPriced(prepaid, nil, 0, pl{f.vnd, fish, 80000, 1, 1000}, pl{f.vnd, honey, 60000, 1, 555})
		return oid, fids[0], items
	}
	st := f.staffAs("staff", 0)
	over := func(name string, rec *httptest.ResponseRecorder, want ...map[string]any) {
		t.Helper()
		e := wantErr(t, name, rec, http.StatusConflict, "weight_over_limit")
		if b, _ := json.Marshal(e.Details); string(b) != mustJSON(map[string]any{"items": want}) {
			t.Errorf("%s details %s", name, b)
		}
	}
	lim := func(id int64, ordered, max int) map[string]any {
		return map[string]any{"item_id": id, "ordered_weight_g": ordered, "max_weight_g": max}
	}

	oid, fid, it := seed(false)
	over("fish +10%+1g", f.adm(st, "POST", packPath(fid), packBody("", it[0], 1101, it[1], 610)), lim(it[0], 1000, 1100))
	over("duita", f.adm(st, "POST", packPath(fid), packBody("", it[0], 1101, it[1], 611)), lim(it[0], 1000, 1100), lim(it[1], 555, 610))
	over("khali note", f.adm(st, "POST", packPath(fid), packBody("   ", it[0], 1101, it[1], 610)), lim(it[0], 1000, 1100))
	e := wantErr(t, "note 501", f.adm(st, "POST", packPath(fid), packBody(strings.Repeat("ক", 501), it[0], 1101, it[1], 610)),
		http.StatusBadRequest, "validation_failed")
	if e.Fields["customer_ok_note"] == "" {
		t.Errorf("note 501 fields %v", e.Fields)
	}
	if got := f.fStatuses(oid) + "|" + f.oMoney(oid) + "|" + f.itemMoney(it[0]) + "|" +
		s(f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1`, oid)); got != "open|113300/0/0/113300|80000/0/-|0" {
		t.Errorf("409 er por kichu likha hoyeche: %s", got)
	}
	note := strings.Repeat("ক", 480) + " customer OK"
	f.admJSON(st, "POST", packPath(fid), packBody(note, it[0], 1101, it[1], 611), http.StatusOK)
	if got := f.oMoney(oid) + "|" + f.str(`SELECT note FROM order_status_history WHERE fulfillment_id = $1 AND to_status = 'packed'`, fid); got != "124740/0/0/124740|"+note {
		t.Errorf("note shoho pack: %s", got)
	}

	oid, fid, it = seed(false)
	f.admJSON(st, "POST", packPath(fid), packBody("", it[0], 1100, it[1], 610), http.StatusOK)
	if got := f.oMoney(oid) + "|" + f.str(`SELECT note FROM order_status_history WHERE fulfillment_id = $1 AND to_status = 'packed'`, fid); got != "124600/0/0/124600|" {
		t.Errorf("thik +10%%: %s", got)
	}
	oid, fid, it = seed(false)
	f.admJSON(st, "POST", packPath(fid), packBody("", it[0], 1, it[1], 300), http.StatusOK)
	if got := f.oMoney(oid); got != "18080/0/0/18080" {
		t.Errorf("halka: %s", got)
	}
	oid, fid, it = seed(true)
	f.admJSON(st, "POST", packPath(fid), packBody("", it[0], 5000, it[1], 2000), http.StatusOK)
	if got := f.oMoney(oid) + "|" + f.itemMoney(it[0]); got != "113300/0/0/113300|80000/0/5000" {
		t.Errorf("prepaid beshi: %s", got)
	}
}

// TestPackFreezeIntegration (8.5 v1.4, 14.2 test 32): marketplace e ek vendor er pack e onno vendor er fulfillment ar
// item er taka hubohu; order er taka fulfillment er jog; tarpor onno vendor er pack e ager vendor er taka hubohu.
func TestPackFreezeIntegration(t *testing.T) {
	f := newCheckout(t)
	va, ra := f.vendorVariant("va", 50)
	vb, rb := f.vendorVariant("vb", 50)
	pc := &pricing.Coupon{Kind: "percent", Value: 10}
	oid, fids, it := f.seedPriced(false, pc, 9000, pl{va, ra, 80000, 1, 1000}, pl{vb, rb, 50000, 1, 2000}, pl{vb, rb, 10000, 3, 0})
	snap := func(fid int64, items ...int64) string {
		out := f.fMoney(fid)
		for _, i := range items {
			out += "|" + f.itemMoney(i)
		}
		return out
	}
	sums := func() string {
		return f.str(`SELECT (o.subtotal = sum(x.subtotal) AND o.discount = sum(x.discount) AND o.delivery_charge = sum(x.delivery_charge)
			AND o.total = sum(x.total))::text FROM orders o JOIN order_fulfillments x ON x.order_id = o.id WHERE o.id = $1 GROUP BY o.id`, oid)
	}
	st := f.staffAs("owner", 0)
	b0 := snap(fids[1], it[1], it[2])
	f.admJSON(st, "POST", packPath(fids[0]), packBody("", it[0], 800), http.StatusOK)
	if b := snap(fids[1], it[1], it[2]); b != b0 {
		t.Errorf("A pack e B bodleche: %s → %s", b0, b)
	}
	if a := f.itemMoney(it[0]); !strings.HasSuffix(a, "/800") || strings.HasPrefix(a, "80000/") {
		t.Errorf("A reprice hoyni: %s", a)
	}
	if sums() != "true" {
		t.Error("A pack er por order ≠ fulfillment er jog")
	}
	a0 := snap(fids[0], it[0])
	f.admJSON(st, "POST", packPath(fids[1]), packBody("", it[1], 1900), http.StatusOK)
	if a := snap(fids[0], it[0]); a != a0 {
		t.Errorf("B pack e A bodleche: %s → %s", a0, a)
	}
	if sums() != "true" || f.jobCount("sms.total_changed", oid) != 2 {
		t.Errorf("B pack er por jog %s, sms %d", sums(), f.jobCount("sms.total_changed", oid))
	}
}
