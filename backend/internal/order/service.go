package order

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/catalog"
	"vendrix/backend/internal/coupon"
	"vendrix/backend/internal/db"
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

var (
	ErrKeyReused     = errors.New("order: ekoi Idempotency-Key e alada body")   // 422 idempotency_key_reused
	ErrPhoneBlocked  = errors.New("order: phone blocked, COD na")               // 403 phone_blocked
	ErrBelowMinOrder = errors.New("order: zone er min_order er kom")            // 409 below_min_order
	ErrVendorClosed  = errors.New("order: vendor active na")                    // 409 vendor_unavailable
	ErrOTPRequired   = errors.New("order: OTP lagbe")                           // 409 otp_required
	ErrOTPInvalid    = errors.New("order: OTP bhul")                            // 400 otp_invalid
	ErrOTPAttempts   = errors.New("order: OTP e 5 bar cheshta")                 // 400 otp_too_many_attempts
	ErrTurnstile     = errors.New("order: Turnstile fail")                      // 400 turnstile_failed
	errKeyTaken      = errors.New("order: ekoi key er order insert e conflict") // lock er por hoy na
)

// LimitError: 429 rate_limited + Retry-After (order er rate limit ba OTP send limit/SMS cap).
type LimitError struct{ Retry time.Duration }

func (e LimitError) Error() string { return "order: rate limit" }

// OutOfStockError: 409 out_of_stock + details {variant_id} (8.19 item 12 v).
type OutOfStockError struct{ VariantID int64 }

func (e OutOfStockError) Error() string { return "order: stock nai" }

// PriceChangedError: 409 price_changed + details notun quote (8.19 item 12 iv).
type PriceChangedError struct{ pricing.Totals }

func (e PriceChangedError) Error() string { return "order: dam bodleche" }

// Person: customer ba recipient; handler e trim/normalize hoye ashe.
type Person struct {
	Name, Phone, Email string
}

// PlaceInput: handler e format check hoye ashe (8.19 item 10); Recipient customer er copy hote pare.
type PlaceInput struct {
	Customer, Recipient                                   Person
	District, Area, Line, Note, CouponCode, PaymentMethod string
	ExpectedTotal                                         int64
	OTPCode, TurnstileToken                               string
	Host, Timezone                                        string // Turnstile er hostname (shop domain), OTP lock alert er din
	Key                                                   string
	Hash                                                  []byte // idempotency_hash (8.19 item 12 ii)
	Locale, IP                                            string
}

// Placed: 7.5 er response; Created false = ager order (200).
type Placed struct {
	OrderNumber string  `json:"order_number"`
	PublicToken string  `json:"public_token"`
	Status      string  `json:"status"`
	PaymentURL  *string `json:"payment_url"`
	Created     bool    `json:"-"`
}

// again: ager order (8.6 step 2): hash mile → sheta, na → ErrKeyReused.
func (p prior) again(hash []byte) (Placed, error) {
	if !bytes.Equal(p.hash, hash) {
		return Placed{}, ErrKeyReused
	}
	return p.resp, nil
}

// authErr: auth er OTP error order er error e (handler ek jaygay map kore).
func authErr(err error) error {
	if retry, ok := auth.RetryAfter(err); ok {
		return LimitError{retry}
	}
	switch {
	case errors.Is(err, auth.ErrOTPInvalid):
		return ErrOTPInvalid
	case errors.Is(err, auth.ErrOTPAttempts):
		return ErrOTPAttempts
	case errors.Is(err, auth.ErrPhoneBlocked):
		return ErrPhoneBlocked
	}
	return err
}

