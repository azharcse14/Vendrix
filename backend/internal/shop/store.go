package shop

import (
	"context"
	"database/sql"
)

// publishedSettings: shop er published settings ar version.
func publishedSettings(ctx context.Context, d *sql.DB, shopID int64) (published []byte, version int, err error) {
	err = d.QueryRowContext(ctx, `SELECT published, version FROM shop_settings WHERE shop_id = $1`, shopID).
		Scan(&published, &version)
	return published, version, err
}

// translations: shop er ek locale er admin override (9.8).
func translations(ctx context.Context, d *sql.DB, shopID int64, locale string) (map[string]string, error) {
	rows, err := d.QueryContext(ctx, `SELECT key, value FROM translations WHERE shop_id = $1 AND locale = $2`, shopID, locale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
