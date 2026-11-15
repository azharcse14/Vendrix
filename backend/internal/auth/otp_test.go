package auth

import (
	"encoding/hex"
	"regexp"
	"testing"
)

// TestOTPCode (8.2 step 2): shob shomoy 6 digit, samne zero thake (000123 bad pore na), protibar alada.
func TestOTPCode(t *testing.T) {
	six := regexp.MustCompile(`^[0-9]{6}$`)
	seen := map[string]bool{}
	leadingZero := false
	// 2000 bar: samne zero na ashar shombhabona 0.9^2000, karjoto shunno.
	for range 2000 {
		c := NewOTPCode()
		if !six.MatchString(c) {
			t.Fatalf("code %q 6 digit na", c)
		}
		leadingZero = leadingZero || c[0] == '0'
		seen[c] = true
	}
	if !leadingZero {
		t.Error("samne zero shoho code kokhono ashe nai")
	}
	if len(seen) < 1900 {
		t.Errorf("2000 er moddhe %d alada: random na", len(seen))
	}
}

// TestOTPHash (8.2 step 3): HMAC-SHA256(OTP_SECRET, phone:purpose:code), openssl er vector er sathe mile;
// phone, purpose, code ba secret bodlale mile na; CheckOTP thik code e true.
func TestOTPHash(t *testing.T) {
	const secret, phone, purpose, code = "test-otp-secret", "+8801712345678", "login", "012345"
	// printf '%s' '+8801712345678:login:012345' | openssl dgst -sha256 -hmac 'test-otp-secret'
	want := "35c5e51af8d780630d7ac372082f80b424214bac6b5e73ffaa3c4bfcd2a4a4e4"
	h := HashOTP(secret, phone, purpose, code)
	if got := hex.EncodeToString(h); got != want {
		t.Fatalf("hash %s, chai %s", got, want)
	}
	if !CheckOTP(secret, phone, purpose, code, h) {
		t.Error("thik code e false")
	}
	for _, c := range []struct{ secret, phone, purpose, code string }{
		{"other-secret", phone, purpose, code},
		{secret, "+8801712345679", purpose, code},
		{secret, phone, "checkout", code},
		{secret, phone, purpose, "12345"},
		{secret, phone, purpose, "012346"},
		{secret, phone, purpose, ""},
	} {
		if CheckOTP(c.secret, c.phone, c.purpose, c.code, h) {
			t.Errorf("%+v e true", c)
		}
	}
	if CheckOTP(secret, phone, purpose, code, nil) || CheckOTP(secret, phone, purpose, code, h[:31]) {
		t.Error("khali ba choto hash e true")
	}
}
