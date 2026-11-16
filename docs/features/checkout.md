# Checkout

Design: 8.6, 8.7, 8.19 item 3, 4 · Milestone: M6 · Code: `backend/internal/order`

## Ki kore

Customer cart theke order dey (`POST /api/orders`). Ekhon porjonto shudhu risk er niyom ache: kon order jhukipurno (flag), kokhon checkout OTP lage, ar kokhon COD order SMS link e confirm lage. Quote, order create, OTP, order view, tracking, cancel ar COD confirm M6 er porer step e (`docs/milestones/M6.md` step 4–13).

## Kivabe kaj kore

1. 8.6 step 8: `LoadRiskHistory` (`store.go`) ei shop e ager order theke tin ta jinish ane: shesh 60 minute e ekoi `customer_phone` **ba** ekoi `ip` er order (shob status, cancelled o), customer ba recipient phone er ager order er kono fulfillment `returned` kina, ar `customer_phone` er ager order ache kina. Ei order insert er age chole.
2. `RiskFlags` (`risk.go`, pure): history, settings er `checkout.risk` (`RiskLimits`), COD kina ar `expected_total` diye flag, 8.7 er table er kram e: `cod_high_amount` (COD ar total `>` `cod_amount_over`), `phone_refused_before`, `many_orders` (ager order `>= max_orders_per_hour - 1`, mane ei ta shoho max), `new_phone_high_amount` (ager order nai ar total `>` `new_phone_amount_over`). Flag `orders.risk_flags` e jabe.
3. `OTPRequired(otp_mode, flags, cod, loggedIn)`: login customer e kokhono na; `never` na; `risky` COD ar flag thakle; `all_cod` shob COD; `all` ar ochena mode e shob.
4. `ConfirmRequired(cod_confirmation, flags, cod, otpVerified)`: online e kokhono na; `none` na; `risky` flag thakle **ar** `otp_verified = false`; `all` ar ochena mode e shob COD.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/order/risk.go` → `RiskFlags`, `OTPRequired`, `ConfirmRequired` | Risk er niyom (pure) |
| `backend/internal/order/store.go` → `LoadRiskHistory` | Flag er jonno ager order er query |
| `backend/internal/order/risk_test.go` | Protiti mode × flag × payment |
| `backend/internal/order/risk_integration_test.go` | Asol Postgres e history query |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Flag er shima `>` (shoman na), `many_orders` `>= max - 1` (8.7, 8.19 item 3) | `RiskFlags` | `TestRiskFlags` |
| `otp_mode` table; login customer e OTP na (8.7, 8.19 item 4) | `OTPRequired` | `TestOTPRequired` |
| `cod_confirmation`: `risky` = flag ar `otp_verified = false`; shudhu COD (8.19 item 4) | `ConfirmRequired` | `TestConfirmRequired` |
| 60 minute, phone ba ip, cancelled o gona, returned customer ba recipient phone, shudhu ei shop (8.19 item 3, Rule 3) | `LoadRiskHistory` | `TestRiskHistoryIntegration` |

## Data

`orders` (`customer_phone`, `ip`, `placed_at`, `risk_flags`, `otp_verified`), `order_fulfillments.status`; index `orders_phone_idx`, `orders_placed_idx` (`001_init.sql`). Settings `checkout.otp_mode`, `checkout.cod_confirmation`, `checkout.risk` (9.2). Notun migration nai.

## Bhul hole ki hoy

- Settings e ochena `otp_mode` ba `cod_confirmation`: OTP ar confirm duitai lage (fail closed), order atke na.
- `ip` jana na (`""`): `many_orders` shudhu phone diye.

## Bodlanor age mone rakho

- Flag er naam `orders.risk_flags` e save hoy ar admin e dekhay (M7): naam bodlale purono order er sathe mele na.
- `phone_refused_before` e ager order er shudhu `customer_phone` mele (`orders_phone_idx`); ager order er `recipient_phone` dhorte hole notun index (`002`) lagbe.
- Niyom bodlale age 8.7/8.19, tarpor `risk_test.go`, tarpor code (`design-change`).

## Proshno-uttor

- **Cancelled order o keno gona?** Bot barbar order diye cancel kore; 8.19 item 3 e user er decision.
- **Login customer e OTP keno na?** Account er phone already OTP e proman, ar form e phone bodlano jay na (8.19 item 4).
