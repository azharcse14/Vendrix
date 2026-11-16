package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"vendrix/backend/internal/coupon"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/pricing"
)

// querier: *sql.DB ba *sql.Tx.
type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// LoadRiskHistory (8.19 item 3): 8.6 step 8 e, ei order insert er age. Phone er query orders_phone_idx,
// ip er orders_placed_idx e (shop er shesh ghonta choto). ip "" = jana nai; IPv6 er /64 ek IP (item 13 c).
func LoadRiskHistory(ctx context.Context, q querier, shopID int64, customerPhone, recipientPhone, ip string) (RiskHistory, error) {
	var h RiskHistory
	err := q.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM orders WHERE shop_id = $1 AND placed_at > now() - interval '60 minutes'
			AND (customer_phone = $2 OR ip::inet <<= NULLIF($4, '')::inet)),
		EXISTS (SELECT 1 FROM orders o JOIN order_fulfillments f ON f.shop_id = o.shop_id AND f.order_id = o.id
			WHERE o.shop_id = $1 AND o.customer_phone IN ($2, $3) AND f.status = 'returned'),
		EXISTS (SELECT 1 FROM orders WHERE shop_id = $1 AND customer_phone = $2)`,
		shopID, customerPhone, recipientPhone, httpx.ClientKey(ip)).Scan(&h.RecentOrders, &h.Returned, &h.PriorOrders)
	return h, err
}

// Settings: published settings er checkout ar marketplace.delivery_charge_mode (9.2). Key na thakle zero
// value: payment method nai (400), ochena otp_mode e OTP (fail closed).
type Settings struct {
	PaymentMethods  []string             `json:"payment_methods"`
	OTPMode         string               `json:"otp_mode"`
	CODConfirmation string               `json:"cod_confirmation"`
	Risk            RiskLimits           `json:"risk"`
	AutoCancelHours int                  `json:"auto_cancel_unconfirmed_hours"`
	DeliveryMode    pricing.DeliveryMode `json:"-"`
}

func loadSettings(ctx context.Context, q querier, shopID int64) (Settings, error) {
	var s Settings
	var raw []byte
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(published->'checkout', '{}'),
		COALESCE(published #>> '{marketplace,delivery_charge_mode}', 'per_order')
		FROM shop_settings WHERE shop_id = $1`, shopID).Scan(&raw, &s.DeliveryMode); err != nil {
		return s, err
	}
	return s, json.Unmarshal(raw, &s)
}

// userPhone: login customer er account phone (8.19 item 4).
func userPhone(ctx context.Context, q querier, shopID, userID int64) (string, error) {
	var p string
	err := q.QueryRowContext(ctx, `SELECT phone FROM users WHERE shop_id = $1 AND id = $2`, shopID, userID).Scan(&p)
	return p, err
}

// prior: ei key er ager order (8.6 step 2, 9). Na thakle found false.
type prior struct {
	found bool
	hash  []byte
	resp  Placed
}

func findByKey(ctx context.Context, q querier, shopID int64, key string) (prior, error) {
	var p prior
	err := q.QueryRowContext(ctx, `SELECT idempotency_hash, order_number, public_token, status FROM orders
		WHERE shop_id = $1 AND idempotency_key = $2`, shopID, key).Scan(&p.hash, &p.resp.OrderNumber, &p.resp.PublicToken, &p.resp.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	p.found = err == nil
	return p, err
}

// orderLimit (7.8, 8.19 item 9): shesh 60 minute e ekoi customer_phone er 5 ba ekoi ip er 10 ta order thakle
// limit; retry = oi shima-r purono order window theke ber howa porjonto. ip "" = shudhu phone. IPv6 e /64 (item 13 c):
// orders.ip TEXT, kintu shob shomoy ClientIP er valid IP, tai ::inet cast nirapod.
func orderLimit(ctx context.Context, q querier, shopID int64, phone, ip string) (time.Duration, bool, error) {
	var secs sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT ceil(max(extract(epoch FROM placed_at + interval '60 minutes' - now())))::bigint FROM (
		(SELECT placed_at FROM orders WHERE shop_id = $1 AND customer_phone = $2 AND placed_at > now() - interval '60 minutes'
		 ORDER BY placed_at DESC OFFSET 4 LIMIT 1)
		UNION ALL
		(SELECT placed_at FROM orders WHERE shop_id = $1 AND ip::inet <<= NULLIF($3, '')::inet AND placed_at > now() - interval '60 minutes'
		 ORDER BY placed_at DESC OFFSET 9 LIMIT 1)) x`, shopID, phone, httpx.ClientKey(ip)).Scan(&secs)
	return time.Duration(secs.Int64) * time.Second, secs.Valid, err
}

// lockKey (6.4): ekoi key er duita checkout ek shathe na.
func lockKey(ctx context.Context, tx *sql.Tx, shopID int64, key string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::bigint::text || ':' || $2, 0))`, shopID, key)
	return err
}

