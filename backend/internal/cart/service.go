package cart

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"maps"
	"slices"
	"strings"

	"vendrix/backend/internal/catalog"
	"vendrix/backend/internal/db"
	"vendrix/backend/internal/pricing"
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

// ErrNotFound: variant ochena/bondho/onno shop er, ba PATCH e cart e nai (404 not_found).
var ErrNotFound = errors.New("cart: variant nai")

// OutOfStockError: 409 out_of_stock er details (8.4 item 3); errors.Is(err, ErrOutOfStock).
type OutOfStockError struct {
	VariantID   int64
	MaxQuantity int
}

func (e OutOfStockError) Error() string { return ErrOutOfStock.Error() }
func (e OutOfStockError) Unwrap() error { return ErrOutOfStock }

// Notice (8.4 item 4): item_removed ba quantity_reduced; naam bortoman bhashay.
type Notice struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Item: 7.4 er cart item.
type Item struct {
	VariantID      int64   `json:"variant_id"`
	ProductSlug    string  `json:"product_slug"`
	Name           string  `json:"name"`
	VariantLabel   string  `json:"variant_label"`
	Image          *string `json:"image"`
	UnitPrice      int64   `json:"unit_price"`
	CompareAtPrice *int64  `json:"compare_at_price"`
	VariableWeight bool    `json:"variable_weight"`
	Quantity       int     `json:"quantity"`
	MaxQuantity    int     `json:"max_quantity"`
	InStock        bool    `json:"in_stock"`
	LineTotal      int64   `json:"line_total"`
}

// Cart: 7.4 er response; CartKey shudhu notun guest cart e.
type Cart struct {
	Items    []Item   `json:"items"`
	Count    int      `json:"count"`
	Subtotal int64    `json:"subtotal"`
	Notices  []Notice `json:"notices"`
	CartKey  string   `json:"cart_key,omitempty"`
}

// line (8.4 item 5): pricing er Line; ojon-variable e WeightG = nominal × qty. nominal NULL e dam hishab
// hoy na, tai false (in_stock na).
func line(it itemRow) (pricing.Line, bool) {
	l := pricing.Line{VendorID: it.VendorID, UnitPrice: it.Price, Qty: it.Quantity, VariableWeight: it.VariableWeight}
	if it.VariableWeight {
		if it.NominalWeightG == nil {
			return l, false
		}
		l.WeightG = *it.NominalWeightG * it.Quantity
	}
	return l, true
}

// label: variant er option er value (key er kram e), locale e.
func label(opts map[string]catalog.Text, locale string) string {
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(opts)) {
		parts = append(parts, opts[k].In(locale))
	}
	return strings.Join(parts, ", ")
}

// View (GET, 8.4 item 4, 5): cart na thakle khali (row banay na); bondho item muche (item_removed), stock komle
// qty komay (quantity_reduced), dam bortoman, subtotal pricing diye shudhu in_stock line er.
func View(ctx context.Context, d *sql.DB, shopID int64, o Owner, locale string) (Cart, error) {
	c := Cart{Items: []Item{}, Notices: []Notice{}}
	cartID, err := findCart(ctx, d, shopID, o)
	if err != nil || cartID == 0 {
		return c, err
	}
	removed, err := removeInactive(ctx, d, shopID, cartID)
	if err != nil {
		return c, err
	}
	for _, n := range removed {
		c.Notices = append(c.Notices, Notice{Code: "item_removed", Name: n.In(locale)})
	}
	rows, err := items(ctx, d, shopID, cartID)
	if err != nil {
		return c, err
	}
	var priced []pricing.Line
	for _, it := range rows {
		if q, reduced := NormalizeQuantity(it.Quantity, it.Stock); reduced {
			if err := reduceItem(ctx, d, shopID, cartID, it.VariantID, q); err != nil {
				return c, err
			}
			it.Quantity = q
			c.Notices = append(c.Notices, Notice{Code: "quantity_reduced", Name: it.Name.In(locale)})
		}
		l, ok := line(it)
		inStock := ok && it.Stock > 0
		var total int64
		if ok {
			total = pricing.LineTotal(l)
		}
		if inStock {
			priced = append(priced, l)
		}
		c.Items = append(c.Items, Item{VariantID: it.VariantID, ProductSlug: it.ProductSlug, Name: it.Name.In(locale),
			VariantLabel: label(it.Options, locale), Image: it.Image, UnitPrice: it.Price, CompareAtPrice: it.CompareAt,
			VariableWeight: it.VariableWeight, Quantity: it.Quantity, MaxQuantity: MaxQuantity(it.Stock),
			InStock: inStock, LineTotal: total})
	}
	r, err := pricing.Calculate(priced, nil, pricing.Zone{}, pricing.PerOrder)
	if err != nil {
		return c, err // bhul input chupchap gile na (8.5)
	}
	c.Count, c.Subtotal = len(c.Items), r.Subtotal
	return c, nil
}

