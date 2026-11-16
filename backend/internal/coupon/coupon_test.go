package coupon

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestCouponNormalize (8.19 item 5): code trim + upper, khali = coupon nai, max 32 okkhor (rune);
// error → API code (409 coupon_invalid ba coupon_limit_reached).
func TestCouponNormalize(t *testing.T) {
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"SAVE10", "SAVE10", true},
		{"  save10 ", "SAVE10", true},
		{"\tEid-25\n", "EID-25", true},
		{"", "", true},
		{"   ", "", true},
		{strings.Repeat("a", 32), strings.Repeat("A", 32), true},
		{strings.Repeat("a", 33), "", false},
		{" " + strings.Repeat("a", 32) + " ", strings.Repeat("A", 32), true}, // trim er por gona
		{strings.Repeat("ঈ", 32), strings.Repeat("ঈ", 32), true},             // byte na, rune
		{strings.Repeat("ঈ", 33), "", false},
	} {
		got, ok := Normalize(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("Normalize(%q) = %q, %v; chai %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}

	for _, c := range []struct {
		err  error
		want string
	}{
		{ErrInvalid, "coupon_invalid"},
		{ErrLimitReached, "coupon_limit_reached"},
		{fmt.Errorf("checkout: %w", ErrLimitReached), "coupon_limit_reached"},
		{errors.New("db"), ""},
		{nil, ""},
	} {
		if got := ErrorCode(c.err); got != c.want {
			t.Errorf("ErrorCode(%v) = %q, chai %q", c.err, got, c.want)
		}
	}
}
