package pricing

import (
	"errors"
	"math/rand"
	"slices"
	"testing"
)

func TestCalculate(t *testing.T) {
	cases := []struct {
		name   string
		lines  []Line
		coupon *Coupon
		zone   Zone
		mode   DeliveryMode
		want   Totals
	}{
		// v1.0 er case gulo (ek vendor): hubohu ager result
		{"simple", []Line{{VendorID: 1, UnitPrice: 18000, Qty: 2}}, nil, Zone{Charge: 6000}, PerOrder,
			Totals{36000, 0, 6000, 42000}},
		{"percent floor + cap", []Line{{VendorID: 1, UnitPrice: 33333, Qty: 1}},
			&Coupon{Kind: "percent", Value: 15, MaxDiscount: 4000}, Zone{Charge: 6000}, PerOrder,
			Totals{33333, 4000, 6000, 33333 - 4000 + 6000}},
		{"percent floor", []Line{{VendorID: 1, UnitPrice: 999, Qty: 1}},
			&Coupon{Kind: "percent", Value: 10}, Zone{}, PerOrder, Totals{999, 99, 0, 900}},
		{"fixed never above subtotal", []Line{{VendorID: 1, UnitPrice: 5000, Qty: 1}},
			&Coupon{Kind: "fixed", Value: 10000}, Zone{Charge: 6000}, PerOrder, Totals{5000, 5000, 6000, 6000}},
		// v1.7: "min order not met" TestCouponNotApplicable e (ekhon ErrCouponNotApplicable, 8.5)
		{"free delivery after discount", []Line{{VendorID: 1, UnitPrice: 110000, Qty: 1}},
			&Coupon{Kind: "fixed", Value: 20000}, Zone{Charge: 6000, FreeDeliveryOver: 100000}, PerOrder,
			Totals{110000, 20000, 6000, 96000}},
		{"variable weight 1.08 kg at 650 tk/kg", []Line{{VendorID: 1, UnitPrice: 65000, Qty: 1, VariableWeight: true, WeightG: 1080}},
			nil, Zone{}, PerOrder, Totals{70200, 0, 0, 70200}},
		{"variable weight rounding", []Line{{VendorID: 1, UnitPrice: 33333, Qty: 1, VariableWeight: true, WeightG: 1}},
			nil, Zone{}, PerOrder, Totals{33, 0, 0, 33}},
		// v1.1: WeightG puro line er ojon, Qty diye gun na
		{"variable weight qty 2, mot 2100 g", []Line{{VendorID: 1, UnitPrice: 65000, Qty: 2, VariableWeight: true, WeightG: 2100}},
			nil, Zone{}, PerOrder, Totals{136500, 0, 0, 136500}},
		// marketplace
		{"duita vendor, per_order delivery", []Line{{VendorID: 1, UnitPrice: 30000, Qty: 1}, {VendorID: 2, UnitPrice: 10000, Qty: 1}},
			nil, Zone{Charge: 6000}, PerOrder, Totals{40000, 0, 6000, 46000}},
		{"duita vendor, per_vendor delivery", []Line{{VendorID: 1, UnitPrice: 30000, Qty: 1}, {VendorID: 2, UnitPrice: 10000, Qty: 1}},
			nil, Zone{Charge: 6000}, PerVendor, Totals{40000, 0, 12000, 52000}},
		{"vendor coupon shudhu oi vendor er line e", []Line{{VendorID: 1, UnitPrice: 30000, Qty: 1}, {VendorID: 2, UnitPrice: 10000, Qty: 1}},
			&Coupon{Kind: "percent", Value: 10, VendorID: 2}, Zone{}, PerOrder, Totals{40000, 1000, 0, 39000}},
		// v1.7: "vendor coupon min_order oi vendor er line e" o TestCouponNotApplicable e
		// v1.4: shima (boundary). Free delivery shima te-i (>=), half-up rounding
		{"free delivery thik shima te", []Line{{VendorID: 1, UnitPrice: 100000, Qty: 1}}, nil,
			Zone{Charge: 6000, FreeDeliveryOver: 100000}, PerOrder, Totals{100000, 0, 0, 100000}},
		{"free delivery shimar 1 poisha niche", []Line{{VendorID: 1, UnitPrice: 99999, Qty: 1}}, nil,
			Zone{Charge: 6000, FreeDeliveryOver: 100000}, PerOrder, Totals{99999, 0, 6000, 105999}},
		{"ojon-variable 1.5 poisha upore", []Line{{VendorID: 1, UnitPrice: 1500, Qty: 1, VariableWeight: true, WeightG: 1}},
			nil, Zone{}, PerOrder, Totals{2, 0, 0, 2}},
		{"ojon-variable 1.499 poisha niche", []Line{{VendorID: 1, UnitPrice: 1499, Qty: 1, VariableWeight: true, WeightG: 1}},
			nil, Zone{}, PerOrder, Totals{1, 0, 0, 1}},
		// v1.5 (mutation test): per_vendor e shima puro order er upor, chhuyle shob parcel free
		{"per_vendor e free delivery shima", []Line{{VendorID: 1, UnitPrice: 60000, Qty: 1}, {VendorID: 2, UnitPrice: 50000, Qty: 1}},
			nil, Zone{Charge: 6000, FreeDeliveryOver: 100000}, PerVendor, Totals{110000, 0, 0, 110000}},
	}
	for _, c := range cases {
		got, err := Calculate(c.lines, c.coupon, c.zone, c.mode)
		if err != nil || got.Totals != c.want {
			t.Errorf("%s: got %+v, %v, want %+v", c.name, got.Totals, err, c.want)
		}
		checkInvariants(t, c.name, c.lines, got)
	}
}

func TestGroupsAndShares(t *testing.T) {
	lines := []Line{{VendorID: 1, UnitPrice: 10000, Qty: 1}, {VendorID: 2, UnitPrice: 10000, Qty: 1}, {VendorID: 1, UnitPrice: 10000, Qty: 1}}
	r := mustCalc(t, lines, &Coupon{Kind: "fixed", Value: 1000}, Zone{Charge: 6001}, PerOrder)
	// 1000 poisha tin bhag: 334, 333, 333 (bhagshesh age-r line e)
	if r.Lines[0].DiscountShare != 334 || r.Lines[1].DiscountShare != 333 || r.Lines[2].DiscountShare != 333 {
		t.Errorf("discount share: %+v", r.Lines)
	}
	if len(r.Groups) != 2 || r.Groups[0].VendorID != 1 || r.Groups[0].Subtotal != 20000 || r.Groups[0].Discount != 667 {
		t.Errorf("group: %+v", r.Groups)
	}
	// 6001 poisha delivery 2:1 → 4001, 2000
	if r.Groups[0].DeliveryCharge != 4001 || r.Groups[1].DeliveryCharge != 2000 {
		t.Errorf("delivery bhag: %+v", r.Groups)
	}
}

