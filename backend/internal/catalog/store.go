package catalog

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Scan: JSONB → Text.
func (t *Text) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, t)
	case string:
		return json.Unmarshal([]byte(v), t)
	}
	return fmt.Errorf("Text: %T", src)
}

// Value: Text → JSONB.
func (t Text) Value() (driver.Value, error) {
	if t == nil {
		return "{}", nil
	}
	b, err := json.Marshal(t)
	return string(b), err
}

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}

type categoryRow struct {
	ID        int64   `json:"id"`
	ParentID  *int64  `json:"parent_id"`
	Slug      string  `json:"slug"`
	Name      Text    `json:"name"`
	ImageKey  *string `json:"image_key"`
	SortOrder int     `json:"sort_order"`
	IsActive  bool    `json:"is_active"`
}

// categories: shop er category (activeOnly hole shudhu active), sort_order kram e.
func categories(ctx context.Context, q querier, shopID int64, activeOnly bool) ([]categoryRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, parent_id, slug, name, image_key, sort_order, is_active
		FROM categories WHERE shop_id = $1 AND (is_active OR NOT $2) ORDER BY sort_order, id`, shopID, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []categoryRow
	for rows.Next() {
		var c categoryRow
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Slug, &c.Name, &c.ImageKey, &c.SortOrder, &c.IsActive); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type listFilter struct {
	Category, Vendor   string
	MinPrice, MaxPrice *int64
	InStock            bool
	Like, Raw          string // searchTerm; khali = search na
	Sort               string // newest, price_asc, price_desc, popular, relevance
	Limit, Offset      int
}

type productCard struct {
	ID, RatingSum, ReviewCount int64
	Slug                       string
	Name                       Text
	Unit                       string
	VariableWeight, InStock    bool
	Price                      int64
	CompareAt                  *int64
	Image                      *string
}

func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// listProducts (7.2, 6.4): active product jar ontoto ekta active variant ache. total = filter er pore mot.
// ponytail: total count(*) OVER () theke, tai shesh page er porer page e total 0; dorkar hole alada COUNT.
func listProducts(ctx context.Context, q querier, shopID int64, f listFilter) ([]productCard, int64, error) {
	rows, err := q.QueryContext(ctx, `
SELECT p.id, p.slug, p.name, p.unit, p.variable_weight, p.rating_sum, p.review_count,
       v.price, v.compare_at, v.in_stock, img.storage_key, count(*) OVER ()
FROM products p
JOIN LATERAL (
  SELECT min(price) AS price, (array_agg(compare_at_price ORDER BY price, sort_order, id))[1] AS compare_at,
         bool_or(stock > 0) AS in_stock
  FROM product_variants WHERE shop_id = p.shop_id AND product_id = p.id AND is_active
) v ON v.price IS NOT NULL
LEFT JOIN LATERAL (
  SELECT storage_key FROM product_images WHERE shop_id = p.shop_id AND product_id = p.id
  ORDER BY sort_order, id LIMIT 1
) img ON true
WHERE p.shop_id = $1 AND p.is_active
  AND ($2::text IS NULL OR EXISTS (SELECT 1 FROM categories c
        WHERE c.shop_id = p.shop_id AND c.id = p.category_id AND c.slug = $2 AND c.is_active))
  AND ($3::text IS NULL OR EXISTS (SELECT 1 FROM vendors vd
        WHERE vd.shop_id = p.shop_id AND vd.id = p.vendor_id AND vd.slug = $3))
  AND EXISTS (SELECT 1 FROM product_variants pv WHERE pv.shop_id = p.shop_id AND pv.product_id = p.id
        AND pv.is_active AND ($4::bigint IS NULL OR pv.price >= $4) AND ($5::bigint IS NULL OR pv.price <= $5))
  AND (NOT $6 OR v.in_stock)
  AND ($7::text IS NULL OR p.search_text ILIKE '%' || normalize($7::text, NFC) || '%')
ORDER BY
  CASE WHEN $9 = 'relevance' THEN word_similarity(normalize($8::text, NFC), p.search_text) END DESC,
  CASE WHEN $9 = 'price_asc' THEN v.price END ASC,
  CASE WHEN $9 = 'price_desc' THEN v.price END DESC,
  CASE WHEN $9 = 'popular' THEN p.review_count END DESC,
  CASE WHEN $9 = 'popular' THEN p.rating_sum END DESC,
  p.created_at DESC, p.id DESC
LIMIT $10 OFFSET $11`,
		shopID, null(f.Category), null(f.Vendor), f.MinPrice, f.MaxPrice, f.InStock, null(f.Like), null(f.Raw),
		f.Sort, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []productCard
	var total int64
	for rows.Next() {
		var c productCard
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Unit, &c.VariableWeight, &c.RatingSum, &c.ReviewCount,
			&c.Price, &c.CompareAt, &c.InStock, &c.Image, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

type productDetail struct {
	ID, RatingSum, ReviewCount int64
	Slug, Unit                 string
	Name, Description          Text
	VariableWeight             bool
	Attributes, SEO            json.RawMessage
	Category                   *string
}

type variantRow struct {
	ID             int64
	SKU            string
	Options        map[string]Text
	Price          int64
	CompareAt      *int64
	NominalWeightG *int64
	InStock        bool
}

type imageRow struct {
	StorageKey    string
	Alt           Text
	Width, Height int
}

// productBySlug: active product (onno shop, inactive ba nai → sql.ErrNoRows), active variant ar chobi.
func productBySlug(ctx context.Context, q querier, shopID int64, slug string) (productDetail, []variantRow, []imageRow, error) {
	var p productDetail
	err := q.QueryRowContext(ctx, `SELECT p.id, p.slug, p.name, p.description, p.unit, p.variable_weight,
		p.attributes, p.seo, p.rating_sum, p.review_count, c.slug
		FROM products p LEFT JOIN categories c ON c.shop_id = p.shop_id AND c.id = p.category_id AND c.is_active
		WHERE p.shop_id = $1 AND p.slug = $2 AND p.is_active`, shopID, slug).
		Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.Unit, &p.VariableWeight, &p.Attributes, &p.SEO,
			&p.RatingSum, &p.ReviewCount, &p.Category)
	if err != nil {
		return p, nil, nil, err
	}
	rows, err := q.QueryContext(ctx, `SELECT id, sku, options, price, compare_at_price, nominal_weight_g, stock > 0
		FROM product_variants WHERE shop_id = $1 AND product_id = $2 AND is_active ORDER BY sort_order, id`, shopID, p.ID)
	if err != nil {
		return p, nil, nil, err
	}
	defer rows.Close()
	var vs []variantRow
	for rows.Next() {
		var v variantRow
		var opts []byte
		if err := rows.Scan(&v.ID, &v.SKU, &opts, &v.Price, &v.CompareAt, &v.NominalWeightG, &v.InStock); err != nil {
			return p, nil, nil, err
		}
		if err := json.Unmarshal(opts, &v.Options); err != nil {
			return p, nil, nil, err
		}
		vs = append(vs, v)
	}
	if err := rows.Err(); err != nil {
		return p, nil, nil, err
	}
	if len(vs) == 0 {
		return p, nil, nil, sql.ErrNoRows
	}
	irows, err := q.QueryContext(ctx, `SELECT storage_key, alt, width, height FROM product_images
		WHERE shop_id = $1 AND product_id = $2 ORDER BY sort_order, id`, shopID, p.ID)
	if err != nil {
		return p, nil, nil, err
	}
	defer irows.Close()
	var is []imageRow
	for irows.Next() {
		var i imageRow
		if err := irows.Scan(&i.StorageKey, &i.Alt, &i.Width, &i.Height); err != nil {
			return p, nil, nil, err
		}
		is = append(is, i)
	}
	return p, vs, is, irows.Err()
}
