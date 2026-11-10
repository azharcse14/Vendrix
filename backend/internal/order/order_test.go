package order

import (
	"slices"
	"testing"
)

var (
	cod    = Payment{Method: "cod"}
	unpaid = Payment{Method: "sslcommerz"}
	paid   = Payment{Method: "sslcommerz", Paid: true}
)

func TestConfirm(t *testing.T) {
	cases := []struct {
		by   Actor
		p    Payment
		want bool
	}{
		{Customer, cod, true}, {Staff, cod, true}, {System, cod, false},
		{Customer, unpaid, false}, {Staff, unpaid, false}, {System, unpaid, false},
		{Customer, paid, false}, {Staff, paid, true}, {System, paid, true},
		{Vendor, cod, false},
	}
	for _, c := range cases {
		if got := CanTransitionOrder(Pending, Confirmed, c.by, c.p, nil); got != c.want {
			t.Errorf("confirm by %s %+v: got %v want %v", c.by, c.p, got, c.want)
		}
	}
}

func TestCancelConfirmed(t *testing.T) {
	cases := []struct {
		by   Actor
		fs   []FStatus
		want bool
	}{
		{Customer, []FStatus{Open, FCancelled}, true},
		{Customer, []FStatus{Packed}, false},
		{Staff, []FStatus{Packed, Open}, true},
		{Staff, []FStatus{Shipped}, false},
		{System, []FStatus{FCancelled, FCancelled}, true}, // cascade
		{System, []FStatus{Open}, false},                  // system emni cancel korte pare na
		{System, nil, false},
		{Vendor, []FStatus{Open}, false},
	}
	for _, c := range cases {
		if got := CanTransitionOrder(Confirmed, Cancelled, c.by, cod, c.fs); got != c.want {
			t.Errorf("cancel by %s %v: got %v want %v", c.by, c.fs, got, c.want)
		}
	}
}

func TestSummary(t *testing.T) {
	cases := []struct {
		o    Status
		fs   []FStatus
		want string
	}{
		{Pending, []FStatus{Open}, "pending"},
		{Confirmed, nil, "confirmed"}, // checkout step 14
		{Confirmed, []FStatus{Open}, "confirmed"},
		{Confirmed, []FStatus{Packed}, "packed"},
		{Confirmed, []FStatus{Delivered, FCancelled}, "delivered"},
		{Confirmed, []FStatus{Packed, Open}, "partial"},
		{Confirmed, []FStatus{FCancelled}, "cancelled"}, // cascade na hole DB CHECK atkabe
		{Cancelled, []FStatus{FCancelled}, "cancelled"},
	}
	for _, c := range cases {
		if got := Summary(c.o, c.fs); got != c.want {
			t.Errorf("Summary(%s, %v) = %q, want %q", c.o, c.fs, got, c.want)
		}
	}
}

// Shesh fulfillment cancel er puro flow: fulfillment cancel → AllCancelled → system order cancel.
func TestLastFulfillmentCancelFlow(t *testing.T) {
	fs := []FStatus{Open}
	if !CanTransitionFulfillment(fs[0], FCancelled, Vendor) {
		t.Fatal("vendor open fulfillment cancel korte pare")
	}
	fs[0] = FCancelled
	if !AllCancelled(fs) || !CanTransitionOrder(Confirmed, Cancelled, System, cod, fs) {
		t.Fatal("cascade cancel hote hobe")
	}
	if Summary(Cancelled, fs) != "cancelled" {
		t.Fatal("summary")
	}
	if len(CancelCascade(fs)) != 0 {
		t.Fatal("upor theke asha cascade e niche abar cancel korar kichu nai")
	}
}