func TestReprice(t *testing.T) {
	fish := func(g int) []Line {
		return []Line{{VendorID: 1, UnitPrice: 100000, Qty: 1, VariableWeight: true, WeightG: g}}
	}
	fixed := &Coupon{Kind: "fixed", Value: 10000, MinOrder: 100000}
	zone := Zone{Charge: 6000, FreeDeliveryOver: 100000}

	// v1.0 er bug: ojon kom hole min_order er niche giye discount 0, total barto (960 → 1010)
	orig := mustCalc(t, fish(1000), fixed, zone, PerOrder)
	cod, codRefund, _ := Reprice(Original{fish(1000), orig, fixed, false, false}, fish(950))
	if want := (Totals{95000, 10000, 6000, 91000}); cod.Totals != want || codRefund != 0 {
		t.Errorf("ojon kom, COD: got %+v refund %d, want %+v refund 0", cod.Totals, codRefund, want)
	}
	// prepaid, ojon kom: refund
	pre, refund, _ := Reprice(Original{fish(1000), orig, fixed, true, false}, fish(950))
	if pre.Total != 91000 || refund != orig.Total-91000 {
		t.Errorf("prepaid kom: total %d refund %d", pre.Total, refund)
	}
	// prepaid, ojon beshi: customer beshi dey na
	pre2, refund2, _ := Reprice(Original{fish(1000), orig, fixed, true, false}, fish(1200))
	if pre2.Total != orig.Total || refund2 != 0 {
		t.Errorf("prepaid beshi: total %d refund %d", pre2.Total, refund2)
	}
	// COD, ojon beshi: notun dam (SMS jabe)
	cod2, _, _ := Reprice(Original{fish(1000), orig, fixed, false, false}, fish(1200))
	if cod2.Subtotal != 120000 || cod2.Total != 116000 {
		t.Errorf("COD beshi: %+v", cod2.Totals)
	}
	// free delivery ache, ojon kome threshold er niche: delivery ager motoi 0 thake
	free := mustCalc(t, fish(1200), nil, zone, PerOrder)
	f2, _, _ := Reprice(Original{fish(1200), free, nil, false, false}, fish(900))
	if free.DeliveryCharge != 0 || f2.DeliveryCharge != 0 {
		t.Errorf("free delivery bodle gelo: %d → %d", free.DeliveryCharge, f2.DeliveryCharge)
	}
	// marketplace: ek vendor er shob line bad → tar delivery 0, onno vendor er ta thake
	two := []Line{{VendorID: 1, UnitPrice: 30000, Qty: 1}, {VendorID: 2, UnitPrice: 10000, Qty: 1}}
	o2 := mustCalc(t, two, nil, Zone{Charge: 6000}, PerVendor)
	after, _, _ := Reprice(Original{two, o2, nil, true, false}, []Line{two[0], {VendorID: 2, UnitPrice: 10000, Qty: 0}})
	if after.Groups[0].DeliveryCharge != 6000 || after.Groups[1].DeliveryCharge != 0 || after.Total != 36000 {
		t.Errorf("vendor bad: %+v", after.Groups)
	}
}

func TestCommission(t *testing.T) {
	lr := LineResult{LineTotal: 100000, DiscountShare: 10000}
	if Commission(lr, false, 1000) != 10000 || Commission(lr, true, 1000) != 9000 || Commission(lr, false, 0) != 0 {
		t.Error("commission")
	}
}

