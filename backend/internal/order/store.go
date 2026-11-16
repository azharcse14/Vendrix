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

// takeStock (6.4): atomic komano, product ar vendor o active (step 7 er read er por bodlate pare); 0 row = stock
// nai (found false), vendor active na hole ErrVendorClosed (8.19 item 13 e).
func takeStock(ctx context.Context, tx *sql.Tx, shopID, variantID int64, qty int) (stockRow, bool, error) {
	var r stockRow
	err := tx.QueryRowContext(ctx, `UPDATE product_variants v SET stock = v.stock - $1
		FROM products p, vendors vd
		WHERE v.id = $2 AND v.shop_id = $3 AND v.stock >= $1 AND v.is_active
		  AND p.shop_id = v.shop_id AND p.id = v.product_id AND p.is_active
		  AND vd.shop_id = v.shop_id AND vd.id = v.vendor_id AND vd.status = 'active'
		RETURNING v.vendor_id, v.price, v.cost_price, v.sku, p.name, v.options, p.variable_weight, v.nominal_weight_g, vd.commission_bps`,
		qty, variantID, shopID).Scan(&r.VendorID, &r.Price, &r.CostPrice, &r.SKU, &r.Name, &r.Options, &r.VariableWeight,
		&r.NominalWeightG, &r.CommissionBps)
	if errors.Is(err, sql.ErrNoRows) {
		var closed bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM product_variants v JOIN vendors vd
			ON vd.shop_id = v.shop_id AND vd.id = v.vendor_id WHERE v.shop_id = $1 AND v.id = $2 AND vd.status <> 'active')`,
			shopID, variantID).Scan(&closed); err != nil {
			return r, false, err
		}
		if closed {
			return r, false, ErrVendorClosed
		}
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

// Admin list tab (11.7 item 13 vii).
const (
	TabAwaitingConfirm = "awaiting_confirm" // COD pending
	TabDeadlineSoon    = "deadline_soon"    // delivery_deadline < now + 24h, shesh hoyni
)

// ListFilter: admin order list; Vendor 0 = shob, > 0 oi vendor er fulfillment ache emon order, -1 = kichu na.
// VendorRole: vendor staff (taka nijer, ShopView na). Phone (normalize kora) ba Number (boro hater), ekta.
type ListFilter struct {
	Status, Tab, From, To, Phone, Number string
	Risk, VendorRole                     bool
	Vendor                               int64
}

// ShopView: shudhu shop staff (vendor role e nil: payment, customer, risk, order er puro taka na; 7.7).
type ShopView struct {
	CustomerName   string          `json:"customer_name"`
	CustomerPhone  string          `json:"customer_phone"`
	PaymentMethod  string          `json:"payment_method"`
	PaymentStatus  string          `json:"payment_status"`
	RiskFlags      json.RawMessage `json:"risk_flags"`
	Subtotal       int64           `json:"subtotal"`
	Discount       int64           `json:"discount"`
	DeliveryCharge int64           `json:"delivery_charge"`
}

// AdminOrder: list er row; Total vendor role e nijer fulfillment er mot.
type AdminOrder struct {
	ID               int64      `json:"id"`
	OrderNumber      string     `json:"order_number"`
	Status           string     `json:"status"`
	DisplayStatus    string     `json:"display_status"`
	RecipientName    string     `json:"recipient_name"`
	RecipientPhone   string     `json:"recipient_phone"`
	District         string     `json:"district"`
	Area             string     `json:"area"`
	Total            int64      `json:"total"`
	PlacedAt         time.Time  `json:"placed_at"`
	DeliveryDeadline *time.Time `json:"delivery_deadline"`
	*ShopView
}

// scanAdminOrder: vendor (hide) e ShopView nil.
func scanAdminOrder(sc func(...any) error, hide bool, extra ...any) (AdminOrder, error) {
	var o AdminOrder
	v := &ShopView{}
	err := sc(append([]any{&o.ID, &o.OrderNumber, &o.Status, &o.DisplayStatus, &o.RecipientName, &o.RecipientPhone,
		&o.District, &o.Area, &o.Total, &o.PlacedAt, &o.DeliveryDeadline, &v.CustomerName, &v.CustomerPhone,
		&v.PaymentMethod, &v.PaymentStatus, &v.RiskFlags, &v.Subtotal, &v.Discount, &v.DeliveryCharge}, extra...)...)
	if !hide {
		o.ShopView = v
	}
	return o, err
}

// listOrders (11.7 item 13 vii): placed_at DESC. Rule 12: static SQL, khali parameter = filter na; count ar list er
// WHERE ekoi (ekta bodlale duita). $1 shop, $2 vendor role er vendor (taka nijer ongsho; count e shudhu type), $3 vendor
// (0 = shob), $4 status, $5 tab, $6 risk, $7 phone, $8 number, $9/$10 from/to (shop timezone er din).
func listOrders(ctx context.Context, d *sql.DB, shopID int64, f ListFilter, page, limit int) ([]AdminOrder, int64, error) {
	own := int64(0)
	if f.VendorRole {
		own = f.Vendor
	}
	args := []any{shopID, own, f.Vendor, f.Status, f.Tab, f.Risk, f.Phone, f.Number, f.From, f.To}
	var total int64
	if err := d.QueryRowContext(ctx, `SELECT count(*) FROM orders o
		WHERE o.shop_id = $1 AND $2::bigint IS NOT NULL
		AND ($3::bigint = 0 OR EXISTS (SELECT 1 FROM order_fulfillments f WHERE f.shop_id = o.shop_id AND f.order_id = o.id AND f.vendor_id = $3))
		AND ($4::text = '' OR o.display_status = $4)
		AND ($5::text <> 'awaiting_confirm' OR (o.payment_method = 'cod' AND o.status = 'pending'))
		AND ($5::text <> 'deadline_soon' OR (o.delivery_deadline < now() + interval '24 hours'
			AND o.display_status NOT IN ('delivered', 'returned', 'cancelled')))
		AND (NOT $6::boolean OR o.risk_flags <> '[]'::jsonb)
		AND ($7::text = '' OR o.customer_phone = $7::text OR o.recipient_phone = $7::text)
		AND ($8::text = '' OR o.order_number = $8)
		AND ($9::text = '' OR o.placed_at >= NULLIF($9, '')::date::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1))
		AND ($10::text = '' OR o.placed_at < (NULLIF($10, '')::date + 1)::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1))`,
		args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := d.QueryContext(ctx, `SELECT o.id, o.order_number, o.status, o.display_status, o.recipient_name, o.recipient_phone,
		o.ship_district, o.ship_area, CASE WHEN $2 > 0 THEN (SELECT COALESCE(sum(f.total), 0) FROM order_fulfillments f
			WHERE f.shop_id = o.shop_id AND f.order_id = o.id AND f.vendor_id = $2) ELSE o.total END,
		o.placed_at, o.delivery_deadline, o.customer_name, o.customer_phone, o.payment_method, o.payment_status, o.risk_flags,
		o.subtotal, o.discount, o.delivery_charge
		FROM orders o WHERE o.shop_id = $1
		AND ($3::bigint = 0 OR EXISTS (SELECT 1 FROM order_fulfillments f WHERE f.shop_id = o.shop_id AND f.order_id = o.id AND f.vendor_id = $3))
		AND ($4::text = '' OR o.display_status = $4)
		AND ($5::text <> 'awaiting_confirm' OR (o.payment_method = 'cod' AND o.status = 'pending'))
		AND ($5::text <> 'deadline_soon' OR (o.delivery_deadline < now() + interval '24 hours'
			AND o.display_status NOT IN ('delivered', 'returned', 'cancelled')))
		AND (NOT $6::boolean OR o.risk_flags <> '[]'::jsonb)
		AND ($7::text = '' OR o.customer_phone = $7::text OR o.recipient_phone = $7::text)
		AND ($8::text = '' OR o.order_number = $8)
		AND ($9::text = '' OR o.placed_at >= NULLIF($9, '')::date::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1))
		AND ($10::text = '' OR o.placed_at < (NULLIF($10, '')::date + 1)::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1))
		ORDER BY o.placed_at DESC, o.id DESC LIMIT $11 OFFSET $12`, append(args, limit, (page-1)*limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AdminOrder
	for rows.Next() {
		o, err := scanAdminOrder(rows.Scan, f.VendorRole)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, o)
	}
	return out, total, rows.Err()
}

// AdminItem: order_items er snapshot (cost_price, commission na).
type AdminItem struct {
	ID             int64           `json:"id"`
	FulfillmentID  int64           `json:"fulfillment_id"`
	VariantID      int64           `json:"variant_id"`
	ProductName    json.RawMessage `json:"product_name"`
	VariantLabel   json.RawMessage `json:"variant_label"`
	SKU            string          `json:"sku"`
	UnitPrice      int64           `json:"unit_price"`
	Quantity       int             `json:"quantity"`
	VariableWeight bool            `json:"variable_weight"`
	OrderedWeightG *int64          `json:"ordered_weight_g"`
	PackedWeightG  *int64          `json:"packed_weight_g"`
	LineTotal      int64           `json:"line_total"`
	DiscountShare  int64           `json:"discount_share"`
	Status         string          `json:"status"`
}

type AdminFulfillment struct {
	ID             int64       `json:"id"`
	VendorID       int64       `json:"vendor_id"`
	Seq            int         `json:"seq"`
	Status         string      `json:"status"`
	Subtotal       int64       `json:"subtotal"`
	Discount       int64       `json:"discount"`
	DeliveryCharge int64       `json:"delivery_charge"`
	Total          int64       `json:"total"`
	Items          []AdminItem `json:"items"`
}

type HistoryRow struct {
	FulfillmentID *int64    `json:"fulfillment_id"`
	From          *string   `json:"from_status"`
	To            string    `json:"to_status"`
	ActorType     string    `json:"actor_type"`
	ActorID       *int64    `json:"actor_id"`
	Note          *string   `json:"note"`
	At            time.Time `json:"at"`
}

// OrderDetail: GET /api/admin/orders/{id}; vendor role e nijer fulfillment/item/history, note na.
type OrderDetail struct {
	AdminOrder
	Address      string             `json:"address"`
	Note         *string            `json:"note"`
	CancelReason *string            `json:"cancel_reason"`
	Fulfillments []AdminFulfillment `json:"fulfillments"`
	History      []HistoryRow       `json:"history"`
	Notes        []Note             `json:"notes,omitempty"`
}

// orderDetail: vendor (> 0 ba -1) er fulfillment na thakle ErrNotFound (onno vendor er order ache kina bola na).
func orderDetail(ctx context.Context, d *sql.DB, shopID, orderID, vendor int64) (OrderDetail, error) {
	var o OrderDetail
	own := max(vendor, 0)
	var err error
	o.AdminOrder, err = scanAdminOrder(d.QueryRowContext(ctx, `SELECT o.id, o.order_number, o.status, o.display_status,
		o.recipient_name, o.recipient_phone, o.ship_district, o.ship_area, CASE WHEN $2 > 0 THEN (SELECT COALESCE(sum(f.total), 0)
			FROM order_fulfillments f WHERE f.shop_id = o.shop_id AND f.order_id = o.id AND f.vendor_id = $2) ELSE o.total END,
		o.placed_at, o.delivery_deadline, o.customer_name, o.customer_phone, o.payment_method, o.payment_status, o.risk_flags,
		o.subtotal, o.discount, o.delivery_charge, o.ship_address, o.note, o.cancel_reason
		FROM orders o WHERE o.shop_id = $1 AND o.id = $3`, shopID, own, orderID).Scan, vendor != 0, &o.Address, &o.Note, &o.CancelReason)
	if errors.Is(err, sql.ErrNoRows) {
		return o, ErrNotFound
	}
	if err != nil {
		return o, err
	}
	rows, err := d.QueryContext(ctx, `SELECT id, vendor_id, seq, status, subtotal, discount, delivery_charge, total
		FROM order_fulfillments WHERE shop_id = $1 AND order_id = $2 AND ($3 = 0 OR vendor_id = $3) ORDER BY id`, shopID, orderID, vendor)
	if err != nil {
		return o, err
	}
	idx := map[int64]int{}
	for rows.Next() {
		var f AdminFulfillment
		if err := rows.Scan(&f.ID, &f.VendorID, &f.Seq, &f.Status, &f.Subtotal, &f.Discount, &f.DeliveryCharge, &f.Total); err != nil {
			rows.Close()
			return o, err
		}
		f.Items = []AdminItem{}
		idx[f.ID] = len(o.Fulfillments)
		o.Fulfillments = append(o.Fulfillments, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return o, err
	}
	if vendor != 0 && len(o.Fulfillments) == 0 {
		return OrderDetail{}, ErrNotFound
	}
	rows, err = d.QueryContext(ctx, `SELECT id, fulfillment_id, variant_id, product_name, variant_label, sku, unit_price, quantity,
		variable_weight, ordered_weight_g, packed_weight_g, line_total, discount_share, status
		FROM order_items WHERE shop_id = $1 AND order_id = $2 AND ($3 = 0 OR vendor_id = $3) ORDER BY id`, shopID, orderID, vendor)
	if err != nil {
		return o, err
	}
	for rows.Next() {
		var it AdminItem
		if err := rows.Scan(&it.ID, &it.FulfillmentID, &it.VariantID, &it.ProductName, &it.VariantLabel, &it.SKU, &it.UnitPrice,
			&it.Quantity, &it.VariableWeight, &it.OrderedWeightG, &it.PackedWeightG, &it.LineTotal, &it.DiscountShare, &it.Status); err != nil {
			rows.Close()
			return o, err
		}
		if i, ok := idx[it.FulfillmentID]; ok {
			o.Fulfillments[i].Items = append(o.Fulfillments[i].Items, it)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return o, err
	}
	rows, err = d.QueryContext(ctx, `SELECT h.fulfillment_id, h.from_status, h.to_status, h.actor_type, h.actor_id, h.note, h.created_at
		FROM order_status_history h WHERE h.shop_id = $1 AND h.order_id = $2 AND ($3 = 0 OR h.fulfillment_id IS NULL
			OR h.fulfillment_id IN (SELECT id FROM order_fulfillments WHERE shop_id = $1 AND order_id = $2 AND vendor_id = $3))
		ORDER BY h.id`, shopID, orderID, vendor)
	if err != nil {
		return o, err
	}
	o.History = []HistoryRow{}
	for rows.Next() {
		var h HistoryRow
		if err := rows.Scan(&h.FulfillmentID, &h.From, &h.To, &h.ActorType, &h.ActorID, &h.Note, &h.At); err != nil {
			rows.Close()
			return o, err
		}
		o.History = append(o.History, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil || vendor != 0 {
		return o, err
	}
	rows, err = d.QueryContext(ctx, `SELECT id, staff_id, kind, note, created_at FROM order_notes
		WHERE shop_id = $1 AND order_id = $2 ORDER BY created_at, id`, shopID, orderID)
	if err != nil {
		return o, err
	}
	defer rows.Close()
	o.Notes = []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.StaffID, &n.Kind, &n.Note, &n.CreatedAt); err != nil {
			return o, err
		}
		o.Notes = append(o.Notes, n)
	}
	return o, rows.Err()
}

// FulfillmentRow: GET /api/admin/fulfillments er row (delivery er tottho; payment na).
type FulfillmentRow struct {
	ID            int64     `json:"id"`
	OrderID       int64     `json:"order_id"`
	OrderNumber   string    `json:"order_number"`
	VendorID      int64     `json:"vendor_id"`
	Seq           int       `json:"seq"`
	Status        string    `json:"status"`
	Total         int64     `json:"total"`
	RecipientName string    `json:"recipient_name"`
	District      string    `json:"district"`
	Area          string    `json:"area"`
	PlacedAt      time.Time `json:"placed_at"`
}

// listFulfillments: vendor 0 = shob, -1 = kichu na; date = order er din (shop timezone).
func listFulfillments(ctx context.Context, d *sql.DB, shopID int64, status, date string, vendor int64, page, limit int) ([]FulfillmentRow, int64, error) {
	args := []any{shopID, vendor, status, date}
	var total int64
	if err := d.QueryRowContext(ctx, `SELECT count(*) FROM order_fulfillments f JOIN orders o ON o.shop_id = f.shop_id AND o.id = f.order_id
		WHERE f.shop_id = $1 AND ($2::bigint = 0 OR f.vendor_id = $2) AND ($3::text = '' OR f.status = $3)
		AND ($4::text = '' OR (o.placed_at >= NULLIF($4, '')::date::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1)
			AND o.placed_at < (NULLIF($4, '')::date + 1)::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1)))`,
		args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := d.QueryContext(ctx, `SELECT f.id, f.order_id, o.order_number, f.vendor_id, f.seq, f.status, f.total, o.recipient_name,
		o.ship_district, o.ship_area, o.placed_at
		FROM order_fulfillments f JOIN orders o ON o.shop_id = f.shop_id AND o.id = f.order_id
		WHERE f.shop_id = $1 AND ($2::bigint = 0 OR f.vendor_id = $2) AND ($3::text = '' OR f.status = $3)
		AND ($4::text = '' OR (o.placed_at >= NULLIF($4, '')::date::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1)
			AND o.placed_at < (NULLIF($4, '')::date + 1)::timestamp AT TIME ZONE (SELECT timezone FROM shops WHERE id = $1)))
		ORDER BY o.placed_at DESC, f.id DESC LIMIT $5 OFFSET $6`, append(args, limit, (page-1)*limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []FulfillmentRow
	for rows.Next() {
		var x FulfillmentRow
		if err := rows.Scan(&x.ID, &x.OrderID, &x.OrderNumber, &x.VendorID, &x.Seq, &x.Status, &x.Total, &x.RecipientName,
			&x.District, &x.Area, &x.PlacedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, x)
	}
	return out, total, rows.Err()
}
