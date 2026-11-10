// Package pricing: shob taka er hishab ekhane. Pure function, database ba HTTP nai.
// Order er mot taka-r pashapashi protiti line er discount er bhag ar protiti vendor er bhag dey.
// v1.3: Calculate ar Reprice bhul input e hishab na kore ErrInvalidInput dey.
// v1.4: Reprice e je vendor er line bodlay nai tar taka hubohu ager; coupon ashe order er snapshot theke.
// v1.5: fixed line e WeightG 0; Reprice Original er taka nijeder moddhe mile kina dekhe; order_items er
// row theke Line shudhu LineFromItem diye (removed item e Qty 0).
// v1.7: Calculate e coupon er eligible subtotal 0 ba min_order er kom hole ErrCouponNotApplicable
// (checkout e 409 coupon_invalid); caller nije eligible subtotal hishab kore na.
//
// Feature doc: docs/features/pricing.md
package pricing

import (
	"errors"
	"fmt"
	"math/bits"
)

// Line: cart ba order er ekta item.
type Line struct {
	VendorID       int64
	UnitPrice      int64 // poisha; VariableWeight hole 1 kg er dam
	Qty            int
	VariableWeight bool
	WeightG        int // VariableWeight hole PURO line er mot ojon (gram); Qty diye gun hoy na
}

// LineTotal: ojon-variable hole dam × gram / 1000, kachakachi poisha te round.
func LineTotal(l Line) int64 {
	if l.VariableWeight {
		return (l.UnitPrice*int64(l.WeightG) + 500) / 1000
	}
	return l.UnitPrice * int64(l.Qty)
}

// Coupon: discount er jonno jotota dorkar.
type Coupon struct {
	Kind        string // "percent" ba "fixed"
	Value       int64  // percent: 1-100, fixed: poisha
	MinOrder    int64  // eligible line gulor subtotal er upor
	MaxDiscount int64  // 0 = shima nai
	VendorID    int64  // 0 = shob vendor er line; na hole shudhu oi vendor er line
}

func (c *Coupon) eligible(l Line) bool { return c.VendorID == 0 || c.VendorID == l.VendorID }

// Discount: percent e niche round (floor), cap, ar kokhono subtotal er beshi na.
// MinOrder na mille 0. v1.7: Calculate eta-r age-i ErrCouponNotApplicable dey; Reprice MinOrder 0 dhore.
func Discount(c Coupon, subtotal int64) int64 {
	if subtotal < c.MinOrder {
		return 0
	}
	d := c.Value
	if c.Kind == "percent" {
		d = subtotal * c.Value / 100
		if c.MaxDiscount > 0 && d > c.MaxDiscount {
			d = c.MaxDiscount
		}
	}
	return min(d, subtotal)
}

// Zone: delivery area.
type Zone struct {
	Charge           int64
	FreeDeliveryOver int64 // 0 = free delivery nai
}

// DeliveryMode: marketplace e delivery charge kivabe (settings marketplace.delivery_charge_mode).
type DeliveryMode string

const (
	PerOrder  DeliveryMode = "per_order"  // ekta charge, vendor gulor moddhe subtotal onujayi bhag
	PerVendor DeliveryMode = "per_vendor" // protiti vendor er parcel e Zone.Charge
)

// Totals: char ta number. Database er CHECK er sathe mile.
type Totals struct {
	Subtotal, Discount, DeliveryCharge, Total int64
}

// LineResult: ek line er hishab (order_items.line_total, discount_share).
type LineResult struct {
	LineTotal, DiscountShare int64
}

// Group: ek vendor er bhag (order_fulfillments er taka).
type Group struct {
	VendorID int64
	Totals
}

// Result: order er taka, line er taka (input er kram e), vendor er taka (prothom dekha kram e).
type Result struct {
	Totals
	Lines  []LineResult
	Groups []Group
}

// Allocate: total ke weights onujayi bhag kore. Largest remainder: bhag gulor jog thik total,
// soman bhagshesh hole age-r index ek poisha beshi pay. Shob weight 0 hole puro ta prothom index e.
func Allocate(total int64, weights []int64) []int64 {
	out := make([]int64, len(weights))
	if len(weights) == 0 || total == 0 {
		return out
	}
	var sum int64
	for _, w := range weights {
		sum += w
	}
	if sum == 0 {
		out[0] = total
		return out
	}
	rem := make([]uint64, len(weights))
	var given int64
	for i, w := range weights {
		hi, lo := bits.Mul64(uint64(total), uint64(w)) // total × w overflow chhara
		q, r := bits.Div64(hi, lo, uint64(sum))
		out[i], rem[i] = int64(q), r
		given += int64(q)
	}
	for left := total - given; left > 0; left-- {
		best := -1
		for i := range weights {
			if weights[i] > 0 && (best < 0 || rem[i] > rem[best]) {
				best = i
			}
		}
		out[best]++
		rem[best] = 0
	}
	return out
}

