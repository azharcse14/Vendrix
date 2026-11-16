package order

import (
	"context"
	"database/sql"
	"errors"

	"vendrix/backend/internal/coupon"
	"vendrix/backend/internal/db"
)

// By: ke bodlay (history er actor_type/actor_id); Vendor e VendorID (shudhu nijer fulfillment).
type By struct {
	Actor    Actor
	ID       int64
	VendorID int64
}

var (
	ErrNotFound          = errors.New("order: nai")                                    // 404 not_found
	ErrInvalidTransition = errors.New("order: ei bodol hoy na")                        // 409 invalid_transition
	ErrPaymentRequired   = errors.New("order: online order, taka ase ni")              // 409 payment_required
	ErrNotImplemented    = errors.New("order: packed/shipped/delivered/returned M7 e") // 8.19 item 8
	errRefundOverPaid    = errors.New("order: refund paid er beshi")                   // bug, ROLLBACK
)

type fRow struct {
	id, total int64
	status    FStatus
}

// locked: lock kora order ar tar fulfillment (id er kram e).
type locked struct {
	id                int64
	status            Status
	method, payStatus string
	fs                []fRow
}

func (o *locked) statuses() []FStatus {
	s := make([]FStatus, len(o.fs))
	for i, f := range o.fs {
		s[i] = f.status
	}
	return s
}

// lockOrder (8.8): order → payment → fulfillment; onno shob code ei kram e lock kore, tai deadlock na.
func lockOrder(ctx context.Context, tx *sql.Tx, shopID, orderID int64) (*locked, error) {
	o := &locked{id: orderID}
	err := tx.QueryRowContext(ctx, `SELECT status, payment_method, payment_status FROM orders
		WHERE shop_id = $1 AND id = $2 FOR UPDATE`, shopID, orderID).Scan(&o.status, &o.method, &o.payStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM payments WHERE shop_id = $1 AND order_id = $2 ORDER BY id FOR UPDATE`,
		shopID, orderID); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, total, status FROM order_fulfillments
		WHERE shop_id = $1 AND order_id = $2 ORDER BY id FOR UPDATE`, shopID, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var f fRow
		if err := rows.Scan(&f.id, &f.total, &f.status); err != nil {
			return nil, err
		}
		o.fs = append(o.fs, f)
	}
	return o, rows.Err()
}

// TransitionOrder (8.8): order er status bodlanor ekmatro path. Cancel e CancelCascade er fulfillment (side
// effect shoho, history te ei actor), coupon ferot; note cancel e cancel_reason o.
func TransitionOrder(ctx context.Context, tx *sql.Tx, shopID, orderID int64, to Status, by By, note string) error {
	o, err := lockOrder(ctx, tx, shopID, orderID)
	if err != nil {
		return err
	}
	return transitionOrder(ctx, tx, shopID, o, to, by, note)
}

