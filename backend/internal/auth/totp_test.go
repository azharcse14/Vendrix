package auth

import (
	"bytes"
	"encoding/base32"
	"encoding/hex"
	"regexp"
	"testing"
	"time"
)

// TestTOTP (8.3 step 2): RFC 6238 appendix B er SHA-1 vector (8 digit er shesh 6 digit = mod 10^6), ±1 step
// chole, ±2 na; 6 digit chhara code na. Secret base32 (padding chhara) e ferot ase.
func TestTOTP(t *testing.T) {
	secret := []byte("12345678901234567890")
	for _, v := range []struct {
		unix int64
		code string
	}{
		{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"},
		{1234567890, "005924"}, {2000000000, "279037"}, {20000000000, "353130"},
	} {
		if got := TOTPCode(secret, v.unix/30); got != v.code {
			t.Errorf("t=%d: %s, chai %s", v.unix, got, v.code)
		}
	}

	code := TOTPCode(secret, 1000)
	for _, c := range []struct {
		step int64
		ok   bool
	}{{998, false}, {999, true}, {1000, true}, {1001, true}, {1002, false}} {
		now := time.Unix(c.step*30+29, 0)
		step, ok := CheckTOTP(secret, code, now)
		if ok != c.ok || (ok && step != 1000) {
			t.Errorf("ekhon step %d: (%d, %v), chai ok=%v step 1000", c.step, step, ok, c.ok)
		}
	}
	now := time.Unix(1000*30, 0)
	for _, bad := range []string{"", "12345", "1234567", "abcdef", " " + code[1:], wrongDigit(code)} {
		if _, ok := CheckTOTP(secret, bad, now); ok {
			t.Errorf("%q e ok", bad)
		}
	}
	if _, ok := CheckTOTP([]byte("other-secret-000000"), code, now); ok {
		t.Error("onno secret e ok")
	}

	raw, b32 := NewTOTPSecret()
	if len(raw) != 20 || !regexp.MustCompile(`^[A-Z2-7]{32}$`).MatchString(b32) {
		t.Fatalf("secret %d byte, base32 %q", len(raw), b32)
	}
	if dec, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(b32); err != nil || !bytes.Equal(dec, raw) {
		t.Errorf("base32 decode: %x %v", dec, err)
	}
	if raw2, _ := NewTOTPSecret(); bytes.Equal(raw, raw2) {
		t.Error("ekoi secret duibar")
	}
}

func wrongDigit(code string) string {
	b := []byte(code)
	b[5] = '0' + (b[5]-'0'+1)%10
	return string(b)
}

// TestTOTPSecretAAD (8.3 step 5): AES-GCM e AAD shop_id:staff_id:key_version; onno staff, shop, version ba
// key e decrypt hoy na (row copy korle kaj kore na).
func TestTOTPSecretAAD(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	secret := []byte("12345678901234567890")
	ct, nonce, err := SealTOTPSecret(key, 1, 10, 1, secret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, secret) {
		t.Error("ciphertext e plain secret")
	}
	if got, err := OpenTOTPSecret(key, 1, 10, 1, ct, nonce); err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("open: %q %v", got, err)
	}
	for name, c := range map[string][3]int64{"onno staff": {1, 11, 1}, "onno shop": {2, 10, 1}, "onno version": {1, 10, 2}} {
		if _, err := OpenTOTPSecret(key, c[0], c[1], int(c[2]), ct, nonce); err == nil {
			t.Errorf("%s e decrypt hoyeche", name)
		}
	}
	if _, err := OpenTOTPSecret([]byte("fedcba9876543210fedcba9876543210"), 1, 10, 1, ct, nonce); err == nil {
		t.Error("onno key e decrypt hoyeche")
	}
}

// TestRecoveryCode (8.3 step 6): 10 ta XXXXX-XXXXX (base32 okkhor), protiti alada; hash
// HMAC-SHA256(RECOVERY_SECRET, staff_id:code) openssl er vector e mile; `-`, space, case bad diye ekoi hash;
// staff, secret ba code bodlale alada.
func TestRecoveryCode(t *testing.T) {
	format := regexp.MustCompile(`^[A-Z2-7]{5}-[A-Z2-7]{5}$`)
	codes := NewRecoveryCodes()
	seen := map[string]bool{}
	for _, c := range codes {
		if !format.MatchString(c) {
			t.Errorf("format %q", c)
		}
		seen[c] = true
	}
	if len(codes) != 10 || len(seen) != 10 {
		t.Fatalf("%d code, %d alada", len(codes), len(seen))
	}

	const secret = "test-recovery-secret"
	// printf '%s' '42:ABCDE23456' | openssl dgst -sha256 -hmac 'test-recovery-secret'
	want := "603c19c44659f4b60a336ab65353d8d2b12fa098415c20630c35ab71bfeca2e7"
	for _, in := range []string{"ABCDE-23456", "abcde-23456", "ABCDE23456", " abcde 23456 ", "AbCdE - 23456"} {
		if got := hex.EncodeToString(HashRecoveryCode(secret, 42, in)); got != want {
			t.Errorf("%q: %s, chai %s", in, got, want)
		}
	}
	h := HashRecoveryCode(secret, 42, "ABCDE-23456")
	for name, other := range map[string][]byte{
		"onno staff":  HashRecoveryCode(secret, 43, "ABCDE-23456"),
		"onno secret": HashRecoveryCode("other", 42, "ABCDE-23456"),
		"onno code":   HashRecoveryCode(secret, 42, "ABCDE-23457"),
	} {
		if bytes.Equal(h, other) {
			t.Errorf("%s e ekoi hash", name)
		}
	}
}
