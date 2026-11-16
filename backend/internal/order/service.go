package order

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/coupon"
	"vendrix/backend/internal/pricing"
	"vendrix/backend/internal/shipping"
)

// ErrCartEmpty: cart e kono line nai (400 cart_empty, 8.6 step 7).
var ErrCartEmpty = errors.New("order: cart khali")

// FieldError: ei field bhul (400 validation_failed + fields).
type FieldError string

func (f FieldError) Error() string { return "order: field bhul: " + string(f) }

// QuoteInput: handler e format check hoye ashe (phone normalize, coupon Normalize, district/area trim).
type QuoteInput struct {
	CouponCode, District, Area, PaymentMethod, Phone string
}

// Quote: 7.5 er response.
type Quote struct {
	Subtotal       int64    `json:"subtotal"`
	Discount       int64    `json:"discount"`
	DeliveryCharge int64    `json:"delivery_charge"`
	Total          int64    `json:"total"`
	OTPRequired    bool     `json:"otp_required"`
	Problems       []string `json:"problems"`
}

func (q *Quote) problem(code string) {
	if !slices.Contains(q.Problems, code) {
		q.Problems = append(q.Problems, code)
	}
}

// QuoteCart (7.5, 8.5, 8.19 item 2, 5): kichu save na (coupon Peek, cart read-only). Total na banano jay
// emon bhul error (area_not_served, cart_empty); baki problems e, total shoho: kena jay na emon line bad,
// coupon na khatle coupon chhara total, zone min_order (discount er por) er kom.
func QuoteCart(ctx context.Context, d *sql.DB, shopID int64, o cart.Owner, in QuoteInput, ip string) (Quote, error) {
	s, err := loadSettings(ctx, d, shopID)
	if err != nil {
		return Quote{}, err
	}
	if !slices.Contains(s.PaymentMethods, in.PaymentMethod) {
		return Quote{}, FieldError("payment_method")
	}
	if o.UserID != 0 { // login e phone account er-i (8.19 item 4)
		p, err := userPhone(ctx, d, shopID, o.UserID)
		if err != nil {
			return Quote{}, err
		}
		if p != in.Phone {
			return Quote{}, FieldError("phone")
		}
	}
	zone, err := shipping.Lookup(ctx, d, shopID, in.District, in.Area)
	if err != nil {
		return Quote{}, err
	}
	lines, err := cart.CheckoutLines(ctx, d, shopID, o)
	if err != nil {
		return Quote{}, err
	}
	if len(lines) == 0 {
		return Quote{}, ErrCartEmpty
	}
	q := Quote{Problems: []string{}}
	var priced []pricing.Line
	for _, l := range lines {
		if l.Problem != "" {
			q.problem(l.Problem)
			continue
		}
		priced = append(priced, l.Line)
	}
	var pc *pricing.Coupon
	if in.CouponCode != "" {
		c, err := coupon.Peek(ctx, d, shopID, in.CouponCode, in.Phone)
		switch code := coupon.ErrorCode(err); {
		case code != "":
			q.problem(code)
		case err != nil:
			return Quote{}, err
		default:
			pc = &pricing.Coupon{Kind: c.Kind, Value: c.Value, MinOrder: c.MinOrder, MaxDiscount: c.MaxDiscount, VendorID: c.VendorID}
		}
	}
	pz := pricing.Zone{Charge: zone.Charge, FreeDeliveryOver: zone.FreeDeliveryOver}
	r, err := pricing.Calculate(priced, pc, pz, s.DeliveryMode)
	if errors.Is(err, pricing.ErrCouponNotApplicable) { // 8.5: coupon chhara abar
		q.problem("coupon_invalid")
		r, err = pricing.Calculate(priced, nil, pz, s.DeliveryMode)
	}
	if err != nil {
		return Quote{}, err // ErrInvalidInput: bug, caller log kore (8.5)
	}
	if r.Subtotal-r.Discount < zone.MinOrder {
		q.problem("below_min_order")
	}
	q.Subtotal, q.Discount, q.DeliveryCharge, q.Total = r.Subtotal, r.Discount, r.DeliveryCharge, r.Total
	if o.UserID == 0 { // login e OTP kokhono na, history lage na
		cod := in.PaymentMethod == "cod"
		h, err := LoadRiskHistory(ctx, d, shopID, in.Phone, in.Phone, ip)
		if err != nil {
			return Quote{}, err
		}
		q.OTPRequired = OTPRequired(s.OTPMode, RiskFlags(h, s.Risk, cod, q.Total), cod, false)
	}
	return q, nil
}