// Random input e niyom gulo kokhono bhange na.
func TestInvariantsRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for n := 0; n < 20000; n++ {
		var lines []Line
		for i := 0; i < 1+rng.Intn(6); i++ {
			l := Line{VendorID: int64(1 + rng.Intn(3)), UnitPrice: int64(rng.Intn(500000)), Qty: 1 + rng.Intn(5)}
			if rng.Intn(3) == 0 {
				l.VariableWeight, l.WeightG = true, 1+rng.Intn(5000)
			}
			lines = append(lines, l)
		}
		var c *Coupon
		if rng.Intn(2) == 0 {
			c = &Coupon{Kind: "fixed", Value: 1 + int64(rng.Intn(200000)), VendorID: int64(rng.Intn(3))}
			if rng.Intn(2) == 0 {
				c = &Coupon{Kind: "percent", Value: 1 + int64(rng.Intn(100)), MaxDiscount: int64(rng.Intn(50000)), VendorID: int64(rng.Intn(3))}
			}
		}
		mode := []DeliveryMode{PerOrder, PerVendor}[rng.Intn(2)]
		zone := Zone{Charge: int64(rng.Intn(15000)), FreeDeliveryOver: int64(rng.Intn(3)) * 100000}
		r, err := Calculate(lines, c, zone, mode)
		if errors.Is(err, ErrCouponNotApplicable) {
			// v1.7: random coupon er min_order 0, tai error shudhu jokhon eligible line er taka 0 (oi vendor
			// er line nai ba shob ৳0). Customer 409 er por coupon bad dey, tai baki test coupon chhara.
			if e := eligibleSubtotal(lines, c); e != 0 {
				t.Fatalf("ErrCouponNotApplicable, kintu eligible subtotal %d", e)
			}
			c = nil
			r, err = Calculate(lines, nil, zone, mode)
		}
		if err != nil {
			t.Fatalf("Calculate: %v", err)
		}
		if c != nil && eligibleSubtotal(lines, c) == 0 {
			t.Fatalf("eligible subtotal 0 kintu ErrCouponNotApplicable na")
		}
		checkInvariants(t, "random", lines, r)

		// Reprice: ojon/qty bodlao, kichu line bad
		nl := make([]Line, len(lines))
		copy(nl, lines)
		for i := range nl {
			switch rng.Intn(5) {
			case 0:
				nl[i].Qty, nl[i].WeightG = 0, 0
			case 1:
				if nl[i].VariableWeight {
					nl[i].WeightG = 1 + rng.Intn(6000)
				}
			case 2: // v1.5: qty ardhek kom (edit), ojon-variable e ojon onupate
				if old := nl[i].Qty; old > 1 {
					nl[i].Qty = 1 + rng.Intn(old-1)
					if nl[i].VariableWeight {
						nl[i].WeightG = EditWeightG(nl[i].WeightG, old, nl[i].Qty)
					}
				}
			}
		}
		grew := false // v1.3: edit e ojon barano error
		for i := range nl {
			if nl[i].VariableWeight && nl[i].WeightG > lines[i].WeightG {
				grew = true
			}
		}
		for _, mode := range [][2]bool{{true, false}, {false, false}, {true, true}, {false, true}} {
			prepaid, edit := mode[0], mode[1]
			rr, refund, err := Reprice(Original{lines, r, c, prepaid, edit}, nl)
			if (err != nil) != (edit && grew) {
				t.Fatalf("reprice: edit=%v ojon bareche=%v, err=%v", edit, grew, err)
			}
			if err != nil {
				continue
			}
			checkInvariants(t, "reprice", nl, rr)
			if prepaid && (rr.Total > r.Total || refund != r.Total-rr.Total) {
				t.Fatalf("prepaid e customer beshi dilo: %d → %d (refund %d)", r.Total, rr.Total, refund)
			}
			if (!prepaid && refund != 0) || (edit && rr.Total > r.Total) { // v1.5: COD e refund na; edit e total bare na
				t.Fatalf("COD refund %d, ba edit e total barlo %d → %d", refund, r.Total, rr.Total)
			}
			checkFrozen(t, lines, nl, r, rr)
			// v1.4: dui dhap (packing er por edit, ba duita vendor alada shomoy pack): ditiyo bar o niyom thake
			nl2 := make([]Line, len(nl))
			copy(nl2, nl)
			for i := range nl2 {
				if nl2[i].Qty > 0 && rng.Intn(3) == 0 {
					nl2[i].Qty, nl2[i].WeightG = 0, 0
				}
			}
			r2, refund2, err := Reprice(Original{nl, rr, c, prepaid, edit}, nl2)
			if err != nil {
				t.Fatalf("ditiyo reprice: %v", err)
			}
			checkInvariants(t, "reprice 2", nl2, r2)
			if prepaid && (r2.Total > rr.Total || refund2 != rr.Total-r2.Total) {
				t.Fatalf("ditiyo reprice e prepaid beshi: %d → %d", rr.Total, r2.Total)
			}
			checkFrozen(t, nl, nl2, rr, r2)
		}
		// v1.4: kichu na bodlale Reprice hubohu ager result (৳0 er line thakleo delivery thake); v1.5: edit eo
		for _, m := range [][2]bool{{true, false}, {false, false}, {true, true}, {false, true}} {
			same, refund, err := Reprice(Original{lines, r, c, m[0], m[1]}, lines)
			if err != nil || refund != 0 || !sameResult(same, r) {
				t.Fatalf("kichu na bodle o bodlalo: %+v → %+v (refund %d, err %v)", r.Totals, same.Totals, refund, err)
			}
		}
	}
}

func checkInvariants(t *testing.T, name string, lines []Line, r Result) {
	t.Helper()
	var share, sub, gs, gd, gdel, gt int64
	for i, lr := range r.Lines {
		if lr.DiscountShare < 0 || lr.DiscountShare > lr.LineTotal {
			t.Fatalf("%s: line %d share %d > line total %d", name, i, lr.DiscountShare, lr.LineTotal)
		}
		share += lr.DiscountShare
		sub += lr.LineTotal
	}
	for _, g := range r.Groups {
		if g.Total != g.Subtotal-g.Discount+g.DeliveryCharge {
			t.Fatalf("%s: group total invariant", name)
		}
		gs, gd, gdel, gt = gs+g.Subtotal, gd+g.Discount, gdel+g.DeliveryCharge, gt+g.Total
	}
	if share != r.Discount || sub != r.Subtotal || gs != r.Subtotal || gd != r.Discount || gdel != r.DeliveryCharge || gt != r.Total {
		t.Fatalf("%s: bhag er jog mile na: %+v", name, r)
	}
	if r.Total != r.Subtotal-r.Discount+r.DeliveryCharge || r.Discount > r.Subtotal {
		t.Fatalf("%s: total invariant", name)
	}
}

// checkFrozen (v1.4): je vendor er kono line bodlay nai, tar line ar fulfillment er taka hubohu ager.
func checkFrozen(t *testing.T, before, after []Line, o, r Result) {
	t.Helper()
	changed := map[int64]bool{}
	for i := range after {
		if after[i].Qty != before[i].Qty || after[i].WeightG != before[i].WeightG {
			changed[after[i].VendorID] = true
		}
	}
	for i := range after {
		if !changed[after[i].VendorID] && r.Lines[i] != o.Lines[i] {
			t.Fatalf("vendor %d er line %d bodle gelo: %+v → %+v", after[i].VendorID, i, o.Lines[i], r.Lines[i])
		}
	}
	old := map[int64]Totals{}
	for _, g := range o.Groups {
		old[g.VendorID] = g.Totals
	}
	for _, g := range r.Groups {
		if !changed[g.VendorID] && g.Totals != old[g.VendorID] {
			t.Fatalf("vendor %d er fulfillment bodle gelo: %+v → %+v", g.VendorID, old[g.VendorID], g.Totals)
		}
	}
}

func sameResult(a, b Result) bool {
	if a.Totals != b.Totals || len(a.Lines) != len(b.Lines) || len(a.Groups) != len(b.Groups) {
		return false
	}
	for i := range a.Lines {
		if a.Lines[i] != b.Lines[i] {
			return false
		}
	}
	for i := range a.Groups {
		if a.Groups[i] != b.Groups[i] {
			return false
		}
	}
	return true
}

