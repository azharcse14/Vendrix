// Package phone: Bangladesh mobile number ke +8801XXXXXXXXX format e ana.
//
// Feature doc: docs/features/phone.md
package phone

import (
	"errors"
	"strings"
	"unicode"
)

var ErrInvalid = errors.New("invalid_phone")

// Normalize: "01712-345678", "8801712345678", "+880 1712 345678", "00880 1712 345678",
// "০১৭১২৩৪৫৬৭৮" shob → "+8801712345678". Digit ar separator (space, ., (, ), "-" ar "–" er moto
// dash) chhara onno okkhor thakle bhul; chupchap fele dey na.
// v1.3: phone theke copy kora lukano format chinho (LRM, zero-width space, U+202A/U+202C) bad jay;
// '+' shudhu shuru te ekbar, ar tarpor 880 thakte hobe ("+1 712 345 678" bhul).
func Normalize(in string) (string, error) {
	var b strings.Builder
	plus := false
	for _, r := range in {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= '০' && r <= '৯': // Bangla shonkhya
			b.WriteRune('0' + (r - '০'))
		case r == '+':
			if plus || b.Len() > 0 {
				return "", ErrInvalid
			}
			plus = true
		case unicode.Is(unicode.Cf, r): // lukano format chinho, bad
		case unicode.IsSpace(r) || unicode.Is(unicode.Pd, r) || strings.ContainsRune(".()\u2212", r):
		default:
			return "", ErrInvalid
		}
	}
	d := b.String()
	if plus && !strings.HasPrefix(d, "880") {
		return "", ErrInvalid
	}
	if strings.HasPrefix(d, "00880") { // international prefix
		d = d[2:]
	}
	d = strings.TrimPrefix(d, "880")
	if len(d) == 10 && d[0] == '1' {
		d = "0" + d
	}
	if len(d) != 11 || d[0] != '0' || d[1] != '1' || d[2] < '3' || d[2] > '9' {
		return "", ErrInvalid
	}
	return "+88" + d, nil
}
