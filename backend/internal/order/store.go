package order

import (
	"context"
	"database/sql"
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
