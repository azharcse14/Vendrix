# Checkout

Design: 7.5, 8.5, 8.6, 8.7, 8.10, 8.16, 8.17, 8.19 item 2–5, 7, 12 · Milestone: M6 · Code: `backend/internal/order`

## Ki kore

Customer cart theke order dey (`POST /api/orders`). Checkout form er live taka quote theke (`POST /api/checkout/quote`, kichu save na). Order create 8.6 er 18 step: idempotent, stock ar coupon race-safe, Turnstile, risk flag, dorkar hole checkout OTP, rate limit, snapshot, job. COD order jhukipurno hole SMS link e confirm (`GET`/`POST /api/confirm/{confirm_token}`, 8.10), na hole `checkout.auto_cancel_unconfirmed_hours` (default 24) por auto-cancel. Customer order page (`GET /api/orders/{token}`), login chhara tracking (`POST /api/orders/track`) ar nijer order list (`GET /api/me/orders`) dekhe. Customer cancel (`POST /api/orders/{token}/cancel`) ar status bodol [order-status.md](order-status.md) e; website er page (`/checkout` + `@client CheckoutForm`, `/order/{token}`, done, `/confirm/{token}`, `/track` SSR POST, `/account/orders`) ar BFF [website.md](website.md) e (item 22–27).

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
3. `PlaceOrder` (`service.go`), transaction er age: `findByKey` e ager order → hash mile `200` ager response, na mille `422 idempotency_key_reused` (step 2, onno user/guest er hole-o, 8.19 item 12 iii). Step 3: `otp_code` thakle Turnstile dake na, kintu `auth.VerifyCheckoutOTP` (customer phone, purpose `checkout`; phone lock, attempt nijer commit e, `hmac.Equal`) shob shomoy, OTP lagbe kina dekhar age; bhul `400 otp_invalid`, 5 attempt `400 otp_too_many_attempts`, phone lock `429`; thik hole OTP er id rakhe (consume na). `otp_code` na thakle `Guard.Turnstile.Verify` (token, client IP, shop domain hostname); fail `400 turnstile_failed`, network bhul `500`. Settings er `payment_methods` e nai → `400`; login e customer phone account er na → `400`. Rate limit (7.8, 8.19 item 9, `orderLimit`): shesh 60 minute e ekoi `customer_phone` er 5 ba ekoi `ip` er 10 ta order (hoya order, DB theke) → `429 rate_limited` + `Retry-After` (purono order window theke ber howa porjonto). `shipping.Lookup` (`409 area_not_served`, step 5). COD e customer ba recipient `blocked_phones` e → `403 phone_blocked` (step 6). `checkoutLines` (step 7): khali `400 cart_empty`, bondho/stock nai `409 out_of_stock` + `details {variant_id}`, vendor `409 vendor_unavailable`; ei bhul e abar `findByKey` (ekoi key er parallel request ei matro commit kore cart khali korle tar order `200`). Risk flag `expected_total` diye (step 8); OTP verify na hoye thakle ar `OTPRequired` hole `auth.SendCheckoutOTP` (customer phone; login er ekoi send limit shob purpose mile, `sms_daily_cap`, blocked, phone lock; limit e `429` + `Retry-After`) tarpor `409 otp_required` + `details {phone_masked}` (`+88017****5678`). `otp_verified` = login ba OTP thik. Status: COD e `ConfirmRequired` hole `pending`, na hole `confirmed`; online e `pending` + `payment_status = 'pending'` (step 14). Deadline `same_city` 5, na hole 10 din.
4. `db.InTx` (`40P01`/`40001` e ekbar puro transaction abar): `lockKey` (`pg_advisory_xact_lock(hashtextextended(shop || ':' || key))`) + abar `findByKey` (step 9); OTP thakle `auth.ConsumeOTP` (conditional; already use → `400 otp_invalid`, onno key er order e code gele); line `variant_id` er kram e sajiye `takeStock` (6.4 er atomic `UPDATE`, product active, `RETURNING` e bortoman dam, kena dam, sku, naam, option, ojon, vendor er `commission_bps`), 0 row → `409 out_of_stock` (step 10); `coupon.Use` (step 11); `pricing.Calculate` (lock er porer dam diye), `ErrCouponNotApplicable` → `409 coupon_invalid`, zone `min_order` → `409 below_min_order`, `total != expected_total` → `409 price_changed` + `details {total, subtotal, discount, delivery_charge}` (step 12); `nextOrderNumber` (`<prefix>-100000+n`), `public_token = rand.Text()` (step 13); `insertOrder` `ON CONFLICT DO NOTHING` (coupon snapshot, COD `pending` hole `confirm_token_hash = ConfirmHash(ConfirmToken(Guard.ConfirmSecret, public_token))`, `risk_flags`, `otp_verified` (login ba OTP e true), `user_id`, `locale` `Accept-Language` theke, `ip`; step 15); protiti vendor group e `order_fulfillments` (`seq`), `insertItem` (snapshot ar `inventory_movements` `order`, `-qty`), `insertPlaced` (history `customer` actor, coupon redemption) (step 16); `cart.Clear` (step 17); `enqueuePlaced` (step 18, ei tx e: `sms.order_placed`, `alert.new_order`, payload shudhu `{order_id}`; COD `pending` hole `order.auto_cancel_unconfirmed` `RunAt = placed_at + checkout.auto_cancel_unconfirmed_hours` (order er shomoyer setting; nai ba 0 hole 24), `dedupe_key` `order.auto_cancel_unconfirmed:<order_id>`). Kono bhul e ROLLBACK, kichu save na (`used_count` o ferot).
5. `201 {order_number, public_token, status, payment_url: null}` (ager order `200`), `Cache-Control: no-store`.

