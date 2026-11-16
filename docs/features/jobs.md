# Background job ar SMS

Design: 6.4, 8.2, 8.16, 8.17, 12.2, 13.4 · Milestone: M0, M4, M5, M6 · Code: `backend/internal/jobs`, `backend/internal/notify`

## Ki kore

Request er baire je kaj (SMS, alert, auto-cancel, reconcile) `jobs` table e queue hoy; `shop-api serve` er bhetore ekta worker goroutine kaj nay ar chalay. Fail hole pore abar chesta, bar bar fail hole thame ar admin alert. Local e SMS pathano hoy na, log e lekha hoy.

## Kivabe kaj kore

1. Kaj jog: `jobs.Enqueue` (`*sql.DB` ba order er `*sql.Tx`). `DedupeKey` thakle ekoi key e duibar row hoy na. `RunAt` khali hole database er `now()` (app er ghori na).
2. `serve` (`cmd/api/main.go` → `runServe`): `jobs.NewWorker` e kind → handler (`sms.send` → `notify.SendSMSJob` (OTP er jonno `auth.SMSCode` shoho), `alert.security` → `notify.SecurityAlertJob`, `cart.cleanup` → `cart.CleanupJob`, `inventory.expire` → `inventory.ExpireJob`; `order.JobHandlers`: `sms.order_placed`, `sms.cancelled`, `sms.total_changed` ([order-status](order-status.md) pack), `alert.new_order`, `order.auto_cancel_unconfirmed`, [checkout](checkout.md)), `Worker.Run` goroutine e; shutdown e worker thame, cholche emon kaj er result lekha hoy, tarpor database close.
3. `Worker.Run`: protiti 2 second e `schedule` (recurring `jobs.sweep` 5 minute er slot `dedupe_key`, `jobs.cleanup` ar `cart.cleanup` UTC din er `dedupe_key`; `Recurring` e onno package er scheduler, jemon `inventory.ScheduleExpire`: shop er timezone e 00:10, `dedupe_key` e shop ar shop er tarikh), tarpor `RunOnce`. 10 ta pele shathe shathe abar.
4. `RunOnce`: 6.4 er claim query (`FOR UPDATE SKIP LOCKED`, `attempts + 1`, `locked_at`). Protiti kaj er age lock notun kore (onno worker niye thakle bad), `call` (1 minute timeout; panic o error), tarpor `finish`.
5. `finish`: shofol hole `done_at` (`sms.send` hole payload `{}`, PII); fail hole `last_error`, `run_at = now + backoff` (1, 5, 15, 60 minute); `attempts = max_attempts` hole `failed_at` ar `alert`. `locked_at` claim er sathe na mille (5 minute por onno worker niyeche) kichu lekhe na.
6. `jobs.sweep`: shesh attempt e crash kora job (`attempts >= max_attempts`, lock 5 minute er purono) `failed_at` ar alert. `jobs.cleanup` (protidin): `done_at` 30 din ar `failed_at` 90 din er purono row muche (`dedupe_key` shoho). `cart.cleanup` (protidin): 30 din er purono guest cart muche ([cart](cart.md)). `inventory.expire` (protidin, `batch_expiry` on shop): expire batch ([inventory](inventory.md)).
7. `notify.NewSMS`: `APP_ENV=local` e log (`phone.Mask`, shesh 3 digit); onno env e `ErrNoProvider` (provider M12 er age, 16.1), tai production e kokhono log mode na. OTP er lekha `OTPText`: kram shop override[locale] → default[locale] → override[bn] → default[bn] (9.8), `{code}` chhara lekha bad; default bn/en Go te (`otpDefault`). Order SMS er lekha `OrderText` (`sms.order_placed`, `sms.order_confirm`, `sms.cancelled`; `{shop}`, `{number}`, `{link}`), ekoi kram; `{number}` ba (link thakle) `{link}` chhara override bad; default `orderDefault`. `sms.total_changed` er lekha `TotalText` (`{shop}`, `{number}`, `{total}`; `{number}` ba `{total}` chhara override bad); `{total}` `taka`: bn `৳1,23,456.50`, en `Tk 1,23,456.50` (GSM-7 e ৳ nai), lakh-style comma, English digit, poisha 0 hole doshomik na (8.16 v1.42).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/jobs/jobs.go` → `Enqueue`, `NewWorker`, `Run`, `RunOnce`, `release`, `finish`, `sweep`, `cleanup`, `backoff` | Queue ar worker |
| `backend/internal/notify/notify.go` → `NewSMS`, `SendSMSJob`, `OTP`, `OTPSource`, `OTPText`, `otpDefault`, `OrderText`, `TotalText`, `taka`, `orderDefault`, `SecurityAlertJob` | SMS sender, `sms.send` handler (OTP payload shoho), `alert.security` |
| `backend/cmd/api/main.go` → `runServe` | Worker chalu (handler register) ar shutdown |
| `backend/internal/order/jobs.go` → `JobHandlers` | Order er job ([checkout](checkout.md)) |
| `backend/internal/inventory/jobs.go` → `ScheduleExpire`, `ExpireJob` | `inventory.expire` ([inventory](inventory.md)) |
| `backend/internal/dbtest/dbtest.go` → `New` | Test er database |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Backoff 1, 5, 15, 60 minute (8.17) | `backoff` | `TestBackoff` |
| Ek kaj ek worker, fail e retry, `max_attempts` e `failed_at` ar alert (6.4) | `RunOnce`, `finish` | `TestWorkerIntegration` |
| Lock 5 minute er por onno worker nile purono worker er result bad (8.17) | `finish` | `TestWorkerIntegration` |
| Batch e chalanor age lock notun; churi hole job bad (SMS duibar na); handler 1 minute er shima | `RunOnce` | `TestBatchStolenLockIntegration` |
| `dedupe_key` e duibar na; panic = fail (8.17) | `Enqueue`, `call` | `TestWorkerIntegration` |
| Crash kora shesh attempt sweep e failed (6.4) | `sweep` | `TestWorkerIntegration` |
| Duita worker ek shathe: protiti job ekbar; 5 minute er purono lock abar neya jay (6.4) | `RunOnce` | `TestConcurrentClaimIntegration` |
| Shutdown ba DB error e batch er baki job chere dey, attempt ferot (8.17) | `RunOnce`, `release` | `TestShutdownReleasesIntegration` |
| `jobs.sweep` protiti 5 minute e ekbar (8.17) | `schedule` | `TestScheduleSweep` |
| `jobs.cleanup` dine ekbar; done 30 din, failed 90 din er purono row muche, `dedupe_key` abar khali (8.17 v1.19) | `schedule`, `cleanup` | `TestJobsCleanupIntegration` |
| `cart.cleanup` dine ekbar (UTC din er `dedupe_key`); handler `cart` package e (8.17 v1.28) | `schedule`, `cart.CleanupJob` | `TestJobsCleanupIntegration`, `TestCartCleanupIntegration` |
| `inventory.expire` shop er timezone e 00:10 dine ekbar, shudhu `batch_expiry` on shop (`Recurring`); idempotent, self-heal (8.17, 11.7 item 3) | `schedule`, `inventory.ScheduleExpire`, `inventory.ExpireJob` | `TestExpireBatchesIntegration` |
| `sms.otp` bn ≤ 70 unit (UCS-2), en ≤ 160 (GSM-7), `{code}`, override ar locale er kram (8.16, 9.8) | `OTPText` | `TestOTPTemplateLength` |
| Order SMS bn ≤ 134 unit (2 part), en ≤ 160; `{number}`/`{link}` chhara override bad (8.16, 8.19 item 7) | `OrderText` | `TestOrderSMSTemplateLength` |
| Order job: SMS retry e ekoi link, auto-cancel idempotent ([checkout](checkout.md); 8.17) | `order.JobHandlers` | `TestOrderPlacedSMSIntegration`, `TestAutoCancelUnconfirmedIntegration` |
| SMS shudhu local e log, production e log mode na (8.2 step 4); log e phone mask (12.2) | `NewSMS` | `TestNewSMS` |
| `sms.total_changed` bn ≤ 134 unit, en ≤ 160 GSM-7, `{total}` format, override er kram (8.16 v1.42) | `TotalText`, `taka` | `TestTotalChangedSMSLength` |
| `sms.send` payload e phone ar text lage; OTP payload (`{otp_id, nonce, ct}`) e `OTPSource` theke phone ar code (8.2 step 4) | `SendSMSJob` | `TestSendSMSJob` |
| OTP `sms.send`: `max_attempts` 2 (1 minute por ekbar), done hole payload `{}` (8.17 v1.19) | `auth.sendOTP`, `finish` | `TestOTPJobPayload` |

## Data

`jobs` (`001_init.sql`, 6.3): `kind`, `payload`, `dedupe_key`, `run_at`, `attempts`, `max_attempts`, `locked_at`, `done_at`, `failed_at`, `last_error`. Index `jobs_due_idx`.

## Bhul hole ki hoy

- Ochena `kind`: error, retry, shesh e `failed_at` ar alert.
- Database e jete na parle: `job worker` error log, 2 second por abar.
- Local chhara env e `sms.send`: `ErrNoProvider` (retry, shesh e failed ar alert); provider M12 er age (16.1).
- Admin alert ekhon shudhu `level=ERROR` log `admin alert: job failed`; email/webhook (`ALERT_EMAIL`, `ALERT_WEBHOOK_URL`, 13.5) M12 e.

## Bodlanor age mone rakho

- Handler idempotent rakho: lock 5 minute er beshi hole ekoi kaj abar chole.
- Handler e HTTP call thakle ctx mano; shutdown e ctx bondho hoy.
- Notun recurring job: `schedule` e slot er `dedupe_key` (din ba ghonta, shop shoho). `dedupe_key` puro table e UNIQUE, tai shop er job er key e shop_id rakho, na hole ek shop er job onno shop er ta atkay.
- `last_error` ar alert log e handler er error puro jay: provider er error e body, URL, phone ba OTP rakhbe na, shudhu status ar provider er code (12.2).
- OTP er `sms.send` `max_attempts` 2: `backoff(1)` = 1 minute, tai alada retry niyom nai; backoff bodlale OTP o bodlay.
- `otpDefault` bodlale `TestOTPTemplateLength` (bn 70 unit) dekho: juktakkhor ar kar-fola alada unit (8.16). Shop er override er length check nai (admin UI M7 e).
- `jobs.cleanup` er por purono `dedupe_key` abar khali; recurring key e tarikh thake bole purono slot abar ashe na.
- `alert.security` ekhon shudhu `level=WARN` log `security alert`, `alert.new_order` `level=INFO` log `new order`; email/webhook M12 e.
- `orderDefault` bodlale `TestOrderSMSTemplateLength` (bn 134 unit, lomba shop/domain diye) dekho.

## Proshno-uttor

- **Integration test kivabe chalabo?** `make db-up`, tarpor `cd backend && TEST_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:5433/postgres?sslmode=disable go test ./internal/jobs/`.
- **Shop er `sms.otp` override e `{code}` na thakle?** Override bad, default lekha (`OTPText`); na hole SMS e code-i jay na.
- **Customer er account na thakle OTP SMS kon bhashay?** Bangla (`users` e row nai; prothom login e notun user er locale `Accept-Language` theke, [auth](auth.md)).
