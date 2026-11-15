package cart

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"vendrix/backend/internal/catalog"
)

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}

// Owner: kar cart. UserID != 0 hole account cart; na hole KeyHash (nil = key nai ba bhul format).
type Owner struct {
	UserID  int64
	KeyHash []byte
}

// findCart: lock chhara (GET). Na thakle 0.
func findCart(ctx context.Context, q querier, shopID int64, o Owner) (int64, error) {
	var id int64
	var err error
	switch {
	case o.UserID != 0:
		err = q.QueryRowContext(ctx, `SELECT id FROM carts WHERE shop_id = $1 AND user_id = $2`, shopID, o.UserID).Scan(&id)
	case o.KeyHash != nil:
		err = q.QueryRowContext(ctx, `SELECT id FROM carts WHERE shop_id = $1 AND session_key = $2 AND user_id IS NULL`,
			shopID, o.KeyHash).Scan(&id)
	default:
		return 0, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// touchCart: write er shuru te cart row lock + updated_at = now() (8.4 item 7, cart.cleanup er 30 din).
// Na thakle 0.
func touchCart(ctx context.Context, tx *sql.Tx, shopID int64, o Owner) (int64, error) {
	var id int64
	var err error
	switch {
	case o.UserID != 0:
		err = tx.QueryRowContext(ctx, `UPDATE carts SET updated_at = now() WHERE shop_id = $1 AND user_id = $2 RETURNING id`,
			shopID, o.UserID).Scan(&id)
	case o.KeyHash != nil:
		err = tx.QueryRowContext(ctx, `UPDATE carts SET updated_at = now()
			WHERE shop_id = $1 AND session_key = $2 AND user_id IS NULL RETURNING id`, shopID, o.KeyHash).Scan(&id)
	default:
		return 0, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// createCart: account cart (parallel add e ON CONFLICT, ekta-i row) ba notun key er guest cart.
func createCart(ctx context.Context, tx *sql.Tx, shopID, userID int64, keyHash []byte) (int64, error) {
	var id int64
	if userID != 0 {
		err := tx.QueryRowContext(ctx, `INSERT INTO carts (shop_id, user_id) VALUES ($1, $2)
			ON CONFLICT (shop_id, user_id) WHERE user_id IS NOT NULL DO UPDATE SET updated_at = now() RETURNING id`,
			shopID, userID).Scan(&id)
		return id, err
	}
	err := tx.QueryRowContext(ctx, `INSERT INTO carts (shop_id, session_key) VALUES ($1, $2) RETURNING id`,
		shopID, keyHash).Scan(&id)
	return id, err
}

type variantRow struct {
	Stock          int
	VariableWeight bool
	NominalWeightG *int
}

// activeVariant (8.4 item 4): shudhu oi shop er active product + variant, vendor active (Rule 13);
// na hole sql.ErrNoRows (404, public catalog e o nai).
func activeVariant(ctx context.Context, q querier, shopID, variantID int64) (variantRow, error) {
	var v variantRow
	err := q.QueryRowContext(ctx, `SELECT v.stock, p.variable_weight, v.nominal_weight_g
		FROM product_variants v
		JOIN products p ON p.shop_id = v.shop_id AND p.id = v.product_id
		JOIN vendors vd ON vd.shop_id = v.shop_id AND vd.id = v.vendor_id
		WHERE v.shop_id = $1 AND v.id = $2 AND v.is_active AND p.is_active AND vd.status = 'active'`,
		shopID, variantID).Scan(&v.Stock, &v.VariableWeight, &v.NominalWeightG)
	return v, err
}

// itemState: variant er qty (0 = nai) ar cart er line shonkhya.
func itemState(ctx context.Context, q querier, shopID, cartID, variantID int64) (qty, lines int, err error) {
	err = q.QueryRowContext(ctx, `SELECT coalesce(max(quantity) FILTER (WHERE variant_id = $3), 0), count(*)
		FROM cart_items WHERE shop_id = $1 AND cart_id = $2`, shopID, cartID, variantID).Scan(&qty, &lines)
	return qty, lines, err
}

func setItem(ctx context.Context, q querier, shopID, cartID, variantID int64, qty int) error {
	_, err := q.ExecContext(ctx, `INSERT INTO cart_items (shop_id, cart_id, variant_id, quantity) VALUES ($1, $2, $3, $4)
		ON CONFLICT (cart_id, variant_id) DO UPDATE SET quantity = EXCLUDED.quantity`, shopID, cartID, variantID, qty)
	return err
}

func deleteItem(ctx context.Context, q querier, shopID, cartID, variantID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM cart_items WHERE shop_id = $1 AND cart_id = $2 AND variant_id = $3`,
		shopID, cartID, variantID)
	return err
}

// removeInactive (8.4 item 4): bondho product/variant ba active na vendor er item ek query te muche, naam ferot.
func removeInactive(ctx context.Context, q querier, shopID, cartID int64) ([]catalog.Text, error) {
	rows, err := q.QueryContext(ctx, `DELETE FROM cart_items ci
		USING product_variants v, products p, vendors vd
		WHERE ci.shop_id = $1 AND ci.cart_id = $2
		  AND v.shop_id = ci.shop_id AND v.id = ci.variant_id
		  AND p.shop_id = v.shop_id AND p.id = v.product_id
		  AND vd.shop_id = v.shop_id AND vd.id = v.vendor_id
		  AND NOT (v.is_active AND p.is_active AND vd.status = 'active')
		RETURNING p.name`, shopID, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []catalog.Text
	for rows.Next() {
		var n catalog.Text
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// reduceItem: normalize (GET); shudhu qty beshi thakle, jate parallel PATCH er kom qty bare na.
func reduceItem(ctx context.Context, q querier, shopID, cartID, variantID int64, qty int) error {
	_, err := q.ExecContext(ctx, `UPDATE cart_items SET quantity = $4
		WHERE shop_id = $1 AND cart_id = $2 AND variant_id = $3 AND quantity > $4`, shopID, cartID, variantID, qty)
	return err
}

type itemRow struct {
	VariantID      int64
	VendorID       int64
	Quantity       int
	ProductSlug    string
	Name           catalog.Text
	Options        map[string]catalog.Text
	Image          *string
	Price          int64
	CompareAt      *int64
	VariableWeight bool
	NominalWeightG *int
	Stock          int
}

// items: cart er line, bortoman dam ar stock shoho (8.4: dam save hoy na). Prothom add age.
func items(ctx context.Context, q querier, shopID, cartID int64) ([]itemRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT ci.variant_id, v.vendor_id, ci.quantity, p.slug, p.name, v.options,
		  (SELECT storage_key FROM product_images i WHERE i.shop_id = p.shop_id AND i.product_id = p.id
		   ORDER BY sort_order, id LIMIT 1),
		  v.price, v.compare_at_price, p.variable_weight, v.nominal_weight_g, v.stock
		FROM cart_items ci
		JOIN product_variants v ON v.shop_id = ci.shop_id AND v.id = ci.variant_id
		JOIN products p ON p.shop_id = v.shop_id AND p.id = v.product_id
		WHERE ci.shop_id = $1 AND ci.cart_id = $2
		ORDER BY ci.added_at, ci.variant_id`, shopID, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []itemRow
	for rows.Next() {
		var it itemRow
		var opts []byte
		if err := rows.Scan(&it.VariantID, &it.VendorID, &it.Quantity, &it.ProductSlug, &it.Name, &opts, &it.Image,
			&it.Price, &it.CompareAt, &it.VariableWeight, &it.NominalWeightG, &it.Stock); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(opts, &it.Options); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// lockGuestCart (merge): guest cart row FOR UPDATE; na thakle (ba parallel merge age muche thakle) 0.
// Merge e age guest, tarpor account lock: shob merge ek kram e, tai deadlock na.
func lockGuestCart(ctx context.Context, tx *sql.Tx, shopID int64, keyHash []byte) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM carts WHERE shop_id = $1 AND session_key = $2 AND user_id IS NULL
		FOR UPDATE`, shopID, keyHash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

type guestRow struct {
	VariantID int64
	Quantity  int
	Stock     int
	Active    bool
	Name      catalog.Text
}

// guestItems (merge): guest cart er line guest er add er kram e (item_dropped e kon ta thakbe), active ar stock shoho.
func guestItems(ctx context.Context, tx *sql.Tx, shopID, cartID int64) ([]guestRow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT ci.variant_id, ci.quantity, v.stock,
		  v.is_active AND p.is_active AND vd.status = 'active', p.name
		FROM cart_items ci
		JOIN product_variants v ON v.shop_id = ci.shop_id AND v.id = ci.variant_id
		JOIN products p ON p.shop_id = v.shop_id AND p.id = v.product_id
		JOIN vendors vd ON vd.shop_id = v.shop_id AND vd.id = v.vendor_id
		WHERE ci.shop_id = $1 AND ci.cart_id = $2
		ORDER BY ci.added_at, ci.variant_id`, shopID, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []guestRow
	for rows.Next() {
		var g guestRow
		if err := rows.Scan(&g.VariantID, &g.Quantity, &g.Stock, &g.Active, &g.Name); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// quantities: cart er variant → qty.
func quantities(ctx context.Context, tx *sql.Tx, shopID, cartID int64) (map[int64]int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT variant_id, quantity FROM cart_items WHERE shop_id = $1 AND cart_id = $2`,
		shopID, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var v int64
		var q int
		if err := rows.Scan(&v, &q); err != nil {
			return nil, err
		}
		out[v] = q
	}
	return out, rows.Err()
}

func deleteCart(ctx context.Context, tx *sql.Tx, shopID, cartID int64) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM carts WHERE shop_id = $1 AND id = $2`, shopID, cartID)
	return err
}