// v1.4: marketplace e vendor A pack korle vendor B er line, discount er bhag ar fulfillment er taka bodlay na.
func TestRepriceFreezesOtherVendors(t *testing.T) {
	lines := []Line{
		{VendorID: 1, UnitPrice: 100000, Qty: 1, VariableWeight: true, WeightG: 1000}, // A: mach, 1 kg
		{VendorID: 2, UnitPrice: 100000, Qty: 1},                                      // B: modhu
	}
	cases := []struct {
		name   string
		coupon *Coupon
		packG  int
		wantA  Totals
	}{
		// ager version e B er total 98000 → 96333
		{"fixed, A 500 g", &Coupon{Kind: "fixed", Value: 10000}, 500, Totals{50000, 5000, 3000, 48000}},
		// ager version e B er total 98000 → 100500; ekhon B thik thake, A cap er bakita pay
		{"percent cap, A 3 kg", &Coupon{Kind: "percent", Value: 10, MaxDiscount: 10000}, 3000, Totals{300000, 5000, 3000, 298000}},
	}
	for _, c := range cases {
		o := mustCalc(t, lines, c.coupon, Zone{Charge: 6000}, PerOrder)
		nl := []Line{lines[0], lines[1]}
		nl[0].WeightG = c.packG
		r, _, err := Reprice(Original{lines, o, c.coupon, false, false}, nl)
		if err != nil || r.Groups[1] != o.Groups[1] || r.Lines[1] != o.Lines[1] || r.Groups[0].Totals != c.wantA {
			t.Errorf("%s: A %+v (want %+v), B %+v → %+v, err %v", c.name, r.Groups[0].Totals, c.wantA, o.Groups[1], r.Groups[1], err)
		}
		checkInvariants(t, c.name, nl, r)
	}
}

// v1.4: ৳0 er line (free gift) wala vendor er delivery kichu bad na dile thake; shob line bad dile 0.
func TestRepriceDeliveryKeepsFreeLineVendor(t *testing.T) {
	lines := []Line{{VendorID: 1, UnitPrice: 30000, Qty: 1}, {VendorID: 2, UnitPrice: 0, Qty: 1}}
	o := mustCalc(t, lines, nil, Zone{Charge: 6000}, PerVendor)
	r, refund, err := Reprice(Original{lines, o, nil, true, true}, []Line{lines[0], lines[1]})
	if err != nil || r.Groups[1].DeliveryCharge != 6000 || refund != 0 {
		t.Errorf("kichu bad na: %+v refund %d err %v", r.Groups, refund, err)
	}
	r, refund, err = Reprice(Original{lines, o, nil, true, true}, []Line{lines[0], {VendorID: 2, UnitPrice: 0, Qty: 0}})
	if err != nil || r.Groups[1].DeliveryCharge != 0 || refund != 6000 {
		t.Errorf("gift bad: %+v refund %d err %v", r.Groups, refund, err)
	}
}

// v1.4: Reprice coupon nay order er snapshot theke; pore coupon bodlaleo snapshot dile hishab ek-i.
func TestRepriceUsesCouponSnapshot(t *testing.T) {
	fish := func(g int) []Line {
		return []Line{{VendorID: 1, UnitPrice: 100000, Qty: 1, VariableWeight: true, WeightG: g}}
	}
	snap := &Coupon{Kind: "percent", Value: 10, MaxDiscount: 20000}
	o := mustCalc(t, fish(1000), snap, Zone{}, PerOrder)
	r, refund, err := Reprice(Original{fish(1000), o, snap, true, false}, fish(1000))
	if err != nil || r.Total != o.Total || refund != 0 {
		t.Errorf("snapshot diye: total %d → %d refund %d err %v", o.Total, r.Total, refund, err)
	}
	// coupon bondho/muche geche bole nil: ager version e prepaid total 90000 → 100000, chupchap
	if _, _, err := Reprice(Original{fish(1000), o, nil, true, false}, fish(1000)); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("coupon nil kintu discount ache: err = %v, want ErrInvalidInput", err)
	}
}

// v1.4: rounding e frozen vendor er bhag p% er cheye ek poisha beshi hote pare (largest remainder).
// Percent e "order er discount koto bodlalo" (delta) diye hishab na korle ditiyo dhape prepaid
// customer ek poisha beshi dito: 3395 er order e 93%, prothome ৳33.92 er line bad, tarpor baki line
// er qty 3 theke 1.
func TestRepricePercentTwoStepsNeverRaisesPrepaid(t *testing.T) {
	c := &Coupon{Kind: "percent", Value: 93}
	l0 := []Line{{VendorID: 2, UnitPrice: 3392, Qty: 1}, {VendorID: 1, UnitPrice: 1, Qty: 3}}
	o := mustCalc(t, l0, c, Zone{}, PerOrder)
	l1 := []Line{{VendorID: 2, UnitPrice: 3392, Qty: 0}, l0[1]}
	r1, ref1, err1 := Reprice(Original{l0, o, c, true, true}, l1)
	l2 := []Line{l1[0], {VendorID: 1, UnitPrice: 1, Qty: 1}}
	r2, ref2, err2 := Reprice(Original{l1, r1, c, true, true}, l2)
	if err1 != nil || err2 != nil || o.Total != 238 || r1.Total != 0 || ref1 != 238 || r2.Total != 0 || ref2 != 0 {
		t.Errorf("total %d → %d → %d, refund %d, %d, err %v %v", o.Total, r1.Total, r2.Total, ref1, ref2, err1, err2)
	}
}

func TestEditProratesFixedCoupon(t *testing.T) {
	c := &Coupon{Kind: "fixed", Value: 10000, MinOrder: 100000}
	z := Zone{Charge: 6000}

	// ৳900 + ৳100, ৳100 coupon. ৳900 bad → ৳100 er item ৳10 discount pay, free na.
	lines := []Line{{VendorID: 1, UnitPrice: 90000, Qty: 1}, {VendorID: 1, UnitPrice: 10000, Qty: 1}}
	orig := mustCalc(t, lines, c, z, PerOrder)
	edited, _, err := Reprice(Original{lines, orig, c, false, true}, []Line{{VendorID: 1, UnitPrice: 90000, Qty: 0}, lines[1]})
	if err != nil || edited.Discount != 1000 || edited.Total != 15000 {
		t.Errorf("abuse case: %+v err=%v", edited.Totals, err)
	}

	// Prepaid, ৳10 er item bad (min_order er niche): total bare na, ৳9 refund.
	lines2 := []Line{{VendorID: 1, UnitPrice: 99000, Qty: 1}, {VendorID: 1, UnitPrice: 1000, Qty: 1}}
	orig2 := mustCalc(t, lines2, c, z, PerOrder)
	pre, refund, _ := Reprice(Original{lines2, orig2, c, true, true}, []Line{lines2[0], {VendorID: 1, UnitPrice: 1000, Qty: 0}})
	if pre.Total > orig2.Total || refund != 900 {
		t.Errorf("prepaid cliff: total %d → %d, refund %d", orig2.Total, pre.Total, refund)
	}

	// Packing (Edit = false) e doc er ager niyom: fixed discount puro thake.
	fish := []Line{{VendorID: 1, UnitPrice: 100000, Qty: 1, VariableWeight: true, WeightG: 1000}}
	o3 := mustCalc(t, fish, c, Zone{Charge: 6000, FreeDeliveryOver: 100000}, PerOrder)
	p, _, _ := Reprice(Original{fish, o3, c, false, false}, []Line{{VendorID: 1, UnitPrice: 100000, Qty: 1, VariableWeight: true, WeightG: 950}})
	if p.Discount != 10000 {
		t.Errorf("packing: %+v", p.Totals)
	}

	// Length guard
	if _, _, err := Reprice(Original{lines, orig, c, true, false}, append(lines, lines[0])); err != ErrLineCount {
		t.Errorf("length guard: %v", err)
	}
}