func phoneBlocked(ctx context.Context, q querier, shopID int64, phones ...string) (bool, error) {
	var b bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM blocked_phones WHERE shop_id = $1 AND phone = ANY($2))`,
		shopID, phones).Scan(&b)
	return b, err
}

// stockRow: stock komanor por variant er snapshot (row lock er por, tai bortoman dam).
type stockRow struct {
	VendorID       int64
	Price          int64
	CostPrice      sql.NullInt64
	SKU            string
	Name, Options  []byte
	VariableWeight bool
	NominalWeightG sql.NullInt64
	CommissionBps  int
}

// takeStock (6.4): atomic komano, product o active; 0 row = stock nai (found false).
func takeStock(ctx context.Context, tx *sql.Tx, shopID, variantID int64, qty int) (stockRow, bool, error) {
	var r stockRow
	err := tx.QueryRowContext(ctx, `UPDATE product_variants v SET stock = v.stock - $1
		FROM products p, vendors vd
		WHERE v.id = $2 AND v.shop_id = $3 AND v.stock >= $1 AND v.is_active
		  AND p.shop_id = v.shop_id AND p.id = v.product_id AND p.is_active
		  AND vd.shop_id = v.shop_id AND vd.id = v.vendor_id
		RETURNING v.vendor_id, v.price, v.cost_price, v.sku, p.name, v.options, p.variable_weight, v.nominal_weight_g, vd.commission_bps`,
		qty, variantID, shopID).Scan(&r.VendorID, &r.Price, &r.CostPrice, &r.SKU, &r.Name, &r.Options, &r.VariableWeight,
		&r.NominalWeightG, &r.CommissionBps)
	if errors.Is(err, sql.ErrNoRows) {
		return r, false, nil
	}
	return r, err == nil, err
}

// nextOrderNumber (6.4): counter er lock commit porjonto, tai tx er shesher dike.
func nextOrderNumber(ctx context.Context, tx *sql.Tx, shopID int64) (string, error) {
	var prefix string
	var n int64
	err := tx.QueryRowContext(ctx, `UPDATE shop_counters c SET value = c.value + 1 FROM shops s
		WHERE c.shop_id = $1 AND c.name = 'order_number' AND s.id = c.shop_id
		RETURNING s.order_prefix, c.value`, shopID).Scan(&prefix, &n)
	return fmt.Sprintf("%s-%d", prefix, 100000+n), err
}

// newOrder: orders er row (8.6 step 13–15).
type newOrder struct {
	ShopID, UserID, ZoneID         int64
	Number, Token, Key             string
	Hash                           []byte
	In                             PlaceInput
	Totals                         pricing.Totals
	Coupon                         *coupon.Coupon
	PaymentStatus, Status, Display string
	Flags                          []string
	OTPVerified                    bool
	DeadlineDays                   int
	ConfirmHash                    []byte // nil = NULL
}

// insertOrder: ON CONFLICT e 0 row (found false); lock er por eta hoy na, tobu chupchap duplicate na.
// placed_at DB er ghori (auto-cancel er RunAt).
func insertOrder(ctx context.Context, tx *sql.Tx, o newOrder) (int64, time.Time, bool, error) {
	var placedAt time.Time
	flags, err := json.Marshal(append([]string{}, o.Flags...)) // nil → [] (null na)
	if err != nil {
		return 0, placedAt, false, err
	}
	var cID, cMax, cVendor sql.NullInt64
	var cCode, cKind, cFunded sql.NullString
	var cValue sql.NullInt64
	if c := o.Coupon; c != nil {
		cID, cValue = sql.NullInt64{Int64: c.ID, Valid: true}, sql.NullInt64{Int64: c.Value, Valid: true}
		cCode, cKind, cFunded = sql.NullString{String: c.Code, Valid: true}, sql.NullString{String: c.Kind, Valid: true},
			sql.NullString{String: c.FundedBy, Valid: true}
		cMax, cVendor = sql.NullInt64{Int64: c.MaxDiscount, Valid: c.MaxDiscount > 0}, sql.NullInt64{Int64: c.VendorID, Valid: c.VendorID > 0}
	}
	in := o.In
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash,
		user_id, customer_name, customer_phone, customer_email, recipient_name, recipient_phone, ship_district, ship_area,
		ship_address, delivery_zone_id, note, locale, subtotal, discount, delivery_charge, total, coupon_id, coupon_code,
		coupon_kind, coupon_value, coupon_max_discount, coupon_vendor_id, coupon_funded_by, payment_method, payment_status,
		status, display_status, risk_flags, otp_verified, ip, delivery_deadline, confirm_token_hash)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, 0), $7, $8, NULLIF($9, ''), $10, $11, $12, $13, $14, $15, NULLIF($16, ''), $17,
		  $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, NULLIF($35, ''),
		  now() + make_interval(days => $36), $37)
		ON CONFLICT (shop_id, idempotency_key) DO NOTHING RETURNING id, placed_at`,
		o.ShopID, o.Number, o.Token, o.Key, o.Hash, o.UserID, in.Customer.Name, in.Customer.Phone, in.Customer.Email,
		in.Recipient.Name, in.Recipient.Phone, in.District, in.Area, in.Line, o.ZoneID, in.Note, in.Locale,
		o.Totals.Subtotal, o.Totals.Discount, o.Totals.DeliveryCharge, o.Totals.Total, cID, cCode, cKind, cValue, cMax, cVendor,
		cFunded, in.PaymentMethod, o.PaymentStatus, o.Status, o.Display, flags, o.OTPVerified, in.IP, o.DeadlineDays, o.ConfirmHash).Scan(&id, &placedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, placedAt, false, nil
	}
	return id, placedAt, err == nil, err
}

