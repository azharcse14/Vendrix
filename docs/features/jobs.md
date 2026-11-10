# Background job ar SMS

Design: 6.4, 8.2, 8.17, 12.2, 13.4 · Milestone: M0 · Code: `backend/internal/jobs`, `backend/internal/notify`

## Ki kore

Request er baire je kaj (SMS, alert, auto-cancel, reconcile) `jobs` table e queue hoy; `shop-api serve` er bhetore ekta worker goroutine kaj nay ar chalay. Fail hole pore abar chesta, bar bar fail hole thame ar admin alert. Local e SMS pathano hoy na, log e lekha hoy.

## Kivabe kaj kore

1. Kaj jog: `jobs.Enqueue` (`*sql.DB` ba order er `*sql.Tx`). `DedupeKey` thakle ekoi key e duibar row hoy na.
2. `serve` (`cmd/api/main.go` → `runServe`): `jobs.NewWorker` e kind → handler (`sms.send` → `notify.SendSMSJob`), `Worker.Run` goroutine e; shutdown e worker thame, cholche emon kaj er result lekha hoy, tarpor database close.
3. `Worker.Run`: protiti 2 second e `schedule` (recurring `jobs.sweep`, 5 minute er slot `dedupe_key`), tarpor `RunOnce`. 10 ta pele shathe shathe abar.
4. `RunOnce`: 6.4 er claim query (`FOR UPDATE SKIP LOCKED`, `attempts + 1`, `locked_at`), protiti kaj `call` (panic o error), tarpor `finish`.
5. `finish`: shofol hole `done_at`; fail hole `last_error`, `run_at = now + backoff` (1, 5, 15, 60 minute); `attempts = max_attempts` hole `failed_at` ar `alert`. `locked_at` claim er sathe na mille (5 minute por onno worker niyeche) kichu lekhe na.
6. `jobs.sweep`: shesh attempt e crash kora job (`attempts >= max_attempts`, lock 5 minute er purono) `failed_at` ar alert.
7. `notify.NewSMS`: `APP_ENV=local` e log (phone mask, shesh 3 digit); onno env e `ErrNoProvider` (provider M4 e), tai production e kokhono log mode na.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/jobs/jobs.go` → `Enqueue`, `NewWorker`, `Run`, `RunOnce`, `finish`, `sweep`, `backoff` | Queue ar worker |
| `backend/internal/notify/notify.go` → `NewSMS`, `SendSMSJob`, `mask` | SMS sender ar `sms.send` handler |
| `backend/cmd/api/main.go` → `runServe` | Worker chalu ar shutdown |
| `backend/internal/dbtest/dbtest.go` → `New` | Test er database |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Backoff 1, 5, 15, 60 minute (8.17) | `backoff` | `TestBackoff` |
| Ek kaj ek worker, fail e retry, `max_attempts` e `failed_at` ar alert (6.4) | `RunOnce`, `finish` | `TestWorkerIntegration` |
| Lock 5 minute er por onno worker nile purono worker er result bad (8.17) | `finish` | `TestWorkerIntegration` |
| `dedupe_key` e duibar na; panic = fail (8.17) | `Enqueue`, `call` | `TestWorkerIntegration` |
| Crash kora shesh attempt sweep e failed (6.4) | `sweep` | `TestWorkerIntegration` |
| `jobs.sweep` protiti 5 minute e ekbar (8.17) | `schedule` | `TestScheduleSweep` |
| SMS shudhu local e log, production e log mode na (8.2 step 4); log e phone mask (12.2) | `NewSMS` | `TestNewSMS` |
| `sms.send` payload e phone ar text lage | `SendSMSJob` | `TestSendSMSJob` |

## Data

`jobs` (`001_init.sql`, 6.3): `kind`, `payload`, `dedupe_key`, `run_at`, `attempts`, `max_attempts`, `locked_at`, `done_at`, `failed_at`, `last_error`. Index `jobs_due_idx`.

## Bhul hole ki hoy

- Ochena `kind`: error, retry, shesh e `failed_at` ar alert.
- Database e jete na parle: `job worker` error log, 2 second por abar.
- Local chhara env e `sms.send`: `ErrNoProvider` (M4 e provider ashle thik).
- Admin alert ekhon shudhu `level=ERROR` log `admin alert: job failed`; email/webhook (`ALERT_EMAIL`, `ALERT_WEBHOOK_URL`, 13.5) M12 e.

## Bodlanor age mone rakho

- Handler idempotent rakho: lock 5 minute er beshi hole ekoi kaj abar chole.
- Handler e HTTP call thakle ctx mano; shutdown e ctx bondho hoy.
- Notun recurring job: `schedule` e slot er `dedupe_key` (din ba ghonta, shop shoho).
- OTP er `sms.send` er alada retry (1 minute por ekbar, 8.17) M4 e.

## Proshno-uttor

- **Integration test kivabe chalabo?** `make db-up`, tarpor `cd backend && TEST_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:5433/postgres?sslmode=disable go test ./internal/jobs/`.
