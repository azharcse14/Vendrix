package order_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"vendrix/backend/internal/order"
)

// TestPrintToken (11.7 item 8 v1.44): base64url(json) + "." + base64url(HMAC-SHA256(secret, "print:" + payload));
// sign/verify, exp, bhul signature/secret/payload, kind ar ids|date er mil na thakle verify fail.
func TestPrintToken(t *testing.T) {
	const sec = "test-print-secret-0123456789abcd"
	now := time.Unix(1_800_000_000, 0)
	slip := order.PrintToken{ShopID: 3, Kind: order.PrintPackingSlip, IDs: []int64{5, 9}, StaffID: 7, VendorID: 4,
		Exp: now.Add(10 * time.Minute).Unix()}
	tok := order.SignPrint(sec, slip)

	payload, sig, ok := strings.Cut(tok, ".")
	if !ok || strings.ContainsAny(tok, "=+/") {
		t.Fatalf("token er akar: %q", tok)
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	want := `{"exp":1800000600,"ids":[5,9],"kind":"packing_slip","shop_id":3,"staff_id":7,"vendor_id":4}`
	if b, _ := json.Marshal(m); string(b) != want {
		t.Errorf("payload %s, chai %s", b, want)
	}
	mac := hmac.New(sha256.New, []byte(sec))
	mac.Write([]byte("print:" + payload))
	if sig != base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) {
		t.Errorf("signature HMAC(secret, print:payload) na")
	}

	if got, ok := order.VerifyPrint(sec, tok, now); !ok || got.ShopID != 3 || got.StaffID != 7 || got.VendorID != 4 ||
		got.Kind != order.PrintPackingSlip || len(got.IDs) != 2 || got.IDs[1] != 9 {
		t.Errorf("verify: %+v %v", got, ok)
	}
	// pick_list: date, vendor_id chhara (json e nai)
	pl := order.PrintToken{ShopID: 3, Kind: order.PrintPickList, Date: "2026-01-11", StaffID: 7, Exp: slip.Exp}
	plTok := order.SignPrint(sec, pl)
	if p, _, _ := strings.Cut(plTok, "."); !strings.Contains(decode(t, p), `"date":"2026-01-11"`) || strings.Contains(decode(t, p), "vendor_id") ||
		strings.Contains(decode(t, p), "ids") {
		t.Errorf("pick_list payload: %s", decode(t, p))
	}
	if got, ok := order.VerifyPrint(sec, plTok, now); !ok || got.Date != "2026-01-11" {
		t.Errorf("pick_list verify: %+v %v", got, ok)
	}

	flip := func(s string) string { // shesh okkhor bodlao
		b := []byte(s)
		if b[len(b)-1] == 'A' {
			b[len(b)-1] = 'B'
		} else {
			b[len(b)-1] = 'A'
		}
		return string(b)
	}
	forged := base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(decode(t, payload), `"shop_id":3`, `"shop_id":4`, 1)))
	for name, c := range map[string]struct {
		tok string
		at  time.Time
	}{
		"exp er thik por":       {tok, now.Add(10*time.Minute + time.Second)},
		"exp er shomoy":         {tok, now.Add(10 * time.Minute)},
		"bhul signature":        {flip(tok), now},
		"onno secret":           {order.SignPrint("other-print-secret-0123456789abc", slip), now},
		"payload bodlano":       {forged + "." + sig, now},
		"signature nai":         {payload, now},
		"duita dot":             {tok + ".x", now},
		"khali":                 {"", now},
		"ochena kind":           {order.SignPrint(sec, order.PrintToken{ShopID: 3, Kind: "label", IDs: []int64{1}, StaffID: 7, Exp: slip.Exp}), now},
		"pick_list e ids":       {order.SignPrint(sec, order.PrintToken{ShopID: 3, Kind: order.PrintPickList, IDs: []int64{1}, Date: "2026-01-11", StaffID: 7, Exp: slip.Exp}), now},
		"pick_list date chhara": {order.SignPrint(sec, order.PrintToken{ShopID: 3, Kind: order.PrintPickList, StaffID: 7, Exp: slip.Exp}), now},
		"invoice e ids nai":     {order.SignPrint(sec, order.PrintToken{ShopID: 3, Kind: order.PrintInvoice, StaffID: 7, Exp: slip.Exp}), now},
		"invoice e date":        {order.SignPrint(sec, order.PrintToken{ShopID: 3, Kind: order.PrintInvoice, IDs: []int64{1}, Date: "2026-01-11", StaffID: 7, Exp: slip.Exp}), now},
		"shop nai":              {order.SignPrint(sec, order.PrintToken{Kind: order.PrintInvoice, IDs: []int64{1}, StaffID: 7, Exp: slip.Exp}), now},
		"staff nai":             {order.SignPrint(sec, order.PrintToken{ShopID: 3, Kind: order.PrintInvoice, IDs: []int64{1}, Exp: slip.Exp}), now},
	} {
		if got, ok := order.VerifyPrint(sec, c.tok, c.at); ok {
			t.Errorf("%s: verify hoye geche %+v", name, got)
		}
	}
}

func decode(t *testing.T, seg string) string {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
