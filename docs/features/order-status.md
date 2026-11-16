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

Database e status bodlay shudhu `transition.go` er duita function (8.8); onno kothao `UPDATE ... SET status` na:

6. `TransitionOrder(ctx, tx, shopID, orderID, to, By{Actor, ID, VendorID}, note)`: `lockOrder` (order → payment → fulfillment `id` er kram e `FOR UPDATE`), online unpaid `pending → confirmed` hole `ErrPaymentRequired` (actor jei hok), `CanTransitionOrder` na hole `ErrInvalidTransition`. Cancel e `CancelCascade` er open/packed fulfillment `cancelFulfillments` (history te order er actor), tarpor `coupon.Release` ar `sms.cancelled` job (ei tx e, `dedupe_key` `sms.cancelled:<order_id>`; [checkout.md](checkout.md) er Job); `orders` e `status`, `display_status = Summary`, `confirmed_at`/`cancelled_at`, cancel e `cancel_reason = note`; history row (`from_status` shoho).
7. `TransitionFulfillment(ctx, tx, shopID, fulfillmentID, to, by, note)`: fulfillment na pele, ba vendor actor er nijer na hole `ErrNotFound`; tarpor `lockOrder`, `CanTransitionFulfillment` na hole `ErrInvalidTransition`; M6 e shudhu `→ cancelled`, baki allowed bodol `ErrNotImplemented` (8.19 item 8, M7 e packing/shipment). Shob fulfillment cancelled (`AllCancelled`) hole ek-i tx e order `→ cancelled` (`System`, upore cascade), na hole shudhu `display_status`.
8. `cancelFulfillments`: `restock` (active item er qty variant e, `variant_id` er kram e lock, tarpor batch `id` er kram e `FOR UPDATE` (8.8); removed item bad, karon edit e already ferot; pack hoye thakle `order_item_batches` theke batch e; movement `cancel`, batch er ongsho `batch_id` shoho), prepaid (online ar `paid`/`partially_refunded`) e protiti fulfillment er bortoman `total > 0` hole `refundCancel` (`refunds` `kind = 'cancel'`, `pending`, method = payment method, shesh succeeded payment; mot refund paid er beshi hole ROLLBACK), status `cancelled` + `cancelled_at`, history.
9. Customer cancel `POST /api/orders/{token}/cancel` (7.5, Rule 11): `Cancel` handler body `{reason?}` (trim, max 200 rune, na hole `400 validation_failed`), `CancelByToken` (`public_token` + `shop_id` diye order, na pele `404 not_found`) → `TransitionOrder(→ cancelled, Customer)` (`actor_id` = session user shudhu order er `user_id` oi user hole, na hole NULL: `linkActor`, 8.19 item 13 d); `200 {"status": "cancelled"}`. Abar cancel ba packed thakle `409 invalid_transition`. Error map `transitionError` (M7 er admin route o eta nebe).

