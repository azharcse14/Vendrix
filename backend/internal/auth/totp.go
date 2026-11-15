package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"time"
)

// TOTPCode (8.3, RFC 6238/4226): HMAC-SHA1(secret, step), dynamic truncation, 6 digit (samne zero shoho).
// Stdlib diye nije (2.2), bahirer library na.
func TOTPCode(secret []byte, step int64) string {
	m := hmac.New(sha1.New, secret)
	binary.Write(m, binary.BigEndian, step)
	s := m.Sum(nil)
	off := s[len(s)-1] & 0x0f
	n := binary.BigEndian.Uint32(s[off:]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1_000_000)
}

// CheckTOTP (8.3 step 2): 30 second step, ±1 step (ghori er tofat); mille kon step e milche dey, jate caller
// staff.totp_last_step er conditional UPDATE e replay atkay. Tulona hmac.Equal (constant time).
func CheckTOTP(secret []byte, code string, now time.Time) (step int64, ok bool) {
	if len(code) != 6 {
		return 0, false
	}
	cur := now.Unix() / 30
	for s := cur - 1; s <= cur+1; s++ {
		if hmac.Equal([]byte(TOTPCode(secret, s)), []byte(code)) {
			return s, true
		}
	}
	return 0, false
}

var totpBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret (8.3 step 6): 20 byte (160 bit, RFC 4226 er suggestion) crypto/rand; base32 (padding chhara)
// authenticator app er otpauth_url ar haate lekhar jonno.
func NewTOTPSecret() (raw []byte, b32 string) {
	raw = make([]byte, 20)
	rand.Read(raw)
	return raw, totpBase32.EncodeToString(raw)
}

// totpAAD (8.3 step 5): ek staff er ciphertext onno staff/shop er row e copy korle decrypt hoy na.
func totpAAD(shopID, staffID int64, keyVersion int) string {
	return fmt.Sprintf("%d:%d:%d", shopID, staffID, keyVersion)
}

// SealTOTPSecret: staff.totp_secret_enc ar totp_nonce; key = TOTP_KEY, version = staff.totp_key_version.
func SealTOTPSecret(key []byte, shopID, staffID int64, keyVersion int, secret []byte) (ct, nonce []byte, err error) {
	return Seal(key, totpAAD(shopID, staffID, keyVersion), secret)
}

// OpenTOTPSecret: SealTOTPSecret er ulta; row er shop, staff ba version na mille error.
func OpenTOTPSecret(key []byte, shopID, staffID int64, keyVersion int, ct, nonce []byte) ([]byte, error) {
	return Open(key, totpAAD(shopID, staffID, keyVersion), ct, nonce)
}
