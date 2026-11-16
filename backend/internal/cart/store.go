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

// countLines (GET /api/cart/count): cart er alada line, shudhu SELECT (row banay na, bondho item o gone).
func countLines(ctx context.Context, q querier, shopID int64, o Owner) (int, error) {
	cartID, err := findCart(ctx, q, shopID, o)
	if err != nil || cartID == 0 {
		return 0, err
	}
	var n int
	err = q.QueryRowContext(ctx, `SELECT count(*) FROM cart_items WHERE shop_id = $1 AND cart_id = $2`, shopID, cartID).Scan(&n)
	return n, err
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

// deleteStaleGuestCarts (8.17 v1.28): 30 din bodol na hoya guest cart, item CASCADE e. Shop filter nai: platform
// er job (shop_id NULL), shob shop ek DELETE e; carts_updated_idx o shop chhara.
func deleteStaleGuestCarts(ctx context.Context, d *sql.DB) error {
	_, err := d.ExecContext(ctx, `DELETE FROM carts WHERE user_id IS NULL AND updated_at < now() - interval '30 days'`)
	return err
}

type wishCard struct {
	ID, RatingSum, ReviewCount int64
	Slug, Unit                 string
	Name                       catalog.Text
	VariableWeight, InStock    bool
	Price                      int64
	CompareAt                  *int64
	Image                      *string
}

// wishCards (8.4 item 8): user er wishlist er card, catalog er card er moto; bondho product, active variant
// nai ba vendor active na hole lukano (row thake). Notun age, max 200.
func wishCards(ctx context.Context, q querier, shopID, userID int64) ([]wishCard, error) {
	rows, err := q.QueryContext(ctx, `SELECT p.id, p.slug, p.name, p.unit, p.variable_weight, p.rating_sum,
		  p.review_count, v.price, v.compare_at, v.in_stock,
		  (SELECT storage_key FROM product_images i WHERE i.shop_id = p.shop_id AND i.product_id = p.id
		   ORDER BY sort_order, id LIMIT 1)
		FROM wishlists w
		JOIN products p ON p.shop_id = w.shop_id AND p.id = w.product_id
		JOIN vendors vd ON vd.shop_id = p.shop_id AND vd.id = p.vendor_id
		JOIN LATERAL (
		  SELECT min(price) AS price, (array_agg(compare_at_price ORDER BY price, sort_order, id))[1] AS compare_at,
		         bool_or(stock > 0) AS in_stock
		  FROM product_variants WHERE shop_id = p.shop_id AND product_id = p.id AND is_active
		) v ON v.price IS NOT NULL
		WHERE w.shop_id = $1 AND w.user_id = $2 AND p.is_active AND vd.status = 'active'
		ORDER BY w.created_at DESC, w.product_id DESC
		LIMIT $3`, shopID, userID, MaxWishlist)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []wishCard
	for rows.Next() {
		var c wishCard
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Unit, &c.VariableWeight, &c.RatingSum, &c.ReviewCount,
			&c.Price, &c.CompareAt, &c.InStock, &c.Image); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// lockWishlist: user row lock (FOR NO KEY UPDATE, onno table er FK atkay na), jate parallel add/merge e
// 200 er limit na bhange. User er wishlist er product ID ferot.
func lockWishlist(ctx context.Context, tx *sql.Tx, shopID, userID int64) (map[int64]bool, error) {
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM users WHERE shop_id = $1 AND id = $2 FOR NO KEY UPDATE`,
		shopID, userID); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT product_id FROM wishlists WHERE shop_id = $1 AND user_id = $2`,
		shopID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	have := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		have[id] = true
	}
	return have, rows.Err()
}

// activeProducts: ids er moddhe oi shop er active product (active variant ache, vendor active; card e dekhay).
func activeProducts(ctx context.Context, q querier, shopID int64, ids []int64) (map[int64]bool, error) {
	rows, err := q.QueryContext(ctx, `SELECT p.id FROM products p
		JOIN vendors vd ON vd.shop_id = p.shop_id AND vd.id = p.vendor_id
		WHERE p.shop_id = $1 AND p.id = ANY($2) AND p.is_active AND vd.status = 'active'
		  AND EXISTS (SELECT 1 FROM product_variants v WHERE v.shop_id = p.shop_id AND v.product_id = p.id AND v.is_active)`,
		shopID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func insertWish(ctx context.Context, tx *sql.Tx, shopID, userID, productID int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO wishlists (shop_id, user_id, product_id) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, shopID, userID, productID)
	return err
}

func deleteWish(ctx context.Context, q querier, shopID, userID, productID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM wishlists WHERE shop_id = $1 AND user_id = $2 AND product_id = $3`,
		shopID, userID, productID)
	return err
}

type checkoutRow struct {
	itemRow
	Active, VendorActive bool
}

// checkoutRows: cart er line, active ar vendor er obostha shoho; shudhu SELECT (quote kichu bodlay na).
func checkoutRows(ctx context.Context, q querier, shopID, cartID int64) ([]checkoutRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT ci.variant_id, v.vendor_id, ci.quantity, v.price, p.variable_weight,
		  v.nominal_weight_g, v.stock, v.is_active AND p.is_active, vd.status = 'active'
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
	var out []checkoutRow
	for rows.Next() {
		var c checkoutRow
		if err := rows.Scan(&c.VariantID, &c.VendorID, &c.Quantity, &c.Price, &c.VariableWeight, &c.NominalWeightG,
			&c.Stock, &c.Active, &c.VendorActive); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
