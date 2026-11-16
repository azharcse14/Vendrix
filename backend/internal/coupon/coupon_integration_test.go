package coupon_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"vendrix/backend/internal/coupon"
	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/shop"
)

const (
	phoneX = "+8801711111111"
	phoneY = "+8801822222222"
)

type fixture struct {
	d       *sql.DB
	shopA   int64
	shopB   int64
	coupons map[string]int64 // code → id
}

// newFixture: duita shop; a te protiti obosthar coupon, b te "BONLY" (Rule 3).
func newFixture(t *testing.T) *fixture {
	t.Helper()
	d := dbtest.New(t)
	f := &fixture{d: d, coupons: map[string]int64{}}
	for _, s := range []string{"a", "b"} {
		id, _, err := shop.Create(context.Background(), d, shop.Params{Slug: s, Prefix: "S" + string(s[0]-32),
			Domain: s + ".com", NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		if s == "a" {
			f.shopA = id
		} else {
			f.shopB = id
		}
	}
	for _, c := range []struct {
		shop        int64
		code, kind  string
		value, min  int64
		maxDiscount any
		limit       any
		used        int
		perPhone    int
		starts, exp any // now() theke interval; nil = NULL
		active      bool
	}{
		{f.shopA, "SAVE10", "percent", 10, 50000, int64(10000), nil, 0, 1, nil, nil, true},
		{f.shopA, "Flat50", "fixed", 5000, 0, nil, int64(5), 4, 1, "-1 day", "1 day", true},
		{f.shopA, "ONCE", "fixed", 5000, 0, nil, int64(1), 1, 1, nil, nil, true},
		{f.shopA, "OFF", "fixed", 5000, 0, nil, nil, 0, 1, nil, nil, false},
		{f.shopA, "OFFFULL", "fixed", 5000, 0, nil, int64(1), 1, 1, nil, nil, false},
		{f.shopA, "LATER", "fixed", 5000, 0, nil, nil, 0, 1, "1 hour", nil, true},
		{f.shopA, "OLD", "fixed", 5000, 0, nil, nil, 0, 1, nil, "-1 second", true},
		{f.shopA, "PP1", "fixed", 5000, 0, nil, nil, 0, 1, nil, nil, true},
		{f.shopA, "PP2", "fixed", 5000, 0, nil, nil, 0, 2, nil, nil, true},
		{f.shopB, "BONLY", "fixed", 5000, 0, nil, nil, 0, 1, nil, nil, true},
	} {
		var id int64
		if err := d.QueryRow(`INSERT INTO coupons (shop_id, code, kind, value, min_order, max_discount, usage_limit,
			used_count, per_phone_limit, starts_at, expires_at, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now() + $10::interval, now() + $11::interval, $12) RETURNING id`,
			c.shop, c.code, c.kind, c.value, c.min, c.maxDiscount, c.limit, c.used, c.perPhone, c.starts, c.exp, c.active).Scan(&id); err != nil {
			t.Fatal(err)
		}
		f.coupons[c.code] = id
	}
	return f
}

// redeem: coupon shoho order ar redemption (checkout er moto), order id ferot.
func (f *fixture) redeem(t *testing.T, shopID int64, code, phone string) int64 {
	t.Helper()
	var orderID int64
	key := fmt.Sprintf("k%d-%s-%s", len(code), code, t.Name())
	if err := f.d.QueryRow(`INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash,
		customer_name, customer_phone, recipient_name, recipient_phone, ship_district, ship_area, ship_address,
		subtotal, total, coupon_id, coupon_code, coupon_kind, coupon_value, coupon_funded_by, payment_method)
		SELECT $1, 'N-' || gen_random_uuid(), gen_random_uuid()::text, gen_random_uuid()::text, '\x00', 'C', $3, 'C', $3,
			'Dhaka', 'Banani', 'Road 1', 100000, 100000, id, code, kind, value, funded_by, 'cod'
		FROM coupons WHERE shop_id = $1 AND code = $2 RETURNING id`, shopID, code, phone).Scan(&orderID); err != nil {
		t.Fatalf("order %s: %v", key, err)
	}
	if _, err := f.d.Exec(`INSERT INTO coupon_redemptions (shop_id, coupon_id, order_id, phone)
		SELECT shop_id, coupon_id, id, customer_phone FROM orders WHERE id = $1`, orderID); err != nil {
		t.Fatal(err)
	}
	return orderID
}

func (f *fixture) used(t *testing.T, code string) int {
	t.Helper()
	var n int
	if err := f.d.QueryRow(`SELECT used_count FROM coupons WHERE id = $1`, f.coupons[code]).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// use: checkout er moto transaction e Use; error hole ROLLBACK, na hole COMMIT.
func (f *fixture) use(t *testing.T, shopID int64, code, phone string) (coupon.Coupon, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.d.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	c, err := coupon.Use(ctx, tx, shopID, code, phone)
	if err != nil {
		tx.Rollback()
		return c, err
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return c, nil
}

// TestCouponUseIntegration (8.13, 6.4, 8.19 item 5): atomic used_count + 1 (limit, shomoy, bondho), tarpor
// phone er redemption count; 0 row e karon: nai/bondho/shomoy er baire → ErrInvalid, usage_limit shesh →
// ErrLimitReached; phone limit → ErrLimitReached ar ROLLBACK e used_count ferot. Release: redemption muche
// used_count - 1 (0 er niche na).
func TestCouponUseIntegration(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// shorto RETURNING theke (snapshot er jonno), code DB te jemon
	c, err := f.use(t, f.shopA, "save10", phoneX)
	want := coupon.Coupon{ID: f.coupons["SAVE10"], Code: "SAVE10", Kind: "percent", Value: 10, MinOrder: 50000,
		MaxDiscount: 10000, PerPhoneLimit: 1, FundedBy: "shop"}
	if err != nil || c != want {
		t.Fatalf("SAVE10: %+v %v\nchai %+v", c, err, want)
	}
	if n := f.used(t, "SAVE10"); n != 1 {
		t.Errorf("SAVE10 used_count %d, chai 1", n)
	}
	// limit er bhitore, shomoy er bhitore
	if c, err := f.use(t, f.shopA, "FLAT50", phoneX); err != nil || c.Code != "Flat50" {
		t.Errorf("FLAT50: %+v %v", c, err)
	}
	if n := f.used(t, "Flat50"); n != 5 {
		t.Errorf("Flat50 used_count %d, chai 5", n)
	}

	for _, c := range []struct {
		name string
		shop int64
		code string
		want error
	}{
		{"limit shesh", f.shopA, "ONCE", coupon.ErrLimitReached},
		{"limit ekhon shesh (5/5)", f.shopA, "FLAT50", coupon.ErrLimitReached},
		{"bondho", f.shopA, "OFF", coupon.ErrInvalid},
		{"bondho ar shesh: invalid age", f.shopA, "OFFFULL", coupon.ErrInvalid},
		{"shuru hoy ni", f.shopA, "LATER", coupon.ErrInvalid},
		{"meyad shesh", f.shopA, "OLD", coupon.ErrInvalid},
		{"nai", f.shopA, "NAI", coupon.ErrInvalid},
		{"khali", f.shopA, "", coupon.ErrInvalid},
		{"onno shop er code", f.shopA, "BONLY", coupon.ErrInvalid},
		{"a er code b te", f.shopB, "SAVE10", coupon.ErrInvalid},
	} {
		before := map[string]int{}
		for code := range f.coupons {
			before[code] = f.used(t, code)
		}
		if _, err := f.use(t, c.shop, c.code, phoneX); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, chai %v", c.name, err, c.want)
		}
		for code := range f.coupons {
			if n := f.used(t, code); n != before[code] {
				t.Errorf("%s: %s used_count %d → %d", c.name, code, before[code], n)
			}
		}
	}

	// per_phone_limit 1: X er ager redemption → X atke (ROLLBACK e used_count ferot), Y chole
	f.redeem(t, f.shopA, "PP1", phoneX)
	if _, err := f.use(t, f.shopA, "PP1", phoneX); !errors.Is(err, coupon.ErrLimitReached) {
		t.Errorf("PP1 X: %v, chai ErrLimitReached", err)
	}
	if n := f.used(t, "PP1"); n != 0 {
		t.Errorf("PP1 phone limit e used_count %d, chai 0 (ROLLBACK)", n)
	}
	if _, err := f.use(t, f.shopA, "PP1", phoneY); err != nil {
		t.Errorf("PP1 Y: %v", err)
	}
	// per_phone_limit 2: ekta redemption e aro ekbar chole, duita te atke
	f.redeem(t, f.shopA, "PP2", phoneX)
	if _, err := f.use(t, f.shopA, "PP2", phoneX); err != nil {
		t.Errorf("PP2 X (1/2): %v", err)
	}
	f.redeem(t, f.shopA, "PP2", phoneX)
	if _, err := f.use(t, f.shopA, "PP2", phoneX); !errors.Is(err, coupon.ErrLimitReached) {
		t.Errorf("PP2 X (2/2): %v, chai ErrLimitReached", err)
	}

	// Release: redemption muche used_count - 1; duibar kichu na; onno shop er ID diye kichu na
	tx, err := f.d.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	order := f.redeem(t, f.shopA, "SAVE10", phoneY)
	if _, err := f.d.Exec(`UPDATE coupons SET used_count = 3 WHERE id = $1`, f.coupons["SAVE10"]); err != nil {
		t.Fatal(err)
	}
	if err := coupon.Release(ctx, tx, f.shopB, order); err != nil {
		t.Fatal(err)
	}
	if err := coupon.Release(ctx, tx, f.shopA, order); err != nil {
		t.Fatal(err)
	}
	if err := coupon.Release(ctx, tx, f.shopA, order); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if n := f.used(t, "SAVE10"); n != 2 {
		t.Errorf("Release er por used_count %d, chai 2", n)
	}
	var left int
	f.d.QueryRow(`SELECT count(*) FROM coupon_redemptions WHERE order_id = $1`, order).Scan(&left)
	if left != 0 {
		t.Errorf("Release er por redemption %d ta", left)
	}
	// admin used_count 0 korle-o Release atke na (GREATEST)
	order = f.redeem(t, f.shopA, "PP2", phoneY)
	if _, err := f.d.Exec(`UPDATE coupons SET used_count = 0 WHERE id = $1`, f.coupons["PP2"]); err != nil {
		t.Fatal(err)
	}
	if err := coupon.Release(ctx, f.d, f.shopA, order); err != nil {
		t.Fatal(err)
	}
	if n := f.used(t, "PP2"); n != 0 {
		t.Errorf("GREATEST: used_count %d, chai 0", n)
	}
	// coupon chhara order (redemption nai): kichu na, error na
	if err := coupon.Release(ctx, f.d, f.shopA, order+1000); err != nil {
		t.Errorf("redemption chhara Release: %v", err)
	}
}

// TestCouponPeekIntegration (8.19 item 5, 13 b): quote er read-only check: Use er karon (per-phone chhara: quote e
// onno phone er coupon use jana jay na, v1.34; Use e TestCouponUseIntegration), used_count bodlay na.
func TestCouponPeekIntegration(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.redeem(t, f.shopA, "PP1", phoneX)

	for _, c := range []struct {
		name string
		shop int64
		code string
		want error
	}{
		{"chole", f.shopA, " save10", nil},
		{"limit er bhitore", f.shopA, "flat50", nil},
		{"limit shesh", f.shopA, "ONCE", coupon.ErrLimitReached},
		{"bondho", f.shopA, "OFF", coupon.ErrInvalid},
		{"bondho ar shesh", f.shopA, "OFFFULL", coupon.ErrInvalid},
		{"shuru hoy ni", f.shopA, "LATER", coupon.ErrInvalid},
		{"meyad shesh", f.shopA, "OLD", coupon.ErrInvalid},
		{"nai", f.shopA, "NAI", coupon.ErrInvalid},
		{"onno shop er code", f.shopA, "BONLY", coupon.ErrInvalid},
		{"phone limit shesh, quote e dekhay na", f.shopA, "PP1", nil},
	} {
		got, err := coupon.Peek(ctx, f.d, c.shop, c.code)
		if !errors.Is(err, c.want) || (c.want == nil && got.ID == 0) {
			t.Errorf("%s: %+v %v, chai %v", c.name, got, err, c.want)
		}
	}
	if c, _ := coupon.Peek(ctx, f.d, f.shopA, "SAVE10"); c.MaxDiscount != 10000 || c.Kind != "percent" {
		t.Errorf("Peek shorto: %+v", c)
	}
	for code, want := range map[string]int{"SAVE10": 0, "Flat50": 4, "PP1": 0, "ONCE": 1} {
		if n := f.used(t, code); n != want {
			t.Errorf("Peek er por %s used_count %d, chai %d", code, n, want)
		}
	}
}
