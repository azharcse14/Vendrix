// Package notify: customer ke SMS (8.17 er sms.send job). APP_ENV=local e SMS na pathiye log e (8.2 step 4);
// onno env e log mode hoy-i na, tai production e OTP log e jay na. Provider (shop_credentials) M12 er age (16.1).
//
// Feature doc: docs/features/jobs.md
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf16"

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

// OTP: OTPSource er result. Locale customer er users.locale (user na thakle khali, tai bn); Override shop er
// translations e sms.otp, locale → lekha (8.16).
type OTP struct {
	Phone, Code, Locale string
	Override            map[string]string
}

// OTPSource (8.2 step 4, v1.19): OTP job er payload ({otp_id, nonce, ct}) theke phone, code, locale ar
// override; auth dey (key, otp_codes ar users notify jane na).
type OTPSource func(ctx context.Context, shopID, otpID int64, nonce, ct []byte) (OTP, error)

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
			o, err := otp(ctx, j.ShopID, p.OTPID, p.Nonce, p.CT)
			if err != nil {
				return fmt.Errorf("sms.send otp %d: %w", p.OTPID, err)
			}
			p.Phone, p.Text = o.Phone, OTPText(o.Locale, o.Override, o.Code)
		}
		if p.Phone == "" || p.Text == "" {
			return errors.New("sms.send payload e phone ba text nai")
		}
		return s.Send(ctx, j.ShopID, p.Phone, p.Text)
	}
}

// otpDefault (8.16 v1.19): sms.otp er default; bn UCS-2 te ek SMS (≤ 70 unit), en GSM-7 e (≤ 160).
// TestOTPTemplateLength dhore.
var otpDefault = map[string]string{
	"bn": "আপনার কোড {code}। ৫ মিনিট চলবে, কাউকে বলবেন না।",
	"en": "Your code is {code}. Valid for 5 minutes. Do not share it.",
}

// OTPText: 9.8 er kram override[locale] → default[locale] → override[bn] → default[bn]; {code} chhara
// lekha bad, karon code chhara OTP SMS kaje lage na.
func OTPText(locale string, override map[string]string, code string) string {
	for _, t := range []string{override[locale], otpDefault[locale], override["bn"], otpDefault["bn"]} {
		if strings.Contains(t, "{code}") {
			return strings.ReplaceAll(t, "{code}", code)
		}
	}
	return strings.ReplaceAll(otpDefault["bn"], "{code}", code)
}

// orderDefault (8.16, 8.19 item 7): order SMS er default; bn ≤ 134 unit (2 part), en ≤ 160.
// TestOrderSMSTemplateLength dhore.
var orderDefault = map[string]map[string]string{
	"sms.order_placed": {
		"bn": "{shop}: অর্ডার {number} পেয়েছি। দেখুন {link}",
		"en": "{shop}: Order {number} received. View: {link}",
	},
	"sms.order_confirm": {
		"bn": "{shop}: অর্ডার {number} কনফার্ম করুন: {link}",
		"en": "{shop}: Please confirm order {number}: {link}",
	},
	"sms.cancelled": {
		"bn": "{shop}: অর্ডার {number} বাতিল হয়েছে।",
		"en": "{shop}: Order {number} has been cancelled.",
	},
	"sms.shipped": {
		"bn": "{shop}: {number} পাঠানো হয়েছে{tracking}। {link}",
		"en": "{shop}: Order {number} has shipped{tracking}. Track: {link}",
	},
	"sms.total_changed": {
		"bn": "{shop}: ওজন অনুযায়ী অর্ডার {number} এর মোট দাম {total}।",
		"en": "{shop}: Order {number} final total after weighing: {total}.",
	},
}

// OrderText: OTPText er kram (9.8); {number} ar (link thakle) {link} chhara lekha bad, karon oi SMS kaje lage na.
func OrderText(key, locale string, override map[string]string, shop, number, link string) string {
	return orderText(key, locale, override, "{shop}", shop, "{number}", number, "{link}", link)
}

// TotalText (8.16 v1.42): sms.total_changed; {number} ar {total} chhara lekha bad.
func TotalText(locale string, override map[string]string, shop, number string, total int64) string {
	return orderText("sms.total_changed", locale, override, "{shop}", shop, "{number}", number, "{total}", taka(total, locale))
}

// ShippedText (8.16, 11.7 item 5): sms.shipped; tracking thakle " (code)". Tracking diye bn 134 ba en 160 unit er
// beshi hole tracking chhara (SMS 2 part er beshi na; link e order page).
func ShippedText(locale string, override map[string]string, shop, number, link, tracking string) string {
	if tracking != "" {
		t := orderText("sms.shipped", locale, override, "{shop}", shop, "{number}", number, "{link}", link, "{tracking}", " ("+tracking+")")
		if n := len(utf16.Encode([]rune(t))); n <= 134 || (n <= 160 && gsm(t)) {
			return t
		}
	}
	return orderText("sms.shipped", locale, override, "{shop}", shop, "{number}", number, "{link}", link, "{tracking}", "")
}

// gsm: shudhu printable ASCII (GSM-7 e ek SMS 160).
func gsm(t string) bool {
	for _, r := range t {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}

// orderText: kv = placeholder, value jora ({shop} prothom); {shop} chhara khali na emon protiti placeholder lekha te lagbe.
func orderText(key, locale string, override map[string]string, kv ...string) string {
	def := orderDefault[key]
	r := strings.NewReplacer(kv...)
	for _, t := range []string{override[locale], def[locale], override["bn"]} {
		ok := t != ""
		for i := 2; i < len(kv); i += 2 {
			ok = ok && (kv[i+1] == "" || strings.Contains(t, kv[i]))
		}
		if ok {
			return r.Replace(t)
		}
	}
	return r.Replace(def["bn"])
}

// taka (8.16 v1.42): poisha → "৳1,23,456.50" (en e "Tk ", GSM-7 e ৳ nai); lakh-style comma, poisha 0 hole doshomik na.
func taka(poisha int64, locale string) string {
	d := strconv.FormatInt(poisha/100, 10)
	out := d[max(len(d)-3, 0):]
	for d = d[:max(len(d)-3, 0)]; d != ""; d = d[:max(len(d)-2, 0)] {
		out = d[max(len(d)-2, 0):] + "," + out
	}
	if p := poisha % 100; p != 0 {
		out += fmt.Sprintf(".%02d", p)
	}
	if locale == "en" {
		return "Tk " + out
	}
	return "৳" + out
}

// SecurityAlertJob (8.17 alert.security, Q9): ekhon shudhu slog.Warn (payload e phone mask); email/webhook M12 e.
func SecurityAlertJob(log *slog.Logger) jobs.Handler {
	return func(ctx context.Context, j jobs.Job) error {
		log.WarnContext(ctx, "security alert", "job_id", j.ID, "shop_id", j.ShopID, "payload", string(j.Payload))
		return nil
	}
}
