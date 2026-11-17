package order

import (
	"context"
	"database/sql"
	"net/http"

	"vendrix/backend/internal/httpx"
)

// Dashboard (11.7 item 13 viii, v1.46): shudhu count; list order list tab ar /inventory/variants?low=1 e.
type Dashboard struct {
	Date         string `json:"date"`
	Orders       int64  `json:"orders"`
	Sales        int64  `json:"sales"`
	Pending      int64  `json:"pending"`
	ToPack       int64  `json:"to_pack"`
	LowStock     int64  `json:"low_stock"`
	Expiring     int64  `json:"expiring"`
	DeadlineSoon int64  `json:"deadline_soon"`
}

// AdminDashboard: GET /api/admin/dashboard (shob role, vendor nijer).
func (h *Handlers) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	st, _ := httpx.StaffFrom(r.Context())
	d, err := LoadDashboard(r.Context(), h.db, st.ShopID, scope(ByStaff(st)))
	if err != nil {
		h.adminError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, d)
}

// LoadDashboard: "aj" shop er timezone e (Rule 5). vendor 0 = shob, > 0 nijer fulfillment/variant, -1 = kichu na.
// orders/sales aj placed, cancelled bad; sales e online unpaid na (bikri hoyni), vendor e nijer fulfillment er
// total. pending = COD pending (confirm er opekkhay tab), to_pack = pick list er moto, deadline = order list tab
// er moto, low stock = low=1 er moto, expiring shudhu batch_expiry on e.
func LoadDashboard(ctx context.Context, d *sql.DB, shopID, vendor int64) (Dashboard, error) {
	var x Dashboard
	err := d.QueryRowContext(ctx, `WITH sh AS (SELECT timezone tz, (now() AT TIME ZONE timezone)::date d FROM shops WHERE id = $1),
		day AS (SELECT d::timestamp AT TIME ZONE tz lo, (d + 1)::timestamp AT TIME ZONE tz hi FROM sh),
		mine AS (SELECT o.* FROM orders o WHERE o.shop_id = $1 AND ($2::bigint = 0 OR EXISTS (SELECT 1 FROM order_fulfillments f
			WHERE f.shop_id = o.shop_id AND f.order_id = o.id AND f.vendor_id = $2))),
		today AS (SELECT m.* FROM mine m, day WHERE m.placed_at >= day.lo AND m.placed_at < day.hi AND m.status <> 'cancelled'),
		ss AS (SELECT published FROM shop_settings WHERE shop_id = $1)
		SELECT to_char((SELECT d FROM sh), 'YYYY-MM-DD'),
		(SELECT count(*) FROM today),
		(SELECT coalesce(sum(CASE WHEN $2::bigint = 0 THEN t.total ELSE (SELECT coalesce(sum(f.total), 0) FROM order_fulfillments f
			WHERE f.shop_id = t.shop_id AND f.order_id = t.id AND f.vendor_id = $2 AND f.status <> 'cancelled') END), 0)::bigint
			FROM today t WHERE t.payment_method = 'cod' OR t.payment_status IN ('paid', 'partially_refunded', 'refunded')),
		(SELECT count(*) FROM mine WHERE status = 'pending' AND payment_method = 'cod'),
		(SELECT count(*) FROM order_fulfillments f JOIN orders o ON o.shop_id = f.shop_id AND o.id = f.order_id
			WHERE f.shop_id = $1 AND ($2::bigint = 0 OR f.vendor_id = $2) AND f.status = 'open' AND o.status = 'confirmed'),
		(SELECT count(*) FROM product_variants v WHERE v.shop_id = $1 AND ($2::bigint = 0 OR v.vendor_id = $2) AND v.stock <= v.low_stock_at),
		(SELECT CASE WHEN coalesce(published->'features' ? 'batch_expiry', false) THEN (SELECT count(*) FROM inventory_batches b
			JOIN product_variants v ON v.shop_id = b.shop_id AND v.id = b.variant_id
			WHERE b.shop_id = $1 AND ($2::bigint = 0 OR v.vendor_id = $2) AND b.quantity_remaining > 0
			AND b.expiry_date BETWEEN (SELECT d FROM sh) AND (SELECT d FROM sh) + coalesce((published->'inventory'->>'expiry_alert_days')::int, 7))
			ELSE 0 END FROM ss),
		(SELECT count(*) FROM mine WHERE delivery_deadline < now() + interval '24 hours'
			AND display_status NOT IN ('delivered', 'returned', 'cancelled'))`, shopID, vendor).Scan(
		&x.Date, &x.Orders, &x.Sales, &x.Pending, &x.ToPack, &x.LowStock, &x.Expiring, &x.DeadlineSoon)
	return x, err
}
