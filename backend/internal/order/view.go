package order

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// View (7.5, 8.19 item 6): GET /api/orders/{token}; ConfirmSummary er mask (phone, thikana shudhu district + area),
// token na (Rule 11: link je pay se dekhe, kintu link forward hote pare).
type View struct {
	ConfirmSummary
	RecipientName        string     `json:"recipient_name"`
	RecipientPhoneMasked string     `json:"recipient_phone_masked"`
	Subtotal             int64      `json:"subtotal"`
	Discount             int64      `json:"discount"`
	DeliveryCharge       int64      `json:"delivery_charge"`
	DeliveryDeadline     *time.Time `json:"delivery_deadline"`
	CanCancel            bool       `json:"can_cancel"`
}

// ViewByToken: public_token diye; na mille ErrNotFound.
func ViewByToken(ctx context.Context, d *sql.DB, shopID int64, token string) (View, error) {
	var v View
	var id int64
	var rp, status, method, payStatus string
	err := d.QueryRowContext(ctx, `SELECT id, recipient_name, recipient_phone, subtotal, discount, delivery_charge,
		delivery_deadline, status, payment_method, payment_status FROM orders WHERE shop_id = $1 AND public_token = $2`,
		shopID, token).Scan(&id, &v.RecipientName, &rp, &v.Subtotal, &v.Discount, &v.DeliveryCharge, &v.DeliveryDeadline,
		&status, &method, &payStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	v.RecipientPhoneMasked = maskPhone(rp)
	if v.ConfirmSummary, err = summary(ctx, d, shopID, id); err != nil {
		return v, err
	}
	rows, err := d.QueryContext(ctx, `SELECT status FROM order_fulfillments WHERE shop_id = $1 AND order_id = $2 ORDER BY id`,
		shopID, id)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	var fs []FStatus
	for rows.Next() {
		var f FStatus
		if err := rows.Scan(&f); err != nil {
			return v, err
		}
		fs = append(fs, f)
	}
	// Cancel route er ekoi niyom (TransitionOrder); button dekhano na-dekhano matro.
	v.CanCancel = CanTransitionOrder(Status(status), Cancelled, Customer, Payment{Method: method, Paid: payStatus == "paid"}, fs)
	return v, rows.Err()
}

// Tracked (8.19 item 6): POST /api/orders/track er uttor; token, naam, phone na.
type Tracked struct {
	OrderNumber   string    `json:"order_number"`
	DisplayStatus string    `json:"display_status"`
	PlacedAt      time.Time `json:"placed_at"`
	History       []Event   `json:"history"`
	Tracking      *struct{} `json:"tracking"` // courier M7 e; ekhon null
}

type Event struct {
	Status string    `json:"status"`
	At     time.Time `json:"at"`
}

// Track: number + phone (customer ba recipient, normalize kora) ekshathe mille; na hole ErrNotFound (kon ta bhul
// bola na). History shudhu order level er (fulfillment er ghotona courier er sathe M7 e).
func Track(ctx context.Context, d *sql.DB, shopID int64, number, phone string) (Tracked, error) {
	var t Tracked
	var id int64
	err := d.QueryRowContext(ctx, `SELECT id, order_number, display_status, placed_at FROM orders
		WHERE shop_id = $1 AND order_number = $2 AND (customer_phone = $3 OR recipient_phone = $3)`,
		shopID, number, phone).Scan(&id, &t.OrderNumber, &t.DisplayStatus, &t.PlacedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	rows, err := d.QueryContext(ctx, `SELECT to_status, created_at FROM order_status_history
		WHERE shop_id = $1 AND order_id = $2 AND fulfillment_id IS NULL AND from_status IS DISTINCT FROM to_status ORDER BY id`, shopID, id)
	if err != nil {
		return t, err
	}
	defer rows.Close()
	t.History = []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Status, &e.At); err != nil {
			return t, err
		}
		t.History = append(t.History, e)
	}
	return t, rows.Err()
}

// Card (8.19 item 12 x): /api/me/orders er ek order.
type Card struct {
	OrderNumber   string    `json:"order_number"`
	PublicToken   string    `json:"public_token"`
	DisplayStatus string    `json:"display_status"`
	Total         int64     `json:"total"`
	PlacedAt      time.Time `json:"placed_at"`
}

// MyOrders (7.3, Rule 16): shudhu ei user er order, notun age.
func MyOrders(ctx context.Context, d *sql.DB, shopID, userID int64, page, limit int) ([]Card, int64, error) {
	var total int64
	if err := d.QueryRowContext(ctx, `SELECT count(*) FROM orders WHERE shop_id = $1 AND user_id = $2`, shopID, userID).
		Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := d.QueryContext(ctx, `SELECT order_number, public_token, display_status, total, placed_at FROM orders
		WHERE shop_id = $1 AND user_id = $2 ORDER BY placed_at DESC, id DESC LIMIT $3 OFFSET $4`,
		shopID, userID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Card
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.OrderNumber, &c.PublicToken, &c.DisplayStatus, &c.Total, &c.PlacedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}