// ErrInvalidInput: v1.3. Line, coupon, zone ba mode bhul. Jemon ojon-variable line e WeightG 0
// (nominal_weight_g NULL) hole line free hoye jeto, coupon kind er banan bhul hole fixed dhore
// nito. Eta caller er bug: ROLLBACK ar log, kokhono chupchap hishab na.
var ErrInvalidInput = errors.New("pricing: invalid input")

func invalid(msg string) error { return fmt.Errorf("%w: %s", ErrInvalidInput, msg) }

// ErrCouponNotApplicable (v1.7): coupon er eligible line er subtotal 0 (vendor coupon er vendor er line
// cart e nai, ba shudhu ৳0 er line) ba min_order er kom. Checkout e ROLLBACK ar 409 coupon_invalid
// (8.6 step 12); quote e coupon chhara abar Calculate kore shomossha dekhay. Ager version e discount 0
// diye order hoto, ar used_count ar redemption (per_phone_limit) khoroch hoto.
var ErrCouponNotApplicable = errors.New("pricing: coupon not applicable")

func validCoupon(c *Coupon) error {
	switch {
	case c == nil:
		return nil
	case c.Kind != "percent" && c.Kind != "fixed":
		return invalid("coupon kind percent ba fixed, pawa gelo " + c.Kind)
	case c.Value <= 0 || (c.Kind == "percent" && c.Value > 100):
		return invalid("coupon value: percent 1-100, fixed > 0")
	case c.MinOrder < 0 || c.MaxDiscount < 0 || c.VendorID < 0:
		return invalid("coupon er min_order, max_discount ba vendor_id minus")
	}
	return nil
}

func validate(lines []Line, c *Coupon, z Zone, mode DeliveryMode) error {
	for i, l := range lines {
		if l.Qty <= 0 || l.UnitPrice < 0 || (l.VariableWeight && l.WeightG <= 0) || (!l.VariableWeight && l.WeightG != 0) {
			return invalid(fmt.Sprintf("line %d: qty > 0, dam >= 0, WeightG ojon-variable hole > 0, fixed hole 0", i))
		}
	}
	if z.Charge < 0 || z.FreeDeliveryOver < 0 {
		return invalid("zone er charge ba free_delivery_over minus")
	}
	if mode != PerOrder && mode != PerVendor {
		return invalid("delivery mode per_order ba per_vendor, pawa gelo " + string(mode))
	}
	return validCoupon(c)
}

// Calculate: checkout ar quote er puro hishab. coupon nil hote pare. v1.3: bhul input e error.
// v1.7: coupon na khatle (eligible subtotal 0 ba min_order er kom) ErrCouponNotApplicable.
func Calculate(lines []Line, coupon *Coupon, zone Zone, mode DeliveryMode) (Result, error) {
	if err := validate(lines, coupon, zone, mode); err != nil {
		return Result{}, err
	}
	r := Result{Lines: make([]LineResult, len(lines))}
	for i, l := range lines {
		r.Lines[i].LineTotal = LineTotal(l)
		r.Subtotal += r.Lines[i].LineTotal
	}
	if coupon != nil {
		var eligible int64
		for i, l := range lines {
			if coupon.eligible(l) {
				eligible += r.Lines[i].LineTotal
			}
		}
		if eligible == 0 || eligible < coupon.MinOrder { // v1.7
			return Result{}, ErrCouponNotApplicable
		}
		r.Discount = Discount(*coupon, eligible)
		allocateDiscount(&r, lines, coupon)
	}
	r.DeliveryCharge = 0
	if !(zone.FreeDeliveryOver > 0 && r.Subtotal-r.Discount >= zone.FreeDeliveryOver) {
		r.DeliveryCharge = zone.Charge
	}
	buildGroups(&r, lines)
	if mode == PerVendor && r.DeliveryCharge > 0 {
		r.DeliveryCharge = zone.Charge * int64(len(r.Groups))
		for i := range r.Groups {
			r.Groups[i].DeliveryCharge = zone.Charge
		}
	} else {
		w := make([]int64, len(r.Groups))
		for i, g := range r.Groups {
			w[i] = g.Subtotal
		}
		for i, d := range Allocate(r.DeliveryCharge, w) {
			r.Groups[i].DeliveryCharge = d
		}
	}
	finish(&r)
	return r, nil
}

