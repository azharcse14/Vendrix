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