// PlaceOrder (8.6 step 1–18). Bhul hole kichu save na.
func PlaceOrder(ctx context.Context, d *sql.DB, g Guard, shopID int64, o cart.Owner, in PlaceInput) (Placed, error) {
	if p, err := findByKey(ctx, d, shopID, in.Key); err != nil || p.found {
		if err != nil {
			return Placed{}, err
		}
		return p.again(in.Hash)
	}
	// step 3: otp_code thakle Turnstile na, kintu code shob shomoy verify (OTP lagbe kina dekhar age); step 8
	// er verify ekhanei, jate bhul code e bot Turnstile chhara zone/cart/rate limit porjonto na pouchay
	var otpID int64
	if in.OTPCode != "" {
		id, err := auth.VerifyCheckoutOTP(ctx, d, g.OTPSecret, shopID, in.Timezone, in.Customer.Phone, in.OTPCode)
		if err != nil {
			return Placed{}, authErr(err)
		}
		otpID = id
	} else {
		ok, err := g.Turnstile.Verify(ctx, in.TurnstileToken, in.IP, in.Host)
		if err != nil {
			return Placed{}, err
		}
		if !ok {
			return Placed{}, ErrTurnstile
		}
	}
	s, err := loadSettings(ctx, d, shopID)
	if err != nil {
		return Placed{}, err
	}
	if !slices.Contains(s.PaymentMethods, in.PaymentMethod) {
		return Placed{}, FieldError("payment_method")
	}
	loggedIn := o.UserID != 0
	if loggedIn { // 8.19 item 4: login e customer phone = account phone
		p, err := userPhone(ctx, d, shopID, o.UserID)
		if err != nil {
			return Placed{}, err
		}
		if p != in.Customer.Phone {
			return Placed{}, FieldError("customer.phone")
		}
	}
	// 7.8, 8.19 item 9: shudhu hoya order gona
	if retry, hit, err := orderLimit(ctx, d, shopID, in.Customer.Phone, in.IP); err != nil || hit {
		if err != nil {
			return Placed{}, err
		}
		return Placed{}, LimitError{retry}
	}
	zone, err := shipping.Lookup(ctx, d, shopID, in.District, in.Area)
	if err != nil {
		return Placed{}, err
	}
	cod := in.PaymentMethod == "cod"
	if cod {
		blocked, err := phoneBlocked(ctx, d, shopID, in.Customer.Phone, in.Recipient.Phone)
		if err != nil {
			return Placed{}, err
		}
		if blocked {
			return Placed{}, ErrPhoneBlocked
		}
	}
	lines, err := checkoutLines(ctx, d, shopID, o)
	if err != nil {
		// ekoi key er onno request ei matro commit kore cart khali/stock 0 korle eta tar fol: ager order dao
		if p, perr := findByKey(ctx, d, shopID, in.Key); perr == nil && p.found {
			return p.again(in.Hash)
		}
		return Placed{}, err
	}
	h, err := LoadRiskHistory(ctx, d, shopID, in.Customer.Phone, in.Recipient.Phone, in.IP)
	if err != nil {
		return Placed{}, err
	}
	flags := RiskFlags(h, s.Risk, cod, in.ExpectedTotal)
	otpVerified := loggedIn || otpID != 0
	if !otpVerified && OTPRequired(s.OTPMode, flags, cod, loggedIn) { // step 8: OTP customer phone e (8.19 item 4)
		if err := auth.SendCheckoutOTP(ctx, d, g.OTPSecret, shopID, in.Timezone, in.Customer.Phone); err != nil {
			return Placed{}, authErr(err)
		}
		return Placed{}, ErrOTPRequired
	}
	status, payStatus := "confirmed", "unpaid"
	switch {
	case !cod:
		status, payStatus = "pending", "pending"
	case ConfirmRequired(s.CODConfirmation, flags, cod, otpVerified):
		status = "pending"
	}
	days := 10
	if zone.SameCity {
		days = 5
	}

	var out Placed
	err = db.InTx(ctx, d, func(tx *sql.Tx) error {
		out = Placed{}
		// step 9
		if err := lockKey(ctx, tx, shopID, in.Key); err != nil {
			return err
		}
		p, err := findByKey(ctx, tx, shopID, in.Key)
		if err != nil {
			return err
		}
		if p.found {
			out, err = p.again(in.Hash)
			return err
		}
		// ROLLBACK hole OTP abar chole (price_changed er por ekoi code, 14.2 test 40)
		if otpID != 0 {
			if ok, err := auth.ConsumeOTP(ctx, tx, shopID, otpID); err != nil || !ok {
				if err != nil {
					return err
				}
				return ErrOTPInvalid
			}
		}
		// step 10: variant_id er kram e (deadlock na, 8.8)
		slices.SortFunc(lines, func(a, b cart.CheckoutLine) int { return cmp.Compare(a.VariantID, b.VariantID) })
		items := make([]item, len(lines))
		priced := make([]pricing.Line, len(lines))
		for i, l := range lines {
			st, ok, err := takeStock(ctx, tx, shopID, l.VariantID, l.Line.Qty)
			if err != nil {
				return err
			}
			if !ok || (st.VariableWeight && !st.NominalWeightG.Valid) {
				return OutOfStockError{VariantID: l.VariantID}
			}
			it := item{VariantID: l.VariantID, Stock: st, Qty: l.Line.Qty}
			priced[i] = pricing.Line{VendorID: st.VendorID, UnitPrice: st.Price, Qty: l.Line.Qty, VariableWeight: st.VariableWeight}
			if st.VariableWeight {
				w := int(st.NominalWeightG.Int64) * l.Line.Qty
				priced[i].WeightG, it.OrderedWeightG = w, sql.NullInt64{Int64: int64(w), Valid: true}
			}
			if it.Label, err = variantLabel(st.Options); err != nil {
				return err
			}
			items[i] = it
		}
		// step 11
		var c *coupon.Coupon
		var pc *pricing.Coupon
		if in.CouponCode != "" {
			cp, err := coupon.Use(ctx, tx, shopID, in.CouponCode, in.Customer.Phone)
			if err != nil {
				return err
			}
			c = &cp
			pc = &pricing.Coupon{Kind: cp.Kind, Value: cp.Value, MinOrder: cp.MinOrder, MaxDiscount: cp.MaxDiscount, VendorID: cp.VendorID}
		}
		// step 12
		r, err := pricing.Calculate(priced, pc, pricing.Zone{Charge: zone.Charge, FreeDeliveryOver: zone.FreeDeliveryOver}, s.DeliveryMode)
		if errors.Is(err, pricing.ErrCouponNotApplicable) {
			return coupon.ErrInvalid
		}
		if err != nil {
			return err // ErrInvalidInput: bug, ROLLBACK + 500 log (8.5)
		}
		if r.Subtotal-r.Discount < zone.MinOrder {
			return ErrBelowMinOrder
		}
		if r.Total != in.ExpectedTotal {
			return PriceChangedError{r.Totals}
		}
		// step 13–15
		number, err := nextOrderNumber(ctx, tx, shopID)
		if err != nil {
			return err
		}
		token := rand.Text()
		var confirmHash []byte // 8.10: shudhu COD pending (confirm lage); token nije DB te na
		if cod && status == "pending" {
			confirmHash = ConfirmHash(ConfirmToken(g.ConfirmSecret, token))
		}
		orderID, placedAt, ok, err := insertOrder(ctx, tx, newOrder{ShopID: shopID, UserID: o.UserID, ZoneID: zone.ID, Number: number,
			Token: token, Key: in.Key, Hash: in.Hash, In: in, Totals: r.Totals, Coupon: c, PaymentStatus: payStatus,
			Status: status, Display: Summary(Status(status), nil), Flags: flags, OTPVerified: otpVerified, DeadlineDays: days,
			ConfirmHash: confirmHash})
		if err != nil {
			return err
		}
		if !ok {
			return errKeyTaken
		}
		// step 16
		fid := map[int64]int64{}
		for i, g := range r.Groups {
			if fid[g.VendorID], err = insertFulfillment(ctx, tx, shopID, orderID, i+1, g); err != nil {
				return err
			}
		}
		vendorFunded := c != nil && c.FundedBy == "vendor"
		for i, it := range items {
			it.FulfillmentID, it.Result = fid[it.Stock.VendorID], r.Lines[i]
			it.Commission = pricing.Commission(it.Result, vendorFunded, it.Stock.CommissionBps)
			if err := insertItem(ctx, tx, shopID, orderID, it); err != nil {
				return err
			}
		}
		if err := insertPlaced(ctx, tx, shopID, orderID, o.UserID, status, c, in.Customer.Phone); err != nil {
			return err
		}
		// step 17
		if err := cart.Clear(ctx, tx, shopID, o); err != nil {
			return err
		}
		// step 18 (online er auto_cancel_unpaid M10 e)
		var autoCancel time.Time
		if cod && status == "pending" {
			hours := s.AutoCancelHours
			if hours <= 0 { // setting nai: 9.2 er default, shathe shathe cancel na
				hours = 24
			}
			autoCancel = placedAt.Add(time.Duration(hours) * time.Hour)
		}
		if err := enqueuePlaced(ctx, tx, shopID, orderID, autoCancel); err != nil {
			return err
		}
		out = Placed{OrderNumber: number, PublicToken: token, Status: status, Created: true}
		return nil
	})
	if errors.Is(err, errKeyTaken) {
		if p, perr := findByKey(ctx, d, shopID, in.Key); perr == nil && p.found {
			return p.again(in.Hash)
		}
	}
	return out, err
}

// checkoutLines (8.6 step 7): khali → ErrCartEmpty; kena jay na emon line thakle fail (chupchap bad na, 8.19 item 12 v).
func checkoutLines(ctx context.Context, d *sql.DB, shopID int64, o cart.Owner) ([]cart.CheckoutLine, error) {
	lines, err := cart.CheckoutLines(ctx, d, shopID, o)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, ErrCartEmpty
	}
	for _, l := range lines {
		switch l.Problem {
		case "vendor_unavailable":
			return nil, ErrVendorClosed
		case "out_of_stock":
			return nil, OutOfStockError{VariantID: l.VariantID}
		}
	}
	return lines, nil
}

// variantLabel: order_items.variant_label snapshot {bn, en} (7.4 er label); option na thakle {}.
func variantLabel(raw []byte) ([]byte, error) {
	var opts map[string]catalog.Text
	if err := json.Unmarshal(raw, &opts); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(catalog.Text{"bn": cart.Label(opts, "bn"), "en": cart.Label(opts, "en")})
}
