package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
)

// textArg: nil Text = PATCH e bodlabe na (SQL e NULL → coalesce).
func textArg(t Text) any {
	if t == nil {
		return nil
	}
	return t
}

func createCategory(ctx context.Context, tx *sql.Tx, shopID int64, in categoryInput) (id int64, before, after json.RawMessage, err error) {
	active := in.IsActive == nil || *in.IsActive
	sort := 0
	if in.SortOrder != nil {
		sort = *in.SortOrder
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO categories (shop_id, parent_id, slug, name, image_key, sort_order, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, to_jsonb(categories.*) - 'shop_id'`,
		shopID, in.ParentID.V, *in.Slug, in.Name, in.ImageKey.V, sort, active).Scan(&id, &after)
	return id, nil, after, err
}

// lockCategoryParents: shop er shob category FOR UPDATE (duita PATCH ek shathe cycle banate na pare), id → parent.
func lockCategoryParents(ctx context.Context, tx *sql.Tx, shopID int64) (map[int64]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, coalesce(parent_id, 0) FROM categories WHERE shop_id = $1 FOR UPDATE`, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var id, p int64
		if err := rows.Scan(&id, &p); err != nil {
			return nil, err
		}
		out[id] = p
	}
	return out, rows.Err()
}

func updateCategory(ctx context.Context, tx *sql.Tx, shopID, id int64, in categoryInput) (int64, json.RawMessage, json.RawMessage, error) {
	var before, after json.RawMessage
	if err := tx.QueryRowContext(ctx, `SELECT to_jsonb(c) - 'shop_id' FROM categories c WHERE shop_id = $1 AND id = $2 FOR UPDATE`,
		shopID, id).Scan(&before); err != nil {
		return id, nil, nil, err
	}
	err := tx.QueryRowContext(ctx, `UPDATE categories SET
		parent_id = CASE WHEN $3 THEN $4::bigint ELSE parent_id END,
		slug = coalesce($5, slug), name = coalesce($6::jsonb, name),
		image_key = CASE WHEN $7 THEN $8::text ELSE image_key END,
		sort_order = coalesce($9, sort_order), is_active = coalesce($10, is_active)
		WHERE shop_id = $1 AND id = $2 RETURNING to_jsonb(categories.*) - 'shop_id'`,
		shopID, id, in.ParentID.Set, in.ParentID.V, in.Slug, textArg(in.Name), in.ImageKey.Set, in.ImageKey.V,
		in.SortOrder, in.IsActive).Scan(&after)
	return id, before, after, err
}

// deleteCategory: product ba child thakle FK error (23503).
func deleteCategory(ctx context.Context, tx *sql.Tx, shopID, id int64) (before json.RawMessage, err error) {
	err = tx.QueryRowContext(ctx, `DELETE FROM categories WHERE shop_id = $1 AND id = $2 RETURNING to_jsonb(categories.*) - 'shop_id'`,
		shopID, id).Scan(&before)
	return before, err
}

// adminProducts: shop er product (vendor != nil hole shudhu oi vendor er), notun age.
// ponytail: total count(*) OVER () theke (listProducts er moto).
func adminProducts(ctx context.Context, d *sql.DB, shopID int64, vendor any, limit, offset int) ([]json.RawMessage, int64, error) {
	rows, err := d.QueryContext(ctx, `SELECT to_jsonb(p) - 'shop_id' - 'search_text', count(*) OVER () FROM products p
		WHERE p.shop_id = $1 AND ($2::bigint IS NULL OR p.vendor_id = $2) ORDER BY p.created_at DESC, p.id DESC LIMIT $3 OFFSET $4`,
		shopID, vendor, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []json.RawMessage
	var total int64
	for rows.Next() {
		var p json.RawMessage
		if err := rows.Scan(&p, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func adminProduct(ctx context.Context, d *sql.DB, shopID, id int64, vendor any) (json.RawMessage, error) {
	var p json.RawMessage
	err := d.QueryRowContext(ctx, `SELECT (to_jsonb(p) - 'shop_id' - 'search_text') || jsonb_build_object(
		'variants', coalesce((SELECT jsonb_agg(to_jsonb(v) - 'shop_id' ORDER BY v.sort_order, v.id) FROM product_variants v
		  WHERE v.shop_id = p.shop_id AND v.product_id = p.id), '[]'),
		'images', coalesce((SELECT jsonb_agg(to_jsonb(i) - 'shop_id' ORDER BY i.sort_order, i.id) FROM product_images i
		  WHERE i.shop_id = p.shop_id AND i.product_id = p.id), '[]'))
		FROM products p WHERE p.shop_id = $1 AND p.id = $2 AND ($3::bigint IS NULL OR p.vendor_id = $3)`,
		shopID, id, vendor).Scan(&p)
	return p, err
}

// createProduct: vendor_id nil hole house vendor.
func createProduct(ctx context.Context, tx *sql.Tx, shopID int64, in productInput) (id int64, before, after json.RawMessage, err error) {
	err = tx.QueryRowContext(ctx, `INSERT INTO products (shop_id, vendor_id, category_id, slug, name, description, unit,
		variable_weight, attributes, search_keywords, seo, is_active)
		VALUES ($1, coalesce($2::bigint, (SELECT id FROM vendors WHERE shop_id = $1 AND is_house)), $3, $4, $5,
		  coalesce($6::jsonb, '{}'), coalesce($7, 'piece'), coalesce($8, false), coalesce($9::jsonb, '{}'),
		  coalesce($10, ''), coalesce($11::jsonb, '{}'), coalesce($12, true))
		RETURNING id, to_jsonb(products.*) - 'shop_id' - 'search_text'`,
		shopID, in.VendorID, in.CategoryID.V, *in.Slug, in.Name, textArg(in.Description), in.Unit, in.VariableWeight,
		rawArg(in.Attributes), in.SearchKeywords, rawArg(in.SEO), in.IsActive).Scan(&id, &after)
	return id, nil, after, err
}

func updateProduct(ctx context.Context, tx *sql.Tx, shopID, id int64, vendor any, in productInput) (int64, json.RawMessage, json.RawMessage, error) {
	var before, after json.RawMessage
	if err := tx.QueryRowContext(ctx, `SELECT to_jsonb(p) - 'shop_id' - 'search_text' FROM products p
		WHERE shop_id = $1 AND id = $2 AND ($3::bigint IS NULL OR vendor_id = $3) FOR UPDATE`, shopID, id, vendor).Scan(&before); err != nil {
		return id, nil, nil, err
	}
	err := tx.QueryRowContext(ctx, `UPDATE products SET
		category_id = CASE WHEN $3 THEN $4::bigint ELSE category_id END, slug = coalesce($5, slug),
		name = coalesce($6::jsonb, name), description = coalesce($7::jsonb, description), unit = coalesce($8, unit),
		variable_weight = coalesce($9, variable_weight), attributes = coalesce($10::jsonb, attributes),
		search_keywords = coalesce($11, search_keywords), seo = coalesce($12::jsonb, seo),
		is_active = coalesce($13, is_active), updated_at = now()
		WHERE shop_id = $1 AND id = $2 AND ($14::bigint IS NULL OR vendor_id = $14) RETURNING to_jsonb(products.*) - 'shop_id' - 'search_text'`,
		shopID, id, in.CategoryID.Set, in.CategoryID.V, in.Slug, textArg(in.Name), textArg(in.Description), in.Unit,
		in.VariableWeight, rawArg(in.Attributes), in.SearchKeywords, rawArg(in.SEO), in.IsActive, vendor).Scan(&after)
	return id, before, after, err
}

// createVariant: shop_id ar vendor_id product theke; product nai ba scope er baire → sql.ErrNoRows.
func createVariant(ctx context.Context, tx *sql.Tx, shopID, productID int64, vendor any, in variantInput) (id int64, before, after json.RawMessage, err error) {
	sort := 0
	if in.SortOrder != nil {
		sort = *in.SortOrder
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, options, price,
		compare_at_price, cost_price, nominal_weight_g, is_active, sort_order)
		SELECT p.shop_id, p.id, p.vendor_id, $4, coalesce($5::jsonb, '{}'), $6, $7, $8, $9, coalesce($10, true), $11
		FROM products p WHERE p.shop_id = $1 AND p.id = $2 AND ($3::bigint IS NULL OR p.vendor_id = $3)
		RETURNING id, to_jsonb(product_variants.*) - 'shop_id'`,
		shopID, productID, vendor, *in.SKU, optionsArg(in.Options), *in.Price, in.CompareAtPrice.V, in.CostPrice.V,
		in.NominalWeightG.V, in.IsActive, sort).Scan(&id, &after)
	return id, nil, after, err
}

func updateVariant(ctx context.Context, tx *sql.Tx, shopID, id int64, vendor any, in variantInput) (int64, json.RawMessage, json.RawMessage, error) {
	var before, after json.RawMessage
	if err := tx.QueryRowContext(ctx, `SELECT to_jsonb(v) - 'shop_id' FROM product_variants v
		WHERE shop_id = $1 AND id = $2 AND ($3::bigint IS NULL OR vendor_id = $3) FOR UPDATE`, shopID, id, vendor).Scan(&before); err != nil {
		return id, nil, nil, err
	}
	err := tx.QueryRowContext(ctx, `UPDATE product_variants SET sku = coalesce($3, sku), options = coalesce($4::jsonb, options),
		price = coalesce($5, price),
		compare_at_price = CASE WHEN $6 THEN $7::bigint ELSE compare_at_price END,
		cost_price = CASE WHEN $8 THEN $9::bigint ELSE cost_price END,
		nominal_weight_g = CASE WHEN $10 THEN $11::int ELSE nominal_weight_g END,
		is_active = coalesce($12, is_active), sort_order = coalesce($13, sort_order)
		WHERE shop_id = $1 AND id = $2 AND ($14::bigint IS NULL OR vendor_id = $14) RETURNING to_jsonb(product_variants.*) - 'shop_id'`,
		shopID, id, in.SKU, optionsArg(in.Options), in.Price, in.CompareAtPrice.Set, in.CompareAtPrice.V,
		in.CostPrice.Set, in.CostPrice.V, in.NominalWeightG.Set, in.NominalWeightG.V, in.IsActive, in.SortOrder, vendor).Scan(&after)
	return id, before, after, err
}
