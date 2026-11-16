package order_test

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/order"
)

// seedF: seed order er ek fulfillment; item {variant, qty}, removed = item status 'removed' (taka 0).
type seedF struct {
	vendor  int64
	status  string
	items   [][2]int64
	removed bool
}

// seedOrder: DB te sorasori order (online/prepaid, packed, marketplace: checkout e hoy na emon obostha).
// Item er dam 10000; stock komay na. Paid hole succeeded payment row o.
func (f *checkoutFx) seedOrder(method, payStatus, status string, fs ...seedF) (int64, []int64) {
	f.t.Helper()
	var total int64
	var sts []order.FStatus
	for _, x := range fs {
		if !x.removed {
			for _, it := range x.items {
				total += 10000 * it[1]
			}
		}
		sts = append(sts, order.FStatus(x.status))
	}
	tok := rand.Text()
	oid := f.id(`INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash, customer_name,
		customer_phone, recipient_name, recipient_phone, ship_district, ship_area, ship_address, subtotal, total,
		payment_method, payment_status, status, display_status)
		VALUES ($1, $2, $2, $2, '\x00', 'R', '+8801711000009', 'R', '+8801711000009', 'Dhaka', 'Dhanmondi', 'x', $3, $3, $4, $5, $6, $7)
		RETURNING id`, f.shop, tok, total, method, payStatus, status, order.Summary(order.Status(status), sts))
	if payStatus == "paid" {
		f.id(`INSERT INTO payments (shop_id, order_id, provider, tran_id, amount, status) VALUES ($1, $2, $3, $4, $5, 'succeeded')
			RETURNING id`, f.shop, oid, method, rand.Text(), total)
	}
	var fids []int64
	for i, x := range fs {
		var sub int64
		if !x.removed {
			for _, it := range x.items {
				sub += 10000 * it[1]
			}
		}
		fid := f.id(`INSERT INTO order_fulfillments (shop_id, order_id, vendor_id, seq, status, subtotal, total)
			VALUES ($1, $2, $3, $4, $5, $6, $6) RETURNING id`, f.shop, oid, x.vendor, i+1, x.status, sub)
		for _, it := range x.items {
			line, st := 10000*it[1], "active"
			if x.removed {
				line, st = 0, "removed"
			}
			f.id(`INSERT INTO order_items (shop_id, order_id, fulfillment_id, vendor_id, variant_id, product_name, sku, unit_price,
				quantity, line_total, status) VALUES ($1, $2, $3, $4, $5, '{"bn":"প"}', 's', 10000, $6, $7, $8) RETURNING id`,
				f.shop, oid, fid, x.vendor, it[0], it[1], line, st)
		}
		fids = append(fids, fid)
	}
	return oid, fids
}

// vendorVariant: notun vendor (marketplace) ar tar ekta variant.
func (f *checkoutFx) vendorVariant(slug string, stock int) (int64, int64) {
	f.t.Helper()
	v := f.id(`INSERT INTO vendors (shop_id, slug, name) VALUES ($1, $2, '{"bn":"ভ"}') RETURNING id`, f.shop, slug)
	p := f.id(`INSERT INTO products (shop_id, vendor_id, slug, name) VALUES ($1, $2, $3, '{"bn":"প"}') RETURNING id`, f.shop, v, slug)
	return v, f.id(`INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock)
		VALUES ($1, $2, $3, $4, 10000, $5) RETURNING id`, f.shop, p, v, slug, stock)
}

func (f *checkoutFx) txOrder(oid int64, to order.Status, by order.By, note string) error {
	return db.InTx(f.ctx, f.d, func(tx *sql.Tx) error { return order.TransitionOrder(f.ctx, tx, f.shop, oid, to, by, note) })
}

func (f *checkoutFx) txFulfillment(fid int64, to order.FStatus, by order.By, note string) error {
	return db.InTx(f.ctx, f.d, func(tx *sql.Tx) error { return order.TransitionFulfillment(f.ctx, tx, f.shop, fid, to, by, note) })
}

func (f *checkoutFx) str(query string, args ...any) string {
	f.t.Helper()
	var s sql.NullString
	if err := f.d.QueryRowContext(f.ctx, query, args...).Scan(&s); err != nil {
		f.t.Fatalf("%s: %v", query, err)
	}
	return s.String
}

