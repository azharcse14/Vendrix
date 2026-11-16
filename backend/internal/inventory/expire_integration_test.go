package inventory_test

import (
	"testing"
	"time"

	"vendrix/backend/internal/inventory"
	"vendrix/backend/internal/jobs"
)

// TestExpireBatchesIntegration (11.7 item 3, 8.12): inventory.expire shop er timezone er "aj" diye
// expiry_date < aj AND remaining > 0 er shob batch (ager din miss holeo) 0 kore, variant.stock - LEAST(qty, stock),
// movement expired (staff chhara); duibar chalaleo ekbar; batch_expiry off shop e kichu na. ScheduleExpire shudhu
// batch_expiry on shop e, shop er timezone e 00:10, dine ekta job.
func TestExpireBatchesIntegration(t *testing.T) {
	f := newFx(t)
	// UTC er tarikh theke alada din er timezone: UTC diye "aj" dhorle nicher kono ek batch bhul hoy
	tz := "Etc/GMT+12"
	if time.Now().UTC().Hour() >= 12 {
		tz = "Pacific/Kiritimati"
	}
	f.exec(`UPDATE shops SET timezone = $2 WHERE id IN ($1, $3)`, f.shop, tz, f.shopB)
	f.batchExpiry(true)
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{features}', '[]') WHERE shop_id = $1`, f.shopB)
	batch := func(shopID, v int64, qty, daysAgo int) int64 {
		t.Helper()
		return f.id(`INSERT INTO inventory_batches (shop_id, variant_id, quantity_received, quantity_remaining, expiry_date)
			VALUES ($1, $2, $3, $3, (now() AT TIME ZONE $5)::date - $4::int) RETURNING id`, shopID, v, qty, daysAgo, tz)
	}
	rice := f.variant(f.shop, f.house, "RICE-1", "চাল", "Rice", 10)
	missed, old, today := batch(f.shop, rice, 3, 3), batch(f.shop, rice, 4, 1), batch(f.shop, rice, 3, 0)
	honey := f.variant(f.shop, f.house, "HONEY-1", "মধু", "Honey", 2)
	short := batch(f.shop, honey, 5, 1) // stock 2: LEAST, 0 er niche na
	houseB := f.id(`SELECT id FROM vendors WHERE shop_id = $1 AND is_house`, f.shopB)
	teaB := f.variant(f.shopB, houseB, "TEA-B", "চা", "Tea", 4)
	offB := batch(f.shopB, teaB, 4, 1)

	run := inventory.ExpireJob(f.d)
	for range 2 {
		for _, s := range []int64{f.shop, f.shopB} {
			if err := run(f.ctx, jobs.Job{ShopID: s, Kind: "inventory.expire"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, c := range []struct {
		name string
		got  int64
		want int64
	}{
		{"miss kora batch", f.remaining(missed), 0},
		{"kal er batch", f.remaining(old), 0},
		{"ajker batch (expire na)", f.remaining(today), 3},
		{"rice stock", f.stock(rice), 3},
		{"short batch", f.remaining(short), 0},
		{"honey stock (LEAST)", f.stock(honey), 0},
		{"off shop batch", f.remaining(offB), 4},
		{"off shop stock", f.stock(teaB), 4},
		{"expired movement", f.id(`SELECT count(*) FROM inventory_movements WHERE reason = 'expired'`), 3},
		{"movement jog", f.id(`SELECT COALESCE(sum(change), 0) FROM inventory_movements WHERE reason = 'expired' AND staff_id IS NULL
			AND batch_id IN ($1, $2, $3)`, missed, old, short), -9},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, chai %d", c.name, c.got, c.want)
		}
	}

	for range 2 {
		if err := inventory.ScheduleExpire(f.ctx, f.d); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.id(`SELECT count(*) FROM jobs WHERE kind = 'inventory.expire' AND shop_id = $1
		AND run_at = ((now() AT TIME ZONE $2)::date + time '00:10') AT TIME ZONE $2`, f.shop, tz); n != 1 {
		t.Errorf("shop a er inventory.expire job %d, 1 chai (00:10 shop er timezone e)", n)
	}
	if n := f.id(`SELECT count(*) FROM jobs WHERE kind = 'inventory.expire' AND shop_id <> $1`, f.shop); n != 0 {
		t.Errorf("batch_expiry off shop e job %d", n)
	}
}
