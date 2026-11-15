package cart

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

// 8.4 v1.28 item 3: ek line e max 50, ek cart e max 50 ta alada variant.
const (
	MaxLineQuantity = 50
	MaxLines        = 50
)

var (
	ErrInvalidQuantity = errors.New("cart: quantity 1 er kom")                // 400 validation_failed
	ErrOutOfStock      = errors.New("cart: quantity min(stock, 50) er beshi") // 409 out_of_stock + max_quantity
	ErrCartFull        = errors.New("cart: cart e 50 ta variant")             // 409 cart_full
)

// NewKey (8.4 item 1): Go-i key banay, jate client nijer pochhonder key diye cart banate na pare.
// rand.Text 26 okkhor base32 (130 bit); DB te shudhu sha256, DB leak e key na pay.
func NewKey() (string, []byte) {
	k := rand.Text()
	h := sha256.Sum256([]byte(k))
	return k, h[:]
}

// KeyHash: ^[A-Z2-7]{26}$ na mile key "nai" (false), tokhon notun cart; onner cart kokhono na.
func KeyHash(key string) ([]byte, bool) {
	if len(key) != 26 {
		return nil, false
	}
	for i := range len(key) {
		if c := key[i]; !(c >= 'A' && c <= 'Z' || c >= '2' && c <= '7') {
			return nil, false
		}
	}
	h := sha256.Sum256([]byte(key))
	return h[:], true
}

// MaxQuantity: item er max_quantity = min(stock, 50); stock 0 ba minus e 0.
func MaxQuantity(stock int) int {
	return max(0, min(stock, MaxLineQuantity))
}

// CheckQuantity (add/patch): customer er nijer kaj, tai chupchap komay na, error dey.
func CheckQuantity(qty, stock int) error {
	if qty < 1 {
		return ErrInvalidQuantity
	}
	if qty > MaxQuantity(stock) {
		return ErrOutOfStock
	}
	return nil
}

// CheckRoom: notun variant e 50 line er limit; cart e already thaka variant er qty bodol atkay na.
func CheckRoom(lines int, inCart bool) error {
	if !inCart && lines >= MaxLines {
		return ErrCartFull
	}
	return nil
}

// NormalizeQuantity (GET, 8.4 item 4): stock komle qty komay (quantity_reduced), jate cart e ja dekhay
// ta-i kena jay; stock 0 er item rakhe (abar ashle kena jay).
func NormalizeQuantity(qty, stock int) (int, bool) {
	if stock > 0 && qty > MaxQuantity(stock) {
		return MaxQuantity(stock), true
	}
	return qty, false
}
