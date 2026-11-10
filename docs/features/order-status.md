# Order ar fulfillment er status

Design: 3.7, 8.8 · Milestone: checkout (M6) ar admin (M7) e use · Code: `backend/internal/order`

## Ki kore

Order (customer ar payment: pending theke confirmed ba cancelled) ar fulfillment (ek vendor er pack-ship: open, packed, shipped, delivered, ba cancelled, returned) er kon bodol ke korte pare, ta thik kore. Customer ar admin je ekta status dekhe (`display_status`) seta hishab kore.

## Kivabe kaj kore

1. `CanTransitionOrder(from, to, actor, payment, fulfillments)`: actor er onumoti; confirm e COD hole customer ba staff, online hole shudhu paid ar system ba staff; confirmed theke cancel e fulfillment er obostha (customer: shob open ba cancelled; staff: packed o; system: shob cancelled).
2. `CanTransitionFulfillment(from, to, actor)`: table dekhe.
3. `CancelCascade(fs)`: order cancel hole kon fulfillment cancel hobe (open ar packed).
4. `AllCancelled(fs)`: shob fulfillment cancel hole system order cancel kore.
5. `Summary(order, fs)`: `display_status`; vendor ra alada stage e hole `partial`.

`TransitionOrder` ar `TransitionFulfillment` (M6) ei function gulo diye database e bodlabe; onno kothao `UPDATE ... SET status` na.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/order/order.go` → `CanTransitionOrder`, `CanTransitionFulfillment` | Onumoti |
| `order.go` → `CancelCascade`, `AllCancelled`, `Summary` | Cascade ar display |
| `backend/internal/order/order_test.go` | Golden table |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Online unpaid order confirm na (Rule 7) | `CanTransitionOrder` | `TestConfirm`, `TestOrderTransitionGolden` |
| Customer packed order cancel korte pare na (8.8) | `CanTransitionOrder` | `TestCancelConfirmed` |
| Shob fulfillment cancel → order cancel (8.8) | `AllCancelled` | `TestLastFulfillmentCancelFlow` |
| Order cancel → open ar packed fulfillment cancel (8.8) | `CancelCascade` | `TestCancelCascade` |
| Shob bodol golden table e (14.2 test 7) | table | `TestOrderTransitionGolden`, `TestFulfillmentTransitionGolden` |
| `display_status` (14.2 test 16) | `Summary` | `TestSummary` |

## Data

`orders.status`, `orders.display_status`, `order_fulfillments.status`, `order_status_history` (6.7). Database er CHECK: online unpaid order confirmed na, pending ar cancelled er display mile (`001_init_test.sql`).

## Bhul hole ki hoy

- Na-allowed bodol: `409 invalid_transition`; online unpaid order confirm: `409 payment_required` (M6 er handler).
- Caller cascade bhule gele `Summary` "cancelled" dey ar database er CHECK transaction atkay.

## Bodlanor age mone rakho

- Notun status ba actor: age design 8.8, tarpor golden table (test), tarpor code.
- Role (owner, admin, staff) er niyom ekhane na, handler e (11.2); `Staff` actor shob shop role.

## Proshno-uttor

- **Duita state machine keno?** Order holo kena ar payment er unit, fulfillment holo ek vendor er pack-ship; marketplace e ek order er duita parcel alada stage e thakte pare.