// v1.3: order cancel er niche-r cascade. Customer, staff ba auto-cancel job (system), je-i cancel
// koruk, CanTransitionOrder true hole CancelCascade er fulfillment gulo cancel hoy, ar tarpor kono
// open ba packed fulfillment thake na.
func TestCancelCascade(t *testing.T) {
	cases := []struct {
		name string
		from Status
		by   Actor
		fs   []FStatus
		ok   bool
		want []int
	}{
		{"customer, confirmed", Confirmed, Customer, []FStatus{Open, FCancelled}, true, []int{0}},
		{"staff, packed shoho", Confirmed, Staff, []FStatus{Packed, Open, FCancelled}, true, []int{0, 1}},
		{"auto-cancel job, pending", Pending, System, []FStatus{Open, Open}, true, []int{0, 1}},
		{"customer, packed", Confirmed, Customer, []FStatus{Packed}, false, nil},
		{"staff, shipped", Confirmed, Staff, []FStatus{Shipped, Open}, false, nil},
	}
	for _, c := range cases {
		ok := CanTransitionOrder(c.from, Cancelled, c.by, cod, c.fs)
		if ok != c.ok {
			t.Errorf("%s: CanTransitionOrder = %v, want %v", c.name, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		got := CancelCascade(c.fs)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: CancelCascade = %v, want %v", c.name, got, c.want)
		}
		after := slices.Clone(c.fs)
		for _, i := range got {
			after[i] = FCancelled
		}
		for _, f := range after {
			if f == Open || f == Packed {
				t.Errorf("%s: cancel er por-o %s fulfillment", c.name, f)
			}
		}
		if Summary(Cancelled, after) != "cancelled" {
			t.Errorf("%s: summary", c.name)
		}
	}
}

// v1.4: golden table. Shob (from, to, actor) — order e payment ar fulfillment er obostha shoho — ekhane
// likha niyomer shathe mile kina. Table e kono actor jog ba bad dile (jemon Vendor ke packed cancel)
// ei test fail kore; v1.3 er test e emon 5 ta bodol dhora porto na.
func TestOrderTransitionGolden(t *testing.T) {
	// (from, to, actor) → kon obosthay allowed. Ekhane na thakle kokhono na.
	golden := map[[3]string]string{
		{"pending", "confirmed", "customer"}:   "cod",
		{"pending", "confirmed", "staff"}:      "cod_or_paid",
		{"pending", "confirmed", "system"}:     "paid_online",
		{"pending", "cancelled", "customer"}:   "always",
		{"pending", "cancelled", "staff"}:      "always",
		{"pending", "cancelled", "system"}:     "always",
		{"confirmed", "cancelled", "customer"}: "fs_open_or_cancelled",
		{"confirmed", "cancelled", "staff"}:    "fs_not_shipped",
		{"confirmed", "cancelled", "system"}:   "fs_all_cancelled",
	}
	payments := []Payment{cod, unpaid, paid, {Method: "bkash"}, {Method: "nagad", Paid: true}}
	fss := [][]FStatus{nil, {Open}, {Packed}, {Shipped}, {Delivered}, {Returned}, {FCancelled},
		{FCancelled, FCancelled}, {Open, FCancelled}, {Open, Packed}, {Packed, FCancelled}, {Shipped, Open}}
	every := func(fs []FStatus, ok ...FStatus) bool {
		for _, f := range fs {
			if !slices.Contains(ok, f) {
				return false
			}
		}
		return true
	}
	for _, from := range []Status{Pending, Confirmed, Cancelled} {
		for _, to := range []Status{Pending, Confirmed, Cancelled} {
			for _, by := range []Actor{System, Staff, Vendor, Customer} {
				for _, p := range payments {
					for _, fs := range fss {
						want := false
						switch golden[[3]string{string(from), string(to), string(by)}] {
						case "always":
							want = true
						case "cod":
							want = p.Method == "cod"
						case "cod_or_paid":
							want = p.Method == "cod" || p.Paid
						case "paid_online":
							want = p.Method != "cod" && p.Paid
						case "fs_open_or_cancelled":
							want = every(fs, Open, FCancelled)
						case "fs_not_shipped":
							want = every(fs, Open, Packed, FCancelled)
						case "fs_all_cancelled":
							want = len(fs) > 0 && every(fs, FCancelled)
						}
						if got := CanTransitionOrder(from, to, by, p, fs); got != want {
							t.Errorf("order %s → %s by %s, %+v, %v: got %v want %v", from, to, by, p, fs, got, want)
						}
					}
				}
			}
		}
	}
}

func TestFulfillmentTransitionGolden(t *testing.T) {
	golden := map[[2]FStatus][]Actor{
		{Open, Packed}:        {Staff, Vendor},
		{Open, FCancelled}:    {Staff, Vendor},
		{Packed, Shipped}:     {Staff, Vendor},
		{Packed, FCancelled}:  {Staff},
		{Shipped, Delivered}:  {Staff, System},
		{Shipped, Returned}:   {Staff, System},
		{Delivered, Returned}: {Staff},
	}
	all := []FStatus{Open, Packed, Shipped, Delivered, FCancelled, Returned}
	for _, from := range all {
		for _, to := range all {
			for _, by := range []Actor{System, Staff, Vendor, Customer} {
				want := slices.Contains(golden[[2]FStatus{from, to}], by)
				if got := CanTransitionFulfillment(from, to, by); got != want {
					t.Errorf("fulfillment %s → %s by %s: got %v want %v", from, to, by, got, want)
				}
			}
		}
	}
}
