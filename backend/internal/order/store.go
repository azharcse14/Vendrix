package order

import (
	"context"
	"database/sql"
	"encoding/json"

	"vendrix/backend/internal/pricing"
)

// querier: *sql.DB ba *sql.Tx.
type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// LoadRiskHistory (8.19 item 3): 8.6 step 8 e, ei order insert er age. Phone er query orders_phone_idx,
// ip er orders_placed_idx e (shop er shesh ghonta choto). ip "" = jana nai.
func LoadRiskHistory(ctx context.Context, q querier, shopID int64, customerPhone, recipientPhone, ip string) (RiskHistory, error) {
	var h RiskHistory
	err := q.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM orders WHERE shop_id = $1 AND placed_at > now() - interval '60 minutes'
			AND (customer_phone = $2 OR ip = NULLIF($4, ''))),
		EXISTS (SELECT 1 FROM orders o JOIN order_fulfillments f ON f.shop_id = o.shop_id AND f.order_id = o.id
			WHERE o.shop_id = $1 AND o.customer_phone IN ($2, $3) AND f.status = 'returned'),
		EXISTS (SELECT 1 FROM orders WHERE shop_id = $1 AND customer_phone = $2)`,
		shopID, customerPhone, recipientPhone, ip).Scan(&h.RecentOrders, &h.Returned, &h.PriorOrders)
	return h, err
}

// Settings: published settings er checkout ar marketplace.delivery_charge_mode (9.2). Key na thakle zero
// value: payment method nai (400), ochena otp_mode e OTP (fail closed).
type Settings struct {
	PaymentMethods  []string             `json:"payment_methods"`
	OTPMode         string               `json:"otp_mode"`
	CODConfirmation string               `json:"cod_confirmation"`
	Risk            RiskLimits           `json:"risk"`
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
