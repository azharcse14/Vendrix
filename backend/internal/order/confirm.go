package order

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"vendrix/backend/internal/db"
)

// ConfirmToken (8.10 item 1): base32(HMAC-SHA256(secret, "cod-confirm:" + public_token)) er prothom 26 okkhor
// (130 bit). public_token theke abar banano jay, tai SMS retry te ekoi link; DB te shudhu ConfirmHash.
func ConfirmToken(secret, publicToken string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("cod-confirm:" + publicToken))
	return base32.StdEncoding.EncodeToString(m.Sum(nil))[:26]
}

// ConfirmHash (8.10 item 2): sha256(uppercase token); link e case bodlaleo chole.
func ConfirmHash(token string) []byte {
	sum := sha256.Sum256([]byte(strings.ToUpper(token)))
	return sum[:]
}

// ConfirmItem: confirm page er line (naam ar variant snapshot {bn, en}).
type ConfirmItem struct {
	Name         json.RawMessage `json:"name"`
	VariantLabel json.RawMessage `json:"variant_label"`
	Quantity     int             `json:"quantity"`
	LineTotal    int64           `json:"line_total"`
}

// ConfirmSummary (8.10 item 2, 8.19 item 6 er mask): phone mask, thikana shudhu district + area; token na.
type ConfirmSummary struct {
	OrderNumber   string        `json:"order_number"`
	DisplayStatus string        `json:"display_status"`
	PlacedAt      time.Time     `json:"placed_at"`
	CustomerName  string        `json:"customer_name"`
	PhoneMasked   string        `json:"phone_masked"`
	District      string        `json:"district"`
	Area          string        `json:"area"`
	PaymentMethod string        `json:"payment_method"`
	Total         int64         `json:"total"`
	Items         []ConfirmItem `json:"items"`
}

// ConfirmView: GET /api/confirm/{confirm_token}; na mille ErrNotFound.
func ConfirmView(ctx context.Context, d *sql.DB, shopID int64, token string) (ConfirmSummary, error) {
	var s ConfirmSummary
	var id int64
	var ph string
	err := d.QueryRowContext(ctx, `SELECT id, order_number, display_status, placed_at, customer_name, customer_phone,
		ship_district, ship_area, payment_method, total FROM orders WHERE shop_id = $1 AND confirm_token_hash = $2`,
		shopID, ConfirmHash(token)).Scan(&id, &s.OrderNumber, &s.DisplayStatus, &s.PlacedAt, &s.CustomerName, &ph,
		&s.District, &s.Area, &s.PaymentMethod, &s.Total)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	if err != nil {
		return s, err
	}
	s.PhoneMasked = maskPhone(ph)
	rows, err := d.QueryContext(ctx, `SELECT product_name, variant_label, quantity, line_total FROM order_items
		WHERE shop_id = $1 AND order_id = $2 AND status = 'active' ORDER BY id`, shopID, id)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	s.Items = []ConfirmItem{}
	for rows.Next() {
		var it ConfirmItem
		if err := rows.Scan(&it.Name, &it.VariantLabel, &it.Quantity, &it.LineTotal); err != nil {
			return s, err
		}
		s.Items = append(s.Items, it)
	}
	return s, rows.Err()
}

// ConfirmByToken: POST /api/confirm/{confirm_token} → TransitionOrder(→ confirmed, customer) (8.10 item 2).
func ConfirmByToken(ctx context.Context, d *sql.DB, shopID int64, token string, by By) error {
	return db.InTx(ctx, d, func(tx *sql.Tx) error {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM orders WHERE shop_id = $1 AND confirm_token_hash = $2`,
			shopID, ConfirmHash(token)).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return TransitionOrder(ctx, tx, shopID, id, Confirmed, by, "")
	})
}