// Original: packing ba order edit er age order er hishab (database e save kora).
type Original struct {
	Lines   []Line
	Result  Result
	Coupon  *Coupon
	Prepaid bool
	Edit    bool // v1.2: true = order edit (item bad/qty kom), false = packing er asol ojon
}

// ErrLineCount: newLines er dairgho Original.Lines er moto na.
var ErrLineCount = errors.New("pricing: newLines length mismatch")

// Reprice: packing (asol ojon) ba order edit (item bad, quantity kom) er por notun hishab.
// newLines er kram ar dairgho Original.Lines er moto; bad deya line e Qty 0 / WeightG 0.
// Niyom (8.11): coupon er shorto abar check hoy na (min_order na); percent coupon notun
// eligible subtotal e; fixed coupon packing e ager discount (notun eligible er beshi na),
// edit e (Edit = true) eligible subtotal er onupate;
// vendor er delivery charge ager ta-i (oi vendor er shob line bad hole 0);
// prepaid hole kono line ager line total er beshi na, ar parthokko refund.
// v1.3: line er vendor, dam ar dhoron snapshot er moto-i thakbe; qty kokhono barbe na (notun order);
// edit e ojon-o barbe na (packing e asol ojon beshi hote pare); bad deya ojon-variable line e
// Qty ar WeightG duitai 0. Na mille ErrInvalidInput.
// v1.4: (1) je vendor er kono line bodlay nai (Qty, WeightG ager moto), tar line total, discount er
// bhag ar delivery hubohu ager, tai ek vendor er packing onno vendor er fulfillment bodlay na. Bodla
// vendor gulo uporer niyom e, kintu shudhu nijeder bhag e (repriceDiscount). (2) Coupon holo order er
// snapshot (orders.coupon_*); ager discount ache kintu Coupon nil hole ErrInvalidInput. (3) Delivery 0
// shudhu jodi vendor er shob line bad (Qty 0); ৳0 er line baki thakleo delivery thake.
// v1.5: (1) fixed line e WeightG 0 (dame lage na; thakle vendor bhul kore "bodla" dhora porto).
// (2) Original database theke ashe, cross-row CHECK nai: checkOriginal na mille ErrInvalidInput.
// (3) Line ar LineResult banabe shudhu LineFromItem (removed item e Qty 0, nije banale COD e abar bill).
func Reprice(o Original, newLines []Line) (r Result, refund int64, err error) {
	if len(newLines) != len(o.Lines) || len(o.Result.Lines) != len(o.Lines) {
		return Result{}, 0, ErrLineCount
	}
	if err := validCoupon(o.Coupon); err != nil {
		return Result{}, 0, err
	}
	if o.Coupon == nil && o.Result.Discount > 0 {
		return Result{}, 0, invalid("ager discount ache kintu coupon nai: order er coupon snapshot dao")
	}
	if err := checkOriginal(o); err != nil {
		return Result{}, 0, err
	}
	changed := map[int64]bool{} // v1.4: je vendor er kono line bodleche
	alive := map[int64]bool{}   // v1.4: je vendor er kono line baki ache
	for i, l := range newLines {
		ol := o.Lines[i]
		switch {
		case l.VendorID != ol.VendorID || l.UnitPrice != ol.UnitPrice || l.VariableWeight != ol.VariableWeight:
			return Result{}, 0, invalid(fmt.Sprintf("line %d: vendor, dam ba dhoron snapshot theke alada", i))
		case !l.VariableWeight && (l.WeightG != 0 || ol.WeightG != 0):
			return Result{}, 0, invalid(fmt.Sprintf("line %d: fixed line e WeightG 0 (v1.5)", i))
		case l.Qty < 0 || l.WeightG < 0 || l.Qty > ol.Qty:
			return Result{}, 0, invalid(fmt.Sprintf("line %d: qty barano jay na (notun order)", i))
		case o.Edit && l.VariableWeight && l.WeightG > ol.WeightG:
			return Result{}, 0, invalid(fmt.Sprintf("line %d: edit e ojon barano jay na", i))
		case l.VariableWeight && (l.Qty == 0) != (l.WeightG == 0):
			return Result{}, 0, invalid(fmt.Sprintf("line %d: bad dile Qty ar WeightG duitai 0", i))
		}
		if l.Qty != ol.Qty || l.WeightG != ol.WeightG {
			changed[l.VendorID] = true
		}
		if l.Qty > 0 {
			alive[l.VendorID] = true
		}
	}
	r = Result{Lines: make([]LineResult, len(newLines))}
	for i, l := range newLines {
		if !changed[l.VendorID] {
			r.Lines[i] = o.Result.Lines[i] // v1.4: freeze
		} else {
			lt := LineTotal(l)
			if o.Prepaid && lt > o.Result.Lines[i].LineTotal {
				lt = o.Result.Lines[i].LineTotal
			}
			r.Lines[i].LineTotal = lt
		}
		r.Subtotal += r.Lines[i].LineTotal
	}
	if o.Coupon != nil && o.Result.Discount > 0 {
		repriceDiscount(&r, o, newLines, changed)
	}
	buildGroups(&r, newLines)
	oldDelivery := map[int64]int64{}
	for _, g := range o.Result.Groups {
		oldDelivery[g.VendorID] = g.DeliveryCharge
	}
	for i := range r.Groups {
		if alive[r.Groups[i].VendorID] {
			r.Groups[i].DeliveryCharge = oldDelivery[r.Groups[i].VendorID]
		}
	}
	finish(&r)
	if o.Prepaid && r.Total < o.Result.Total {
		refund = o.Result.Total - r.Total
	}
	return r, refund, nil
}