COD confirm (8.10, Rule 11; public + optional login):

1. `ConfirmToken` (`confirm.go`, pure): `base32(HMAC-SHA256(CONFIRM_SECRET, "cod-confirm:" + public_token))` er prothom 26 okkhor; DB te shudhu `ConfirmHash` = `sha256(uppercase token)`. Token kono response e na; SMS job `public_token` theke abar banay (niche).
2. `GET /api/confirm/{confirm_token}` → `ConfirmView`: `shop_id` + `confirm_token_hash` diye order; `{order_number, display_status, placed_at, customer_name, phone_masked, district, area, payment_method, total, items: [{name, variant_label, quantity, line_total}]}` (active item; phone `maskPhone`, thikana line na, token na), `Cache-Control: no-store`.
3. `POST /api/confirm/{confirm_token}` (body lage na) → `ConfirmByToken` → `TransitionOrder(→ confirmed, Customer)` (login thakle history `actor_id`); `200 {"status":"confirmed"}`. Abar ba cancelled order e `409 invalid_transition`.
4. Na mille (public_token, ochena token, onno shop, confirm na laga order) `404 not_found`.

Order view, tracking, me/orders (7.3, 7.5, 8.19 item 6, Rule 11, 16; `view.go`):

1. `GET /api/orders/{token}` (public + optional login) → `ViewByToken`: shudhu `shop_id` + `public_token` (order number, confirm token, ochena `404 not_found`); `summary` (confirm page er ekoi shar-songkhep: phone `maskPhone`, district + area, line na, active item) + `recipient_name`, `recipient_phone_masked`, `subtotal`, `discount`, `delivery_charge`, `delivery_deadline`, `can_cancel` (= `CanTransitionOrder(→ cancelled, Customer)`, cancel route er ekoi niyom). Token response e na. `Cache-Control: no-store`.
2. `POST /api/orders/track` `{number, phone}` (public er upore client IP te minute e 10, `httpx.RateLimit`; `429` + `Retry-After`) → `phone.Normalize` (bhul `400 validation_failed`), number trim + uppercase → `Track`: ek query te `order_number` ar (`customer_phone` ba `recipient_phone`); na mille `404 not_found` (number ache kina bola na). `{order_number, display_status, placed_at, history: [{status, at}], tracking: null}`: history shudhu order level er `order_status_history` (fulfillment er ghotona M7 e courier er sathe); naam, phone, token na. `no-store`.
3. `GET /api/me/orders` (public + `RequireCustomer`, `MeRoutes`) → `MyOrders`: `shop_id` + session er `user_id` (Rule 16), `placed_at DESC, id DESC`, `httpx.Page` (7.1), card `{order_number, public_token, display_status, total, placed_at}`. Guest order account e ashe shudhu OTP login e (`auth` `linkGuestOrders`, [auth.md](auth.md)). `no-store`.

Job (8.17; `jobs.go` → `JobHandlers`, `cmd/api` worker e register; shob idempotent):