// v1.5 (mutation test): largest remainder e kono index ek poisha-r beshi pay na; shob weight 0 hole
// puro ta prothom index e; khali weights e panic na.
func TestAllocate(t *testing.T) {
	cases := []struct {
		total int64
		w     []int64
		want  []int64
	}{
		{2, []int64{1, 1, 1}, []int64{1, 1, 0}},
		{5, []int64{1, 1, 1, 1, 1, 1, 1}, []int64{1, 1, 1, 1, 1, 0, 0}},
		{100, []int64{1, 2, 3}, []int64{17, 33, 50}},
		{7, []int64{0, 0}, []int64{7, 0}},
		{7, nil, nil},
	}
	for _, c := range cases {
		if got := Allocate(c.total, c.w); !slices.Equal(got, c.want) {
			t.Errorf("Allocate(%d, %v) = %v, want %v", c.total, c.w, got, c.want)
		}
	}
}

// v1.5 (mutation test): Reprice percent coupon er min_order abar dekhe na; notun eligible er percent.
func TestRepricePercentMinOrderNotRechecked(t *testing.T) {
	c := &Coupon{Kind: "percent", Value: 10, MinOrder: 100000}
	fish := func(g int) []Line {
		return []Line{{VendorID: 1, UnitPrice: 100000, Qty: 1, VariableWeight: true, WeightG: g}}
	}
	o := mustCalc(t, fish(1200), c, Zone{}, PerOrder)
	r, _, err := Reprice(Original{fish(1200), o, c, false, false}, fish(500))
	if err != nil || o.Discount != 12000 || r.Discount != 5000 {
		t.Errorf("discount %d → %d (want 12000 → 5000), err %v", o.Discount, r.Discount, err)
	}
}

// v1.5 (mutation test): checkout e discount 0 hole (rounding) Reprice e-o 0, ojon barleo.
func TestRepriceZeroDiscountStaysZero(t *testing.T) {
	c := &Coupon{Kind: "percent", Value: 1}
	fish := func(g int) []Line {
		return []Line{{VendorID: 1, UnitPrice: 1000, Qty: 1, VariableWeight: true, WeightG: g}}
	}
	o := mustCalc(t, fish(50), c, Zone{}, PerOrder)
	r, _, err := Reprice(Original{fish(50), o, c, false, false}, fish(30000))
	if err != nil || o.Discount != 0 || r.Discount != 0 || r.Subtotal != 30000 {
		t.Errorf("discount %d → %d, subtotal %d, err %v", o.Discount, r.Discount, r.Subtotal, err)
	}
}

// v1.5: order_items theke Line. Removed item DB te ager quantity niye thake, kintu hishab e Qty 0.
func TestLineFromItem(t *testing.T) {
	cases := []struct {
		name string
		it   Item
		want Line
	}{
		{"fixed: ojon dileo WeightG 0", Item{VendorID: 1, UnitPrice: 5000, Quantity: 2, OrderedWeightG: 900},
			Line{VendorID: 1, UnitPrice: 5000, Qty: 2}},
		{"ojon-variable, pack er age", Item{VendorID: 1, UnitPrice: 65000, Quantity: 2, VariableWeight: true, OrderedWeightG: 2000},
			Line{VendorID: 1, UnitPrice: 65000, Qty: 2, VariableWeight: true, WeightG: 2000}},
		{"ojon-variable, packed", Item{VendorID: 1, UnitPrice: 65000, Quantity: 2, VariableWeight: true, OrderedWeightG: 2000, PackedWeightG: 2150},
			Line{VendorID: 1, UnitPrice: 65000, Qty: 2, VariableWeight: true, WeightG: 2150}},
		{"removed", Item{VendorID: 2, UnitPrice: 5000, Quantity: 3, Removed: true},
			Line{VendorID: 2, UnitPrice: 5000}},
		{"removed ojon-variable", Item{VendorID: 2, UnitPrice: 65000, Quantity: 1, VariableWeight: true, OrderedWeightG: 1000, Removed: true},
			Line{VendorID: 2, UnitPrice: 65000, VariableWeight: true}},
	}
	for _, c := range cases {
		if got, lr := LineFromItem(c.it); got != c.want || lr != (LineResult{c.it.LineTotal, c.it.DiscountShare}) {
			t.Errorf("%s: got %+v %+v, want %+v", c.name, got, lr, c.want)
		}
	}

	// COD: edit e item bad (removed, DB te qty 3), tarpor ekoi vendor er mach pack. Removed item abar bill
	// e ashbe na; DB er quantity diye nije Line banale checkOriginal atkay.
	items := []Item{
		{VendorID: 1, UnitPrice: 5000, Quantity: 3, Removed: true},
		{VendorID: 1, UnitPrice: 100000, Quantity: 1, VariableWeight: true, OrderedWeightG: 1000, LineTotal: 100000},
	}
	var lines []Line
	var lrs []LineResult
	for _, it := range items {
		l, lr := LineFromItem(it)
		lines, lrs = append(lines, l), append(lrs, lr)
	}
	o := Result{Totals: Totals{100000, 0, 6000, 106000}, Lines: lrs,
		Groups: []Group{{VendorID: 1, Totals: Totals{100000, 0, 6000, 106000}}}}
	packed := []Line{lines[0], lines[1]}
	packed[1].WeightG = 1100
	r, _, err := Reprice(Original{lines, o, nil, false, false}, packed)
	if err != nil || r.Lines[0].LineTotal != 0 || r.Total != 110000+6000 {
		t.Errorf("removed item abar bill: %+v err %v", r.Totals, err)
	}
	naive := []Line{{VendorID: 1, UnitPrice: 5000, Qty: 3}, lines[1]}
	if _, _, err := Reprice(Original{naive, o, nil, false, false}, []Line{naive[0], packed[1]}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("DB quantity diye removed line: err = %v, want ErrInvalidInput", err)
	}
}

