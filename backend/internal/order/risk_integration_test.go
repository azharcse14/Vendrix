package order_test

import (
	"context"
	"testing"

	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/order"
	"vendrix/backend/internal/shop"
)

// 8.19 item 3: flag er history query: ei shop, shesh 60 minute e phone ba ip, shob status, returned
// (customer ba recipient phone er ager order), ager order ache kina.
func TestRiskHistoryIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	shops := map[string]int64{}
	for _, s := range []string{"a", "b"} {
		id, _, err := shop.Create(ctx, d, shop.Params{Slug: s, Prefix: "S" + string(s[0]-32), Domain: s + ".com",
			NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		shops[s] = id
	}
	n := 0
	add := func(s, phone, ip, ago, status, fstatus string) {
		t.Helper()
		n++
		var id int64
		err := d.QueryRowContext(ctx, `INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash,
			customer_name, customer_phone, recipient_name, recipient_phone, ship_district, ship_area, ship_address,
			subtotal, total, payment_method, status, display_status, ip, placed_at)
			VALUES ($1, 'X-' || $2::int, 'tok' || $2::int, 'key' || $2::int, '\x00', 'C', $3, 'R', '+8801900000000', 'Dhaka', 'Mirpur', 'line',
			100, 100, 'cod', $4, $4, NULLIF($5, ''), now() - $6::interval) RETURNING id`,
			shops[s], n, phone, status, ip, ago).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		if fstatus != "" {
			if _, err := d.ExecContext(ctx, `INSERT INTO order_fulfillments (shop_id, order_id, vendor_id, seq, status, subtotal, total)
				SELECT $1, $2, id, 1, $3, 100, 100 FROM vendors WHERE shop_id = $1`, shops[s], id, fstatus); err != nil {
				t.Fatal(err)
			}
		}
	}
	const p1, p2, p3, p4 = "+8801711111111", "+8801822222222", "+8801633333333", "+8801544444444"
	add("a", p1, "1.1.1.1", "10 minutes", "cancelled", "cancelled") // cancelled o gona
	add("a", p2, "1.1.1.1", "20 minutes", "pending", "")            // onno phone, ekoi ip
	add("a", p1, "", "2 hours", "confirmed", "open")                // 60 minute er baire
	add("a", p3, "", "30 days", "confirmed", "returned")
	add("b", p4, "1.1.1.1", "5 minutes", "pending", "") // onno shop (Rule 3)
	add("b", p4, "", "1 day", "confirmed", "returned")  // onno shop

	cases := []struct {
		name                string
		cust, recipient, ip string
		want                order.RiskHistory
	}{
		{"phone ar ip", p1, p1, "1.1.1.1", order.RiskHistory{RecentOrders: 2, PriorOrders: true}},
		{"shudhu phone", p1, p1, "9.9.9.9", order.RiskHistory{RecentOrders: 1, PriorOrders: true}},
		{"ip na", p1, p1, "", order.RiskHistory{RecentOrders: 1, PriorOrders: true}},
		{"returned customer phone", p3, p3, "", order.RiskHistory{Returned: true, PriorOrders: true}},
		{"returned recipient phone", p4, p3, "", order.RiskHistory{Returned: true}},
		{"onno shop er data na", p4, p4, "1.1.1.1", order.RiskHistory{RecentOrders: 2}},
	}
	for _, c := range cases {
		got, err := order.LoadRiskHistory(ctx, d, shops["a"], c.cust, c.recipient, c.ip)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}
