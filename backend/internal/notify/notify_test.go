package notify

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"unicode/utf16"

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
	otp := func(_ context.Context, shopID, otpID int64, nonce, ct []byte) (OTP, error) {
		if shopID != 1 || otpID != 7 || string(nonce) != "n" || string(ct) != "c" {
			return OTP{}, errors.New("bhul otp payload")
		}
		return OTP{Phone: "+8801722222222", Code: "012345", Locale: "en"}, nil
	}
	if err := SendSMSJob(sms, otp)(context.Background(), jobs.Job{ShopID: 1, Payload: []byte(`{"otp_id":7,"nonce":"bg==","ct":"Yw=="}`)}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "+8801722222222|"+OTPText("en", nil, "012345") {
		t.Errorf("OTP SMS (customer er locale e): %q", got)
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

// TestOrderSMSTemplateLength (8.16, 8.19 item 7): order SMS er default bn ≤ 134 unit (UCS-2, 2 part), en ≤ 160
// (GSM-7, ASCII), lomba shop/number/domain diye; placeholder bosano; override er kram OTPText er moto, {number}
// ba (link thakle) {link} chhara override bad.
func TestOrderSMSTemplateLength(t *testing.T) {
	const (
		shopBn = "সবুজবাজার অর্গানিক"
		shopEn = "Sobujbazar Organic"
		number = "SBORG-1000001"
		domain = "https://sobujbazar-organic.com"
		token  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" // 26 okkhor
	)
	for key, link := range map[string]string{
		"sms.order_placed":  domain + "/order/" + token,
		"sms.order_confirm": domain + "/confirm/" + token,
		"sms.cancelled":     "",
	} {
		bn, en := OrderText(key, "bn", nil, shopBn, number, link), OrderText(key, "en", nil, shopEn, number, link)
		if n := len(utf16.Encode([]rune(bn))); n > 134 || !strings.Contains(bn, number) || !strings.Contains(bn, link) {
			t.Errorf("%s bn %d unit (≤ 134 chai): %q", key, n, bn)
		}
		for _, r := range en {
			if r > 0x7e || (r < 0x20 && r != '\n') {
				t.Errorf("%s en e GSM-7 er baire %q: %q", key, r, en)
			}
		}
		if len(en) > 160 || !strings.Contains(en, number) || !strings.Contains(en, link) || en == bn {
			t.Errorf("%s en %d (≤ 160 chai): %q", key, len(en), en)
		}
		if strings.Contains(bn+en, "{") {
			t.Errorf("%s placeholder roye geche: %q %q", key, bn, en)
		}
		if bn != OrderText(key, "", nil, shopBn, number, link) || bn != OrderText(key, "fr", nil, shopBn, number, link) {
			t.Errorf("%s: ochena locale e bn chai", key)
		}
	}
	if OrderText("sms.order_placed", "bn", nil, "S", "N", "L") == OrderText("sms.order_confirm", "bn", nil, "S", "N", "L") {
		t.Error("confirm SMS er lekha order_placed er moto (8.19 item 7: alada)")
	}
	def := OrderText("sms.order_placed", "en", nil, "S", "N-1", "L")
	for _, c := range []struct {
		locale   string
		override map[string]string
		want     string
	}{
		{"en", map[string]string{"en": "{shop} #{number} {link}"}, "S #N-1 L"},
		{"en", map[string]string{"en": "#{number} {link}"}, "#N-1 L"},
		{"en", map[string]string{"en": "{shop} {link}"}, def},    // number nai
		{"en", map[string]string{"en": "{shop} #{number}"}, def}, // link nai
		{"fr", map[string]string{"bn": "দোকান {number} {link}"}, "দোকান N-1 L"},
	} {
		if got := OrderText("sms.order_placed", c.locale, c.override, "S", "N-1", "L"); got != c.want {
			t.Errorf("%q %v: %q, chai %q", c.locale, c.override, got, c.want)
		}
	}
	// cancel e link nai, tai {link} chhara override chole
	if got := OrderText("sms.cancelled", "en", map[string]string{"en": "#{number} cancelled"}, "S", "N-1", ""); got != "#N-1 cancelled" {
		t.Errorf("sms.cancelled override: %q", got)
	}
}

// TestOTPTemplateLength (8.16, Q10 iv): sms.otp er default bn ek SMS e (UCS-2, ≤ 70 code unit), en ≤ 160
// (GSM-7, ASCII); {code} bosano; kram override[locale] → default[locale] → override[bn] → default[bn] (9.8);
// {code} chhara override bad (code chhara OTP SMS kaje lage na).
func TestOTPTemplateLength(t *testing.T) {
	const code = "012345"
	bn, en := OTPText("bn", nil, code), OTPText("en", nil, code)
	if n := len(utf16.Encode([]rune(bn))); n > 70 || !strings.Contains(bn, code) {
		t.Errorf("bn %d unit (≤ 70 chai): %q", n, bn)
	}
	for _, r := range en {
		if r > 0x7e || (r < 0x20 && r != '\n') {
			t.Errorf("en e GSM-7 er baire %q: %q", r, en)
		}
	}
	if len(en) > 160 || !strings.Contains(en, code) || en == bn {
		t.Errorf("en %d (≤ 160 chai): %q", len(en), en)
	}
	if strings.Contains(bn+en, "{code}") {
		t.Errorf("placeholder roye geche: %q %q", bn, en)
	}
	for _, c := range []struct {
		locale   string
		override map[string]string
		want     string
	}{
		{"", nil, bn},
		{"fr", nil, bn},
		{"en", map[string]string{"en": "Shop: {code}"}, "Shop: 012345"},
		{"en", map[string]string{"bn": "দোকান {code}"}, en},
		{"bn", map[string]string{"en": "Shop: {code}"}, bn},
		{"bn", map[string]string{"bn": "দোকান {code}, {code}"}, "দোকান 012345, 012345"},
		{"en", map[string]string{"en": "no code"}, en},
		{"bn", map[string]string{"bn": "kod nai"}, bn},
		{"fr", map[string]string{"bn": "দোকান {code}"}, "দোকান 012345"},
	} {
		if got := OTPText(c.locale, c.override, code); got != c.want {
			t.Errorf("%q %v: %q, chai %q", c.locale, c.override, got, c.want)
		}
	}
}

// TestTotalChangedSMSLength (8.16 v1.42, 11.7 item 13 iv): sms.total_changed er default bn ≤ 134 unit, en ≤ 160
// GSM-7 (৳ na, Tk); {total} lakh-style comma, English digit, poisha 0 hole doshomik na; {number} ba {total}
// chhara override bad.
func TestTotalChangedSMSLength(t *testing.T) {
	for _, c := range []struct {
		poisha int64
		bn, en string
	}{
		{88800, "৳888", "Tk 888"},
		{50, "৳0.50", "Tk 0.50"},
		{100000, "৳1,000", "Tk 1,000"},
		{12345600, "৳1,23,456", "Tk 1,23,456"},
		{12345678999, "৳12,34,56,789.99", "Tk 12,34,56,789.99"},
	} {
		bn, en := TotalText("bn", nil, "S", "N-1", c.poisha), TotalText("en", nil, "S", "N-1", c.poisha)
		if !strings.HasSuffix(bn, " "+c.bn+"।") || !strings.HasSuffix(en, " "+c.en+".") { // template er shesh e {total}
			t.Errorf("%d: %q %q, chai %s / %s", c.poisha, bn, en, c.bn, c.en)
		}
	}
	const (
		shopBn = "সবুজবাজার অর্গানিক"
		shopEn = "Sobujbazar Organic"
		number = "SBORG-1000001"
	)
	bn, en := TotalText("bn", nil, shopBn, number, 12345678999), TotalText("en", nil, shopEn, number, 12345678999)
	if n := len(utf16.Encode([]rune(bn))); n > 134 || !strings.Contains(bn, number) || !strings.Contains(bn, shopBn) {
		t.Errorf("bn %d unit (≤ 134 chai): %q", n, bn)
	}
	for _, r := range en {
		if r > 0x7e || r < 0x20 {
			t.Errorf("en e GSM-7 er baire %q: %q", r, en)
		}
	}
	if len(en) > 160 || !strings.Contains(en, number) || strings.Contains(bn+en, "{") || en == bn {
		t.Errorf("en %d (≤ 160 chai): %q", len(en), en)
	}
	def := TotalText("en", nil, "S", "N-1", 100)
	for _, c := range []struct {
		override map[string]string
		want     string
	}{
		{map[string]string{"en": "#{number} = {total}"}, "#N-1 = Tk 1"},
		{map[string]string{"en": "#{number}"}, def},
		{map[string]string{"en": "{total}"}, def},
	} {
		if got := TotalText("en", c.override, "S", "N-1", 100); got != c.want {
			t.Errorf("%v: %q, chai %q", c.override, got, c.want)
		}
	}
}

// TestShippedSMSLength (8.16, 11.7 item 5): sms.shipped er default bn ≤ 134 unit, en ≤ 160 GSM-7; {number} ar {link}
// lagbe, tracking thakle lekha te; tracking eto lomba je shima pere jay hole tracking chhara (link e order page).
func TestShippedSMSLength(t *testing.T) {
	const (
		shopBn = "সবুজবাজার অর্গানিক"
		shopEn = "Sobujbazar Organic"
		number = "SBORG-1000001"
		link   = "https://sobujbazar-organic.com/order/ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		track  = "DL121224VS8TN1"
	)
	for _, tr := range []string{"", track} {
		bn, en := ShippedText("bn", nil, shopBn, number, link, tr), ShippedText("en", nil, shopEn, number, link, tr)
		if n := len(utf16.Encode([]rune(bn))); n > 134 || !strings.Contains(bn, number) || !strings.Contains(bn, link) || !strings.Contains(bn, tr) {
			t.Errorf("bn %d unit (≤ 134 chai): %q", n, bn)
		}
		for _, r := range en {
			if r > 0x7e || r < 0x20 {
				t.Errorf("en e GSM-7 er baire %q: %q", r, en)
			}
		}
		if len(en) > 160 || !strings.Contains(en, number) || !strings.Contains(en, link) || !strings.Contains(en, tr) ||
			strings.Contains(bn+en, "{") || en == bn {
			t.Errorf("en %d (≤ 160 chai): %q", len(en), en)
		}
	}
	if ShippedText("bn", nil, shopBn, number, link, "") == ShippedText("bn", nil, shopBn, number, link, track) {
		t.Error("tracking lekha te nai")
	}
	long := strings.Repeat("X", 64)
	if bn := ShippedText("bn", nil, shopBn, number, link, long); strings.Contains(bn, long) || len(utf16.Encode([]rune(bn))) > 134 {
		t.Errorf("lomba tracking: %q", bn)
	}
	def := ShippedText("en", nil, "S", "N-1", "L", "T1")
	for _, c := range []struct {
		override map[string]string
		tracking string
		want     string
	}{
		{map[string]string{"en": "#{number}{tracking} {link}"}, "T1", "#N-1 (T1) L"},
		{map[string]string{"en": "#{number} {link}"}, "T1", def},    // tracking ache, lekha te nai
		{map[string]string{"en": "#{number}{tracking}"}, "T1", def}, // link nai
	} {
		if got := ShippedText("en", c.override, "S", "N-1", "L", c.tracking); got != c.want {
			t.Errorf("%v: %q, chai %q", c.override, got, c.want)
		}
	}
}