func TestEditWeightG(t *testing.T) {
	cases := [][4]int{{2100, 2, 1, 1050}, {5, 3, 1, 1}, {1, 3, 1, 1}, {1000, 2, 0, 0}, {1000, 0, 1, 0}}
	for _, c := range cases {
		if got := EditWeightG(c[0], c[1], c[2]); got != c[3] {
			t.Errorf("EditWeightG(%d, %d, %d) = %d, want %d", c[0], c[1], c[2], got, c[3])
		}
	}
}

func mustCalc(t *testing.T, lines []Line, c *Coupon, z Zone, m DeliveryMode) Result {
	t.Helper()
	r, err := Calculate(lines, c, z, m)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	return r
}

// v1.3: bhul input e hishab na, error.
func TestInvalidInput(t *testing.T) {
	one := []Line{{VendorID: 1, UnitPrice: 5000, Qty: 1}}
	bad := []struct {
		name   string
		lines  []Line
		coupon *Coupon
		zone   Zone
		mode   DeliveryMode
	}{
		{"ojon-variable, WeightG 0 (nominal_weight_g NULL)", []Line{{VendorID: 1, UnitPrice: 65000, Qty: 2, VariableWeight: true}}, nil, Zone{}, PerOrder},
		{"qty 0", []Line{{VendorID: 1, UnitPrice: 5000}}, nil, Zone{}, PerOrder},
		{"qty minus", []Line{{VendorID: 1, UnitPrice: 5000, Qty: -2}}, nil, Zone{}, PerOrder},
		{"dam minus", []Line{{VendorID: 1, UnitPrice: -1, Qty: 1}}, nil, Zone{}, PerOrder},
		{"coupon kind er banan", one, &Coupon{Kind: "Percent", Value: 10}, Zone{}, PerOrder},
		{"percent 100 er beshi", one, &Coupon{Kind: "percent", Value: 101}, Zone{}, PerOrder},
		{"coupon min_order minus", one, &Coupon{Kind: "fixed", Value: 100, MinOrder: -1}, Zone{}, PerOrder},
		{"delivery charge minus", one, nil, Zone{Charge: -1}, PerOrder},
		{"mode khali", one, nil, Zone{}, ""},
		// v1.4: protiti guard er test (mutation test e egulo dhora porto na)
		{"coupon value 0", one, &Coupon{Kind: "fixed", Value: 0}, Zone{}, PerOrder},
		{"coupon max_discount minus", one, &Coupon{Kind: "percent", Value: 10, MaxDiscount: -1}, Zone{}, PerOrder},
		{"free_delivery_over minus", one, nil, Zone{FreeDeliveryOver: -1}, PerOrder},
		// v1.5
		{"fixed line e WeightG", []Line{{VendorID: 1, UnitPrice: 5000, Qty: 1, WeightG: 500}}, nil, Zone{}, PerOrder},
	}
	for _, c := range bad {
		if _, err := Calculate(c.lines, c.coupon, c.zone, c.mode); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", c.name, err)
		}
	}

	item := []Line{{VendorID: 1, UnitPrice: 50000, Qty: 1}}
	o := mustCalc(t, item, nil, Zone{Charge: 6000}, PerOrder)
	fish := []Line{{VendorID: 1, UnitPrice: 100000, Qty: 1, VariableWeight: true, WeightG: 1000}}
	of := mustCalc(t, fish, nil, Zone{}, PerOrder)
	heavy := []Line{{VendorID: 1, UnitPrice: 100000, Qty: 1, VariableWeight: true, WeightG: 1200}}
	cf := &Coupon{Kind: "fixed", Value: 1000}
	oc := mustCalc(t, item, cf, Zone{Charge: 6000}, PerOrder)
	reprice := []struct {
		name string
		o    Original
		nl   []Line
	}{
		{"prepaid edit e qty 1 theke 3 (2 ta free hoto)", Original{item, o, nil, true, true}, []Line{{VendorID: 1, UnitPrice: 50000, Qty: 3}}},
		{"dam bodlano", Original{item, o, nil, false, true}, []Line{{VendorID: 1, UnitPrice: 40000, Qty: 1}}},
		{"vendor bodlano", Original{item, o, nil, false, false}, []Line{{VendorID: 2, UnitPrice: 50000, Qty: 1}}},
		{"edit e ojon barano", Original{fish, of, nil, false, true}, heavy},
		{"ojon-variable bad, kintu WeightG baki", Original{fish, of, nil, false, true}, []Line{{VendorID: 1, UnitPrice: 100000, VariableWeight: true, WeightG: 500}}},
		// v1.4
		{"dhoron bodlano (ojon-variable theke fixed)", Original{fish, of, nil, false, false}, []Line{{VendorID: 1, UnitPrice: 100000, Qty: 1, WeightG: 1000}}},
		{"qty minus", Original{item, o, nil, false, true}, []Line{{VendorID: 1, UnitPrice: 50000, Qty: -1}}},
		{"ojon minus", Original{fish, of, nil, false, true}, []Line{{VendorID: 1, UnitPrice: 100000, Qty: 1, VariableWeight: true, WeightG: -5}}},
		// v1.5
		{"coupon er kind bhul (snapshot)", Original{item, o, &Coupon{Kind: "Percent", Value: 10}, false, false}, item},
		{"fixed line e WeightG (vendor bhul kore bodla dhorto)", Original{item, o, nil, false, false}, []Line{{VendorID: 1, UnitPrice: 50000, Qty: 1, WeightG: 700}}},
		{"original er total mile na", Original{item, tampered(o, func(r *Result) { r.Total++ }), nil, false, false}, item},
		{"original e vendor er fulfillment nai", Original{fish, tampered(of, func(r *Result) { r.Groups = nil }), nil, false, false}, fish},
		{"original e ek vendor er duita fulfillment", Original{fish, tampered(of, func(r *Result) { r.Groups = append(r.Groups, r.Groups[0]) }), nil, false, false}, fish},
		{"original e ochena vendor er fulfillment", Original{item, tampered(o, func(r *Result) { r.Groups = append(r.Groups, Group{VendorID: 99}) }), nil, false, false}, item},
		{"original e fulfillment er subtotal line er jog na", Original{item, tampered(o, func(r *Result) { r.Groups[0].Subtotal++; r.Groups[0].Total++ }), nil, false, false}, item},
		{"original e fulfillment er discount line er jog na", Original{item, tampered(o, func(r *Result) { r.Groups[0].Discount++; r.Groups[0].Total-- }), nil, false, false}, item},
		{"original e fulfillment er delivery minus", Original{item, tampered(o, func(r *Result) {
			r.Groups[0].DeliveryCharge, r.Groups[0].Total, r.DeliveryCharge, r.Total = -1, 49999, -1, 49999
		}), nil, false, false}, item},
		{"original e order er subtotal line er jog na", Original{item, tampered(o, func(r *Result) { r.Subtotal++; r.Total++ }), nil, false, false}, item},
		{"original e order er discount line er jog na", Original{item, tampered(oc, func(r *Result) { r.Discount++; r.Total-- }), cf, false, false}, item},
		{"original e discount er bhag minus", Original{item, tampered(oc, func(r *Result) {
			r.Lines[0].DiscountShare, r.Discount, r.Groups[0].Discount = -1, -1, -1
			r.Groups[0].Total, r.Total = 56001, 56001
		}), cf, false, false}, item},
		{"original e discount er bhag line total er beshi", Original{item, tampered(o, func(r *Result) { r.Lines[0].DiscountShare = 50001 }), nil, false, false}, item},
		{"original e removed (Qty 0) line e taka", Original{[]Line{{VendorID: 1, UnitPrice: 50000}}, o, nil, false, true}, []Line{{VendorID: 1, UnitPrice: 50000}}},
		{"original e line er taka snapshot theke alada (COD)", Original{item, tampered(o, func(r *Result) {
			r.Lines = []LineResult{{LineTotal: 40000}}
			r.Subtotal, r.Total = 40000, 46000
			r.Groups = []Group{{VendorID: 1, Totals: Totals{40000, 0, 6000, 46000}}}
		}), nil, false, false}, item},
	}
	for _, c := range reprice {
		if _, _, err := Reprice(c.o, c.nl); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", c.name, err)
		}
	}
	// packing e (Edit = false) asol ojon beshi hote pare
	if _, _, err := Reprice(Original{fish, of, nil, false, false}, heavy); err != nil {
		t.Errorf("packing e beshi ojon: %v", err)
	}
}