Prothom status (M6, 8.6 step 14) checkout e insert er shomoy boshe, bodol na: `PlaceOrder` ([checkout.md](checkout.md)) COD e `ConfirmRequired` hole `pending`, na hole `confirmed`; online e `pending`. `display_status = Summary(status, nil)` (fulfillment tokhono insert hoyni), fulfillment `open`, `order_status_history` e `from_status` NULL, actor `customer`. COD `pending` theke customer er confirm SMS er link e (`ConfirmByToken`, 8.10, [checkout.md](checkout.md)) `TransitionOrder(→ confirmed, Customer)` diye; confirm na hole `order.auto_cancel_unconfirmed` job (setting er ghonta por) `TransitionOrder(→ cancelled, System)`.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/order/doc.go` | Package comment (risk er niyom [checkout.md](checkout.md) e) |
| `backend/internal/order/order.go` → `CanTransitionOrder`, `CanTransitionFulfillment` | Onumoti |
| `order.go` → `CancelCascade`, `AllCancelled`, `Summary` | Cascade ar display |
| `backend/internal/order/transition.go` → `TransitionOrder`, `TransitionFulfillment` | DB e bodol, lock, history, display |
| `transition.go` → `lockOrder`, `cancelFulfillments`, `restock`, `refundCancel`, `history` | Side effect |
| `transition.go` → `ByStaff` | Admin staff → `By` (11.7 item 13 i) |
| `transition.go` → `CancelByToken`, `linkActor`; `handler.go` → `Cancel`, `transitionError` | Customer cancel route |
| `backend/internal/order/order_test.go` | Golden table |
| `backend/internal/order/transition_integration_test.go` | DB er bodol, cancel, display |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Online unpaid order confirm na (Rule 7) | `CanTransitionOrder` | `TestConfirm`, `TestOrderTransitionGolden` |
| Customer packed order cancel korte pare na (8.8) | `CanTransitionOrder` | `TestCancelConfirmed` |
| Shob fulfillment cancel → order cancel (8.8) | `AllCancelled` | `TestLastFulfillmentCancelFlow` |
| Order cancel → open ar packed fulfillment cancel (8.8) | `CancelCascade` | `TestCancelCascade` |
| Shob bodol golden table e (14.2 test 7) | table | `TestOrderTransitionGolden`, `TestFulfillmentTransitionGolden` |
| Admin staff → `By`: owner/admin/staff → `Staff`, vendor_owner/vendor_staff → `Vendor` + session er `vendor_id`; ochena role e Actor khali (kono transition na) (11.7 item 13 i) | `ByStaff` | `TestByStaff` |
| `display_status` (14.2 test 16) | `Summary` | `TestSummary` |
| DB e na-allowed bodol `invalid_transition`, online unpaid confirm `payment_required`, `confirmed_at`/history, M6 e packed `ErrNotImplemented`, onno shop/vendor `ErrNotFound`, route er `404`/`409`/`400` (8.8; 14.2 test 7) | `TransitionOrder`, `TransitionFulfillment`, `Cancel` | `TestTransitionIntegration` |
| Cancel e stock (`cancel` movement), packed e batch, coupon (`used_count`, redemption) ferot, `cancel_reason`; prepaid e protiti fulfillment e ekta refund (0 total e na), order level e na; removed item er stock abar na (8.8, 8.12, 8.13; 14.2 test 8) | `cancelFulfillments`, `restock`, `refundCancel`, `coupon.Release` | `TestCancelRestoresIntegration` |
| Customer cancel pending/confirmed (shob open), packed e na; staff packed shoho; system live order na; shob fulfillment cancel → order (system); vendor shudhu nijer (8.8; 14.2 test 26) | `CanTransitionOrder`, `CancelCascade`, `AllCancelled` | `TestCancelCascadeIntegration` |
| DB `display_status` = `Summary`: checkout, cancel, marketplace `partial` (14.2 test 16) | `TransitionOrder`, `TransitionFulfillment` | `TestDisplayStatusIntegration` |
| Auto-cancel (system) e cascade, stock/coupon ferot, `sms.cancelled` ekbar; customer cancel e o `sms.cancelled` (8.8, 8.10 item 4; 14.2 test 26) | `TransitionOrder`, `enqueueCancelled` | `TestAutoCancelUnconfirmedIntegration` |
| Checkout e prothom status, `display_status`, history row (8.6 step 14, 16) | `PlaceOrder` | `TestCheckoutSnapshotIntegration` |

## Data

`orders.status`, `orders.display_status`, `confirmed_at`, `cancelled_at`, `cancel_reason`, `order_fulfillments.status`/`cancelled_at`, `order_status_history` (6.7). Cancel e: `product_variants.stock`, `inventory_batches.quantity_remaining`, `inventory_movements` (`cancel`), `coupon_redemptions`/`coupons.used_count`, `refunds` (`refunds_cancel_once`), `jobs` (`sms.cancelled`). Database er CHECK: online unpaid order confirmed na, pending ar cancelled er display mile (`001_init_test.sql`).

## Bhul hole ki hoy

- Na-allowed bodol: `409 invalid_transition`; online unpaid order confirm: `409 payment_required` (`transitionError`). Kono side effect fail hole puro tx ROLLBACK (status o bodlay na).
- Refund paid er beshi hole (`errRefundOverPaid`, bug ba vul data) ROLLBACK + `500` log.
- Refund row `pending`; gateway e asol refund ar `payment_status` (`refunded`) M10 e.
- Caller cascade bhule gele `Summary` "cancelled" dey ar database er CHECK transaction atkay.

## Bodlanor age mone rakho

- Notun status ba actor: age design 8.8, tarpor golden table (test), tarpor code.
- Role (owner, admin, staff) er niyom ekhane na, handler/service e (11.2); `Staff` actor shob shop role. Admin handler `By` nije banabe na, `ByStaff(st)` diye.

## Proshno-uttor

- **Online unpaid confirm e customer o `payment_required` keno, `invalid_transition` na?** Taka na asha-i asol karon; 14.2 test 7 duita actor er jonno ekoi code chay.
- **Vendor onno vendor er fulfillment e `404` keno?** Ache kina bola na (IDOR).
- **Duita state machine keno?** Order holo kena ar payment er unit, fulfillment holo ek vendor er pack-ship; marketplace e ek order er duita parcel alada stage e thakte pare.
