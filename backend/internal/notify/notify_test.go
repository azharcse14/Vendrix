package notify

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"vendrix/backend/internal/jobs"
)

// TestNewSMS (8.2 step 4): shudhu local e log; onno env e log mode hoy-i na.
func TestNewSMS(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	if err := NewSMS("local", log).Send(context.Background(), 1, "+8801711111111", "code 123456"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "code 123456") || strings.Contains(logs.String(), "+8801711111111") ||
		!strings.Contains(logs.String(), "111") {
		t.Errorf("local log e SMS nai ba phone mask hoyni (12.2): %s", logs.String())
	}
	for _, env := range []string{"staging", "production", ""} {
		logs.Reset()
		err := NewSMS(env, log).Send(context.Background(), 1, "+8801711111111", "code 654321")
		if !errors.Is(err, ErrNoProvider) || strings.Contains(logs.String(), "654321") {
			t.Errorf("%q: err %v, log %q", env, err, logs.String())
		}
	}
}

// TestSendSMSJob: sms.send er payload theke phone ar lekha; OTP payload ({otp_id, nonce, ct}, 8.2 step 4)
// OTPSource diye phone ar code; bhul payload e error (job retry ba fail).
func TestSendSMSJob(t *testing.T) {
	var got []string
	sms := smsFunc(func(_ context.Context, shopID int64, phone, text string) error {
		got = append(got, phone+"|"+text)
		return nil
	})
	otp := func(_ context.Context, shopID, otpID int64, nonce, ct []byte) (string, string, error) {
		if shopID != 1 || otpID != 7 || string(nonce) != "n" || string(ct) != "c" {
			return "", "", errors.New("bhul otp payload")
		}
		return "+8801722222222", "012345", nil
	}
	if err := SendSMSJob(sms, otp)(context.Background(), jobs.Job{ShopID: 1, Payload: []byte(`{"otp_id":7,"nonce":"bg==","ct":"Yw=="}`)}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "+8801722222222|") || !strings.Contains(got[0], "012345") {
		t.Errorf("OTP SMS: %q", got)
	}
	got = nil
	for _, p := range []string{`{"otp_id":8,"nonce":"bg==","ct":"Yw=="}`, `{"otp_id":7}`} {
		if err := SendSMSJob(sms, otp)(context.Background(), jobs.Job{ShopID: 1, Payload: []byte(p)}); err == nil {
			t.Errorf("%s: error chai", p)
		}
	}
	if err := SendSMSJob(sms, nil)(context.Background(), jobs.Job{ShopID: 1, Payload: []byte(`{"otp_id":7,"nonce":"bg==","ct":"Yw=="}`)}); err == nil || len(got) != 0 {
		t.Error("OTPSource chhara OTP payload e error chai")
	}
	h := SendSMSJob(sms, nil)
	if err := h(context.Background(), jobs.Job{ShopID: 1, Payload: []byte(`{"phone":"+8801711111111","text":"hi"}`)}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "+8801711111111|hi" {
		t.Errorf("got %q", got)
	}
	for _, p := range []string{`{`, `{"phone":"","text":"x"}`, `{"phone":"+8801711111111"}`} {
		if err := h(context.Background(), jobs.Job{Payload: []byte(p)}); err == nil {
			t.Errorf("%s: error chai", p)
		}
	}
}

type smsFunc func(ctx context.Context, shopID int64, phone, text string) error

func (f smsFunc) Send(ctx context.Context, shopID int64, phone, text string) error {
	return f(ctx, shopID, phone, text)
}