// tampered: Result er copy (slice o notun), test e bhul Original banate.
func tampered(r Result, f func(*Result)) Result {
	r.Lines = slices.Clone(r.Lines)
	r.Groups = slices.Clone(r.Groups)
	f(&r)
	return r
}

// eligibleSubtotal (v1.7): test er jonno, coupon er eligible line er LineTotal er jog.
func eligibleSubtotal(lines []Line, c *Coupon) int64 {
	var s int64
	for _, l := range lines {
		if c.eligible(l) {
			s += LineTotal(l)
		}
	}
	return s
}

// v1.7 (8.5, 8.6 step 12): coupon er eligible subtotal 0 ba min_order er kom hole Calculate
// ErrCouponNotApplicable dey (checkout e 409 coupon_invalid), chupchap 0 discount na. TestCalculate er
// "min order not met" ar "vendor coupon min_order oi vendor er line e" ekhane: error, ar coupon chhara
// Calculate er total ager want-i (quote e coupon chhara dekhano).
func TestCouponNotApplicable(t *testing.T) {
	two := []Line{{VendorID: 1, UnitPrice: 90000, Qty: 1}, {VendorID: 2, UnitPrice: 10000, Qty: 1}}
	bad := []struct {
		name     string
		lines    []Line
		coupon   Coupon
		zone     Zone
		noCoupon Totals // coupon chhara (quote)
	}{
		{"min order not met", []Line{{VendorID: 1, UnitPrice: 5000, Qty: 1}},
			Coupon{Kind: "fixed", Value: 1000, MinOrder: 50000}, Zone{Charge: 6000}, Totals{5000, 0, 6000, 11000}},
		{"vendor coupon min_order oi vendor er line e", two,
			Coupon{Kind: "fixed", Value: 2000, MinOrder: 50000, VendorID: 2}, Zone{}, Totals{100000, 0, 0, 100000}},
		{"min order er 1 poisha niche", []Line{{VendorID: 1, UnitPrice: 49999, Qty: 1}},
			Coupon{Kind: "percent", Value: 10, MinOrder: 50000}, Zone{}, Totals{49999, 0, 0, 49999}},
		{"vendor coupon, oi vendor er line nai", []Line{two[0]},
			Coupon{Kind: "percent", Value: 10, VendorID: 2}, Zone{}, Totals{90000, 0, 0, 90000}},
		{"vendor coupon, shudhu ৳0 er line (gift)", []Line{two[0], {VendorID: 2, UnitPrice: 0, Qty: 1}},
			Coupon{Kind: "fixed", Value: 1000, VendorID: 2}, Zone{}, Totals{90000, 0, 0, 90000}},
		{"line nai", nil, Coupon{Kind: "fixed", Value: 1000}, Zone{Charge: 6000}, Totals{}},
	}
	for _, c := range bad {
		cp := c.coupon
		_, err := Calculate(c.lines, &cp, c.zone, PerOrder)
		if !errors.Is(err, ErrCouponNotApplicable) || errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrCouponNotApplicable", c.name, err)
		}
		if r := mustCalc(t, c.lines, nil, c.zone, PerOrder); r.Totals != c.noCoupon {
			t.Errorf("%s: coupon chhara %+v, want %+v", c.name, r.Totals, c.noCoupon)
		}
	}
	// Shima: eligible subtotal = min_order hole cholbe; percent e rounding e discount 0 holeo cholbe.
	ok := []struct {
		name   string
		lines  []Line
		coupon Coupon
		want   Totals
	}{
		{"min order thik shima te", []Line{{VendorID: 1, UnitPrice: 50000, Qty: 1}},
			Coupon{Kind: "percent", Value: 10, MinOrder: 50000}, Totals{50000, 5000, 0, 45000}},
		{"vendor coupon, oi vendor er line", two,
			Coupon{Kind: "fixed", Value: 2000, VendorID: 2}, Totals{100000, 2000, 0, 98000}},
		{"percent rounding e 0 (TestRepriceZeroDiscountStaysZero er moto)", []Line{{VendorID: 1, UnitPrice: 50, Qty: 1}},
			Coupon{Kind: "percent", Value: 1}, Totals{50, 0, 0, 50}},
	}
	for _, c := range ok {
		cp := c.coupon
		if r, err := Calculate(c.lines, &cp, Zone{}, PerOrder); err != nil || r.Totals != c.want {
			t.Errorf("%s: got %+v, %v, want %+v", c.name, r.Totals, err, c.want)
		}
	}
	// Discount nijei (exported, Reprice o use kore): min_order er niche 0, shima te puro.
	fixed := Coupon{Kind: "fixed", Value: 1000, MinOrder: 5000}
	if Discount(fixed, 4999) != 0 || Discount(fixed, 5000) != 1000 {
		t.Errorf("Discount min_order shima: %d, %d", Discount(fixed, 4999), Discount(fixed, 5000))
	}
}