1. `sms.order_placed`: `loadSMSOrder` (`shop_id` + `order_id`): `public_token`, number, `customer_phone`, `orders.locale`, shop er naam (locale, na thakle bn), primary domain (`shop_domains.is_primary`), shop er `translations` override. `confirm_token_hash` thakle `ConfirmToken(CONFIRM_SECRET, public_token)` (hash na mille error: rotate er por bhul link pathay na) → template `sms.order_confirm` + `https://<domain>/confirm/<token>`; na hole `sms.order_placed` + `https://<domain>/order/<public_token>`. Lekha order row theke, tai retry e ekoi link. `notify.OrderText` diye lekha, `notify.SMS` e pathay (local e log).
2. `alert.new_order`: `slog` Info `new order` (shop, order id, number, total, payment method; phone/thikana na); email/SMS M12 e.
3. `order.auto_cancel_unconfirmed`: tx e `lockOrder`; ekhono `pending` COD hole `TransitionOrder(→ cancelled, System, "auto_cancel_unconfirmed")` (stock, coupon ferot, `sms.cancelled`); na hole kichu na (confirm/cancel hoye geche, ba abar chalano).
4. `sms.cancelled` ([order-status.md](order-status.md)): order cancel er tx e enqueue; template `sms.cancelled` (`{shop}`, `{number}`, karon na).

Risk:

1. 8.6 step 8: `LoadRiskHistory` (`store.go`) ei shop e ager order theke tin ta jinish ane: shesh 60 minute e ekoi `customer_phone` **ba** ekoi `ip` er order (shob status, cancelled o), customer ba recipient phone er ager order er kono fulfillment `returned` kina, ar `customer_phone` er ager order ache kina. Ei order insert er age chole.
2. `RiskFlags` (`risk.go`, pure): history, settings er `checkout.risk` (`RiskLimits`), COD kina ar `expected_total` diye flag, 8.7 er table er kram e: `cod_high_amount` (COD ar total `>` `cod_amount_over`), `phone_refused_before`, `many_orders` (ager order `>= max_orders_per_hour - 1`, mane ei ta shoho max), `new_phone_high_amount` (ager order nai ar total `>` `new_phone_amount_over`). Flag `orders.risk_flags` e jabe.
3. `OTPRequired(otp_mode, flags, cod, loggedIn)`: login customer e kokhono na; `never` na; `risky` COD ar flag thakle; `all_cod` shob COD; `all` ar ochena mode e shob.
4. `ConfirmRequired(cod_confirmation, flags, cod, otpVerified)`: online e kokhono na; `none` na; `risky` flag thakle **ar** `otp_verified = false`; `all` ar ochena mode e shob COD.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/cmd/api/router.go` → `newHandler` | Route: quote, order, view, cancel, confirm public (+ optional login), track e `RateLimit(10, minute)`, me/orders `RequireCustomer`; `Guard` (Turnstile, `OTP_SECRET`, `CONFIRM_SECRET`) |
| `backend/internal/order/handler.go` → `Routes`, `MeRoutes`, `Guard`, `Quote`, `place` | Route, quote er input format |
| `backend/internal/order/handler.go` → `Place`, `hashed`, `placeInput`, `placeResult`, `maskPhone` | Order route: key, hash, field check, error → HTTP |
| `backend/internal/order/handler.go` → `View`, `Track`, `MyOrders`, `ConfirmView`, `Confirm`, `Cancel`, `transitionError` | Order view/track/me, confirm ar cancel route |
| `backend/internal/order/service.go` → `QuoteCart`, `Quote`, `FieldError`, `ErrCartEmpty` | Quote er kram |
| `backend/internal/order/service.go` → `PlaceOrder`, `authErr`, `checkoutLines`, `variantLabel`, `OutOfStockError`, `PriceChangedError`, `LimitError` | Order create er 8.6 kram |
| `backend/internal/order/risk.go` → `RiskFlags`, `OTPRequired`, `ConfirmRequired` | Risk er niyom (pure) |
| `backend/internal/order/store.go` → `loadSettings`, `Settings`, `userPhone`, `LoadRiskHistory` | Checkout settings, login er phone, flag er jonno ager order |
| `backend/internal/order/store.go` → `findByKey`, `lockKey`, `orderLimit`, `phoneBlocked`, `takeStock`, `nextOrderNumber`, `insertOrder`, `insertFulfillment`, `insertItem`, `insertPlaced` | Order create er SQL (6.4) |
| `backend/internal/cart/service.go` → `CheckoutLines`, `CheckoutLine`, `Clear`, `Label` | Cart er line read-only, kena jay kina; order er por cart khali; variant label ([cart.md](cart.md)) |
| `backend/internal/order/confirm.go` → `ConfirmToken`, `ConfirmHash`, `ConfirmView`, `ConfirmSummary`, `summary`, `ConfirmByToken` | COD confirm token ar route er kaj; mask kora shar-songkhep |
| `backend/internal/order/view.go` → `ViewByToken`, `View`, `Track`, `Tracked`, `MyOrders`, `Card` | Order page, tracking, nijer order list |
| `backend/internal/order/jobs.go` → `enqueuePlaced`, `enqueueCancelled`, `JobHandlers`, `loadSMSOrder` | Step 18 er job, cancel er SMS, handler |
| `backend/internal/notify/notify.go` → `OrderText` | Order SMS er lekha |
| `backend/internal/order/risk_test.go`, `risk_integration_test.go` | Protiti mode × flag × payment; asol Postgres e history query |
| `backend/internal/order/quote_integration_test.go` | Quote er HTTP test, asol Postgres |
| `backend/internal/order/checkout_integration_test.go` | Order create er HTTP test, race, asol Postgres |
| `backend/internal/order/confirm_test.go`, `confirm_integration_test.go` | Confirm token (unit), confirm route (asol Postgres) |
| `backend/internal/order/view_integration_test.go` | View, tracking, me/orders, shop isolation, guest → login (asol Postgres) |
| `backend/internal/order/jobs_integration_test.go` | Job er test, asol Postgres |

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
| `otp_code` na thakle Turnstile (hostname shoho) `400 turnstile_failed`; `otp_code` thakle Turnstile dake na kintu shob shomoy verify (OTP na lagleo), bhul `400 otp_invalid`; retry ekoi key notun token ager order (8.6 step 2, 3; 14.2 test 23) | `PlaceOrder`, `auth.VerifyCheckoutOTP` | `TestCheckoutTurnstileIntegration` |
| OTP thik kintu `price_changed`/`out_of_stock` → ROLLBACK e OTP thake, ekoi code e order; order er por ekoi code onno key `400 otp_invalid`; risky e OTP proman hole `confirmed` (8.6 step 9; 14.2 test 40) | `PlaceOrder`, `auth.ConsumeOTP` | `TestCheckoutOTPRetryIntegration` |
| Flag `orders.risk_flags` e; login e OTP na (`otp_verified` true); guest risky `409 otp_required` + `phone_masked` + checkout OTP; send limit login er sathe ar `sms_daily_cap` `429`; phone 5/IP 10 ghontay hoya order, `price_changed` gone na, idempotent retry `200` (8.7; 8.19 item 3, 4, 9; 7.8) | `PlaceOrder`, `orderLimit`, `auth.SendCheckoutOTP` | `TestCheckoutRiskIntegration` |
| Confirm token = HMAC er base32 26 okkhor, ekoi input e ekoi; hash uppercase kore (8.10 item 1, 2) | `ConfirmToken`, `ConfirmHash` | `TestConfirmToken` |
| `public_token`/ochena/onno shop e confirm `404`; GET mask; lowercase link e `confirmed`; duibar `409 invalid_transition`; confirm na lagle ba online e `confirm_token_hash` nai (8.10, Rule 11, 14.2 test 31) | `ConfirmView`, `ConfirmByToken`, `insertOrder` | `TestConfirmTokenIntegration` |
| Step 18: `sms.order_placed` ar `alert.new_order` payload shudhu `order_id`, idempotent retry e notun job na; confirm lagle `sms.order_confirm` + `/confirm/<ConfirmToken>`, na hole `sms.order_placed` + `/order/<public_token>`; primary domain; `orders.locale`; retry e ekoi lekha; onno shop er job e order na; alert e phone na (8.6 step 18, 8.10 item 1, 8.19 item 7; 14.2 test 31) | `enqueuePlaced`, `JobHandlers`, `loadSMSOrder` | `TestOrderPlacedSMSIntegration` |
| Order SMS bn ≤ 134 unit, en ≤ 160 (GSM-7); override er kram, `{number}`/`{link}` chhara override bad (8.16, 8.19 item 7) | `notify.OrderText` | `TestOrderSMSTemplateLength` |
| Auto-cancel `RunAt = placed_at + order er shomoyer setting`, `dedupe_key`; shomoy hole system cancel, stock + coupon ferot, `sms.cancelled`; duibar chalale kichu na; confirmed e kichu na; shomoy na hole chole na (8.10 item 4, 8.19 item 12 viii; 14.2 test 26) | `enqueuePlaced`, `JobHandlers` | `TestAutoCancelUnconfirmedIntegration` |
| Order page shudhu `public_token` diye (order number/confirm token/ochena `404`); phone mask, thikana district + area, token/line na; `can_cancel` cancel er por false; `no-store` (7.5, 8.19 item 6, Rule 11) | `ViewByToken`, `summary` | `TestOrderViewIntegration` |
| Tracking: number + customer ba recipient phone (local/+880, number er case jai hok); na mille `404`; bhul phone `400`; history, `tracking: null`, token/phone/naam na; minute e 10 er por `429` + `Retry-After` (7.5, 7.8, 8.19 item 6) | `Track`, `Handlers.Track` | `TestTrackIntegration` |
| `/api/me/orders` shudhu nijer `user_id` (onno user, ekoi phone er guest order na), notun age, paged, card e 5 ta field, login chhara `401` (7.3, Rule 16, 8.19 item 12 x) | `MyOrders` | `TestMyOrdersIntegration` |
| Shop a er token, number + phone, confirm token, Bearer shop b te `404`/`401`; shop b er coupon/zone shop a te na (Rule 3; 14.2 test 5) | `ViewByToken`, `Track`, `ConfirmByToken`, `QuoteCart` | `TestOrderShopIsolationIntegration` |
| Guest checkout → OTP login → `/api/me/orders` e oi order; onno phone er order na (8.2 step 5; 14.2 test 11) | `auth` `linkGuestOrders`, `MyOrders` | `TestCheckoutGuestThenLoginIntegration` |
| 60 minute, phone ba ip, cancelled o gona, returned customer ba recipient phone, shudhu ei shop (8.19 item 3, Rule 3) | `LoadRiskHistory` | `TestRiskHistoryIntegration` |

## Data

`orders` (`customer_phone`, `ip`, `placed_at`, `risk_flags`, `otp_verified`, `confirm_token_hash` + `orders_confirm_token_cod_check` + `orders_confirm_token_uniq`), `order_fulfillments.status`; index `orders_phone_idx`, `orders_placed_idx` (`001_init.sql`). Settings `checkout.payment_methods`, `checkout.otp_mode`, `checkout.cod_confirmation`, `checkout.risk`, `checkout.auto_cancel_unconfirmed_hours`, `marketplace.delivery_charge_mode` (9.2). Quote pore: `carts`/`cart_items`, `delivery_zones`, `coupons`/`coupon_redemptions`, `users.phone`. Order create likhe: `orders` (`idempotency_key`/`idempotency_hash`, coupon snapshot, `delivery_deadline`), `order_fulfillments`, `order_items`, `inventory_movements` (`order`), `order_status_history`, `coupon_redemptions`, `shop_counters.order_number`, `product_variants.stock`, `coupons.used_count`, `jobs`; job pore `shops.name`, `shop_domains`, `translations`; pore `blocked_phones`; `cart_items` muche. Notun migration nai.

## Bhul hole ki hoy

- Settings e ochena `otp_mode` ba `cod_confirmation`: OTP ar confirm duitai lage (fail closed), order atke na.
- `ip` jana na (`""`): `many_orders` ar order rate limit shudhu phone diye.
- Turnstile siteverify er network bhul: `500` (token bhul bola na), order na.
- Checkout OTP SMS cap ba send limit shesh: `429` + `Retry-After`; COD na hole-o blocked phone e OTP pathay na (`403 phone_blocked`, fail closed).
- Settings e `payment_methods` nai: quote `400` (kono method chalu na); `delivery_charge_mode` nai: `per_order`.
- Quote e `pricing` `ErrInvalidInput` (code er bug): `500`, log.
- Order e kono step fail: puro ROLLBACK (stock, `used_count`, counter, cart ager moto); deadlock/serialization e ekbar abar, tarpor `500`.
- Order SMS job e primary domain nai, `confirm_token_hash` `CONFIRM_SECRET` er sathe mile na, ba provider nai: job error, retry, shesh e `failed_at` ar admin alert; order thake (SMS chhara confirm staff er phone e, 8.10 item 3).
- Lock er porer dam quote theke alada: `409 price_changed` notun total shoho; customer abar submit kore (notun key lage na, body bodlay bole form notun key dey).

## Bodlanor age mone rakho

- Flag er naam `orders.risk_flags` e save hoy ar admin e dekhay (M7): naam bodlale purono order er sathe mele na.
- `phone_refused_before` e ager order er shudhu `customer_phone` mele (`orders_phone_idx`); ager order er `recipient_phone` dhorte hole notun index (`002`) lagbe.
- Checkout er lock kram: idempotency lock → OTP consume (row update, onno flow er lock er sathe joray na) → variant (`variant_id` sajano) → coupon → counter (8.8). Notun lock jog korle ei kram e.
- OTP ROLLBACK e thake: `price_changed`/`out_of_stock` er por ekoi code e abar submit (14.2 test 40). Protiti submit e attempt gone (5 tomo te notun OTP lage; resend = `otp_code` chhara, notun Turnstile).
- Order rate limit er `ip` query te index nai (`orders_placed_idx` e shop er shesh ghonta); lagle `002`.
- Job handler idempotent rakho (8.17): SMS er lekha order row theke abar banao, payload e token/phone rakhbe na (8.10 item 1). Order SMS template bodlale `TestOrderSMSTemplateLength` (bn 134 unit) dekho; shop override er length check nai (admin M7).
- Auto-cancel er ghonta order er shomoy `RunAt` e boshe; pore setting bodlaleo ager order er job bodlay na.
- `CONFIRM_SECRET` rotate korle pending COD order er `confirm_token_hash` notun secret e abar likho (8.10 item 5); na hole SMS retry er link `404`.
- Niyom bodlale age 8.7/8.19, tarpor `risk_test.go`, tarpor code (`design-change`).

## Proshno-uttor

- **Tracking e `public_token` ferot dey na keno?** Number + phone jana je keu tahole cancel korte parto (8.19 item 6); order page er link shudhu SMS e jay.
- **Order page e puro thikana na keno?** SMS link forward hote pare (12.2 Data 5); thikana courier ar admin e.
- **Quote e `below_min_order` problem keno, `409` na?** Form e total dekhiye "aro kinun" bola jay; order (`POST /api/orders`) e eta `409 below_min_order`. Zone ba cart na thakle total-i hoy na, tai `409 area_not_served`/`400 cart_empty`.
- **Quote cart normalize kore na keno?** 7.5 "kichu save hoy na"; normalize `GET /api/cart` e. Stock er beshi qty quote e `out_of_stock` problem.

- **Dam lock er por abar pori keno, cart theke na?** `takeStock` er `UPDATE ... RETURNING` row lock er por bortoman dam dey; `expected_total` er sathe tulona eta diye-i, tai admin er shesh muhurter dam bodol `price_changed` dey, purono dam e order na.
- **Step 7 er bhul e abar key dekhi keno?** Ekoi key er duita request: prothom ta commit kore cart khali ar stock 0 kore dile ditiyo ta step 7 e `cart_empty`/`out_of_stock` peto; commit er por key-o dekha jay, tai tokhon ager order `200` (14.2 test 4).
- **Cancelled order o keno gona?** Bot barbar order diye cancel kore; 8.19 item 3 e user er decision.
- **`otp_code` er verify step 8 er bodole step 3 er jaygay keno?** 8.6 step 3: `otp_code` thakle shob shomoy verify; agei korle bhul code e bot Turnstile chhara zone/cart/rate limit er uttor dekhte pare na.
- **OTP/`price_changed` er retry rate limit e gone na keno?** Shudhu hoya order gona (8.19 item 9); OTP flow e ek order e 2–3 request.
- **Confirm SMS er job payload e token nai keno?** Token DB/payload e plain text e na (8.10 item 1); worker `public_token` theke abar banay, tai retry e ekoi link.
- **Order SMS kar phone e?** `customer_phone` (je order diyeche ar OTP/confirm kore), recipient e na.
- **Confirm e `public_token` keno na?** `public_token` order-dewa browser o pay; tar diye confirm hole onner number e fake order dewa manush nijei confirm korto (8.10 item 1, Rule 11).
- **Login customer e OTP keno na?** Account er phone already OTP e proman, ar form e phone bodlano jay na (8.19 item 4).