func transitionOrder(ctx context.Context, tx *sql.Tx, shopID int64, o *locked, to Status, by By, note string) error {
	fs := o.statuses()
	paid := o.payStatus == "paid"
	if o.status == Pending && to == Confirmed && o.method != "cod" && !paid {
		return ErrPaymentRequired
	}
	if !CanTransitionOrder(o.status, to, by.Actor, Payment{Method: o.method, Paid: paid}, fs) {
		return ErrInvalidTransition
	}
	if to == Cancelled {
		if err := cancelFulfillments(ctx, tx, shopID, o, CancelCascade(fs), by, note); err != nil {
			return err
		}
		// coupon variant er pore (lock er kram)
		if err := coupon.Release(ctx, tx, shopID, o.id); err != nil {
			return err
		}
		if err := enqueueCancelled(ctx, tx, shopID, o.id); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET status = $3::text, display_status = $4,
		confirmed_at = CASE WHEN $3::text = 'confirmed' THEN now() ELSE confirmed_at END,
		cancelled_at = CASE WHEN $3::text = 'cancelled' THEN now() ELSE cancelled_at END,
		cancel_reason = CASE WHEN $3::text = 'cancelled' THEN NULLIF($5, '') ELSE cancel_reason END
		WHERE shop_id = $1 AND id = $2`, shopID, o.id, string(to), Summary(to, o.statuses()), note); err != nil {
		return err
	}
	if err := history(ctx, tx, shopID, o.id, 0, string(o.status), string(to), by, note); err != nil {
		return err
	}
	o.status = to
	return nil
}

// TransitionFulfillment (8.8): M6 e shudhu → cancelled er side effect; baki allowed bodol ErrNotImplemented
// (8.19 item 8). Shob fulfillment cancelled hole order o cancelled (System), na hole display_status abar.
func TransitionFulfillment(ctx context.Context, tx *sql.Tx, shopID, fulfillmentID int64, to FStatus, by By, note string) error {
	var orderID, vendorID int64
	err := tx.QueryRowContext(ctx, `SELECT order_id, vendor_id FROM order_fulfillments WHERE shop_id = $1 AND id = $2`,
		shopID, fulfillmentID).Scan(&orderID, &vendorID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && by.Actor == Vendor && vendorID != by.VendorID) {
		return ErrNotFound // onno vendor er ta ache kina-o bola na
	}
	if err != nil {
		return err
	}
	o, err := lockOrder(ctx, tx, shopID, orderID)
	if err != nil {
		return err
	}
	i := 0
	for i < len(o.fs) && o.fs[i].id != fulfillmentID {
		i++
	}
	if i == len(o.fs) {
		return ErrNotFound
	}
	if !CanTransitionFulfillment(o.fs[i].status, to, by.Actor) {
		return ErrInvalidTransition
	}
	if to != FCancelled {
		return ErrNotImplemented
	}
	if err := cancelFulfillments(ctx, tx, shopID, o, []int{i}, by, note); err != nil {
		return err
	}
	if AllCancelled(o.statuses()) {
		return transitionOrder(ctx, tx, shopID, o, Cancelled, By{Actor: System}, note)
	}
	_, err = tx.ExecContext(ctx, `UPDATE orders SET display_status = $3 WHERE shop_id = $1 AND id = $2`,
		shopID, o.id, Summary(o.status, o.statuses()))
	return err
}

// cancelFulfillments (8.8 "Fulfillment → cancelled"): stock ferot, pack hole batch ferot, prepaid e protiti
// fulfillment er bortoman total refund (0 hole na), status, history. Actor er onumoti caller dekheche.
func cancelFulfillments(ctx context.Context, tx *sql.Tx, shopID int64, o *locked, idx []int, by By, note string) error {
	if len(idx) == 0 {
		return nil
	}
	ids := make([]int64, len(idx))
	for k, i := range idx {
		ids[k] = o.fs[i].id
	}
	if err := restock(ctx, tx, shopID, ids); err != nil {
		return err
	}
	prepaid := o.method != "cod" && (o.payStatus == "paid" || o.payStatus == "partially_refunded")
	for _, i := range idx {
		f := &o.fs[i]
		if prepaid && f.total > 0 {
			if err := refundCancel(ctx, tx, shopID, o, f, note); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillments SET status = 'cancelled', cancelled_at = now()
			WHERE shop_id = $1 AND id = $2`, shopID, f.id); err != nil {
			return err
		}
		if err := history(ctx, tx, shopID, o.id, f.id, string(f.status), string(FCancelled), by, note); err != nil {
			return err
		}
		f.status = FCancelled
	}
	return nil
}

