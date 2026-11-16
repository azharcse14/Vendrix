package coupon

import (
	"context"
	"database/sql"
	"errors"
)

// Coupon: shorto, order e snapshot hoy (8.6 step 15). MaxDiscount ar VendorID 0 = nai (pricing.Coupon er mane).
type Coupon struct {
	ID            int64
	Code          string
	Kind          string
	Value         int64
	MinOrder      int64
	MaxDiscount   int64
	PerPhoneLimit int
	VendorID      int64
	FundedBy      string
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func scan(row *sql.Row) (Coupon, error) {
	var c Coupon
	err := row.Scan(&c.ID, &c.Code, &c.Kind, &c.Value, &c.MinOrder, &c.MaxDiscount, &c.PerPhoneLimit, &c.VendorID, &c.FundedBy)
	return c, err
}

// Use (8.6 step 11, 6.4): checkout er tx e; age used_count + 1 (row lock), tarpor phone count. Age count
// korle ek phone er duita checkout duitai 0 dekhe. Error hole caller ROLLBACK kore (used_count ferot).
func Use(ctx context.Context, tx querier, shopID int64, code, phone string) (Coupon, error) {
	code, _ = Normalize(code) // beshi lomba "" hoy, mile na → ErrInvalid
	c, err := scan(tx.QueryRowContext(ctx, `UPDATE coupons SET used_count = used_count + 1
		WHERE shop_id = $1 AND upper(code) = upper($2) AND is_active
		  AND (usage_limit IS NULL OR used_count < usage_limit)
		  AND (starts_at IS NULL OR starts_at <= now())
		  AND (expires_at IS NULL OR expires_at > now())
		RETURNING id, code, kind, value, min_order, COALESCE(max_discount, 0), per_phone_limit, COALESCE(vendor_id, 0), funded_by`, shopID, code))
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := check(ctx, tx, shopID, code); err != nil {
			return Coupon{}, err
		}
		return Coupon{}, ErrLimitReached // check er age onno checkout shesh slot nilo
	}
	if err != nil {
		return Coupon{}, err
	}
	return c, checkPhone(ctx, tx, shopID, c, phone)
}

// Peek: quote er read-only check (8.19 item 5), Use er karon kintu per-phone chhara (item 13 b: quote e onno
// phone er coupon use jana jay na; Use e dhore). Kichu bodlay na.
func Peek(ctx context.Context, q querier, shopID int64, code string) (Coupon, error) {
	code, _ = Normalize(code)
	return check(ctx, q, shopID, code)
}

// check: karon (8.19 item 5): nai/bondho/shomoy er baire age (ErrInvalid), tarpor usage_limit.
func check(ctx context.Context, q querier, shopID int64, code string) (Coupon, error) {
	var c Coupon
	var valid, full bool
	err := q.QueryRowContext(ctx, `SELECT id, code, kind, value, min_order, COALESCE(max_discount, 0), per_phone_limit, COALESCE(vendor_id, 0), funded_by,
		is_active AND (starts_at IS NULL OR starts_at <= now()) AND (expires_at IS NULL OR expires_at > now()),
		usage_limit IS NOT NULL AND used_count >= usage_limit
		FROM coupons WHERE shop_id = $1 AND upper(code) = upper($2)`, shopID, code).
		Scan(&c.ID, &c.Code, &c.Kind, &c.Value, &c.MinOrder, &c.MaxDiscount, &c.PerPhoneLimit, &c.VendorID, &c.FundedBy, &valid, &full)
	switch {
	case errors.Is(err, sql.ErrNoRows) || (err == nil && !valid):
		return Coupon{}, ErrInvalid
	case err != nil:
		return Coupon{}, err
	case full:
		return Coupon{}, ErrLimitReached
	}
	return c, nil
}

// checkPhone: ei phone (customer_phone) er redemption per_phone_limit er kom kina.
func checkPhone(ctx context.Context, q querier, shopID int64, c Coupon, phone string) error {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM coupon_redemptions WHERE shop_id = $1 AND coupon_id = $2 AND phone = $3`,
		shopID, c.ID, phone).Scan(&n); err != nil {
		return err
	}
	if n >= c.PerPhoneLimit {
		return ErrLimitReached
	}
	return nil
}

// Release (8.8 cancel, 8.13): order er redemption muche used_count - 1; GREATEST karon admin used_count
// komale CHECK e cancel atkabe na. Redemption na thakle kichu na, tai duibar dakleo thik.
func Release(ctx context.Context, q querier, shopID, orderID int64) error {
	_, err := q.ExecContext(ctx, `WITH r AS (
			DELETE FROM coupon_redemptions WHERE shop_id = $1 AND order_id = $2 RETURNING coupon_id)
		UPDATE coupons c SET used_count = GREATEST(c.used_count - 1, 0)
		FROM r WHERE c.shop_id = $1 AND c.id = r.coupon_id`, shopID, orderID)
	return err
}