func (f *checkoutFx) orderStatus(oid int64) string {
	return f.str(`SELECT status || '/' || display_status FROM orders WHERE id = $1`, oid)
}

// fStatuses: order er fulfillment status, id er kram e, koma diye.
func (f *checkoutFx) fStatuses(oid int64) string {
	return f.str(`SELECT string_agg(status, ',' ORDER BY id) FROM order_fulfillments WHERE order_id = $1`, oid)
}

var (
	staff    = order.By{Actor: order.Staff, ID: 0}
	customer = order.By{Actor: order.Customer}
	system   = order.By{Actor: order.System}
)

// TestTransitionIntegration (14.2 test 7, 8.8): DB e na-allowed bodol invalid_transition, online unpaid confirm
// payment_required, side effect (confirmed_at, history, display_status); M6 e packed/shipped ErrNotImplemented.
func TestTransitionIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 10000, 50)

	// COD pending → confirmed (customer, 8.10 er link)
	cod, _ := f.seedOrder("cod", "unpaid", "pending", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}})
	if err := f.txOrder(cod, order.Confirmed, customer, ""); err != nil {
		t.Fatalf("COD confirm: %v", err)
	}
	if s := f.orderStatus(cod); s != "confirmed/confirmed" {
		t.Errorf("COD confirm er por %s", s)
	}
	if n := f.count(`SELECT count(*) FROM orders WHERE id = $1 AND confirmed_at IS NOT NULL`, cod); n != 1 {
		t.Error("confirmed_at nai")
	}
	if n := f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND fulfillment_id IS NULL
		AND from_status = 'pending' AND to_status = 'confirmed' AND actor_type = 'customer'`, cod); n != 1 {
		t.Errorf("confirm er history row %d", n)
	}
	for _, c := range []struct {
		name string
		to   order.Status
		by   order.By
	}{
		{"abar confirm", order.Confirmed, customer},
		{"confirmed → pending", order.Pending, staff},
	} {
		if err := f.txOrder(cod, c.to, c.by, ""); !errors.Is(err, order.ErrInvalidTransition) {
			t.Errorf("%s: %v, ErrInvalidTransition chai", c.name, err)
		}
	}

	// online unpaid: customer ba staff confirm → payment_required, kichu bodlay na
	online, _ := f.seedOrder("sslcommerz", "pending", "pending", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}})
	for _, by := range []order.By{customer, staff, system} {
		if err := f.txOrder(online, order.Confirmed, by, ""); !errors.Is(err, order.ErrPaymentRequired) {
			t.Errorf("online unpaid confirm (%s): %v, ErrPaymentRequired chai", by.Actor, err)
		}
	}
	if s := f.orderStatus(online); s != "pending/pending" {
		t.Errorf("online unpaid %s", s)
	}
	// online paid: customer na (invalid), system hoy
	paid, _ := f.seedOrder("sslcommerz", "paid", "pending", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}})
	if err := f.txOrder(paid, order.Confirmed, customer, ""); !errors.Is(err, order.ErrInvalidTransition) {
		t.Errorf("online paid customer confirm: %v", err)
	}
	if err := f.txOrder(paid, order.Confirmed, system, ""); err != nil {
		t.Errorf("online paid system confirm: %v", err)
	}

	// fulfillment: na-allowed invalid (open → packed M7a step 5a e hoy: TestPackValidationIntegration)
	_, fids := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}})
	for _, c := range []struct {
		name string
		to   order.FStatus
		by   order.By
		want error
	}{
		{"open → shipped", order.Shipped, staff, order.ErrInvalidTransition},
		{"customer fulfillment cancel", order.FCancelled, customer, order.ErrInvalidTransition},
		{"onno vendor", order.FCancelled, order.By{Actor: order.Vendor, VendorID: f.vnd + 1000}, order.ErrNotFound},
	} {
		if err := f.txFulfillment(fids[0], c.to, c.by, ""); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, %v chai", c.name, err, c.want)
		}
	}
	if s := f.fStatuses(f.id(`SELECT order_id FROM order_fulfillments WHERE id = $1`, fids[0])); s != "open" {
		t.Errorf("bhul bodol er por fulfillment %s", s)
	}
	// onno shop er order/fulfillment: not found
	other := f.id(`SELECT id FROM shops WHERE slug = 'b'`)
	err := db.InTx(f.ctx, f.d, func(tx *sql.Tx) error {
		return order.TransitionOrder(f.ctx, tx, other, cod, order.Cancelled, staff, "")
	})
	if !errors.Is(err, order.ErrNotFound) {
		t.Errorf("onno shop er order: %v", err)
	}

	// HTTP: ochena token 404, cancelled order abar cancel 409 invalid_transition, reason 200 er beshi 400
	wantErr(t, "ochena token", f.post("/api/orders/NOPE/cancel", "", "", "", map[string]any{}), http.StatusNotFound, "not_found")
	tok := f.str(`SELECT public_token FROM orders WHERE id = $1`, cod)
	long := make([]rune, 201)
	for i := range long {
		long[i] = 'ক'
	}
	wantErr(t, "lomba reason", f.post("/api/orders/"+tok+"/cancel", "", "", "", map[string]any{"reason": string(long)}),
		http.StatusBadRequest, "validation_failed")
	if rec := f.post("/api/orders/"+tok+"/cancel", "", "", "", map[string]any{}); rec.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, "abar cancel", f.post("/api/orders/"+tok+"/cancel", "", "", "", map[string]any{}), http.StatusConflict, "invalid_transition")
}

// TestCancelRestoresIntegration (14.2 test 8, 8.8, 8.12, 8.13): cancel e stock (movement cancel), batch, coupon
// ferot; prepaid e protiti fulfillment e thik ekta refund (bortoman total, 0 hole na), order level e na.
func TestCancelRestoresIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 20000, 10)
	dal := f.variant("dal", 15000, 10)
	cid := f.id(`INSERT INTO coupons (shop_id, code, kind, value) VALUES ($1, 'TAKA', 'fixed', 5000) RETURNING id`, f.shop)

	// customer cancel (checkout er asol order, HTTP)
	ck := f.guestCart([2]int64{rice, 2}, [2]int64{dal, 3})
	ph := "01811000077"
	b := orderBody(ph, f.quoteTotal(ck, "", "TAKA", "Dhaka", "Dhanmondi", ph))
	b["coupon_code"] = "TAKA"
	p := decodePlaced(t, f.place(idemKey(), ck, b), http.StatusCreated)
	if n := f.count(`SELECT used_count FROM coupons WHERE id = $1`, cid); n != 1 {
		t.Fatalf("checkout er por used_count %d", n)
	}
	rec := f.post("/api/orders/"+p.PublicToken+"/cancel", "", "", "", map[string]any{"reason": "bhul thikana"})
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	oid := f.id(`SELECT id FROM orders WHERE public_token = $1`, p.PublicToken)
	if s := f.orderStatus(oid); s != "cancelled/cancelled" {
		t.Errorf("order %s", s)
	}
	if r := f.str(`SELECT cancel_reason FROM orders WHERE id = $1 AND cancelled_at IS NOT NULL`, oid); r != "bhul thikana" {
		t.Errorf("cancel_reason %q", r)
	}
	if s := f.fStatuses(oid); s != "cancelled" {
		t.Errorf("fulfillment %s", s)
	}
	if n := f.count(`SELECT count(*) FROM order_fulfillments WHERE order_id = $1 AND cancelled_at IS NULL`, oid); n != 0 {
		t.Error("fulfillment cancelled_at nai")
	}
	for v, qty := range map[int64]int64{rice: 2, dal: 3} {
		if n := f.count(`SELECT stock FROM product_variants WHERE id = $1`, v); n != 10 {
			t.Errorf("variant %d stock %d, 10 chai", v, n)
		}
		if n := f.count(`SELECT COALESCE(sum(change), 0) FROM inventory_movements WHERE order_id = $1 AND variant_id = $2
			AND reason = 'cancel'`, oid, v); n != qty {
			t.Errorf("variant %d cancel movement %d, %d chai", v, n, qty)
		}
	}
	if n := f.count(`SELECT used_count FROM coupons WHERE id = $1`, cid); n != 0 {
		t.Errorf("cancel er por used_count %d", n)
	}
	if n := f.count(`SELECT count(*) FROM coupon_redemptions WHERE order_id = $1`, oid); n != 0 {
		t.Error("redemption muche nai")
	}
	if n := f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'cancelled'
		AND actor_type = 'customer' AND from_status IS NOT NULL`, oid); n != 2 {
		t.Errorf("cancel er history row %d, 2 chai (order + fulfillment)", n)
	}
	if n := f.count(`SELECT count(*) FROM refunds WHERE order_id = $1`, oid); n != 0 {
		t.Error("COD e refund hoyeche")
	}

	// staff packed cancel: batch e ferot (order_item_batches theke), movement e batch_id
	fish := f.variant("fish", 10000, 0)
	batch := f.id(`INSERT INTO inventory_batches (shop_id, variant_id, quantity_received, quantity_remaining)
		VALUES ($1, $2, 10, 7) RETURNING id`, f.shop, fish)
	packed, _ := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "packed", items: [][2]int64{{fish, 3}}})
	f.exec(`INSERT INTO order_item_batches (shop_id, order_item_id, batch_id, variant_id, quantity)
		SELECT $1, id, $2, $3, 3 FROM order_items WHERE order_id = $4`, f.shop, batch, fish, packed)
	if err := f.txOrder(packed, order.Cancelled, staff, "customer phone e mana"); err != nil {
		t.Fatalf("staff packed cancel: %v", err)
	}
	if n := f.count(`SELECT quantity_remaining FROM inventory_batches WHERE id = $1`, batch); n != 10 {
		t.Errorf("batch remaining %d, 10 chai", n)
	}
	if n := f.count(`SELECT stock FROM product_variants WHERE id = $1`, fish); n != 3 {
		t.Errorf("fish stock %d, 3 chai", n)
	}
	if n := f.count(`SELECT count(*) FROM inventory_movements WHERE order_id = $1 AND reason = 'cancel' AND batch_id = $2
		AND change = 3`, packed, batch); n != 1 {
		t.Errorf("batch er cancel movement %d", n)
	}

	// prepaid marketplace: protiti fulfillment e ekta refund (total), 0 total (shob item removed) e na, order level e na
	vb, vbv := f.vendorVariant("vb", 0)
	vc, vcv := f.vendorVariant("vc", 0)
	pre, fids := f.seedOrder("sslcommerz", "paid", "confirmed",
		seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 2}}},
		seedF{vendor: vb, status: "open", items: [][2]int64{{vbv, 3}}},
		seedF{vendor: vc, status: "open", items: [][2]int64{{vcv, 4}}, removed: true})
	if err := f.txOrder(pre, order.Cancelled, staff, ""); err != nil {
		t.Fatalf("prepaid cancel: %v", err)
	}
	rows, err := f.d.QueryContext(f.ctx, `SELECT fulfillment_id, amount, kind, method FROM refunds WHERE order_id = $1 ORDER BY fulfillment_id`, pre)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got [][2]int64
	for rows.Next() {
		var fid sql.NullInt64
		var amt int64
		var kind, method string
		if err := rows.Scan(&fid, &amt, &kind, &method); err != nil {
			t.Fatal(err)
		}
		if !fid.Valid || kind != "cancel" || method != "sslcommerz" {
			t.Errorf("refund row fulfillment %v kind %s method %s", fid, kind, method)
		}
		got = append(got, [2]int64{fid.Int64, amt})
	}
	if len(got) != 2 || got[0] != [2]int64{fids[0], 20000} || got[1] != [2]int64{fids[1], 30000} {
		t.Errorf("refund %v, %v", got, fids)
	}
	if n := f.count(`SELECT stock FROM product_variants WHERE id = $1`, vcv); n != 0 {
		t.Errorf("removed item er stock abar ferot: %d", n)
	}
	if n := f.count(`SELECT stock FROM product_variants WHERE id = $1`, vbv); n != 3 {
		t.Errorf("vb stock %d, 3 chai", n)
	}
}