// restock (8.12): active item er qty variant e ferot (removed item er stock edit e ferot hoye geche), batch e
// order_item_batches theke; movement cancel (batch er ongsho batch_id shoho). Variant, tarpor batch id er kram e
// lock (8.8; UPDATE ... FROM er nijer kram e deadlock hote parto).
func restock(ctx context.Context, tx *sql.Tx, shopID int64, fids []int64) error {
	for _, q := range []string{
		`SELECT 1 FROM product_variants WHERE shop_id = $1 AND id IN (SELECT variant_id FROM order_items
			WHERE shop_id = $1 AND fulfillment_id = ANY($2) AND status = 'active') ORDER BY id FOR UPDATE`,
		`UPDATE product_variants v SET stock = v.stock + x.q
			FROM (SELECT variant_id, sum(quantity) q FROM order_items
				WHERE shop_id = $1 AND fulfillment_id = ANY($2) AND status = 'active' GROUP BY variant_id) x
			WHERE v.shop_id = $1 AND v.id = x.variant_id`,
		`SELECT 1 FROM inventory_batches WHERE shop_id = $1 AND id IN (SELECT ob.batch_id FROM order_items i
			JOIN order_item_batches ob ON ob.shop_id = i.shop_id AND ob.order_item_id = i.id
			WHERE i.shop_id = $1 AND i.fulfillment_id = ANY($2) AND i.status = 'active') ORDER BY id FOR UPDATE`,
		`UPDATE inventory_batches b SET quantity_remaining = b.quantity_remaining + x.q
			FROM (SELECT ob.batch_id, sum(ob.quantity) q FROM order_items i
				JOIN order_item_batches ob ON ob.shop_id = i.shop_id AND ob.order_item_id = i.id
				WHERE i.shop_id = $1 AND i.fulfillment_id = ANY($2) AND i.status = 'active' GROUP BY ob.batch_id) x
			WHERE b.shop_id = $1 AND b.id = x.batch_id`,
		`INSERT INTO inventory_movements (shop_id, variant_id, batch_id, change, reason, order_id)
			SELECT $1, i.variant_id, ob.batch_id, ob.quantity, 'cancel', i.order_id FROM order_items i
				JOIN order_item_batches ob ON ob.shop_id = i.shop_id AND ob.order_item_id = i.id
				WHERE i.shop_id = $1 AND i.fulfillment_id = ANY($2) AND i.status = 'active'
			UNION ALL
			SELECT $1, i.variant_id, NULL, i.quantity - COALESCE(sum(ob.quantity), 0), 'cancel', i.order_id FROM order_items i
				LEFT JOIN order_item_batches ob ON ob.shop_id = i.shop_id AND ob.order_item_id = i.id
				WHERE i.shop_id = $1 AND i.fulfillment_id = ANY($2) AND i.status = 'active'
				GROUP BY i.id HAVING i.quantity - COALESCE(sum(ob.quantity), 0) > 0`,
	} {
		if _, err := tx.ExecContext(ctx, q, shopID, fids); err != nil {
			return err
		}
	}
	return nil
}

// refundCancel (8.8): refunds_cancel_once e ekbar-i; mot refund (failed bad) paid er beshi na. Status pending:
// gateway er refund M10 e.
func refundCancel(ctx context.Context, tx *sql.Tx, shopID int64, o *locked, f *fRow, note string) error {
	if note == "" {
		note = "cancel"
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO refunds (shop_id, order_id, fulfillment_id, payment_id, amount, method, kind, reason)
		SELECT $1, $2, $3, (SELECT max(id) FROM payments WHERE shop_id = $1 AND order_id = $2 AND status = 'succeeded'),
			$4::bigint, $5, 'cancel', $6
		WHERE (SELECT COALESCE(sum(amount), 0) FROM refunds WHERE shop_id = $1 AND order_id = $2 AND status <> 'failed') + $4::bigint
			<= (SELECT COALESCE(sum(amount), 0) FROM payments WHERE shop_id = $1 AND order_id = $2 AND status = 'succeeded')`,
		shopID, o.id, f.id, f.total, o.method, note)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		err = errRefundOverPaid
	}
	return err
}

// linkActor (8.19 item 13 d): link (token) diye customer er kaj e actor_id shudhu order er nijer user hole; onno
// account er login e link khulle tar naam audit e na.
func linkActor(by By, user sql.NullInt64) By {
	if !user.Valid || user.Int64 != by.ID {
		by.ID = 0
	}
	return by
}

func history(ctx context.Context, tx *sql.Tx, shopID, orderID, fid int64, from, to string, by By, note string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO order_status_history (shop_id, order_id, fulfillment_id, from_status, to_status,
		actor_type, actor_id, note) VALUES ($1, $2, NULLIF($3, 0), $4, $5, $6, NULLIF($7, 0), NULLIF($8, ''))`,
		shopID, orderID, fid, from, to, string(by.Actor), by.ID, note)
	return err
}

// CancelByToken (7.5, Rule 11): customer cancel order link er public_token diye; reason cancel_reason e.
func CancelByToken(ctx context.Context, d *sql.DB, shopID int64, token string, by By, reason string) error {
	return db.InTx(ctx, d, func(tx *sql.Tx) error {
		var id int64
		var user sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT id, user_id FROM orders WHERE shop_id = $1 AND public_token = $2`, shopID, token).Scan(&id, &user)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return TransitionOrder(ctx, tx, shopID, id, Cancelled, linkActor(by, user), reason)
	})
}
