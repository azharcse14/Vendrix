# Checkout

Design: 7.5, 8.5, 8.6, 8.7, 8.19 item 2–5 · Milestone: M6 · Code: `backend/internal/order`

## Ki kore

Customer cart theke order dey (`POST /api/orders`). Ekhon porjonto ache: quote (`POST /api/checkout/quote`: checkout form er live taka, kichu save na), order create (8.6 step 1–2, 4–17, OTP chhara; idempotent, stock ar coupon race-safe) ar risk er niyom: kon order jhukipurno (flag), kokhon checkout OTP lage, ar kokhon COD order SMS link e confirm lage. Turnstile, checkout OTP ar rate limit (step 3, 8, 9; M6 step 6), confirm token (step 8 er M6), job/SMS (step 18; M6 step 9), order view, tracking, cancel M6 er porer step e (`docs/milestones/M6.md`).

## Kivabe kaj kore

Quote (`POST /api/checkout/quote`, public + optional login, cart er moto owner: login e account cart, na hole `X-Cart-Key`):

1. `Quote` (`handler.go`): body `{coupon_code, district, area, payment_method, phone}`, ochena field (`total`, `expected_total` ...) `400 validation_failed` (`DisallowUnknownFields`, Rule 1). Phone `phone.Normalize` (bhul `fields.phone = invalid_phone`), coupon `coupon.Normalize` (32 er beshi bhul), district/area trim kore 1–100 rune.
2. `QuoteCart` (`service.go`): settings (`loadSettings`) er `payment_methods` e nai → `400` (`fields.payment_method`); login customer er body phone account er phone na hole `400` (`fields.phone`, 8.19 item 4).
3. `shipping.Lookup` → zone; na pele `409 area_not_served`.
4. `cart.CheckoutLines`: read-only (GET er moto normalize na, cart bodlay na). Line na thakle `400 cart_empty`. Bondho product/variant, stock er beshi qty ba nominal ojon nai → problem `out_of_stock`, vendor active na → `vendor_unavailable`; egulo total e na.
5. Coupon thakle `coupon.Peek` (read-only); `coupon_invalid`/`coupon_limit_reached` hole problem, coupon chhara.
6. `pricing.Calculate(lines, coupon, zone, marketplace.delivery_charge_mode)`; `ErrCouponNotApplicable` hole problem `coupon_invalid` ar coupon chhara abar `Calculate` (8.5, 14.2 test 39). Free delivery `>=` discount er porer taka (pricing er niyom).
7. Zone `min_order` (discount er porer taka) er kom → problem `below_min_order` (order e `409`, quote e total shoho dekhay).
8. Guest e `LoadRiskHistory` (customer = recipient = body phone, client IP) → `RiskFlags` (total = quote er total, order e eta-i `expected_total`) → `OTPRequired`; login e `false`.
9. `200 {subtotal, discount, delivery_charge, total, otp_required, problems}` (`problems` khali hole `[]`), `Cache-Control: no-store`.

Order create (`POST /api/orders`, quote er moto owner), 8.6 er step:

1. `Place` (`handler.go`): `Idempotency-Key` `^[A-Za-z0-9_-]{16,64}$` na hole `400 missing_idempotency_key` (step 1). Body (7.5) `DisallowUnknownFields` (dam/total field `400`, Rule 1); `expected_total` na thakle `400 validation_failed`. `idempotency_hash` = `hashed` struct (customer, recipient, address, note, `coupon_code`, `payment_method`, `expected_total`, `consent`) er `encoding/json` er `sha256`; `otp_code`, `turnstile_token`, `website` hash e na.
2. `placeInput` (step 4, 8.19 item 10): naam 1–100 rune (trim), phone `phone.Normalize`, email optional (`net/mail`, max 254), recipient `null` hole customer er copy, district/area 1–100, thikana line 5–500, note max 500, coupon `coupon.Normalize`; honeypot `website` khali na hole ba `consent` `true` na hole `400` (`fields`).
3. `PlaceOrder` (`service.go`), transaction er age: `findByKey` e ager order → hash mile `200` ager response, na mille `422 idempotency_key_reused` (step 2, onno user/guest er hole-o, 8.19 item 12 iii). `otp_code` thakle ekhon `400 otp_invalid` (verify M6 step 6). Settings er `payment_methods` e nai → `400`; login e customer phone account er na → `400`. `shipping.Lookup` (`409 area_not_served`, step 5). COD e customer ba recipient `blocked_phones` e → `403 phone_blocked` (step 6). `checkoutLines` (step 7): khali `400 cart_empty`, bondho/stock nai `409 out_of_stock` + `details {variant_id}`, vendor `409 vendor_unavailable`; ei bhul e abar `findByKey` (ekoi key er parallel request ei matro commit kore cart khali korle tar order `200`). Risk flag `expected_total` diye (step 8); `OTPRequired` hole ekhon `409 otp_required` (OTP pathano M6 step 6). Status: COD e `ConfirmRequired` hole `pending`, na hole `confirmed`; online e `pending` + `payment_status = 'pending'` (step 14). Deadline `same_city` 5, na hole 10 din.
4. `db.InTx` (`40P01`/`40001` e ekbar puro transaction abar): `lockKey` (`pg_advisory_xact_lock(hashtextextended(shop || ':' || key))`) + abar `findByKey` (step 9); line `variant_id` er kram e sajiye `takeStock` (6.4 er atomic `UPDATE`, product active, `RETURNING` e bortoman dam, kena dam, sku, naam, option, ojon, vendor er `commission_bps`), 0 row → `409 out_of_stock` (step 10); `coupon.Use` (step 11); `pricing.Calculate` (lock er porer dam diye), `ErrCouponNotApplicable` → `409 coupon_invalid`, zone `min_order` → `409 below_min_order`, `total != expected_total` → `409 price_changed` + `details {total, subtotal, discount, delivery_charge}` (step 12); `nextOrderNumber` (`<prefix>-100000+n`), `public_token = rand.Text()` (step 13); `insertOrder` `ON CONFLICT DO NOTHING` (coupon snapshot, `risk_flags`, `otp_verified` (login e true), `user_id`, `locale` `Accept-Language` theke, `ip`; step 15); protiti vendor group e `order_fulfillments` (`seq`), `insertItem` (snapshot ar `inventory_movements` `order`, `-qty`), `insertPlaced` (history `customer` actor, coupon redemption) (step 16); `cart.Clear` (step 17). Kono bhul e ROLLBACK, kichu save na (`used_count` o ferot).
5. `201 {order_number, public_token, status, payment_url: null}` (ager order `200`), `Cache-Control: no-store`.

Risk:

