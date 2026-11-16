package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/jobs"
	"vendrix/backend/internal/notify"
)

// jobPayload (8.10 item 1): shudhu order_id; token, phone payload e na.
type jobPayload struct {
	OrderID int64 `json:"order_id"`
}

// enqueuePlaced (8.6 step 18): order er tx e, tai commit holei job. Pending COD (confirm lage) e auto-cancel
// RunAt = placed_at + order er shomoyer setting (8.19 item 12 viii).
func enqueuePlaced(ctx context.Context, tx *sql.Tx, shopID, orderID int64, autoCancel time.Time) error {
	p := jobPayload{orderID}
	for _, n := range []jobs.New{
		{ShopID: shopID, Kind: "sms.order_placed", Payload: p},
		{ShopID: shopID, Kind: "alert.new_order", Payload: p},
	} {
		if err := jobs.Enqueue(ctx, tx, n); err != nil {
			return err
		}
	}
	if autoCancel.IsZero() {
		return nil
	}
	return jobs.Enqueue(ctx, tx, jobs.New{ShopID: shopID, Kind: "order.auto_cancel_unconfirmed", Payload: p, RunAt: autoCancel,
		DedupeKey: fmt.Sprintf("order.auto_cancel_unconfirmed:%d", orderID)})
}

// enqueueCancelled (8.8 cancel er SMS): dedupe, ek order ekbar-i cancel hoy.
func enqueueCancelled(ctx context.Context, tx *sql.Tx, shopID, orderID int64) error {
	return jobs.Enqueue(ctx, tx, jobs.New{ShopID: shopID, Kind: "sms.cancelled", Payload: jobPayload{orderID},
		DedupeKey: fmt.Sprintf("sms.cancelled:%d", orderID)})
}

// JobHandlers (8.17): order er job, worker e register (cmd/api). Shob idempotent: SMS er lekha order row theke
// abar banano (retry e ekoi link), auto-cancel pending na hole kichu na.
func JobHandlers(d *sql.DB, sms notify.SMS, log *slog.Logger, confirmSecret string) map[string]jobs.Handler {
	return map[string]jobs.Handler{
		"sms.order_placed": func(ctx context.Context, j jobs.Job) error {
			o, err := loadSMSOrder(ctx, d, j, "sms.order_placed", "sms.order_confirm")
			if err != nil {
				return err
			}
			key, link := "sms.order_placed", "https://"+o.domain+"/order/"+o.token
			if o.confirmHash != nil {
				tok := ConfirmToken(confirmSecret, o.token)
				// secret rotate er por hash na mille link 404 dito (8.10 item 5): pathano na, fail ar alert
				if string(ConfirmHash(tok)) != string(o.confirmHash) {
					return fmt.Errorf("sms.order_placed: order %d er confirm_token_hash CONFIRM_SECRET er sathe mile na", o.id)
				}
				key, link = "sms.order_confirm", "https://"+o.domain+"/confirm/"+tok
			}
			return sms.Send(ctx, j.ShopID, o.phone, notify.OrderText(key, o.locale, o.override[key], o.shop, o.number, link))
		},
		"sms.cancelled": func(ctx context.Context, j jobs.Job) error {
			o, err := loadSMSOrder(ctx, d, j, "sms.cancelled")
			if err != nil {
				return err
			}
			return sms.Send(ctx, j.ShopID, o.phone, notify.OrderText("sms.cancelled", o.locale, o.override["sms.cancelled"], o.shop, o.number, ""))
		},
		// ekhon shudhu log; email/SMS M12 e (ALERT_EMAIL)
		"alert.new_order": func(ctx context.Context, j jobs.Job) error {
			var p jobPayload
			if err := json.Unmarshal(j.Payload, &p); err != nil {
				return err
			}
			var number, method string
			var total int64
			if err := d.QueryRowContext(ctx, `SELECT order_number, total, payment_method FROM orders WHERE shop_id = $1 AND id = $2`,
				j.ShopID, p.OrderID).Scan(&number, &total, &method); err != nil {
				return err
			}
			log.InfoContext(ctx, "new order", "shop_id", j.ShopID, "order_id", p.OrderID, "order_number", number, "total", total,
				"payment_method", method)
			return nil
		},
		"order.auto_cancel_unconfirmed": func(ctx context.Context, j jobs.Job) error {
			var p jobPayload
			if err := json.Unmarshal(j.Payload, &p); err != nil {
				return err
			}
			return db.InTx(ctx, d, func(tx *sql.Tx) error {
				o, err := lockOrder(ctx, tx, j.ShopID, p.OrderID)
				if err != nil {
					return err
				}
				if o.status != Pending || o.method != "cod" {
					return nil // confirm ba cancel hoye geche
				}
				return transitionOrder(ctx, tx, j.ShopID, o, Cancelled, By{Actor: System}, "auto_cancel_unconfirmed")
			})
		},
	}
}

// smsOrder: order SMS er jonno order, shop er naam (locale), primary domain ar translations override (key → locale → lekha).
type smsOrder struct {
	id                                 int64
	token, number, phone, locale, shop string
	domain                             string
	confirmHash                        []byte
	override                           map[string]map[string]string
}

func loadSMSOrder(ctx context.Context, d *sql.DB, j jobs.Job, keys ...string) (smsOrder, error) {
	var p jobPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return smsOrder{}, err
	}
	o := smsOrder{id: p.OrderID}
	var override []byte
	err := d.QueryRowContext(ctx, `SELECT o.public_token, o.order_number, o.customer_phone, o.locale,
		  coalesce(nullif(s.name->>o.locale, ''), s.name->>'bn'), coalesce(sd.domain, ''), o.confirm_token_hash,
		  (SELECT coalesce(jsonb_object_agg(k.key, k.v), '{}') FROM (SELECT t.key, jsonb_object_agg(t.locale, t.value) v
		   FROM translations t WHERE t.shop_id = o.shop_id AND t.key = ANY($3) GROUP BY t.key) k)
		FROM orders o JOIN shops s ON s.id = o.shop_id
		LEFT JOIN shop_domains sd ON sd.shop_id = o.shop_id AND sd.is_primary
		WHERE o.shop_id = $1 AND o.id = $2`, j.ShopID, p.OrderID, keys).Scan(&o.token, &o.number, &o.phone, &o.locale,
		&o.shop, &o.domain, &o.confirmHash, &override)
	if errors.Is(err, sql.ErrNoRows) {
		return o, fmt.Errorf("%s: order %d nai (shop %d)", j.Kind, p.OrderID, j.ShopID)
	}
	if err != nil {
		return o, err
	}
	if o.domain == "" {
		return o, fmt.Errorf("%s: shop %d er primary domain nai", j.Kind, j.ShopID)
	}
	return o, json.Unmarshal(override, &o.override)
}
