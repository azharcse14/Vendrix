package order_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"vendrix/backend/internal/inventory"
	"vendrix/backend/internal/jobs"
	"vendrix/backend/internal/order"
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

	f.admJSON(st, "POST", packPath(fids[0]), body(w(fishItem, 1000000)), http.StatusOK)
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