1. 8.6 step 8: `LoadRiskHistory` (`store.go`) ei shop e ager order theke tin ta jinish ane: shesh 60 minute e ekoi `customer_phone` **ba** ekoi `ip` er order (shob status, cancelled o), customer ba recipient phone er ager order er kono fulfillment `returned` kina, ar `customer_phone` er ager order ache kina. Ei order insert er age chole.
2. `RiskFlags` (`risk.go`, pure): history, settings er `checkout.risk` (`RiskLimits`), COD kina ar `expected_total` diye flag, 8.7 er table er kram e: `cod_high_amount` (COD ar total `>` `cod_amount_over`), `phone_refused_before`, `many_orders` (ager order `>= max_orders_per_hour - 1`, mane ei ta shoho max), `new_phone_high_amount` (ager order nai ar total `>` `new_phone_amount_over`). Flag `orders.risk_flags` e jabe.
3. `OTPRequired(otp_mode, flags, cod, loggedIn)`: login customer e kokhono na; `never` na; `risky` COD ar flag thakle; `all_cod` shob COD; `all` ar ochena mode e shob.
4. `ConfirmRequired(cod_confirmation, flags, cod, otpVerified)`: online e kokhono na; `none` na; `risky` flag thakle **ar** `otp_verified = false`; `all` ar ochena mode e shob COD.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/order/handler.go` → `Handlers.Routes`, `Quote`, `place` | Quote route, input format |
| `backend/internal/order/handler.go` → `Place`, `hashed`, `placeInput`, `placeResult` | Order route: key, hash, field check, error → HTTP |
| `backend/internal/order/service.go` → `QuoteCart`, `Quote`, `FieldError`, `ErrCartEmpty` | Quote er kram |
| `backend/internal/order/service.go` → `PlaceOrder`, `checkoutLines`, `variantLabel`, `OutOfStockError`, `PriceChangedError` | Order create er 8.6 kram |
| `backend/internal/order/store.go` → `findByKey`, `lockKey`, `phoneBlocked`, `takeStock`, `nextOrderNumber`, `insertOrder`, `insertFulfillment`, `insertItem`, `insertPlaced` | Order create er SQL (6.4) |
| `backend/internal/cart/service.go` → `CheckoutLines`, `CheckoutLine`, `Clear`, `Label` | Cart er line read-only, kena jay kina; order er por cart khali; variant label |
| `backend/internal/order/store.go` → `loadSettings`, `Settings`, `userPhone` | Checkout settings, login er phone |
| `backend/internal/order/risk.go` → `RiskFlags`, `OTPRequired`, `ConfirmRequired` | Risk er niyom (pure) |
| `backend/internal/order/store.go` → `LoadRiskHistory` | Flag er jonno ager order er query |
| `backend/internal/order/risk_test.go` | Protiti mode × flag × payment |
| `backend/internal/order/risk_integration_test.go` | Asol Postgres e history query |
| `backend/internal/order/quote_integration_test.go` | Quote er HTTP test, asol Postgres |
| `backend/internal/order/checkout_integration_test.go` | Order create er HTTP test, race, asol Postgres |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Flag er shima `>` (shoman na), `many_orders` `>= max - 1` (8.7, 8.19 item 3) | `RiskFlags` | `TestRiskFlags` |
| `otp_mode` table; login customer e OTP na (8.7, 8.19 item 4) | `OTPRequired` | `TestOTPRequired` |
| `cod_confirmation`: `risky` = flag ar `otp_verified = false`; shudhu COD (8.19 item 4) | `ConfirmRequired` | `TestConfirmRequired` |
| Quote: taka `pricing.Calculate` theke, free delivery `>=`, district-wide zone, `below_min_order` problem, `409 area_not_served`, `400 cart_empty`, kena jay na line `out_of_stock` problem (7.5, 8.5, 8.19 item 2) | `QuoteCart` | `TestQuoteIntegration` |
| Quote e coupon na khatle coupon chhara total + `coupon_invalid`/`coupon_limit_reached`, `used_count` bodlay na (8.5, 8.19 item 5, 14.2 test 39) | `QuoteCart`, `coupon.Peek` | `TestQuoteIntegration` |
| Quote e client er dam/total field `400`, kichu save na (order, coupon, redemption, OTP, job, cart) (Rule 1, 7.5) | `Quote`, `QuoteCart` | `TestQuoteIntegration` |
| Quote e guest risky `otp_required`, login e na (8.19 item 4) | `QuoteCart` | `TestQuoteIntegration` |
| Ekoi key: parallel e ek order, ek bar stock, duplicate `200` ager order; commit er por retry `200`; alada body `422`; key nai/bhul `400 missing_idempotency_key`; `expected_total` nai `400` (8.6 step 1, 2, 9, 15; 14.2 test 4) | `Place`, `PlaceOrder`, `lockKey`, `findByKey` | `TestCheckoutIdempotencyIntegration` |
| Stock 1 e 10 parallel → 1 order, baki `409 out_of_stock` (`variant_id`); variant sajano, deadlock nai (8.6 step 10; 14.2 test 1) | `takeStock`, `PlaceOrder` | `TestCheckoutStockRaceIntegration`, `TestCheckoutLockOrderIntegration` |
| `usage_limit` race e ek discount; ek phone e `per_phone_limit` er beshi na, ROLLBACK e `used_count` ferot (8.6 step 11; 14.2 test 2, 3) | `coupon.Use` in `PlaceOrder` | `TestCheckoutCouponLimitRaceIntegration`, `TestCheckoutCouponPhoneLimitIntegration` |
| Coupon na khatle (`min_order`, eligible 0) `409 coupon_invalid`, `used_count`/redemption bodlay na (8.5; 14.2 test 39) | `PlaceOrder` | `TestCheckoutCouponNotApplicableIntegration` |
| DB te `sum(discount_share) = discount`, line er jog = subtotal, fulfillment er jog = order (14.2 test 15) | `PlaceOrder`, `insertItem` | `TestCheckoutTotalsSavedIntegration` |
| Coupon pore bodlaleo `LineFromItem` + `orders.coupon_*` diye `Reprice` ager result (14.2 test 33) | `insertOrder` (snapshot) | `TestCouponSnapshotIntegration` |
| Honeypot, consent, field limit, client dam field `400`; `area_not_served`, `phone_blocked` (customer/recipient), `price_changed` details, `below_min_order`, bondho product `out_of_stock`, `cart_empty`; bhul e kichu save na (8.6 step 3–7, 12; 8.19 item 10) | `placeInput`, `PlaceOrder` | `TestCheckoutValidationIntegration` |
| Snapshot (naam, label, sku, dam, kena dam, ojon, thikana, coupon), number `SA-100001`, `public_token`, deadline 5/10 din, `pending`/`confirmed`, history, movement, redemption, cart khali, login e `user_id` + `otp_verified`, `locale` (8.6 step 13–17, Rule 8) | `PlaceOrder`, `insertOrder`, `insertItem`, `insertPlaced` | `TestCheckoutSnapshotIntegration` |
| 60 minute, phone ba ip, cancelled o gona, returned customer ba recipient phone, shudhu ei shop (8.19 item 3, Rule 3) | `LoadRiskHistory` | `TestRiskHistoryIntegration` |

## Data

`orders` (`customer_phone`, `ip`, `placed_at`, `risk_flags`, `otp_verified`), `order_fulfillments.status`; index `orders_phone_idx`, `orders_placed_idx` (`001_init.sql`). Settings `checkout.payment_methods`, `checkout.otp_mode`, `checkout.cod_confirmation`, `checkout.risk`, `marketplace.delivery_charge_mode` (9.2). Quote pore: `carts`/`cart_items`, `delivery_zones`, `coupons`/`coupon_redemptions`, `users.phone`. Order create likhe: `orders` (`idempotency_key`/`idempotency_hash`, coupon snapshot, `delivery_deadline`), `order_fulfillments`, `order_items`, `inventory_movements` (`order`), `order_status_history`, `coupon_redemptions`, `shop_counters.order_number`, `product_variants.stock`, `coupons.used_count`; pore `blocked_phones`; `cart_items` muche. Notun migration nai.

## Bhul hole ki hoy

- Settings e ochena `otp_mode` ba `cod_confirmation`: OTP ar confirm duitai lage (fail closed), order atke na.
- `ip` jana na (`""`): `many_orders` shudhu phone diye.
- Settings e `payment_methods` nai: quote `400` (kono method chalu na); `delivery_charge_mode` nai: `per_order`.
- Quote e `pricing` `ErrInvalidInput` (code er bug): `500`, log.
- Order e kono step fail: puro ROLLBACK (stock, `used_count`, counter, cart ager moto); deadlock/serialization e ekbar abar, tarpor `500`.
- Lock er porer dam quote theke alada: `409 price_changed` notun total shoho; customer abar submit kore (notun key lage na, body bodlay bole form notun key dey).

## Bodlanor age mone rakho

- Flag er naam `orders.risk_flags` e save hoy ar admin e dekhay (M7): naam bodlale purono order er sathe mele na.
- `phone_refused_before` e ager order er shudhu `customer_phone` mele (`orders_phone_idx`); ager order er `recipient_phone` dhorte hole notun index (`002`) lagbe.
- Checkout er lock kram: idempotency lock → (step 6 e OTP consume) → variant (`variant_id` sajano) → coupon → counter (8.8). Notun lock jog korle ei kram e.
- Ekhon Turnstile check nai ar `otp_code` thakle `400 otp_invalid`, OTP lagle `409 otp_required` (pathano na): M6 step 6 e puro hobe; tar age route production e jabe na.
- Step 18 er job (`sms.order_placed`, `alert.new_order`, `order.auto_cancel_unconfirmed`) ar `confirm_token_hash` ekhono na (M6 step 8, 9): `pending` COD order ekhon nijer theke cancel hoy na.
- Niyom bodlale age 8.7/8.19, tarpor `risk_test.go`, tarpor code (`design-change`).

## Proshno-uttor

- **Quote e `below_min_order` problem keno, `409` na?** Form e total dekhiye "aro kinun" bola jay; order (`POST /api/orders`) e eta `409 below_min_order`. Zone ba cart na thakle total-i hoy na, tai `409 area_not_served`/`400 cart_empty`.
- **Quote cart normalize kore na keno?** 7.5 "kichu save hoy na"; normalize `GET /api/cart` e. Stock er beshi qty quote e `out_of_stock` problem.

- **Dam lock er por abar pori keno, cart theke na?** `takeStock` er `UPDATE ... RETURNING` row lock er por bortoman dam dey; `expected_total` er sathe tulona eta diye-i, tai admin er shesh muhurter dam bodol `price_changed` dey, purono dam e order na.
- **Step 7 er bhul e abar key dekhi keno?** Ekoi key er duita request: prothom ta commit kore cart khali ar stock 0 kore dile ditiyo ta step 7 e `cart_empty`/`out_of_stock` peto; commit er por key-o dekha jay, tai tokhon ager order `200` (14.2 test 4).
- **Cancelled order o keno gona?** Bot barbar order diye cancel kore; 8.19 item 3 e user er decision.
- **Login customer e OTP keno na?** Account er phone already OTP e proman, ar form e phone bodlano jay na (8.19 item 4).
