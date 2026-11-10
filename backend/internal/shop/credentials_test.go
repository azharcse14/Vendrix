package shop

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"vendrix/backend/internal/dbtest"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

// TestSealOpen (2.2, 12.2): AES-GCM; AAD (shop_id:provider:key_version) ba key bhul hole decrypt hoy na.
func TestSealOpen(t *testing.T) {
	ct, nonce, err := seal(testKey, aad(1, "sms", 1), []byte(`{"token":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, []byte("token")) {
		t.Error("ciphertext e plain text")
	}
	if got, err := open(testKey, aad(1, "sms", 1), ct, nonce); err != nil || string(got) != `{"token":"x"}` {
		t.Errorf("open: %q %v", got, err)
	}
	for name, a := range map[string]string{"onno shop": aad(2, "sms", 1), "onno provider": aad(1, "sslcommerz", 1), "onno version": aad(1, "sms", 2)} {
		if _, err := open(testKey, a, ct, nonce); err == nil {
			t.Errorf("%s er AAD e decrypt hoyeche", name)
		}
	}
	if _, err := open([]byte("fedcba9876543210fedcba9876543210"), aad(1, "sms", 1), ct, nonce); err == nil {
		t.Error("onno key e decrypt hoyeche")
	}
	if _, _, err := seal([]byte("short"), "a", nil); err == nil {
		t.Error("16/24/32 byte chhara key")
	}
}

// TestSetCredentialIntegration (3.8): slug diye shop, JSON object chhara na, upsert, row decrypt hoy.
func TestSetCredentialIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	var shopID int64
	if err := d.QueryRow(`INSERT INTO shops (slug, name, order_prefix) VALUES ('sobujbazar', '{"en":"Sobuj"}', 'SB') RETURNING id`).Scan(&shopID); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`{"token":"a"}`, `{"token":"b"}`} {
		if err := SetCredential(ctx, d, testKey, "sobujbazar", "sms", []byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	var ct, nonce []byte
	var version int
	if err := d.QueryRow(`SELECT secret_enc, nonce, key_version FROM shop_credentials WHERE shop_id = $1 AND provider = 'sms'`, shopID).
		Scan(&ct, &nonce, &version); err != nil {
		t.Fatal(err)
	}
	if got, err := open(testKey, aad(shopID, "sms", version), ct, nonce); err != nil || string(got) != `{"token":"b"}` {
		t.Errorf("upsert er por: %q %v", got, err)
	}

	cases := map[string]struct{ slug, provider, secret, want string }{
		"ochena shop":     {"nai", "sms", `{}`, "shop"},
		"ochena provider": {"sobujbazar", "fax", `{}`, "provider"},
		"JSON na":         {"sobujbazar", "sms", `token=x`, "JSON"},
		"object na":       {"sobujbazar", "sms", `"x"`, "JSON"},
		"khali":           {"sobujbazar", "sms", ``, "JSON"},
	}
	for name, c := range cases {
		err := SetCredential(ctx, d, testKey, c.slug, c.provider, []byte(c.secret))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
