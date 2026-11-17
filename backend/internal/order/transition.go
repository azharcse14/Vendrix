package order

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"vendrix/backend/internal/coupon"
	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
)

// By: ke bodlay (history er actor_type/actor_id); Vendor e VendorID (shudhu nijer fulfillment).
type By struct {
	Actor    Actor
	ID       int64
	VendorID int64
}

// ByStaff (11.7 item 13 i): admin staff → By. Ochena role e Actor khali, tai kono transition pay na
// (fail-closed); vendor_id chhara vendor role e VendorID 0, kono fulfillment er na.
func ByStaff(st httpx.Staff) By {
	switch st.Role {
	case "owner", "admin", "staff":
		return By{Actor: Staff, ID: st.ID}
	case "vendor_owner", "vendor_staff":
		b := By{Actor: Vendor, ID: st.ID}
		if st.VendorID != nil {
			b.VendorID = *st.VendorID
		}
		return b
	}
	return By{ID: st.ID}
}

var (
	ErrNotFound          = errors.New("order: nai")                       // 404 not_found
	ErrInvalidTransition = errors.New("order: ei bodol hoy na")           // 409 invalid_transition
	ErrPaymentRequired   = errors.New("order: online order, taka ase ni") // 409 payment_required
	errRefundOverPaid    = errors.New("order: refund paid er beshi")      // bug, ROLLBACK
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

// TransitionFulfillment (8.8): shob bodol er side effect: → cancelled, → packed (ojon chhara: shudhu fixed item er
// fulfillment; ojon shoho Pack), packed → open (undo), → shipped (shipment age Ship e), → delivered (active shipment
// delivered, COD paid), → returned (restock chhara; item ferot ReturnFulfillment er restock diye). Shob fulfillment
// cancelled hole order o cancelled (System), na hole display_status abar.
func TransitionFulfillment(ctx context.Context, tx *sql.Tx, shopID, fulfillmentID int64, to FStatus, by By, note string) error {
	return transitionFulfillment(ctx, tx, shopID, fulfillmentID, to, by, note, nil, nil)
}

// transitionFulfillment: weights = pack er ojon-variable item → packed_weight_g; pack e note = customer_ok_note;
// restock = return e stock e ferot item (oi fulfillment er active item).
func transitionFulfillment(ctx context.Context, tx *sql.Tx, shopID, fulfillmentID int64, to FStatus, by By, note string,
	weights map[int64]int, restockIDs []int64) error {
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
	switch to {
	case FCancelled:
		if err := cancelFulfillments(ctx, tx, shopID, o, []int{i}, by, note); err != nil {
			return err
		}
		if AllCancelled(o.statuses()) {
			return transitionOrder(ctx, tx, shopID, o, Cancelled, By{Actor: System}, note)
		}
	case Packed:
		if o.status != Confirmed {
			return ErrInvalidTransition
		}
		if err := pack(ctx, tx, shopID, o, fulfillmentID, weights, note != ""); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillments SET status = 'packed', packed_at = now()
			WHERE shop_id = $1 AND id = $2`, shopID, fulfillmentID); err != nil {
			return err
		}
		if err := history(ctx, tx, shopID, o.id, fulfillmentID, string(o.fs[i].status), string(Packed), by, note); err != nil {
			return err
		}
		o.fs[i].status = Packed
	case Open: // pack undo (8.8 v1.37)
		if note == "" {
			return FieldError("note")
		}
		if o.method != "cod" {
			return ErrInvalidTransition // prepaid M10 e abar dekha
		}
		var shipped bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM shipments WHERE shop_id = $1 AND fulfillment_id = $2
			AND status NOT IN ('cancelled', 'returned'))`, shopID, fulfillmentID).Scan(&shipped); err != nil {
			return err
		}
		if shipped {
			return ErrInvalidTransition
		}
		if err := unpack(ctx, tx, shopID, o, fulfillmentID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillments SET status = 'open', packed_at = NULL
			WHERE shop_id = $1 AND id = $2`, shopID, fulfillmentID); err != nil {
			return err
		}
		if err := history(ctx, tx, shopID, o.id, fulfillmentID, string(Packed), string(Open), by, note); err != nil {
			return err
		}
		o.fs[i].status = Open
	case Shipped: // shipment Ship e age (11.7 item 5); /transition e shipment chhara 409
		var n int64
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM shipments WHERE shop_id = $1 AND fulfillment_id = $2 AND status IN ('booked', 'picked', 'in_transit')`,
			shopID, fulfillmentID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return ErrInvalidTransition
		}
		if err := setFStatus(ctx, tx, shopID, o, i, Shipped, by, note); err != nil {
			return err
		}
	case Delivered:
		res, err := tx.ExecContext(ctx, `UPDATE shipments SET status = 'delivered', delivered_at = now()
			WHERE shop_id = $1 AND fulfillment_id = $2 AND status IN ('booked', 'picked', 'in_transit')`, shopID, fulfillmentID)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			if err == nil {
				err = ErrInvalidTransition // parcel ferot/bad: abar ship ba return
			}
			return err
		}
		if err := setFStatus(ctx, tx, shopID, o, i, Delivered, by, note); err != nil {
			return err
		}
	case Returned:
		if err := returnFulfillment(ctx, tx, shopID, o, i, restockIDs, note); err != nil {
			return err
		}
		if err := setFStatus(ctx, tx, shopID, o, i, Returned, by, note); err != nil {
			return err
		}
	}
	if err := codPaid(ctx, tx, shopID, o); err != nil {
		return err
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
	var items []int64
	rows, err := tx.QueryContext(ctx, `SELECT id FROM order_items WHERE shop_id = $1 AND fulfillment_id = ANY($2) AND status = 'active'`,
		shopID, ids)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		items = append(items, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if err := restock(ctx, tx, shopID, items, "cancel"); err != nil {
		return err
	}
	prepaid := o.method != "cod" && (o.payStatus == "paid" || o.payStatus == "partially_refunded")
	for _, i := range idx {
		f := &o.fs[i]
		if prepaid && f.total > 0 {
			reason := note
			if reason == "" {
				reason = "cancel"
			}
			if err := insertRefund(ctx, tx, shopID, o, f.id, f.total, "cancel", reason); err != nil {
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

// restock (8.12): item (active; removed item er stock edit e ferot hoye geche) er qty variant e ferot, batch e
// order_item_batches theke; movement reason (cancel ba return; batch er ongsho batch_id shoho). Batch already expire (shop er aj er
// age) hole oi ongsho batch e na ar variant theke abar bad, movement expired (11.7 item 3; inventory.expire er moto).
// Variant, tarpor batch id er kram e lock (8.8; UPDATE ... FROM er nijer kram e deadlock hote parto).
func restock(ctx context.Context, tx *sql.Tx, shopID int64, items []int64, reason string) error {
	for _, q := range []string{
		`SELECT 1 FROM product_variants WHERE shop_id = $1 AND id IN (SELECT variant_id FROM order_items
			WHERE shop_id = $1 AND id = ANY($2) AND status = 'active') ORDER BY id FOR UPDATE`,
		`UPDATE product_variants v SET stock = v.stock + x.q
			FROM (SELECT variant_id, sum(quantity) q FROM order_items
				WHERE shop_id = $1 AND id = ANY($2) AND status = 'active' GROUP BY variant_id) x
			WHERE v.shop_id = $1 AND v.id = x.variant_id`,
		`SELECT 1 FROM inventory_batches WHERE shop_id = $1 AND id IN (SELECT ob.batch_id FROM order_items i
			JOIN order_item_batches ob ON ob.shop_id = i.shop_id AND ob.order_item_id = i.id
			WHERE i.shop_id = $1 AND i.id = ANY($2) AND i.status = 'active') ORDER BY id FOR UPDATE`,
		`UPDATE inventory_batches b SET quantity_remaining = b.quantity_remaining + x.q
			FROM (SELECT ob.batch_id, sum(ob.quantity) q FROM order_items i
				JOIN order_item_batches ob ON ob.shop_id = i.shop_id AND ob.order_item_id = i.id
				WHERE i.shop_id = $1 AND i.id = ANY($2) AND i.status = 'active' GROUP BY ob.batch_id) x
			WHERE b.shop_id = $1 AND b.id = x.batch_id AND NOT COALESCE(b.expiry_date < (SELECT (now() AT TIME ZONE timezone)::date FROM shops WHERE id = $1), false)`,
		`UPDATE product_variants v SET stock = v.stock - x.q
			FROM (SELECT ob.variant_id, sum(ob.quantity) q FROM order_items i
				JOIN order_item_batches ob ON ob.shop_id = i.shop_id AND ob.order_item_id = i.id
				JOIN inventory_batches b ON b.shop_id = ob.shop_id AND b.id = ob.batch_id
				WHERE i.shop_id = $1 AND i.id = ANY($2) AND i.status = 'active' AND COALESCE(b.expiry_date < (SELECT (now() AT TIME ZONE timezone)::date FROM shops WHERE id = $1), false)
				GROUP BY ob.variant_id) x
			WHERE v.shop_id = $1 AND v.id = x.variant_id`,
		`INSERT INTO inventory_movements (shop_id, variant_id, batch_id, change, reason, order_id)
			SELECT $1, i.variant_id, ob.batch_id, ob.quantity, $3, i.order_id FROM order_items i
				JOIN order_item_batches ob ON ob.shop_id = i.shop_id AND ob.order_item_id = i.id
				WHERE i.shop_id = $1 AND i.id = ANY($2) AND i.status = 'active'
			UNION ALL
			SELECT $1, i.variant_id, ob.batch_id, -ob.quantity, 'expired', i.order_id FROM order_items i
				JOIN order_item_batches ob ON ob.shop_id = i.shop_id AND ob.order_item_id = i.id
				JOIN inventory_batches b ON b.shop_id = ob.shop_id AND b.id = ob.batch_id
				WHERE i.shop_id = $1 AND i.id = ANY($2) AND i.status = 'active' AND COALESCE(b.expiry_date < (SELECT (now() AT TIME ZONE timezone)::date FROM shops WHERE id = $1), false)
			UNION ALL
			SELECT $1, i.variant_id, NULL, i.quantity - COALESCE(sum(ob.quantity), 0), $3, i.order_id FROM order_items i
				LEFT JOIN order_item_batches ob ON ob.shop_id = i.shop_id AND ob.order_item_id = i.id
				WHERE i.shop_id = $1 AND i.id = ANY($2) AND i.status = 'active'
				GROUP BY i.id HAVING i.quantity - COALESCE(sum(ob.quantity), 0) > 0`,
	} {
		args := []any{shopID, items}
		if strings.Contains(q, "$3") { // shudhu movement er INSERT e reason
			args = append(args, reason)
		}
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}
	return nil
}

// insertRefund (8.8, 8.11): fid 0 = NULL (edit, puro order). cancel e refunds_cancel_once e ekbar-i; mot refund (failed
// bad) paid er beshi na; paid = succeeded payment + delivered shipment er cod_amount (COD e payment row nai; v1.43; returnFulfillment e ekoi hishab). Method payment er (COD e cash, v1.43). Status pending: gateway er refund M10 e.
func insertRefund(ctx context.Context, tx *sql.Tx, shopID int64, o *locked, fid, amount int64, kind, reason string) error {
	res, err := tx.ExecContext(ctx, `INSERT INTO refunds (shop_id, order_id, fulfillment_id, payment_id, amount, method, kind, reason)
		SELECT $1, $2, NULLIF($3::bigint, 0), (SELECT max(id) FROM payments WHERE shop_id = $1 AND order_id = $2 AND status = 'succeeded'),
			$4::bigint, CASE WHEN $5 = 'cod' THEN 'cash' ELSE $5 END, $7, $6
		WHERE (SELECT COALESCE(sum(amount), 0) FROM refunds WHERE shop_id = $1 AND order_id = $2 AND status <> 'failed') + $4::bigint
			<= (SELECT COALESCE(sum(amount), 0) FROM payments WHERE shop_id = $1 AND order_id = $2 AND status = 'succeeded')
	+ (SELECT COALESCE(sum(cod_amount), 0) FROM shipments WHERE shop_id = $1 AND order_id = $2 AND status = 'delivered')`,
		shopID, o.id, fid, amount, o.method, reason, kind)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		err = errRefundOverPaid
	}
	return err
}

// setFStatus: shipped/delivered/returned: status, tar shomoy column, history.
func setFStatus(ctx context.Context, tx *sql.Tx, shopID int64, o *locked, i int, to FStatus, by By, note string) error {
	f := &o.fs[i]
	if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillments SET status = $3::text,
		shipped_at = CASE WHEN $3::text = 'shipped' THEN now() ELSE shipped_at END,
		delivered_at = CASE WHEN $3::text = 'delivered' THEN now() ELSE delivered_at END,
		returned_at = CASE WHEN $3::text = 'returned' THEN now() ELSE returned_at END
		WHERE shop_id = $1 AND id = $2`, shopID, f.id, string(to)); err != nil {
		return err
	}
	if err := history(ctx, tx, shopID, o.id, f.id, string(f.status), string(to), by, note); err != nil {
		return err
	}
	f.status = to
	return nil
}

// returnFulfillment (8.8 "→ returned", 11.7 item 6 v1.43): note lagbe; rasta e shipment thakle 409 (age shipment
// returned); restockIDs oi fulfillment er active item (na hole FieldError), stock/batch e ferot (movement return,
// expire batch er niyom restock er). Taka newa hoye thakle (prepaid paid, ba COD e delivered theke) refund return
// min(total, paid − ager refund), 0 hole na.
func returnFulfillment(ctx context.Context, tx *sql.Tx, shopID int64, o *locked, i int, restockIDs []int64, note string) error {
	f := o.fs[i]
	if note == "" {
		return FieldError("note")
	}
	var n int64
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM order_items WHERE shop_id = $1 AND fulfillment_id = $2 AND status = 'active'
		AND id = ANY($3)`, shopID, f.id, restockIDs).Scan(&n); err != nil {
		return err
	}
	if n != int64(len(restockIDs)) {
		return FieldError("restock_item_ids")
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM shipments WHERE shop_id = $1 AND fulfillment_id = $2 AND status IN ('booked', 'picked', 'in_transit')`,
		shopID, f.id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrInvalidTransition
	}
	if len(restockIDs) > 0 {
		if err := restock(ctx, tx, shopID, restockIDs, "return"); err != nil {
			return err
		}
	}
	paid := o.payStatus == "paid" || o.payStatus == "partially_refunded"
	if o.method == "cod" {
		paid = f.status == Delivered // shipped theke ferot: customer taka dey ni
	}
	if !paid || f.total == 0 {
		return nil
	}
	var left int64
	if err := tx.QueryRowContext(ctx, `SELECT (SELECT COALESCE(sum(amount), 0) FROM payments WHERE shop_id = $1 AND order_id = $2 AND status = 'succeeded')
	+ (SELECT COALESCE(sum(cod_amount), 0) FROM shipments WHERE shop_id = $1 AND order_id = $2 AND status = 'delivered') - (SELECT COALESCE(sum(amount), 0) FROM refunds
		WHERE shop_id = $1 AND order_id = $2 AND status <> 'failed')`, shopID, o.id).Scan(&left); err != nil {
		return err
	}
	if amount := min(f.total, left); amount > 0 {
		return insertRefund(ctx, tx, shopID, o, f.id, amount, "return", note)
	}
	return nil
}

// codPaid (8.8 v1.43): COD e kono fulfillment open/packed/shipped nai ar kom-poksha ekta shipment delivered hole
// payment_status paid (taka rider/courier er hate, M11 settlement).
func codPaid(ctx context.Context, tx *sql.Tx, shopID int64, o *locked) error {
	if o.method != "cod" || o.payStatus != "unpaid" {
		return nil
	}
	for _, f := range o.fs {
		if f.status == Open || f.status == Packed || f.status == Shipped {
			return nil
		}
	}
	res, err := tx.ExecContext(ctx, `UPDATE orders SET payment_status = 'paid' WHERE shop_id = $1 AND id = $2
		AND EXISTS (SELECT 1 FROM shipments WHERE shop_id = $1 AND order_id = $2 AND status = 'delivered')`, shopID, o.id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		o.payStatus = "paid"
	}
	return nil
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
