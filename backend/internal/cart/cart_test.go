package cart

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"regexp"
	"strings"
	"testing"
)

// TestCartKeyFormat (8.4 v1.28 item 1): notun key rand.Text (^[A-Z2-7]{26}$), DB te shudhu sha256;
// bhul format er key "nai" (notun cart pay, onner cart na).
func TestCartKeyFormat(t *testing.T) {
	re := regexp.MustCompile(`^[A-Z2-7]{26}$`)
	seen := map[string]bool{}
	for range 100 {
		key, hash := NewKey()
		if !re.MatchString(key) {
			t.Fatalf("NewKey() = %q, format ^[A-Z2-7]{26}$ na", key)
		}
		if seen[key] {
			t.Fatalf("NewKey() ekoi key duibar: %q", key)
		}
		seen[key] = true
		want := sha256.Sum256([]byte(key))
		if !bytes.Equal(hash, want[:]) {
			t.Fatalf("NewKey() hash sha256(key) na")
		}
		got, ok := KeyHash(key)
		if !ok || !bytes.Equal(got, hash) {
			t.Fatalf("KeyHash(%q) = %x, %v; want NewKey er hash", key, got, ok)
		}
	}
	good := strings.Repeat("A", 24) + "27"
	if _, ok := KeyHash(good); !ok {
		t.Errorf("KeyHash(%q) ok = false, want true", good)
	}
	bad := []string{
		"",
		strings.Repeat("A", 25),
		strings.Repeat("A", 27),
		strings.Repeat("a", 26),        // choto hater okkhor na
		strings.Repeat("A", 25) + "1",  // 0, 1, 8, 9 base32 e nai
		strings.Repeat("A", 25) + "=",  // padding na
		strings.Repeat("A", 25) + " ",  // space na
		strings.Repeat("A", 24) + "অ",  // byte gunle 26, okkhor na
		" " + strings.Repeat("A", 26),  // trim kore na
		strings.Repeat("A", 26) + "\n", // regexp er $ er moto newline mane na
	}
	for _, k := range bad {
		if h, ok := KeyHash(k); ok || h != nil {
			t.Errorf("KeyHash(%q) = %x, %v; want nil, false", k, h, ok)
		}
	}
}

// TestMaxQuantity (8.4 v1.28 item 3, 4): max_quantity = min(stock, 50); stock 0 ba minus hole 0.
func TestMaxQuantity(t *testing.T) {
	for stock, want := range map[int]int{-3: 0, 0: 0, 1: 1, 7: 7, 49: 49, 50: 50, 51: 50, 1000: 50} {
		if got := MaxQuantity(stock); got != want {
			t.Errorf("MaxQuantity(%d) = %d, want %d", stock, got, want)
		}
	}
}

// TestCheckQuantity (8.4 v1.28 item 3): add/patch e 1..min(stock, 50); 0/minus ErrInvalidQuantity
// (400 validation_failed); beshi (50 er beshi o) ba stock 0 e ErrOutOfStock (409 + max_quantity).
func TestCheckQuantity(t *testing.T) {
	cases := []struct {
		qty, stock int
		want       error
	}{
		{1, 1, nil},
		{5, 10, nil},
		{10, 10, nil},
		{50, 100, nil},
		{0, 10, ErrInvalidQuantity},
		{-1, 10, ErrInvalidQuantity},
		{0, 0, ErrInvalidQuantity},
		{11, 10, ErrOutOfStock},
		{51, 100, ErrOutOfStock},
		{1000, 100, ErrOutOfStock},
		{1, 0, ErrOutOfStock},
		{1, -2, ErrOutOfStock},
	}
	for _, c := range cases {
		if err := CheckQuantity(c.qty, c.stock); !errors.Is(err, c.want) {
			t.Errorf("CheckQuantity(%d, %d) = %v, want %v", c.qty, c.stock, err, c.want)
		}
	}
}

// TestCheckRoom (8.4 v1.28 item 3): ek cart e max 50 ta alada variant; 51 tomo notun variant e
// ErrCartFull; cart e already thaka variant (qty bodol) atkay na.
func TestCheckRoom(t *testing.T) {
	cases := []struct {
		lines  int
		inCart bool
		want   error
	}{
		{0, false, nil},
		{49, false, nil},
		{50, false, ErrCartFull},
		{51, false, ErrCartFull},
		{50, true, nil},
	}
	for _, c := range cases {
		if err := CheckRoom(c.lines, c.inCart); !errors.Is(err, c.want) {
			t.Errorf("CheckRoom(%d, %v) = %v, want %v", c.lines, c.inCart, err, c.want)
		}
	}
	if MaxLines != 50 || MaxLineQuantity != 50 {
		t.Errorf("MaxLines = %d, MaxLineQuantity = %d; want 50, 50 (8.4)", MaxLines, MaxLineQuantity)
	}
}

// TestNormalizeQuantity (8.4 v1.28 item 4): GET e qty > stock (stock > 0) hole min(stock, 50) ar
// quantity_reduced; stock 0 er item thake (qty bodlay na); thik qty bodlay na.
func TestNormalizeQuantity(t *testing.T) {
	cases := []struct {
		qty, stock, want int
		reduced          bool
	}{
		{3, 10, 3, false},
		{10, 10, 10, false},
		{50, 1000, 50, false},
		{10, 4, 4, true},
		{50, 1, 1, true},
		{5, 0, 5, false},
		{5, -1, 5, false},
	}
	for _, c := range cases {
		got, reduced := NormalizeQuantity(c.qty, c.stock)
		if got != c.want || reduced != c.reduced {
			t.Errorf("NormalizeQuantity(%d, %d) = %d, %v; want %d, %v", c.qty, c.stock, got, reduced, c.want, c.reduced)
		}
	}
}
