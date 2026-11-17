package order_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"vendrix/backend/internal/httpx"
)

// dashboard: GET /api/admin/dashboard er count gulo "key=value" (date alada).
func (f *checkoutFx) dashboard(st httpx.Staff) (string, map[string]json.Number) {
	f.t.Helper()
	m := f.admJSON(st, "GET", "/api/admin/dashboard", nil, http.StatusOK)
	out := map[string]json.Number{}
	for k, v := range m {
		if k != "date" {
			out[k] = v.(json.Number)
		}
	}
	d, _ := m["date"].(string)
	return d, out
}

func wantCounts(t *testing.T, what string, got map[string]json.Number, want map[string]int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: key %v, chai %v", what, got, want)
	}
	for k, w := range want {
		if got[k] != num(w) {
			t.Errorf("%s: %s = %s, chai %d", what, k, got[k], w)
		}
	}
}

// TestDashboardIntegration (11.7 item 13 viii, v1.46, Rule 5): shop er timezone (UTC theke +14) e ajker order
// ar bikri (cancelled ar online unpaid bikri te na), COD pending (je kono din), pack baki (confirmed er open
// fulfillment, je kono din), low stock (stock <= low_stock_at), expiry kache (inventory.expiry_alert_days,
// batch_expiry off e 0), deadline kache (order list tab er moto); shob shop role dekhe.
func TestDashboardIntegration(t *testing.T) {
	f := newCheckout(t)
	f.exec(`UPDATE shops SET timezone = 'Pacific/Kiritimati' WHERE id = $1`, f.shop)
	lo := f.variant("lo", 10000, 3)
	eq := f.variant("eq", 10000, 5)
	hi := f.variant("hi", 10000, 50)
	one := func(v int64, st string) seedF { return seedF{vendor: f.vnd, status: st, items: [][2]int64{{v, 1}}} }
	// shop er ajker shuru (local midnight) theke minute
	at := func(oid int64, min int) {
		f.exec(`UPDATE orders SET placed_at = (now() AT TIME ZONE 'Pacific/Kiritimati')::date::timestamp AT TIME ZONE 'Pacific/Kiritimati'
			+ make_interval(mins => $2) WHERE id = $1`, oid, min)
	}
	o1, _ := f.seedOrder("cod", "unpaid", "pending", one(hi, "open"))
	at(o1, 1)
	o2, _ := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{hi, 2}}})
	at(o2, 2)
	f.exec(`UPDATE orders SET delivery_deadline = now() + interval '1 hour' WHERE id = $1`, o2)
	o3, _ := f.seedOrder("cod", "unpaid", "cancelled", one(hi, "cancelled"))
	at(o3, 3)
	f.exec(`UPDATE orders SET delivery_deadline = now() + interval '1 hour' WHERE id = $1`, o3)
	o4, _ := f.seedOrder("sslcommerz", "unpaid", "pending", one(hi, "open")) // online unpaid: order e, bikri te na
	at(o4, 4)
	o5, _ := f.seedOrder("cod", "unpaid", "confirmed", one(hi, "open")) // kal rat: aj na, pack baki
	at(o5, -1)
	o6, _ := f.seedOrder("cod", "unpaid", "pending", one(hi, "open")) // kal: pending e
	f.exec(`UPDATE orders SET placed_at = now() - interval '2 days' WHERE id = $1`, o6)
	o8, _ := f.seedOrder("sslcommerz", "paid", "confirmed", one(hi, "packed"))
	at(o8, 5)
	o9, _ := f.seedOrder("cod", "unpaid", "confirmed", one(hi, "delivered")) // shesh: deadline gele o na
	f.exec(`UPDATE orders SET delivery_deadline = now() - interval '1 hour', placed_at = now() - interval '3 days' WHERE id = $1`, o9)

	f.batchExpiry(true)
	f.batchAt(lo, 2, 0, 1)                                                                              // aj expire
	f.batchAt(eq, 2, 7, 1)                                                                              // aj + 7
	f.batchAt(eq, 2, 8, 1)                                                                              // aj + 8: 7 din er baire
	f.batchAt(hi, 2, -1, 9)                                                                             // already expire (job chaleni): na
	f.batchAt(hi, 2, nil, 1)                                                                            // expiry nai
	f.exec(`UPDATE inventory_batches SET quantity_remaining = 0 WHERE id = $1`, f.batchAt(hi, 2, 1, 1)) // shesh batch

	want := map[string]int64{"orders": 4, "sales": 40000, "pending": 2, "to_pack": 2, "low_stock": 2, "expiring": 2, "deadline_soon": 1}
	today := f.str(`SELECT to_char((now() AT TIME ZONE 'Pacific/Kiritimati')::date, 'YYYY-MM-DD')`)
	for _, role := range []string{"owner", "admin", "staff"} {
		d, got := f.dashboard(f.staffAs(role, 0))
		if d != today {
			t.Errorf("%s date %q, chai %q", role, d, today)
		}
		wantCounts(t, role, got, want)
	}
	owner := f.staffAs("owner", 0)

	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{inventory,expiry_alert_days}', '8') WHERE shop_id = $1`, f.shop)
	_, got := f.dashboard(owner)
	want["expiring"] = 3
	wantCounts(t, "alert 8 din", got, want)

	f.batchExpiry(false)
	_, got = f.dashboard(owner)
	want["expiring"] = 0
	wantCounts(t, "batch_expiry off", got, want)

	// onno shop er order dekhe na
	b := f.id(`SELECT id FROM shops WHERE slug = 'b'`)
	ob := httpx.Staff{ID: f.id(`SELECT id FROM staff WHERE shop_id = $1 AND role = 'owner'`, b), ShopID: b, Role: "owner"}
	_, got = f.dashboard(ob)
	wantCounts(t, "shop b", got, map[string]int64{"orders": 0, "sales": 0, "pending": 0, "to_pack": 0, "low_stock": 0,
		"expiring": 0, "deadline_soon": 0})
}
