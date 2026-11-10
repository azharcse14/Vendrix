// Package notify: customer ke SMS (8.17 er sms.send job). APP_ENV=local e SMS na pathiye log e (8.2 step 4);
// onno env e log mode hoy-i na, tai production e OTP log e jay na. Provider (shop_credentials, 16.1) M4 e.
//
// Feature doc: docs/features/jobs.md
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"vendrix/backend/internal/jobs"
)

// SMS: ek shop er hoye ek phone e lekha pathay. Provider bodlano shohoj rakhte interface (16.1).
type SMS interface {
	Send(ctx context.Context, shopID int64, phone, text string) error
}

// ErrNoProvider: local chhara env e SMS provider ekhono nai; sms.send job retry kore, shesh e failed ar alert.
var ErrNoProvider = errors.New("SMS provider nai (M4)")

// NewSMS: env onujayi sender.
func NewSMS(env string, log *slog.Logger) SMS {
	if env == "local" {
		return logSMS{log}
	}
	return noProvider{}
}

type logSMS struct{ log *slog.Logger }

func (l logSMS) Send(_ context.Context, shopID int64, phone, text string) error {
	l.log.Info("sms (local, pathano hoyni)", "shop_id", shopID, "phone", mask(phone), "text", text)
	return nil
}

type noProvider struct{}

func (noProvider) Send(context.Context, int64, string, string) error { return ErrNoProvider }

// mask (12.2): log e phone er shesh 3 digit.
func mask(phone string) string {
	if len(phone) <= 3 {
		return "***"
	}
	return "***" + phone[len(phone)-3:]
}

// SendSMSJob: sms.send job er handler; payload {"phone": E.164, "text": ...}.
func SendSMSJob(s SMS) jobs.Handler {
	return func(ctx context.Context, j jobs.Job) error {
		var p struct{ Phone, Text string }
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return fmt.Errorf("sms.send payload: %w", err)
		}
		if p.Phone == "" || p.Text == "" {
			return errors.New("sms.send payload e phone ba text nai")
		}
		return s.Send(ctx, j.ShopID, p.Phone, p.Text)
	}
}
