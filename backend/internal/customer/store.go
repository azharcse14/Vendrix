package customer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func collect(rows *sql.Rows, err error) ([]json.RawMessage, int64, error) {
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []json.RawMessage
	var total int64
	for rows.Next() {
		var v json.RawMessage
		if err := rows.Scan(&v, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

// likeEscape: q er %, _ ar \ hubohu (ILIKE er wildcard na).
var likeEscape = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// listCustomers: deleted chhara user; order_count shob order (user_id), order_total cancelled bad (v1.45).
// ph ba name nil = filter na.
func listCustomers(ctx context.Context, q querier, shopID int64, ph, name any, limit, offset int) ([]json.RawMessage, int64, error) {
	if s, ok := name.(string); ok {
		name = likeEscape.Replace(s)
	}
	return collect(q.QueryContext(ctx, `SELECT jsonb_build_object('id', u.id, 'phone', u.phone, 'name', u.name, 'email', u.email,
			'locale', u.locale, 'created_at', u.created_at,
			'order_count', (SELECT count(*) FROM orders o WHERE o.shop_id = u.shop_id AND o.user_id = u.id),
			'order_total', (SELECT coalesce(sum(o.total), 0) FROM orders o WHERE o.shop_id = u.shop_id AND o.user_id = u.id AND o.status <> 'cancelled'),
			'blocked', EXISTS (SELECT 1 FROM blocked_phones b WHERE b.shop_id = u.shop_id AND b.phone = u.phone)),
			count(*) OVER ()
		FROM users u WHERE u.shop_id = $1 AND u.deleted_at IS NULL
		  AND ($2::text IS NULL OR u.phone = $2) AND ($3::text IS NULL OR u.name ILIKE '%' || $3 || '%')
		ORDER BY u.created_at DESC, u.id DESC LIMIT $4 OFFSET $5`, shopID, ph, name, limit, offset))
}

// customerDetail: profile + blocked + shesh 50 order (guest order phone search e, order list e).
func customerDetail(ctx context.Context, q querier, shopID, id int64) (json.RawMessage, error) {
	var c json.RawMessage
	err := q.QueryRowContext(ctx, `SELECT jsonb_build_object('id', u.id, 'phone', u.phone, 'name', u.name, 'email', u.email,
			'locale', u.locale, 'created_at', u.created_at,
			'blocked', EXISTS (SELECT 1 FROM blocked_phones b WHERE b.shop_id = u.shop_id AND b.phone = u.phone),
			'orders', coalesce((SELECT jsonb_agg(o ORDER BY o.placed_at DESC, o.id DESC) FROM (
				SELECT id, order_number, display_status, payment_method, payment_status, total, placed_at FROM orders
				WHERE shop_id = u.shop_id AND user_id = u.id ORDER BY placed_at DESC, id DESC LIMIT 50) o), '[]'))
		FROM users u WHERE u.shop_id = $1 AND u.id = $2 AND u.deleted_at IS NULL`, shopID, id).Scan(&c)
	return c, err
}

func listBlocked(ctx context.Context, q querier, shopID int64, limit, offset int) ([]json.RawMessage, int64, error) {
	return collect(q.QueryContext(ctx, `SELECT to_jsonb(b) - 'shop_id', count(*) OVER () FROM blocked_phones b
		WHERE shop_id = $1 ORDER BY created_at DESC, phone LIMIT $2 OFFSET $3`, shopID, limit, offset))
}

// block: notun row hole created; already thakle ager row (kichu bodlay na).
func block(ctx context.Context, tx *sql.Tx, shopID, staffID int64, phone string, reason any) (json.RawMessage, bool, error) {
	var row json.RawMessage
	err := tx.QueryRowContext(ctx, `INSERT INTO blocked_phones (shop_id, phone, reason, staff_id) VALUES ($1, $2, $3, $4)
		ON CONFLICT (shop_id, phone) DO NOTHING RETURNING to_jsonb(blocked_phones.*) - 'shop_id'`, shopID, phone, reason, staffID).Scan(&row)
	if !errors.Is(err, sql.ErrNoRows) {
		return row, err == nil, err
	}
	err = tx.QueryRowContext(ctx, `SELECT to_jsonb(b) - 'shop_id' FROM blocked_phones b WHERE shop_id = $1 AND phone = $2`,
		shopID, phone).Scan(&row)
	return row, false, err
}

func unblock(ctx context.Context, tx *sql.Tx, shopID int64, phone string) (reason any, found bool, err error) {
	var r sql.NullString
	err = tx.QueryRowContext(ctx, `DELETE FROM blocked_phones WHERE shop_id = $1 AND phone = $2 RETURNING reason`, shopID, phone).Scan(&r)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if r.Valid {
		reason = r.String
	}
	return reason, err == nil, err
}

func userByPhone(ctx context.Context, q querier, shopID int64, phone string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM users WHERE shop_id = $1 AND phone = $2`, shopID, phone).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}
