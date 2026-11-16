package inventory

import (
	"context"
	"database/sql"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/jobs"
)

// ScheduleExpire (8.17, 11.7 item 3): batch_expiry on protiti shop e shop er timezone er ajker 00:10 e
// inventory.expire; dedupe_key e shop ar shop er tarikh, tai worker er protiti slot e dakleo dine ekta.
func ScheduleExpire(ctx context.Context, d *sql.DB) error {
	_, err := d.ExecContext(ctx, `INSERT INTO jobs (shop_id, kind, payload, dedupe_key, run_at, max_attempts)
		SELECT s.id, 'inventory.expire', '{}', 'inventory.expire:' || s.id || ':' || to_char(t.d, 'YYYY-MM-DD'),
			(t.d + time '00:10') AT TIME ZONE s.timezone, 5
		FROM shops s JOIN shop_settings ss ON ss.shop_id = s.id
		CROSS JOIN LATERAL (SELECT (now() AT TIME ZONE s.timezone)::date d) t
		WHERE COALESCE(ss.published->'features' ? 'batch_expiry', false)
		ON CONFLICT (dedupe_key) DO NOTHING`)
	return err
}

// ExpireJob (inventory.expire, 11.7 item 3): shop er aj er age expire hoya remaining > 0 er shob batch (miss
// kora din o) 0, variant.stock - LEAST(qty, stock), movement expired. Variant protiti alada tx e: variant lock,
// tarpor batch id kram (pack/restock er moto, deadlock na). Abar chalale kichu pay na (idempotent).
func ExpireJob(d *sql.DB) jobs.Handler {
	return func(ctx context.Context, j jobs.Job) error {
		if on, err := batchExpiry(ctx, d, j.ShopID); err != nil || !on {
			return err
		}
		today, err := shopToday(ctx, d, j.ShopID)
		if err != nil {
			return err
		}
		rows, err := d.QueryContext(ctx, `SELECT DISTINCT variant_id FROM inventory_batches
			WHERE shop_id = $1 AND quantity_remaining > 0 AND expiry_date < $2::date ORDER BY variant_id`, j.ShopID, today)
		if err != nil {
			return err
		}
		var vids []int64
		for rows.Next() {
			var v int64
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return err
			}
			vids = append(vids, v)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, v := range vids {
			if err := db.InTx(ctx, d, func(tx *sql.Tx) error { return expireVariant(ctx, tx, j.ShopID, v, today) }); err != nil {
				return err
			}
		}
		return nil
	}
}

func expireVariant(ctx context.Context, tx *sql.Tx, shopID, variantID int64, today string) error {
	stock, err := lockVariant(ctx, tx, shopID, variantID, nil)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, quantity_remaining FROM inventory_batches
		WHERE shop_id = $1 AND variant_id = $2 AND quantity_remaining > 0 AND expiry_date < $3::date
		ORDER BY id FOR UPDATE`, shopID, variantID, today)
	if err != nil {
		return err
	}
	var parts []Part
	for rows.Next() {
		var p Part
		if err := rows.Scan(&p.BatchID, &p.Qty); err != nil {
			rows.Close()
			return err
		}
		parts = append(parts, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range parts {
		if err := takeBatch(ctx, tx, shopID, p.BatchID, p.Qty); err != nil {
			return err
		}
		// purono stock bhul hole batch er beshi kom thakte pare: 0 er niche na (8.12)
		take := min(p.Qty, stock)
		if take == 0 {
			continue
		}
		stock -= take
		if err := addStock(ctx, tx, shopID, variantID, -take); err != nil {
			return err
		}
		if err := movement(ctx, tx, shopID, variantID, p.BatchID, -take, "expired", 0, ""); err != nil {
			return err
		}
	}
	return nil
}