// TestCancelCascadeIntegration (14.2 test 26, 8.8): customer cancel pending/confirmed (shob open), packed thakle na;
// staff cancel packed shoho; shob fulfillment cancel hole order o (system); vendor shudhu nijer.
func TestCancelCascadeIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 10000, 100)
	vb, vbv := f.vendorVariant("vb", 100)
	two := func(status, s1, s2 string) (int64, []int64) {
		return f.seedOrder("cod", "unpaid", status,
			seedF{vendor: f.vnd, status: s1, items: [][2]int64{{rice, 1}}},
			seedF{vendor: vb, status: s2, items: [][2]int64{{vbv, 1}}})
	}
	noLive := func(name string, oid int64) {
		t.Helper()
		if n := f.count(`SELECT count(*) FROM order_fulfillments WHERE order_id = $1 AND status IN ('open', 'packed')`, oid); n != 0 {
			t.Errorf("%s: %d ta open/packed baki", name, n)
		}
		if s := f.orderStatus(oid); s != "cancelled/cancelled" {
			t.Errorf("%s: order %s", name, s)
		}
	}

	for _, st := range []string{"pending", "confirmed"} {
		oid, _ := two(st, "open", "open")
		if err := f.txOrder(oid, order.Cancelled, customer, ""); err != nil {
			t.Fatalf("customer cancel %s: %v", st, err)
		}
		noLive("customer "+st, oid)
	}

	// customer packed thakle na; kichu bodlay na
	oid, _ := two("confirmed", "packed", "open")
	if err := f.txOrder(oid, order.Cancelled, customer, ""); !errors.Is(err, order.ErrInvalidTransition) {
		t.Errorf("customer packed cancel: %v", err)
	}
	if s := f.fStatuses(oid); s != "packed,open" {
		t.Errorf("bhul cancel er por %s", s)
	}
	// system confirmed order cancel korte pare na jodi fulfillment live (shudhu cascade)
	if err := f.txOrder(oid, order.Cancelled, system, ""); !errors.Is(err, order.ErrInvalidTransition) {
		t.Errorf("system live order cancel: %v", err)
	}
	// staff pare, packed shoho
	if err := f.txOrder(oid, order.Cancelled, staff, ""); err != nil {
		t.Fatalf("staff cancel: %v", err)
	}
	noLive("staff packed", oid)
	if n := f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND fulfillment_id IS NOT NULL
		AND to_status = 'cancelled' AND actor_type = 'staff'`, oid); n != 2 {
		t.Errorf("niche cascade er history %d, 2 chai (order er actor)", n)
	}

	// upore cascade: vendor nijer ta, tarpor staff shesh ta → order cancelled (system)
	oid, fids := two("confirmed", "open", "open")
	if err := f.txFulfillment(fids[0], order.FCancelled, order.By{Actor: order.Vendor, VendorID: vb}, ""); !errors.Is(err, order.ErrNotFound) {
		t.Errorf("vendor onno vendor er fulfillment: %v", err)
	}
	if err := f.txFulfillment(fids[1], order.FCancelled, order.By{Actor: order.Vendor, VendorID: vb}, ""); err != nil {
		t.Fatalf("vendor nijer cancel: %v", err)
	}
	if s := f.orderStatus(oid); s != "confirmed/confirmed" {
		t.Errorf("ek fulfillment cancel er por order %s", s)
	}
	if err := f.txFulfillment(fids[0], order.FCancelled, staff, ""); err != nil {
		t.Fatalf("shesh fulfillment cancel: %v", err)
	}
	noLive("upore cascade", oid)
	if n := f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND fulfillment_id IS NULL
		AND from_status = 'confirmed' AND to_status = 'cancelled' AND actor_type = 'system'`, oid); n != 1 {
		t.Errorf("upore cascade er order history %d", n)
	}
	if n := f.count(`SELECT stock FROM product_variants WHERE id = $1`, vbv); n != 100+4 {
		t.Errorf("vb stock %d (4 ta cancel e +1 kore)", n)
	}
}

