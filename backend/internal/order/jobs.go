package order

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"vendrix/backend/internal/jobs"
	"vendrix/backend/internal/notify"
)

// JobHandlers (8.17): order er job; M6 step 9 e asol code.
func JobHandlers(d *sql.DB, sms notify.SMS, log *slog.Logger, confirmSecret string) map[string]jobs.Handler {
	todo := func(context.Context, jobs.Job) error { return errors.New("order job: M6 step 9") }
	return map[string]jobs.Handler{
		"sms.order_placed":              todo,
		"sms.cancelled":                 todo,
		"alert.new_order":               todo,
		"order.auto_cancel_unconfirmed": todo,
	}
}
