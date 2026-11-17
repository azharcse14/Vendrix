# Coupon

Design: 8.13, 6.4, 8.6 step 11, 8.19 item 5, 7.7, 11.7 item 13 v · Milestone: M6, M7a · Code: `backend/internal/coupon`

## Ki kore

Checkout e coupon code diye discount: code thik, chalu, shomoy er bhitore, mot `usage_limit` ar ek phone er `per_phone_limit` er bhitore hole coupon er shorto ferot dey (discount er hishab `pricing` e). Quote e ekoi check read-only. Order cancel e coupon ferot (redemption muche, `used_count - 1`). Owner/admin admin theke coupon toiri ar bodlay (`/api/admin/coupons`).

## Kivabe kaj kore

1. `Normalize`: code `trim` + `upper`; `""` = coupon nai; 32 rune er beshi hole `ok = false` (form e `400 validation_failed`). `Use`/`Peek` nijeo normalize kore.
2. `Use` (checkout er transaction, `order.PlaceOrder`, 8.6 step 11): age 6.4 er atomic `UPDATE coupons SET used_count = used_count + 1 ... RETURNING` (coupon row lock), tarpor `checkPhone` (`coupon_redemptions` e ei shop er (`shop_id`, Rule 3) ei `customer_phone` er count `>= per_phone_limit` hole `ErrLimitReached`). Kram ulto hole ek phone er duita checkout duitai 0 dekhe (6.4). `UPDATE` 0 row dile `check` diye karon.
3. `check`: code dhore row; nai, `is_active = false` ba shomoy er baire → `ErrInvalid`; tarpor `usage_limit` shesh → `ErrLimitReached`. Bondho ar shesh duitai hole `ErrInvalid` (invalid age).
4. `Peek` (quote, `order.QuoteCart`; error hole `problems` e code, coupon chhara total): shudhu `check` (per-phone na: quote e onno phone er coupon use jana jay na, 8.19 item 13 b; `Use` e dhore), kichu bodlay na.
5. Error hole caller transaction `ROLLBACK` kore, tai `used_count` ferot ashe. `ErrorCode` error theke API code (`coupon_invalid`, `coupon_limit_reached`).
6. `Release` (cancel, 8.8): ek statement e order er redemption `DELETE`, tar coupon e `used_count = GREATEST(used_count - 1, 0)`. Redemption na thakle kichu na. Caller `order.TransitionOrder(→ cancelled)` (fulfillment er restock er pore, lock er kram 8.8; [order-status.md](order-status.md)).
7. Admin (`admin.go`, owner/admin, 11.2): `GET /api/admin/coupons` (paged, notun age), `POST` (`201`), `PATCH /{id}` (`200`). PATCH e row `FOR UPDATE` (`lockCoupon`; checkout er `Use` er sathe `used_count` thik), `row.apply` input boshay: code `Normalize` + `^[A-Z0-9_-]{1,32}$`, `kind` `percent`/`fixed`, `value` > 0 (percent ≤ 100), `min_order` ≥ 0, `max_discount`/`usage_limit` > 0 ba `null`, `per_phone_limit` ≥ 1, tarikh RFC 3339 (UTC e save), `starts_at < expires_at`. `used_count > 0` hole code/kind/value bodlale `400 fields.<f>: locked` (ekoi value chole); `usage_limit < used_count` `fields.usage_limit: below_used`. `saveCoupon` puro row likhe (`used_count` kokhono na), audit `coupon.create`/`coupon.update` ek tx e. Duplicate (`coupons_code_uniq`) `409 coupon_exists`; `vendor_id`/`funded_by`/`used_count` ochena field `400` (MV); delete na (`is_active`).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/coupon/service.go` → `Normalize`, `ErrorCode` | Code er niyom, error → API code |
| `backend/internal/coupon/store.go` → `Use`, `Peek`, `Release` | SQL: atomic use, karon (`check`), phone count (`checkPhone`), ferot |
| `backend/internal/coupon/admin.go` → `AdminRoutes`, `AdminCoupons`, `write`, `row.apply`, `lockCoupon`, `saveCoupon` | Admin list, create, update, lock, audit |
| `backend/internal/coupon/coupon_test.go` | Unit test |
| `backend/internal/coupon/coupon_integration_test.go`, `admin_integration_test.go` | Asol Postgres e test |

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
| Admin: code format, duplicate `409 coupon_exists`, use hole code/kind/value lock, `usage_limit` ≥ `used_count`, owner/admin, audit (11.7 item 13 v, v1.45) | `admin.go` → `row.apply`, `write` | `TestAdminCouponIntegration` |
| Onno shop er coupon admin e `404`, list e nai (14.2 test 5) | `admin.go` er `shop_id` | `TestAdminCouponIntegration`, `TestAdminShopIsolationIntegration` |

## Data

`coupons` (001_init.sql): `code` (`coupons_code_uniq` `upper(code)` shop e), `kind`, `value`, `min_order`, `max_discount`, `usage_limit` (NULL = unlimited), `used_count` (`coupons_used_le_limit_check`), `per_phone_limit`, `starts_at`/`expires_at`, `is_active`, `vendor_id`, `funded_by`. `coupon_redemptions`: `order_id` UNIQUE, `phone`, FK order er coupon; index `coupon_redemptions_phone_idx (coupon_id, phone)`. Admin `/api/admin/coupons` (M7a); local e `scripts/dev-seed.sql` (WELCOME10, FLAT50).

## Bhul hole ki hoy

- Code nai, bondho, shuru hoy ni ba meyad shesh: `ErrInvalid` → `409 coupon_invalid`.
- `usage_limit` shesh ba ei phone er `per_phone_limit` shesh: `ErrLimitReached` → `409 coupon_limit_reached`.
- `Use` er error e caller `ROLLBACK` na korle `used_count` bere thake; checkout er transaction e-i dakte hobe.
- Admin: bhul field `400 validation_failed` + `fields` (`locked`, `below_used` ba `invalid`); duplicate `409 coupon_exists`; onno shop `404`; `staff`/vendor role `403`.
- Coupon thik kintu cart e khate na (`min_order`, eligible 0): eta `pricing.Calculate` er `ErrCouponNotApplicable` (8.5), ekhane na.

## Bodlanor age mone rakho

- `Use` er kram (age `UPDATE`, tarpor count) ar `READ COMMITTED` bodlale phone limit race e bhange (6.4); `TestCheckoutCouponLimitRaceIntegration`, `TestCheckoutCouponPhoneLimitIntegration` ([checkout.md](checkout.md)) dhore.
- Lock er kram: coupon row variant er pore, counter er age (8.8); `Use` checkout er oi jaygay dakte hobe.
- `Coupon` er field order er `coupon_*` snapshot e jay (8.6 step 15); field bodlale checkout ar `pricing.md` o.

## Proshno-uttor

- **Keno limit er karon bola hoy (`coupon_limit_reached`)?** User 15 November 2026 (M6 Q5): customer thik karon dekhe; code guess Turnstile ar 7.8 er limit e atke.