// TestDisplayStatusIntegration (14.2 test 16): DB er display_status = Summary: checkout e confirmed/pending,
// cancel e cancelled, marketplace e mixed partial.
func TestDisplayStatusIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 10000, 100)
	for _, c := range []struct{ mode, want string }{{"risky", "confirmed"}, {"all", "pending"}} {
		f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,cod_confirmation}', to_jsonb($2::text)) WHERE shop_id = $1`,
			f.shop, c.mode)
		ck := f.guestCart([2]int64{rice, 3}) // Dhanmondi min_order 30000
		p := decodePlaced(t, f.place(idemKey(), ck, orderBody("01811000055", f.quoteTotal(ck, "", "", "Dhaka", "Dhanmondi", "01811000055"))),
			http.StatusCreated)
		oid := f.id(`SELECT id FROM orders WHERE public_token = $1`, p.PublicToken)
		if s := f.orderStatus(oid); s != c.want+"/"+c.want {
			t.Errorf("%s: checkout e %s", c.mode, s)
		}
		if rec := f.post("/api/orders/"+p.PublicToken+"/cancel", "", "", "", map[string]any{}); rec.Code != http.StatusOK {
			t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
		}
		if s := f.orderStatus(oid); s != "cancelled/cancelled" {
			t.Errorf("%s: cancel er por %s", c.mode, s)
		}
	}

	vb, vbv := f.vendorVariant("vb", 10)
	vc, vcv := f.vendorVariant("vc", 10)
	oid, fids := f.seedOrder("cod", "unpaid", "confirmed",
		seedF{vendor: f.vnd, status: "packed", items: [][2]int64{{rice, 1}}},
		seedF{vendor: vb, status: "open", items: [][2]int64{{vbv, 1}}},
		seedF{vendor: vc, status: "open", items: [][2]int64{{vcv, 1}}})
	for _, c := range []struct {
		fid  int64
		want string
	}{{fids[2], "partial"}, {fids[1], "packed"}} {
		if err := f.txFulfillment(c.fid, order.FCancelled, staff, ""); err != nil {
			t.Fatal(err)
		}
		if s := f.orderStatus(oid); s != "confirmed/"+c.want {
			t.Errorf("display %s, confirmed/%s chai", s, c.want)
		}
	}
}

