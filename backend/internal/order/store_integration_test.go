package order

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/shop"
)

// TestTakeStockVendorIntegration (8.6 step 7, 10; 8.19 item 13 e): tx er bhitore vendor active na hole
// ErrVendorClosed (vendor_unavailable), stock komena; product bondho hole ager moto out_of_stock (ok false).
func TestTakeStockVendorIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	shopID, _, err := shop.Create(ctx, d, shop.Params{Slug: "a", Prefix: "SA", Domain: "a.com", NameBn: "দোকান",
		OwnerEmail: "o@a.com", OwnerName: "O", Preset: "organic"})
	if err != nil {
		t.Fatal(err)
	}
	var vnd, prod, v int64
	q := func(dst *int64, query string, args ...any) {
		t.Helper()
		if err := d.QueryRowContext(ctx, query, args...).Scan(dst); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	q(&vnd, `SELECT id FROM vendors WHERE shop_id = $1 AND is_house`, shopID)
	q(&prod, `INSERT INTO products (shop_id, vendor_id, slug, name) VALUES ($1, $2, 'rice', '{"bn":"চাল"}') RETURNING id`, shopID, vnd)
	q(&v, `INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock, options)
		VALUES ($1, $2, $3, 'rice', 50000, 5, '{}') RETURNING id`, shopID, prod, vnd)
	take := func() (bool, error) {
		var ok bool
		err := db.InTx(ctx, d, func(tx *sql.Tx) error {
			var err error
			_, ok, err = takeStock(ctx, tx, shopID, v, 1)
			if err == nil {
				err = errors.New("rollback") // stock bodlay na
			}
			return err
		})
		if err != nil && err.Error() == "rollback" {
			err = nil
		}
		return ok, err
	}
	if ok, err := take(); !ok || err != nil {
		t.Fatalf("active: %v %v", ok, err)
	}
	if _, err := d.ExecContext(ctx, `UPDATE vendors SET status = 'suspended' WHERE id = $1`, vnd); err != nil {
		t.Fatal(err)
	}
	if ok, err := take(); ok || !errors.Is(err, ErrVendorClosed) {
		t.Errorf("vendor bondho: ok %v err %v, ErrVendorClosed chai", ok, err)
	}
	for _, s := range []string{`UPDATE vendors SET status = 'active' WHERE id = $1`, `UPDATE products SET is_active = false WHERE id = $1`} {
		id := vnd
		if s[7] == 'p' {
			id = prod
		}
		if _, err := d.ExecContext(ctx, s, id); err != nil {
			t.Fatal(err)
		}
	}
	if ok, err := take(); ok || err != nil {
		t.Errorf("product bondho: ok %v err %v, out_of_stock (false, nil) chai", ok, err)
	}
	var stock int
	if err := d.QueryRowContext(ctx, `SELECT stock FROM product_variants WHERE id = $1`, v).Scan(&stock); err != nil || stock != 5 {
		t.Errorf("stock %d %v", stock, err)
	}
}
