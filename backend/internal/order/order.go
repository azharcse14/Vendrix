// Package order: order ar fulfillment er status kon theke kon e jete pare.
// v1.2 fix: (1) confirm e payment dekha hoy, (2) shob fulfillment cancel hole system order cancel
// korte pare, (3) checkout e fulfillment insert er age Summary "confirmed" dey.
// v1.3 fix: order cancel hole kon fulfillment cancel hobe sheta CancelCascade dey (niche-r cascade).
//
// Feature doc: docs/features/order-status.md
package order

// Status: order er nijer status (customer ar payment).
type Status string

const (
	Pending   Status = "pending"
	Confirmed Status = "confirmed"
	Cancelled Status = "cancelled"
)

// FStatus: fulfillment er status (ek vendor er pack-ship).
type FStatus string

const (
	Open       FStatus = "open"
	Packed     FStatus = "packed"
	Shipped    FStatus = "shipped"
	Delivered  FStatus = "delivered"
	FCancelled FStatus = "cancelled"
	Returned   FStatus = "returned"
)

type Actor string

const (
	System   Actor = "system"
	Staff    Actor = "staff"  // shop er staff
	Vendor   Actor = "vendor" // vendor staff, shudhu nijer fulfillment
	Customer Actor = "customer"
)

// Payment: confirm korar shomoy order er payment er obostha.
type Payment struct {
	Method string // orders.payment_method: "cod", "sslcommerz", "bkash", "nagad"
	Paid   bool   // orders.payment_status = 'paid' (shudhu validated IPN er por)
}

var orderAllowed = map[Status]map[Status][]Actor{
	Pending:   {Confirmed: {System, Staff, Customer}, Cancelled: {System, Staff, Customer}},
	Confirmed: {Cancelled: {System, Staff, Customer}}, // System shudhu cascade e (niche)
}

var fulfillmentAllowed = map[FStatus]map[FStatus][]Actor{
	Open:      {Packed: {Staff, Vendor}, FCancelled: {Staff, Vendor}},
	Packed:    {Shipped: {Staff, Vendor}, FCancelled: {Staff}},
	Shipped:   {Delivered: {Staff, System}, Returned: {Staff, System}},
	Delivered: {Returned: {Staff}},
}

func allowed(as []Actor, a Actor) bool {
	for _, x := range as {
		if x == a {
			return true
		}
	}
	return false
}

// CanTransitionOrder:
//   - pending → confirmed: COD e customer (link) ba staff (phone). Online e shudhu paid hole,
//     system (IPN) ba staff; customer kokhono na.
//   - confirmed → cancelled: customer shudhu shob fulfillment open/cancelled thakle; staff packed
//     thakleo; system shudhu jokhon shob fulfillment already cancelled (cascade).
//   - Cancel allowed hole kon fulfillment gulo cancel hobe: CancelCascade (v1.3).
func CanTransitionOrder(from, to Status, by Actor, p Payment, fs []FStatus) bool {
	if !allowed(orderAllowed[from][to], by) {
		return false
	}
	switch {
	case from == Pending && to == Confirmed:
		if p.Method == "cod" {
			return by == Customer || by == Staff
		}
		return p.Paid && (by == System || by == Staff)
	case from == Confirmed && to == Cancelled:
		if by == System {
			return AllCancelled(fs)
		}
		for _, f := range fs {
			if !(f == Open || f == FCancelled || (f == Packed && by == Staff)) {
				return false
			}
		}
	}
	return true
}

// CanTransitionFulfillment: pack korar age order confirmed kina, sheta Transition function dekhe.
func CanTransitionFulfillment(from, to FStatus, by Actor) bool {
	return allowed(fulfillmentAllowed[from][to], by)
}

// CancelCascade (v1.3): order cancel hole (CanTransitionOrder true howar por) kon fulfillment gulo
// cancel hobe: open ar packed, input er index e. Actor er onumoti order level e age-i dekha hoyeche
// (customer: shob open/cancelled; staff: packed o; system: pending order, ba AllCancelled), tai
// ekhane CanTransitionFulfillment na. TransitionOrder egulo ek-i transaction e cancel kore
// (fulfillment cancel er side effect shoho, history te order er actor), TransitionFulfillment
// dake na, tai upore abar TransitionOrder ashe na. AllCancelled theke asha cascade e list khali.
func CancelCascade(fs []FStatus) []int {
	var idx []int
	for i, f := range fs {
		if f == Open || f == Packed {
			idx = append(idx, i)
		}
	}
	return idx
}

// AllCancelled: ekta-o fulfillment ache ar shob cancelled. TransitionFulfillment ei obosthay
// ek-i transaction e TransitionOrder(confirmed → cancelled, System) dake, tarpor Summary.
func AllCancelled(fs []FStatus) bool {
	if len(fs) == 0 {
		return false
	}
	for _, f := range fs {
		if f != FCancelled {
			return false
		}
	}
	return true
}

// Summary: orders.display_status.
// fs khali (checkout e fulfillment insert er age) hole "confirmed". Order confirmed kintu shob
// fulfillment cancelled hole "cancelled" dey: caller cascade bhule gele DB CHECK transaction atkabe.
func Summary(o Status, fs []FStatus) string {
	switch o {
	case Pending:
		return "pending"
	case Cancelled:
		return "cancelled"
	}
	var first FStatus
	n := 0
	for _, f := range fs {
		if f == FCancelled {
			continue
		}
		if n > 0 && f != first {
			return "partial"
		}
		first, n = f, n+1
	}
	switch {
	case n == 0 && len(fs) == 0:
		return "confirmed"
	case n == 0:
		return "cancelled"
	case first == Open:
		return "confirmed"
	}
	return string(first)
}