// TestRestockExpiredBatchIntegration (11.7 item 3): cancel e pack er batch already expire (shop er timezone e)
// hole oi ongsho batch e ferot na, variant.stock e o na: movement cancel +qty ar expired −qty (net 0); expire
// na hoya batch ar batch chhara ongsho age er moto ferot. Undo (step 6) ar return (step 8) case pore.
func TestRestockExpiredBatchIntegration(t *testing.T) {
	f := newCheckout(t)
	fish := f.variant("fish", 10000, 5)
	batch := func(rem, daysAgo int) int64 {
		return f.id(`INSERT INTO inventory_batches (shop_id, variant_id, quantity_received, quantity_remaining, expiry_date)
			VALUES ($1, $2, 10, $3, (SELECT (now() AT TIME ZONE timezone)::date - $4::int FROM shops WHERE id = $1)) RETURNING id`,
			f.shop, fish, rem, daysAgo)
	}
	ok, exp := batch(7, 0), batch(0, 1) // ajker batch expire na; kalker ta expire (job 0 koreche)
	oid, _ := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "packed", items: [][2]int64{{fish, 6}}})
	f.exec(`INSERT INTO order_item_batches (shop_id, order_item_id, batch_id, variant_id, quantity)
		SELECT $1, i.id, b.id, $2, b.q FROM order_items i, (VALUES ($3::bigint, 2), ($4::bigint, 3)) b(id, q)
		WHERE i.order_id = $5`, f.shop, fish, ok, exp, oid)
	if err := f.txOrder(oid, order.Cancelled, staff, "mana"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name      string
		got, want int64
	}{
		{"ok batch", f.count(`SELECT quantity_remaining FROM inventory_batches WHERE id = $1`, ok), 9},
		{"expire batch", f.count(`SELECT quantity_remaining FROM inventory_batches WHERE id = $1`, exp), 0},
		{"stock (2 batch + 1 batch chhara)", f.count(`SELECT stock FROM product_variants WHERE id = $1`, fish), 8},
		{"expire batch cancel +3", f.count(`SELECT count(*) FROM inventory_movements WHERE order_id = $1 AND batch_id = $2
			AND reason = 'cancel' AND change = 3`, oid, exp), 1},
		{"expire batch expired −3", f.count(`SELECT count(*) FROM inventory_movements WHERE order_id = $1 AND batch_id = $2
			AND reason = 'expired' AND change = -3`, oid, exp), 1},
		{"movement net = stock er bodol", f.count(`SELECT sum(change) FROM inventory_movements WHERE order_id = $1`, oid), 3},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, chai %d", c.name, c.got, c.want)
		}
	}
}