// checkVariant: active variant (na hole ErrNotFound) ar qty er niyom; ojon-variable e nominal NULL hole
// dam hoy na, tai out_of_stock (8.4 item 5).
func checkVariant(ctx context.Context, tx *sql.Tx, shopID, variantID int64, qty int) error {
	v, err := activeVariant(ctx, tx, shopID, variantID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	stock := v.Stock
	if v.VariableWeight && v.NominalWeightG == nil {
		stock = 0
	}
	if err := CheckQuantity(qty, stock); err != nil {
		if errors.Is(err, ErrOutOfStock) {
			return OutOfStockError{VariantID: variantID, MaxQuantity: MaxQuantity(stock)}
		}
		return err
	}
	return nil
}

// Add (POST, 8.4 item 1, 3): cart na thakle toiri (guest e Go er notun key, ferot dey; client er key kokhono na).
// Ekoi variant abar add e qty jog, jog o 1..min(stock, 50). Cart row lock kore, tai parallel add e 50 er
// limit bhange na.
func Add(ctx context.Context, d *sql.DB, shopID int64, o Owner, variantID int64, qty int) (newKey string, err error) {
	if qty < 1 {
		return "", ErrInvalidQuantity
	}
	err = db.InTx(ctx, d, func(tx *sql.Tx) error {
		newKey = ""
		cartID, err := touchCart(ctx, tx, shopID, o)
		if err != nil {
			return err
		}
		if cartID == 0 {
			var hash []byte
			if o.UserID == 0 {
				newKey, hash = NewKey()
			}
			if cartID, err = createCart(ctx, tx, shopID, o.UserID, hash); err != nil {
				return err
			}
		}
		cur, lines, err := itemState(ctx, tx, shopID, cartID, variantID)
		if err != nil {
			return err
		}
		if err := checkVariant(ctx, tx, shopID, variantID, cur+qty); err != nil {
			return err
		}
		if err := CheckRoom(lines, cur > 0); err != nil {
			return err
		}
		return setItem(ctx, tx, shopID, cartID, variantID, cur+qty)
	})
	return newKey, err
}

// SetQuantity (PATCH): shudhu cart e thaka variant (na hole ErrNotFound), qty 1..min(stock, 50).
func SetQuantity(ctx context.Context, d *sql.DB, shopID int64, o Owner, variantID int64, qty int) error {
	if qty < 1 {
		return ErrInvalidQuantity
	}
	return db.InTx(ctx, d, func(tx *sql.Tx) error {
		cartID, err := touchCart(ctx, tx, shopID, o)
		if err != nil || cartID == 0 {
			return cmp.Or(err, ErrNotFound)
		}
		cur, _, err := itemState(ctx, tx, shopID, cartID, variantID)
		if err != nil || cur == 0 {
			return cmp.Or(err, ErrNotFound)
		}
		if err := checkVariant(ctx, tx, shopID, variantID, qty); err != nil {
			return err
		}
		return setItem(ctx, tx, shopID, cartID, variantID, qty)
	})
}

// Remove (DELETE, 8.4 item 7): idempotent; cart ba item na thakleo nil.
func Remove(ctx context.Context, d *sql.DB, shopID int64, o Owner, variantID int64) error {
	return db.InTx(ctx, d, func(tx *sql.Tx) error {
		cartID, err := touchCart(ctx, tx, shopID, o)
		if err != nil || cartID == 0 {
			return err
		}
		return deleteItem(ctx, tx, shopID, cartID, variantID)
	})
}