// v1.7: asol kram: order → edit (pack er age; item bad, qty kom) → protiti vendor alada dhape pack (asol
// ojon, kom ba beshi). Protiti dhap er Original ashe order_items er row theke LineFromItem diye, jemon
// database theke; Reprice er result abar row e save hoy (8.11 step 4). Ager random test er dui dhap ekoi
// Edit flag ar ditiyo dhape shudhu line bad korto, tai ei kram test hoto na.
func TestRepriceEditThenPackRandom(t *testing.T) {
	editThenPack(t, rand.New(rand.NewSource(7)), 20000)
}

func editThenPack(t *testing.T, rng *rand.Rand, n int) {
	t.Helper()
	for k := 0; k < n; k++ {
		var lines []Line
		for i := 0; i < 1+rng.Intn(6); i++ {
			l := Line{VendorID: int64(1 + rng.Intn(3)), UnitPrice: int64(rng.Intn(500000)), Qty: 1 + rng.Intn(5)}
			if rng.Intn(3) == 0 {
				l.VariableWeight, l.WeightG = true, l.Qty*(200+rng.Intn(1800)) // nominal_weight_g × qty
			}
			lines = append(lines, l)
		}
		var c *Coupon
		switch rng.Intn(3) {
		case 1:
			c = &Coupon{Kind: "fixed", Value: 1 + int64(rng.Intn(200000)), MinOrder: int64(rng.Intn(2)) * 50000, VendorID: int64(rng.Intn(3))}
		case 2:
			c = &Coupon{Kind: "percent", Value: 1 + int64(rng.Intn(100)), MaxDiscount: int64(rng.Intn(50000)), VendorID: int64(rng.Intn(3))}
		}
		mode := []DeliveryMode{PerOrder, PerVendor}[rng.Intn(2)]
		zone := Zone{Charge: int64(rng.Intn(15000)), FreeDeliveryOver: int64(rng.Intn(3)) * 100000}
		orig, err := Calculate(lines, c, zone, mode)
		if errors.Is(err, ErrCouponNotApplicable) {
			c = nil
			orig, err = Calculate(lines, nil, zone, mode)
		}
		if err != nil {
			t.Fatalf("Calculate: %v", err)
		}
		prepaid := rng.Intn(2) == 0
		items := make([]Item, len(lines)) // order_items er row (8.6 step 16)
		for i, l := range lines {
			items[i] = Item{VendorID: l.VendorID, UnitPrice: l.UnitPrice, Quantity: l.Qty, VariableWeight: l.VariableWeight,
				LineTotal: orig.Lines[i].LineTotal, DiscountShare: orig.Lines[i].DiscountShare}
			if l.VariableWeight {
				items[i].OrderedWeightG = l.WeightG
			}
		}
		cur, refunds := orig, int64(0)
		step := func(edit bool, change func(it *Item)) {
			before, after := make([]Line, len(items)), make([]Line, len(items))
			next := slices.Clone(items)
			for i := range items {
				before[i], _ = LineFromItem(items[i])
				change(&next[i])
				after[i], _ = LineFromItem(next[i])
			}
			r, refund, err := Reprice(Original{before, cur, c, prepaid, edit}, after)
			if err != nil {
				t.Fatalf("edit=%v: %v", edit, err)
			}
			checkInvariants(t, "edit→pack", after, r)
			checkFrozen(t, before, after, cur, r)
			if (!prepaid && refund != 0) || ((edit || prepaid) && r.Total > cur.Total) {
				t.Fatalf("edit=%v prepaid=%v: total %d → %d, refund %d", edit, prepaid, cur.Total, r.Total, refund)
			}
			for i := range next { // database e save; removed item e taka 0 (order_items_removed_zero_check)
				next[i].LineTotal, next[i].DiscountShare = r.Lines[i].LineTotal, r.Lines[i].DiscountShare
				if next[i].Removed && (next[i].LineTotal != 0 || next[i].DiscountShare != 0) {
					t.Fatalf("removed item e taka: %+v", next[i])
				}
			}
			items, cur, refunds = next, r, refunds+refund
		}
		step(true, func(it *Item) { // edit: item bad, ba qty kom (ojon-variable e EditWeightG)
			switch rng.Intn(4) {
			case 0:
				it.Removed = true
			case 1:
				if old := it.Quantity; old > 1 {
					it.Quantity = 1 + rng.Intn(old-1)
					if it.VariableWeight {
						it.OrderedWeightG = EditWeightG(it.OrderedWeightG, old, it.Quantity)
					}
				}
			}
		})
		for v := int64(1); v <= 3; v++ { // packing: vendor onujayi alada dhap, asol ojon 70%–130%
			step(false, func(it *Item) {
				if it.VendorID == v && it.VariableWeight && !it.Removed {
					it.PackedWeightG = max(1, it.OrderedWeightG*(70+rng.Intn(61))/100)
				}
			})
		}
		if prepaid && (cur.Total > orig.Total || refunds != orig.Total-cur.Total) {
			t.Fatalf("prepaid: paid %d, shesh total %d, mot refund %d", orig.Total, cur.Total, refunds)
		}
	}
}
