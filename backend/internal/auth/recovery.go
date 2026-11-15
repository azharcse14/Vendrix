package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"strconv"
	"strings"
	"unicode"
)

const recoveryAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567" // base32: 0/O, 1/I gulie jay na

// NewRecoveryCodes (8.3 step 6): 10 ta, protiti crypto/rand 10 okkhor base32 (50 bit), XXXXX-XXXXX.
func NewRecoveryCodes() []string {
	codes := make([]string, 10)
	for i := range codes {
		b := make([]byte, 10)
		rand.Read(b)
		for j := range b {
			b[j] = recoveryAlphabet[b[j]&31] // 256 % 32 == 0, tai bias nai
		}
		codes[i] = string(b[:5]) + "-" + string(b[5:])
	}
	return codes
}

// HashRecoveryCode (8.3 step 5): staff_recovery_codes.code_hash = HMAC-SHA256(RECOVERY_SECRET, staff_id:code);
// code theke `-`, space bad ar boro haat, jate staff je bhabe-i likhuk ekoi hash.
func HashRecoveryCode(secret string, staffID int64, code string) []byte {
	code = strings.Map(func(r rune) rune {
		if r == '-' || unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToUpper(r)
	}, code)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(strconv.FormatInt(staffID, 10) + ":" + code))
	return m.Sum(nil)
}
