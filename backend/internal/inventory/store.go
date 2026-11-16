package inventory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Batch: inventory_batches er row (tarikh YYYY-MM-DD). CostPrice staff/vendor_staff e nil (11.7 item 1).
type Batch struct {
	ID                int64     `json:"id"`
	VariantID         int64     `json:"variant_id"`
	BatchCode         *string   `json:"batch_code"`
	QuantityReceived  int       `json:"quantity_received"`
	QuantityRemaining int       `json:"quantity_remaining"`
	CostPrice         *int64    `json:"cost_price,omitempty"`
	HarvestDate       *string   `json:"harvest_date"`
	ExpiryDate        *string   `json:"expiry_date"`
	ReceivedAt        time.Time `json:"received_at"`
}

func (b *Batch) scan(s interface{ Scan(...any) error }) error {
	return s.Scan(&b.ID, &b.VariantID, &b.BatchCode, &b.QuantityReceived, &b.QuantityRemaining, &b.CostPrice,
		&b.HarvestDate, &b.ExpiryDate, &b.ReceivedAt)
}

// lockVariant: variant row lock (shob inventory kaj er prothom lock), vendor scope (nil = shob); nai hole ErrNotFound.
func lockVariant(ctx context.Context, tx *sql.Tx, shopID, variantID int64, vendor any) (int, error) {
	var stock int
	err := tx.QueryRowContext(ctx, `SELECT stock FROM product_variants
		WHERE shop_id = $1 AND id = $2 AND ($3::bigint IS NULL OR vendor_id = $3) FOR UPDATE`, shopID, variantID, vendor).Scan(&stock)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return stock, err
}

// BatchExpiry (9.2): shop er published features e batch_expiry.
func BatchExpiry(ctx context.Context, q querier, shopID int64) (bool, error) {
	var on bool
	err := q.QueryRowContext(ctx, `SELECT COALESCE(published->'features' ? 'batch_expiry', false) FROM shop_settings
		WHERE shop_id = $1`, shopID).Scan(&on)
	return on, err
}

// shopToday: shop er timezone e ajker tarikh (Rule 5), YYYY-MM-DD.
func shopToday(ctx context.Context, q querier, shopID int64) (string, error) {
	var d string
	err := q.QueryRowContext(ctx, `SELECT to_char((now() AT TIME ZONE timezone)::date, 'YYYY-MM-DD') FROM shops WHERE id = $1`,
		shopID).Scan(&d)
	return d, err
}

func insertBatch(ctx context.Context, tx *sql.Tx, shopID int64, in BatchInput) (Batch, error) {
	var b Batch
	err := b.scan(tx.QueryRowContext(ctx, `INSERT INTO inventory_batches (shop_id, variant_id, batch_code, quantity_received,
		quantity_remaining, cost_price, harvest_date, expiry_date) VALUES ($1, $2, $3, $4, $4, $5, $6::date, $7::date)
		RETURNING id, variant_id, batch_code, quantity_received, quantity_remaining, cost_price,
	to_char(harvest_date, 'YYYY-MM-DD'), to_char(expiry_date, 'YYYY-MM-DD'), received_at`, shopID, in.VariantID, in.BatchCode, in.Quantity, in.CostPrice, in.HarvestDate, in.ExpiryDate))
	return b, err
}

func addStock(ctx context.Context, tx *sql.Tx, shopID, variantID int64, change int) error {
	_, err := tx.ExecContext(ctx, `UPDATE product_variants SET stock = stock + $3 WHERE shop_id = $1 AND id = $2`,
		shopID, variantID, change)
	return err
}