// repriceDiscount (v1.4): frozen vendor er line er discount er bhag already ager moto (r.Lines e copy).
// Bodla vendor gulor discount shuru hoy tader nijeder ager discount theke: percent e order er percent
// (cap shoho) jotota bodlay, tara thik totota pay (tai prepaid e total kokhono bare na, ek poisha o na);
// fixed e packing e ager ta, edit e eligible er onupate. Kokhono tader notun eligible er beshi na.
// Tarpor tader eligible line e line total onujayi bhag.
func repriceDiscount(r *Result, o Original, newLines []Line, changed map[int64]bool) {
	c := o.Coupon
	var frozenD, frozenE, origE, newE int64
	for i, l := range newLines {
		switch {
		case !changed[l.VendorID]:
			frozenD += o.Result.Lines[i].DiscountShare
			if c.eligible(l) {
				frozenE += o.Result.Lines[i].LineTotal
			}
		case c.eligible(l):
			origE += o.Result.Lines[i].LineTotal
			newE += r.Lines[i].LineTotal
		}
	}
	d := max(o.Result.Discount-frozenD, 0) // bodla vendor gulor ager discount
	switch {
	case c.Kind == "percent":
		pc := *c
		pc.MinOrder = 0
		d = max(d+Discount(pc, frozenE+newE)-Discount(pc, frozenE+origE), 0)
	case o.Edit && newE < origE:
		// v1.2: edit e fixed discount eligible subtotal er onupate kome. Item bad diye baki item
		// free hoy na, ar min_order er "cliff" e prepaid customer er total-o bare na.
		hi, lo := bits.Mul64(uint64(d), uint64(newE))
		q, _ := bits.Div64(hi, lo, uint64(origE))
		d = int64(q)
	}
	d = min(d, newE)
	w := make([]int64, len(newLines))
	for i, l := range newLines {
		if changed[l.VendorID] && c.eligible(l) {
			w[i] = r.Lines[i].LineTotal
		}
	}
	for i, s := range Allocate(d, w) {
		if changed[newLines[i].VendorID] {
			r.Lines[i].DiscountShare = s
		}
	}
	r.Discount = frozenD + d
}

// checkOriginal (v1.5): Original er line, vendor ar order er taka nijeder moddhe mile kina. Qty 0 er
// line e taka 0; COD te protiti line total = LineTotal(line), prepaid e tar beshi na (cap). Removed item
// ke database er quantity diye pathale COD te eta-i dhore (prepaid e cap thakay bill hoy na).
func checkOriginal(o Original) error {
	type sum struct{ sub, disc int64 }
	vendors := map[int64]*sum{}
	var sub, disc int64
	for i, lr := range o.Result.Lines {
		l := o.Lines[i]
		lt := LineTotal(l)
		switch {
		case lr.DiscountShare < 0 || lr.DiscountShare > lr.LineTotal: // line total minus holeo eta-i dhore
			return invalid(fmt.Sprintf("original line %d: line total ba discount er bhag bhul", i))
		case l.Qty == 0 && lr.LineTotal != 0:
			return invalid(fmt.Sprintf("original line %d: Qty 0 (removed) kintu taka ache", i))
		case !o.Prepaid && lr.LineTotal != lt, o.Prepaid && lr.LineTotal > lt:
			return invalid(fmt.Sprintf("original line %d: line total snapshot er sathe mile na (removed item e Qty 0?)", i))
		}
		if vendors[l.VendorID] == nil {
			vendors[l.VendorID] = &sum{}
		}
		vendors[l.VendorID].sub += lr.LineTotal
		vendors[l.VendorID].disc += lr.DiscountShare
		sub += lr.LineTotal
		disc += lr.DiscountShare
	}
	seen := map[int64]bool{}
	var delivery int64
	for _, g := range o.Result.Groups {
		v := vendors[g.VendorID]
		if v == nil || seen[g.VendorID] || g.Subtotal != v.sub || g.Discount != v.disc || g.DeliveryCharge < 0 ||
			g.Total != g.Subtotal-g.Discount+g.DeliveryCharge {
			return invalid(fmt.Sprintf("original vendor %d er fulfillment er taka line er sathe mile na", g.VendorID))
		}
		seen[g.VendorID] = true
		delivery += g.DeliveryCharge
	}
	t := o.Result.Totals
	if len(seen) != len(vendors) || t.Subtotal != sub || t.Discount != disc || t.DeliveryCharge != delivery ||
		t.Total != t.Subtotal-t.Discount+t.DeliveryCharge {
		return invalid("original order er taka line ar fulfillment er jog er sathe mile na")
	}
	return nil
}

