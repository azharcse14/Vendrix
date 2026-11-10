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
