# Dam er hishab

Design: 8.5 (ar 8.6 step 12, 8.11, 8.13) · Milestone: checkout (M6) ar packing (M7) e use · Code: `backend/internal/pricing`

## Ki kore

Order er shob taka: line total, coupon er discount, delivery charge, mot taka, ar protiti vendor (fulfillment) er bhag. Packing (asol ojon) ar order edit e notun hishab, prepaid hole refund er poriman. Pure function: database, HTTP, shomoy, random na.

## Kivabe kaj kore

1. Checkout ar quote, `Calculate(lines, coupon, zone, mode)`:
   1. Input check (`validate`); bhul hole `ErrInvalidInput`.
   2. Line total (`LineTotal`; ojon-variable e dam × gram / 1000, half-up).
   3. Coupon: eligible line er subtotal 0 ba `min_order` er kom hole `ErrCouponNotApplicable`; na hole `Discount` (percent e floor, cap, subtotal er beshi na), line e bhag (`Allocate`, largest remainder).
   4. Delivery: discount er porer taka `free_delivery_over` er shoman ba beshi hole 0; `per_vendor` e protiti vendor e charge, `per_order` e vendor der moddhe subtotal onujayi bhag.
   5. `finish`: vendor ar order er total.
2. Packing ba edit, `Reprice(Original, newLines)`:
   1. `checkOriginal`: database theke asha ager hishab nijeder moddhe mile kina.
   2. Je vendor er kono line bodlay nai tar taka hubohu ager (freeze).
   3. Bodla vendor: line total (prepaid e ager er beshi na), discount (percent e delta, fixed e edit e onupate), delivery ager ta (shob line bad hole 0).
   4. Prepaid hole parthokko refund.
3. Database er row theke line shudhu `LineFromItem`; edit e ojon `EditWeightG`.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/pricing/pricing.go` → `Calculate`, `validate`, `Discount`, `Allocate`, `buildGroups`, `finish` | Checkout er hishab |
| `pricing.go` → `Reprice`, `checkOriginal`, `repriceDiscount` | Packing ar edit |
| `pricing.go` → `LineFromItem`, `EditWeightG`, `Commission` | Row theke line, ojon, commission |
| `backend/internal/pricing/pricing_test.go` | Golden, random ar guard test |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Taka integer poisha, percent e floor (Rule 2) | `Discount`, `LineTotal` | `TestCalculate` |
| Bhag er jog thik mot (14.2 test 15) | `Allocate`, `buildGroups`, `finish` | `TestAllocate`, `TestGroupsAndShares`, `TestInvariantsRandom` |
| Coupon na khatle 409, chupchap 0 na (8.5) | `Calculate` | `TestCouponNotApplicable` |
| Prepaid e customer beshi dey na (8.11) | `Reprice` | `TestReprice`, `TestRepriceEditThenPackRandom` |
| Edit e fixed coupon onupate (14.2 test 22) | `repriceDiscount` | `TestEditProratesFixedCoupon` |
| Onno vendor er taka freeze (14.2 test 32) | `Reprice` | `TestRepriceFreezesOtherVendors` |
| Percent e dui dhape-o prepaid bare na (8.5) | `repriceDiscount` | `TestRepricePercentTwoStepsNeverRaisesPrepaid` |
| Coupon order er snapshot theke (14.2 test 33) | `Reprice` | `TestRepriceUsesCouponSnapshot` |
| Reprice e min_order abar dekhe na (8.11) | `repriceDiscount` | `TestRepricePercentMinOrderNotRechecked`, `TestRepriceZeroDiscountStaysZero` |
| ৳0 er line baki thakle delivery thake (8.5) | `Reprice` | `TestRepriceDeliveryKeepsFreeLineVendor` |
| Bhul input e error (14.2 test 27) | `validate`, `checkOriginal` | `TestInvalidInput` |
| Removed item abar bill e na (14.2 test 36) | `LineFromItem` | `TestLineFromItem` |
| Edit e ojon onupate, kom-poksha 1 gram | `EditWeightG` | `TestEditWeightG` |
| Commission (8.18) | `Commission` | `TestCommission` |

## Data

Pure; caller `order_items` (`line_total`, `discount_share`, `commission`, `commission_bps`), `order_fulfillments` ar `orders` er taka save kore (6.7). Coupon ashe `orders.coupon_*` snapshot theke, `coupons` table theke na.

## Bhul hole ki hoy

- `ErrInvalidInput`: caller er bug: ROLLBACK, log, `500`; kokhono chupchap hishab na.
- `ErrCouponNotApplicable`: checkout `409 coupon_invalid` (step 11 er `used_count` o ROLLBACK e ferot); quote coupon chhara abar hishab kore.
- `ErrLineCount`: notun line er shonkhya Original er sathe mile na.

## Bodlanor age mone rakho

- Eta design er asol spec: niyom bodlate `design-change` skill (age 8.5 er lekha).
- Test durbol korbe na; golden case bodlale karon ar section.
- Bodoler por `go test -race ./internal/pricing` er shob golden ar random test.

## Proshno-uttor

- **Largest remainder keno?** Bhag er jog thik mot er shoman, ek poisha o hariye jay na; soman bhagshesh e ager line ek poisha beshi pay.
- **"Freeze" keno?** Marketplace e ek vendor er packing onno vendor er already ship hoya parcel er taka bodlabe na.