func insertFulfillment(ctx context.Context, tx *sql.Tx, shopID, orderID int64, seq int, g pricing.Group) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `INSERT INTO order_fulfillments (shop_id, order_id, vendor_id, seq, subtotal, discount,
		delivery_charge, total) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		shopID, orderID, g.VendorID, seq, g.Subtotal, g.Discount, g.DeliveryCharge, g.Total).Scan(&id)
	return id, err
}

// item: order_items er row (8.6 step 16; snapshot Rule 8).
type item struct {
	FulfillmentID, VariantID int64
	Stock                    stockRow
	Label                    []byte
	Qty                      int
	OrderedWeightG           sql.NullInt64
	Result                   pricing.LineResult
	Commission               int64
}

// insertItem: item, stock movement (order) ek shathe.
func insertItem(ctx context.Context, tx *sql.Tx, shopID, orderID int64, it item) error {
	s := it.Stock
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_items (shop_id, order_id, fulfillment_id, vendor_id, variant_id,
		product_name, variant_label, sku, unit_price, cost_price, quantity, variable_weight, ordered_weight_g, line_total,
		discount_share, commission, commission_bps)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		shopID, orderID, it.FulfillmentID, s.VendorID, it.VariantID, s.Name, it.Label, s.SKU, s.Price, s.CostPrice, it.Qty,
		s.VariableWeight, it.OrderedWeightG, it.Result.LineTotal, it.Result.DiscountShare, it.Commission, s.CommissionBps); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO inventory_movements (shop_id, variant_id, change, reason, order_id)
		VALUES ($1, $2, $3, 'order', $4)`, shopID, it.VariantID, -it.Qty, orderID)
	return err
}

// insertPlaced: prothom history row ar coupon redemption.
func insertPlaced(ctx context.Context, tx *sql.Tx, shopID, orderID, userID int64, status string, c *coupon.Coupon, phone string) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO order_status_history (shop_id, order_id, to_status, actor_type, actor_id)
		VALUES ($1, $2, $3, 'customer', NULLIF($4, 0))`, shopID, orderID, status, userID); err != nil {
		return err
	}
	if c == nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO coupon_redemptions (shop_id, coupon_id, order_id, phone) VALUES ($1, $2, $3, $4)`,
		shopID, c.ID, orderID, phone)
	return err
}
