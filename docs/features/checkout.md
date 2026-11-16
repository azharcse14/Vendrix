# Checkout

Design: 7.5, 8.5, 8.6, 8.7, 8.19 item 2–5 · Milestone: M6 · Code: `backend/internal/order`

## Ki kore

Customer cart theke order dey (`POST /api/orders`). Ekhon porjonto ache: quote (`POST /api/checkout/quote`: checkout form er live taka, kichu save na) ar risk er niyom: kon order jhukipurno (flag), kokhon checkout OTP lage, ar kokhon COD order SMS link e confirm lage. Order create, OTP, order view, tracking, cancel ar COD confirm M6 er porer step e (`docs/milestones/M6.md` step 5–13).

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
| `backend/internal/order/service.go` → `QuoteCart`, `Quote`, `FieldError`, `ErrCartEmpty` | Quote er kram |
| `backend/internal/cart/service.go` → `CheckoutLines`, `CheckoutLine` | Cart er line read-only, kena jay kina |
| `backend/internal/order/store.go` → `loadSettings`, `Settings`, `userPhone` | Checkout settings, login er phone |
| `backend/internal/order/risk.go` → `RiskFlags`, `OTPRequired`, `ConfirmRequired` | Risk er niyom (pure) |
| `backend/internal/order/store.go` → `LoadRiskHistory` | Flag er jonno ager order er query |
| `backend/internal/order/risk_test.go` | Protiti mode × flag × payment |
| `backend/internal/order/risk_integration_test.go` | Asol Postgres e history query |
| `backend/internal/order/quote_integration_test.go` | Quote er HTTP test, asol Postgres |

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
| 60 minute, phone ba ip, cancelled o gona, returned customer ba recipient phone, shudhu ei shop (8.19 item 3, Rule 3) | `LoadRiskHistory` | `TestRiskHistoryIntegration` |

## Data

`orders` (`customer_phone`, `ip`, `placed_at`, `risk_flags`, `otp_verified`), `order_fulfillments.status`; index `orders_phone_idx`, `orders_placed_idx` (`001_init.sql`). Settings `checkout.payment_methods`, `checkout.otp_mode`, `checkout.cod_confirmation`, `checkout.risk`, `marketplace.delivery_charge_mode` (9.2). Quote pore: `carts`/`cart_items`, `delivery_zones`, `coupons`/`coupon_redemptions`, `users.phone`. Notun migration nai.

## Bhul hole ki hoy

- Settings e ochena `otp_mode` ba `cod_confirmation`: OTP ar confirm duitai lage (fail closed), order atke na.
- `ip` jana na (`""`): `many_orders` shudhu phone diye.
- Settings e `payment_methods` nai: quote `400` (kono method chalu na); `delivery_charge_mode` nai: `per_order`.
- Quote e `pricing` `ErrInvalidInput` (code er bug): `500`, log.

## Bodlanor age mone rakho

- Flag er naam `orders.risk_flags` e save hoy ar admin e dekhay (M7): naam bodlale purono order er sathe mele na.
- `phone_refused_before` e ager order er shudhu `customer_phone` mele (`orders_phone_idx`); ager order er `recipient_phone` dhorte hole notun index (`002`) lagbe.
- Niyom bodlale age 8.7/8.19, tarpor `risk_test.go`, tarpor code (`design-change`).

## Proshno-uttor

- **Quote e `below_min_order` problem keno, `409` na?** Form e total dekhiye "aro kinun" bola jay; order (`POST /api/orders`) e eta `409 below_min_order`. Zone ba cart na thakle total-i hoy na, tai `409 area_not_served`/`400 cart_empty`.
- **Quote cart normalize kore na keno?** 7.5 "kichu save hoy na"; normalize `GET /api/cart` e. Stock er beshi qty quote e `out_of_stock` problem.

- **Cancelled order o keno gona?** Bot barbar order diye cancel kore; 8.19 item 3 e user er decision.
- **Login customer e OTP keno na?** Account er phone already OTP e proman, ar form e phone bodlano jay na (8.19 item 4).