// movement: inventory_movements e row (batchID 0 = batch chhara, staffID 0 = job).
func movement(ctx context.Context, tx *sql.Tx, shopID, variantID, batchID int64, change int, reason string, staffID int64, note string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO inventory_movements (shop_id, variant_id, batch_id, change, reason, staff_id, note)
		VALUES ($1, $2, NULLIF($3, 0), $4, $5, NULLIF($6, 0), NULLIF($7, ''))`, shopID, variantID, batchID, change, reason, staffID, note)
	return err
}

// lockBatch: variant er ekta batch lock (variant er lock er por); nai ba onno variant er hole ErrNotFound.
func lockBatch(ctx context.Context, tx *sql.Tx, shopID, variantID, batchID int64) (int, error) {
	var rem int
	err := tx.QueryRowContext(ctx, `SELECT quantity_remaining FROM inventory_batches
		WHERE shop_id = $1 AND variant_id = $2 AND id = $3 FOR UPDATE`, shopID, variantID, batchID).Scan(&rem)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return rem, err
}

func takeBatch(ctx context.Context, tx *sql.Tx, shopID, batchID int64, qty int) error {
	_, err := tx.ExecContext(ctx, `UPDATE inventory_batches SET quantity_remaining = quantity_remaining - $3
		WHERE shop_id = $1 AND id = $2`, shopID, batchID, qty)
	return err
}

// Part: ek batch theke koto.
type Part struct {
	BatchID int64
	Qty     int
}

// Allocate (6.4 FIFO): variant lock er por, batch age id kram e lock (deadlock na, 11.7 item 2), tarpor expiry age,
// NULLS LAST, received_at kram e qty porjonto komay; expire batch (expiry < shop er aj) bad. rest = batch e na pawa
// ongsho (caller thik kore: adjust e purono stock, pack e out_of_stock).
func Allocate(ctx context.Context, tx *sql.Tx, shopID, variantID int64, qty int) (parts []Part, rest int, err error) {
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM inventory_batches WHERE shop_id = $1 AND variant_id = $2
		AND quantity_remaining > 0 ORDER BY id FOR UPDATE`, shopID, variantID); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, quantity_remaining FROM inventory_batches
		WHERE shop_id = $1 AND variant_id = $2 AND quantity_remaining > 0
		  AND (expiry_date IS NULL OR expiry_date >= (SELECT (now() AT TIME ZONE timezone)::date FROM shops WHERE id = $1))
		ORDER BY expiry_date NULLS LAST, received_at, id`, shopID, variantID)
	if err != nil {
		return nil, 0, err
	}
	rest = qty
	for rows.Next() && rest > 0 {
		var p Part
		if err := rows.Scan(&p.BatchID, &p.Qty); err != nil {
			rows.Close()
			return nil, 0, err
		}
		p.Qty = min(p.Qty, rest)
		rest -= p.Qty
		parts = append(parts, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	for _, p := range parts {
		if err := takeBatch(ctx, tx, shopID, p.BatchID, p.Qty); err != nil {
			return nil, 0, err
		}
	}
	return parts, rest, nil
}

// VariantRow: /inventory variant list er row (11.7 item 1); dam, draft, SEO nai.
type VariantRow struct {
	VariantID      int64           `json:"variant_id"`
	ProductID      int64           `json:"product_id"`
	ProductName    json.RawMessage `json:"product_name"`
	VariantLabel   json.RawMessage `json:"variant_label"`
	SKU            string          `json:"sku"`
	Stock          int             `json:"stock"`
	LowStockAt     int             `json:"low_stock_at"`
	BatchRemaining int64           `json:"batch_remaining"`
	NextExpiry     *string         `json:"next_expiry"`
}

// listVariants: batch_remaining/next_expiry = remaining > 0 er batch; q = SKU ba naam (bn/en; NFC, chhoto-boro
// haat na); low = stock <= low_stock_at; vendor scope. Count ar page query er WHERE ek (static SQL, Rule 12).
func listVariants(ctx context.Context, d *sql.DB, shopID int64, vendor any, q string, low bool, page, limit int) ([]VariantRow, int64, error) {
	var total int64
	if err := d.QueryRowContext(ctx, `SELECT count(*) FROM product_variants v JOIN products p ON p.shop_id = v.shop_id AND p.id = v.product_id
	WHERE v.shop_id = $1 AND ($2::bigint IS NULL OR v.vendor_id = $2)
	  AND ($3 = '' OR strpos(lower(normalize(v.sku || ' ' || COALESCE(p.name->>'bn', '') || ' ' || COALESCE(p.name->>'en', ''))),
		lower(normalize($3::text))) > 0)
	  AND (NOT $4 OR v.stock <= v.low_stock_at)`, shopID, vendor, q, low).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := d.QueryContext(ctx, `SELECT v.id, v.product_id, p.name, v.options, v.sku, v.stock, v.low_stock_at,
		COALESCE(b.rem, 0), to_char(b.next, 'YYYY-MM-DD') FROM product_variants v JOIN products p ON p.shop_id = v.shop_id AND p.id = v.product_id
	LEFT JOIN LATERAL (SELECT sum(quantity_remaining) rem, min(expiry_date) next FROM inventory_batches
		WHERE shop_id = v.shop_id AND variant_id = v.id AND quantity_remaining > 0) b ON true
	WHERE v.shop_id = $1 AND ($2::bigint IS NULL OR v.vendor_id = $2)
	  AND ($3 = '' OR strpos(lower(normalize(v.sku || ' ' || COALESCE(p.name->>'bn', '') || ' ' || COALESCE(p.name->>'en', ''))),
		lower(normalize($3::text))) > 0)
	  AND (NOT $4 OR v.stock <= v.low_stock_at)
		ORDER BY v.id LIMIT $5 OFFSET $6`, shopID, vendor, q, low, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []VariantRow
	for rows.Next() {
		var r VariantRow
		if err := rows.Scan(&r.VariantID, &r.ProductID, (*[]byte)(&r.ProductName), (*[]byte)(&r.VariantLabel), &r.SKU, &r.Stock,
			&r.LowStockAt, &r.BatchRemaining, &r.NextExpiry); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// listBatches: variant er batch (FIFO kram); variant scope e na hole ErrNotFound.
func listBatches(ctx context.Context, d *sql.DB, shopID, variantID int64, vendor any, page, limit int) ([]Batch, int64, error) {
	var total int64
	err := d.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM inventory_batches WHERE shop_id = $1 AND variant_id = v.id)
		FROM product_variants v WHERE v.shop_id = $1 AND v.id = $2 AND ($3::bigint IS NULL OR v.vendor_id = $3)`,
		shopID, variantID, vendor).Scan(&total)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	rows, err := d.QueryContext(ctx, `SELECT id, variant_id, batch_code, quantity_received, quantity_remaining, cost_price,
	to_char(harvest_date, 'YYYY-MM-DD'), to_char(expiry_date, 'YYYY-MM-DD'), received_at FROM inventory_batches WHERE shop_id = $1 AND variant_id = $2
		ORDER BY expiry_date NULLS LAST, received_at, id LIMIT $3 OFFSET $4`, shopID, variantID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Batch
	for rows.Next() {
		var b Batch
		if err := b.scan(rows); err != nil {
			return nil, 0, err
		}
		out = append(out, b)
	}
	return out, total, rows.Err()
}

