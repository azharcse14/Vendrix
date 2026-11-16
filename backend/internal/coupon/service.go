package coupon

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalid      = errors.New("coupon: nai, bondho ba shomoy er baire") // 409 coupon_invalid
	ErrLimitReached = errors.New("coupon: limit shesh")                    // 409 coupon_limit_reached
)

// MaxLen: form er coupon field (8.19 item 10).
const MaxLen = 32

// Normalize: trim + upper; "" = coupon nai. MaxLen rune er beshi hole ok false (400 validation_failed).
func Normalize(code string) (string, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if utf8.RuneCountInString(code) > MaxLen {
		return "", false
	}
	return code, true
}

// ErrorCode: API er error code (409 ba quote er problems); coupon er error na hole "".
func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrInvalid):
		return "coupon_invalid"
	case errors.Is(err, ErrLimitReached):
		return "coupon_limit_reached"
	}
	return ""
}