// Item (v1.5): order_items er ek row, Reprice er Original ar notun line banate. Ojon 0 = NULL.
type Item struct {
	VendorID       int64
	UnitPrice      int64
	Quantity       int  // database er quantity; removed item eo ager quantity thake (CHECK > 0)
	VariableWeight bool // order_items.variable_weight (snapshot), products theke na
	OrderedWeightG int
	PackedWeightG  int
	Removed        bool // status = 'removed'
	LineTotal      int64
	DiscountShare  int64
}

// LineFromItem (v1.5): order_items er row theke Line ar tar ager LineResult. Packing ar edit, shob
// jaygay ei function-i; nije Line banabe na. Removed hole Qty ar WeightG 0; ojon-variable e WeightG =
// packed_weight_g, na thakle ordered_weight_g; fixed line e WeightG 0.
func LineFromItem(it Item) (Line, LineResult) {
	l := Line{VendorID: it.VendorID, UnitPrice: it.UnitPrice, Qty: it.Quantity, VariableWeight: it.VariableWeight}
	switch {
	case it.Removed:
		l.Qty = 0
	case it.VariableWeight && it.PackedWeightG > 0:
		l.WeightG = it.PackedWeightG
	case it.VariableWeight:
		l.WeightG = it.OrderedWeightG
	}
	return l, LineResult{LineTotal: it.LineTotal, DiscountShare: it.DiscountShare}
}

// EditWeightG (v1.5): edit e ojon-variable line er qty komale notun mot ojon onupate (niche round,
// kom-poksha 1 gram); qty 0 hole 0. Edit pack er age (8.11), tai packed_weight_g thake na.
func EditWeightG(orderedG, oldQty, newQty int) int {
	if newQty <= 0 || oldQty <= 0 {
		return 0
	}
	return max(1, orderedG*newQty/oldQty)
}

// Commission: marketplace e platform er bhag. shop-funded coupon e vendor er commission
// discount er ager dam theke (discount platform bohon kore), vendor-funded e discount er porer dam theke.
func Commission(lr LineResult, vendorFunded bool, bps int) int64 {
	base := lr.LineTotal
	if vendorFunded {
		base -= lr.DiscountShare
	}
	return base * int64(bps) / 10000
}

func allocateDiscount(r *Result, lines []Line, c *Coupon) {
	w := make([]int64, len(lines))
	for i, l := range lines {
		if c.eligible(l) {
			w[i] = r.Lines[i].LineTotal
		}
	}
	for i, d := range Allocate(r.Discount, w) {
		r.Lines[i].DiscountShare = d
	}
}

func buildGroups(r *Result, lines []Line) {
	idx := map[int64]int{}
	r.Groups = r.Groups[:0]
	for i, l := range lines {
		g, ok := idx[l.VendorID]
		if !ok {
			g = len(r.Groups)
			idx[l.VendorID] = g
			r.Groups = append(r.Groups, Group{VendorID: l.VendorID})
		}
		r.Groups[g].Subtotal += r.Lines[i].LineTotal
		r.Groups[g].Discount += r.Lines[i].DiscountShare
	}
}

func finish(r *Result) {
	r.DeliveryCharge = 0
	for i := range r.Groups {
		g := &r.Groups[i]
		g.Total = g.Subtotal - g.Discount + g.DeliveryCharge
		r.DeliveryCharge += g.DeliveryCharge
	}
	r.Total = r.Subtotal - r.Discount + r.DeliveryCharge
}