// Movement: inventory_movements er row; CostPrice = batch er kena dam (staff/vendor_staff e nil).
type Movement struct {
	ID        int64     `json:"id"`
	VariantID int64     `json:"variant_id"`
	BatchID   *int64    `json:"batch_id"`
	Change    int       `json:"change"`
	Reason    string    `json:"reason"`
	OrderID   *int64    `json:"order_id"`
	StaffID   *int64    `json:"staff_id"`
	Note      *string   `json:"note"`
	CostPrice *int64    `json:"cost_price,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func listMovements(ctx context.Context, d *sql.DB, shopID int64, vendor any, variantID int64, page, limit int) ([]Movement, int64, error) {
	var total int64
	if err := d.QueryRowContext(ctx, `SELECT count(*) FROM inventory_movements m JOIN product_variants v ON v.shop_id = m.shop_id AND v.id = m.variant_id
	LEFT JOIN inventory_batches b ON b.shop_id = m.shop_id AND b.id = m.batch_id
	WHERE m.shop_id = $1 AND ($2::bigint IS NULL OR v.vendor_id = $2) AND ($3::bigint = 0 OR m.variant_id = $3)`, shopID, vendor, variantID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := d.QueryContext(ctx, `SELECT m.id, m.variant_id, m.batch_id, m.change, m.reason, m.order_id, m.staff_id, m.note,
		b.cost_price, m.created_at FROM inventory_movements m JOIN product_variants v ON v.shop_id = m.shop_id AND v.id = m.variant_id
	LEFT JOIN inventory_batches b ON b.shop_id = m.shop_id AND b.id = m.batch_id
	WHERE m.shop_id = $1 AND ($2::bigint IS NULL OR v.vendor_id = $2) AND ($3::bigint = 0 OR m.variant_id = $3) ORDER BY m.id DESC LIMIT $4 OFFSET $5`, shopID, vendor, variantID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Movement
	for rows.Next() {
		var m Movement
		if err := rows.Scan(&m.ID, &m.VariantID, &m.BatchID, &m.Change, &m.Reason, &m.OrderID, &m.StaffID, &m.Note,
			&m.CostPrice, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}
