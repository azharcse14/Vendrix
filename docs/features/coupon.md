# Coupon

Design: 8.13, 6.4, 8.6 step 11, 8.19 item 5 · Milestone: M6 · Code: `backend/internal/coupon`

## Ki kore

Checkout e coupon code diye discount: code thik, chalu, shomoy er bhitore, mot `usage_limit` ar ek phone er `per_phone_limit` er bhitore hole coupon er shorto ferot dey (discount er hishab `pricing` e). Quote e ekoi check read-only. Order cancel e coupon ferot (redemption muche, `used_count - 1`).

## Kivabe kaj kore

1. `Normalize`: code `trim` + `upper`; `""` = coupon nai; 32 rune er beshi hole `ok = false` (form e `400 validation_failed`). `Use`/`Peek` nijeo normalize kore.
2. `Use` (checkout er transaction, `order.PlaceOrder`, 8.6 step 11): age 6.4 er atomic `UPDATE coupons SET used_count = used_count + 1 ... RETURNING` (coupon row lock), tarpor `checkPhone` (`coupon_redemptions` e ei shop er (`shop_id`, Rule 3) ei `customer_phone` er count `>= per_phone_limit` hole `ErrLimitReached`). Kram ulto hole ek phone er duita checkout duitai 0 dekhe (6.4). `UPDATE` 0 row dile `check` diye karon.
3. `check`: code dhore row; nai, `is_active = false` ba shomoy er baire → `ErrInvalid`; tarpor `usage_limit` shesh → `ErrLimitReached`. Bondho ar shesh duitai hole `ErrInvalid` (invalid age).
4. `Peek` (quote, `order.QuoteCart`; error hole `problems` e code, coupon chhara total): shudhu `check` (per-phone na: quote e onno phone er coupon use jana jay na, 8.19 item 13 b; `Use` e dhore), kichu bodlay na.
5. Error hole caller transaction `ROLLBACK` kore, tai `used_count` ferot ashe. `ErrorCode` error theke API code (`coupon_invalid`, `coupon_limit_reached`).
6. `Release` (cancel, 8.8): ek statement e order er redemption `DELETE`, tar coupon e `used_count = GREATEST(used_count - 1, 0)`. Redemption na thakle kichu na. Caller `order.TransitionOrder(→ cancelled)` (fulfillment er restock er pore, lock er kram 8.8; [order-status.md](order-status.md)).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/coupon/service.go` → `Normalize`, `ErrorCode` | Code er niyom, error → API code |
| `backend/internal/coupon/store.go` → `Use`, `Peek`, `Release` | SQL: atomic use, karon (`check`), phone count (`checkPhone`), ferot |
| `backend/internal/coupon/coupon_test.go` | Unit test |
| `backend/internal/coupon/coupon_integration_test.go` | Asol Postgres e test |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Code `trim` + `upper`, max 32, khali = nai (8.19 item 5, 10) | `service.go` → `Normalize` | `TestCouponNormalize` |
| Error → `409 coupon_invalid` / `coupon_limit_reached` (8.19 item 5) | `service.go` → `ErrorCode` | `TestCouponNormalize` |
| Atomic `used_count + 1` (limit, shomoy, bondho), tarpor phone count (6.4, 8.6 step 11) | `store.go` → `Use` | `TestCouponUseIntegration`, `TestCancelRestoresIntegration` |
| Karon: nai/bondho/shomoy → invalid, limit → limit_reached; error e ROLLBACK e `used_count` ferot (8.19 item 5) | `store.go` → `check`, `checkPhone` | `TestCouponUseIntegration` |
| Onno shop er code chole na (Rule 3) | protiti query er `shop_id` | `TestCouponUseIntegration`, `TestCouponPeekIntegration` |
| Quote e read-only, `Use` er karon per-phone chhara (8.19 item 5, 13 b) | `store.go` → `Peek` | `TestCouponPeekIntegration` |
| Cancel e redemption muche `GREATEST(used_count - 1, 0)` (8.13) | `store.go` → `Release` | `TestCouponUseIntegration` |

## Data

`coupons` (001_init.sql): `code` (`coupons_code_uniq` `upper(code)` shop e), `kind`, `value`, `min_order`, `max_discount`, `usage_limit` (NULL = unlimited), `used_count` (`coupons_used_le_limit_check`), `per_phone_limit`, `starts_at`/`expires_at`, `is_active`, `vendor_id`, `funded_by`. `coupon_redemptions`: `order_id` UNIQUE, `phone`, FK order er coupon; index `coupon_redemptions_phone_idx (coupon_id, phone)`. Coupon admin (toiri, edit) M7 e.

## Bhul hole ki hoy

- Code nai, bondho, shuru hoy ni ba meyad shesh: `ErrInvalid` → `409 coupon_invalid`.
- `usage_limit` shesh ba ei phone er `per_phone_limit` shesh: `ErrLimitReached` → `409 coupon_limit_reached`.
- `Use` er error e caller `ROLLBACK` na korle `used_count` bere thake; checkout er transaction e-i dakte hobe.
- Coupon thik kintu cart e khate na (`min_order`, eligible 0): eta `pricing.Calculate` er `ErrCouponNotApplicable` (8.5), ekhane na.

## Bodlanor age mone rakho

- `Use` er kram (age `UPDATE`, tarpor count) ar `READ COMMITTED` bodlale phone limit race e bhange (6.4); `TestCheckoutCouponLimitRaceIntegration`, `TestCheckoutCouponPhoneLimitIntegration` ([checkout.md](checkout.md)) dhore.
- Lock er kram: coupon row variant er pore, counter er age (8.8); `Use` checkout er oi jaygay dakte hobe.
- `Coupon` er field order er `coupon_*` snapshot e jay (8.6 step 15); field bodlale checkout ar `pricing.md` o.

## Proshno-uttor

- **Keno limit er karon bola hoy (`coupon_limit_reached`)?** User 15 November 2026 (M6 Q5): customer thik karon dekhe; code guess Turnstile ar 7.8 er limit e atke.
