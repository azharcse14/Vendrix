package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// NewOTPCode (8.2 step 2): crypto/rand e 0..999999, samne zero shoho 6 digit.
func NewOTPCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err) // crypto/rand fail kore na (Go 1.24+); hole process thamano-i nirapod
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// HashOTP (8.2 step 3): otp_codes.code_hash = HMAC-SHA256(OTP_SECRET, phone:purpose:code). Phone E.164 e
// (phone.Normalize er por), na hole ekoi number er hash mile na.
func HashOTP(secret, phone, purpose, code string) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(phone + ":" + purpose + ":" + code))
	return m.Sum(nil)
}

// CheckOTP: hmac.Equal (constant time), jate shomoy dekhe code anda na jay.
func CheckOTP(secret, phone, purpose, code string, hash []byte) bool {
	return hmac.Equal(HashOTP(secret, phone, purpose, code), hash)
}

// phoneTag (7.8 v1.26): alert er dedupe_key e phone er bodole HMAC(OTP_SECRET, "otp_lock:"+phone) er 16 hex.
func phoneTag(secret, phone string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("otp_lock:" + phone))
	return hex.EncodeToString(m.Sum(nil)[:8])
}
