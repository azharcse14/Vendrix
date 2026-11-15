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
	"vendrix/backend/internal/phone"
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

func (l logSMS) Send(_ context.Context, shopID int64, to, text string) error {
	l.log.Info("sms (local, pathano hoyni)", "shop_id", shopID, "phone", phone.Mask(to), "text", text)
	return nil
}

type noProvider struct{}

func (noProvider) Send(context.Context, int64, string, string) error { return ErrNoProvider }

// OTPSource (8.2 step 4, v1.19): OTP job er payload ({otp_id, nonce, ct}) theke phone ar code; auth dey
// (key ar otp_codes notify jane na).
type OTPSource func(ctx context.Context, shopID, otpID int64, nonce, ct []byte) (phone, code string, err error)

// SendSMSJob: sms.send job er handler; payload {"phone": E.164, "text": ...}, ba OTP hole {otp_id, nonce, ct}.
// Done hole worker payload {} kore (8.17).
func SendSMSJob(s SMS, otp OTPSource) jobs.Handler {
	return func(ctx context.Context, j jobs.Job) error {
		var p struct {
			Phone string `json:"phone"`
			Text  string `json:"text"`
			OTPID int64  `json:"otp_id"`
			Nonce []byte `json:"nonce"`
			CT    []byte `json:"ct"`
		}
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return fmt.Errorf("sms.send payload: %w", err)
		}
		if p.OTPID != 0 {
			if otp == nil {
				return errors.New("sms.send: OTP payload, kintu OTPSource nai")
			}
			var code string
			var err error
			if p.Phone, code, err = otp(ctx, j.ShopID, p.OTPID, p.Nonce, p.CT); err != nil {
				return fmt.Errorf("sms.send otp %d: %w", p.OTPID, err)
			}
			p.Text = otpText(code)
		}
		if p.Phone == "" || p.Text == "" {
			return errors.New("sms.send payload e phone ba text nai")
		}
		return s.Send(ctx, j.ShopID, p.Phone, p.Text)
	}
}

// otpText: sms.otp er lekha. ponytail: ekhon ekta Bangla lekha; template, locale ar shop override M4 step 9 e (8.16).
func otpText(code string) string {
	return "আপনার কোড " + code + "। ৫ মিনিট চলবে, কাউকে বলবেন না।"
}

// SecurityAlertJob (8.17 alert.security, Q9): ekhon shudhu slog.Warn (payload e phone mask); email/webhook M12 e.
func SecurityAlertJob(log *slog.Logger) jobs.Handler {
	return func(ctx context.Context, j jobs.Job) error {
		log.WarnContext(ctx, "security alert", "job_id", j.ID, "shop_id", j.ShopID, "payload", string(j.Payload))
		return nil
	}
}
