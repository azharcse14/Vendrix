# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **M0 shesh** (12 November 2026): CI green (run 43151580056), toolchain go1.27.4 (v1.15). **M3 (Website catalog) shesh** (15 November 2026): Lighthouse 16.1 (a) (v1.18); report [M3](milestones/M3.md). Mac e `make check` er `TestSearchRankingIntegration` macOS `C.UTF-8` er karone fail (Linux e green; M3 Report). **M4 (Login) shesh** (15 November 2026, design v1.19–v1.27): customer OTP login + session, `/api/me`, staff login + TOTP/recovery + setup, admin session, `staff reset-2fa` CLI, website `/login` + `/account`, local https (`make https`); report [M4](milestones/M4.md). **M5 (Cart ar wishlist) cholche** (plan approve 15 November 2026: shob recommendation, design v1.28; step 0 shesh). Push baki: `339de22`..HEAD (user: `gh auth switch -u azharcse14`, `git push origin main`).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 85/85 (Postgres 16, C.UTF-8).
- M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M5 step 1, key ar quantity niyom: test age** ([M5](milestones/M5.md), 8.4 v1.28 item 1 ar 3–4; Q1, Q4, Q7): notun package `backend/internal/cart` (`doc.go` e `Feature doc: docs/features/cart.md`), `cart_test.go` e `TestCartKeyFormat` (`rand.Text()` 26 okkhor `^[A-Z2-7]{26}$`, bhul/khali key "nai", DB te `sha256`) ar quantity/cap er pure function test (1..`min(stock, 50)`, 0/minus `validation_failed`, beshi hole `out_of_stock` + `max_quantity`, normalize e qty > stock → `min(stock, 50)`, 50 variant e `cart_full`). Age test chaliye fail dekho, commit `test(M5): cart key ar quantity niyom`; tarpor code, commit `feat(M5): cart key ar quantity niyom`; `make quick-check`. Tarpor step 2 (`milestone` skill).
2. **User: push ar CI**: `git push origin main` (`339de22`..HEAD; `gh auth switch -u azharcse14`), tarpor CI green kina dekho (Go job, `ci(M3): website build`, Dart job); Linux e `TestSearchRankingIntegration` (`backend/internal/catalog`) green howa uchit (Mac e macOS `C.UTF-8` e fail, M3/M4 Report). Local browser login er jonno `backend/.env` e `OTP_SECRET`, `TURNSTILE_SECRET`, `TOTP_KEY`, `RECOVERY_SECRET` ar `website/.env` e `TURNSTILE_SITE_KEY` (`env.example` dekho).

## User er kache proshno

- **M12 er age**: client JS er `?v=<git sha>` (10.10 item 10): Jaspr nijei script tag dey, hook nai; recommendation deploy e Cloudflare purge (16.1).
- **Ekhon-i (user er kaj)**: `.claude/settings.json` e `"GOTOOLCHAIN": "go1.27.4"` (hook e shudhu user bodlay); na hole Claude Code er Go command 1.27.3 (GO-2026-6859) e chole.
- **M7 er age**: COD e packing e beshi ojon er shima; packed fulfillment undo (16.1).
- **M10 er age**: SSLCommerz validation er HTTP call DB lock er bhetore naki baire (16.1).
- **M12 er age**: OTP SMS cap shesh hole oi din OTP login bondho (bot DoS): (a) ekhon er moto + monitor/alert, (b) cap er kache extra limit, (c) cap shudhu alert (16.1, v1.26).
- **M12 er age**: production domain (doc e `tomarshop.com` placeholder), server region, SMS provider (16.1).
- **Public launch er age**: IP-bhittik rate limit ar CGNAT; shathe ochena `X-Shop-Host` er rate limit (M1 review) (16.1).
- **M7 er age**: `vendor_owner` admin category list porte parbe kina (11.2, 16.1).

## Milestone

| # | Naam | Obostha | Plan |
|---|---|---|---|
| M0 | Project setup | shesh (12 November 2026) | [M0](milestones/M0.md) |
| M1 | Shop, vendor, settings, catalog API | shesh (10 November 2026) | [M1](milestones/M1.md) |
| M2 | Shared Dart package | shesh (10 November 2026) | [M2](milestones/M2.md) |
| M3 | Website catalog | shesh (15 November 2026) | [M3](milestones/M3.md) |
| M4 | Login | shesh (15 November 2026) | [M4](milestones/M4.md) |
| M5 | Cart ar wishlist | cholche (15 November 2026 theke) | [M5](milestones/M5.md) |
| M6 | Checkout (COD) | baki | |
| M7 | Admin panel v1 | baki | |
| M8 | Store settings | baki | |
| M9 | Review | baki | |
| M10 | SSLCommerz | baki | |
| M11 | Taka, support, compliance | baki | |
| M12 | Ops ar launch | baki | |
| MV | Marketplace (Phase 3) | baki | |
| MT | Multi-tenant (Phase 3) | baki | |

## Session log

- 15 November 2026: M5 plan approve (user: shob recommendation, Q1–Q11), design v1.28 (3.2, 7.1, 7.4, 7.8, 8.2, 8.4, 8.17, 10.1–10.3, 10.5, 10.9, 10.10, notun 10.12, 12.2); M5 step 0.
- 15 November 2026: M5 plan draft `docs/milestones/M5.md` (scope, Already ache, acceptance → test, 11 ta Open question recommendation shoho, step 0–11); user approve baki, code na.
- 15 November 2026: **M4 shesh**: step 15 `make check` (gofmt, vet, docsync, rule green; DB 85/85; shared 78, website 123; `-race` e shudhu jana `TestSearchRankingIntegration` macOS locale), `dart format` fix `7b90d9b` (`account.dart`), acceptance ☑, [M4](milestones/M4.md) Report. Porer kaj: push + CI, tarpor M5 plan.
- 15 November 2026: 16.1 local `__Host-` cookie: user (a), design v1.27 `bf33e68` (13.1, 13.3); `make https` (Caddy `tls internal`, `infra/caddy/Caddyfile.local`, `scripts/https.sh`); Chromium e `https://sobujbazar.localhost:8443` OTP login → `/account` e `__Host-sid` thake. User: `brew install caddy`, ekbar `caddy trust`.
- 15 November 2026: M4 step 15 design review baki finding: design v1.26 `bb57e10` (error code, response, rate limit note, audit action, 16.1 SMS cap DoS), test `7f765fe`, fix `4bc4b88` (OTP lock alert key HMAC, account phone placeholder), feature doc `868f489`; M4.md test 23 ☑.
- 15 November 2026: M4 step 15 TOTP pending (user decision, security-reviewer M2): design v1.25 `d35a877`, test `aea4dcb` (fail dekhe), fix `2cf0ede` (setup er secret `totp_pending_*` e, enable e live; TOTP live + `must_change_password` e password bodlale session `active`); `make db-test` 85/85, `-race` green. `TestStaffLockRaceIntegration` er flake: shombhoboto Mac sleep (wall clock 5 minute er beshi agay), code race na; 22 bar `-race` pass.
- 15 November 2026: M4 step 15 review fix: design v1.24 `8d61dfe` (staff lock atomic reservation, account lock e `401` user decision), test `b7d95fa` (fail dekhe), fix `d3b2c72` (`reserveFailed`, purono temp_token revoke, `/auth/password` gona, audit e `email_domain`, auth `no-store`, totp/recovery IP limit, alert `dedupe_key` din); asol Postgres e `-race` green. Step 15 er baki: `make check`, acceptance, report.
- 15 November 2026: M4 step 14: feature doc M4 er code er sathe milano (`auth.md` code map/Proshno-uttor, `website.md`, `server.md`, `http-foundation.md`, `jobs.md`, `shop.md`, `credentials.md`, `catalog.md`, `audit.md`, `api-client.md`, `README.md`); naam ar test grep e mile; docsync green.
- 15 November 2026: M4 step 13 manual proman: customer (Chromium 131, local https proxy) phone → Turnstile test key → log er OTP → `/account` (naam, bhasha) → logout, `__Host-sid` flag, CSP, console error 0, 360/1280 screenshot; staff (curl, shop `qa13`) password → TOTP → recovery → `/api/admin/me` → logout, `reset-2fa` er por abar setup. Bug nai; finding: http e `__Host-` cookie `InvalidPrefix` (16.1 v1.23).
- 15 November 2026: M4 step 12: test `ed916bd` (`login_test.dart`, Turnstile CSP, `client_components_test` e `login_form.dart`, account form `POST` → `PATCH`, `TURNSTILE_SITE_KEY`, shared `Account`; fail dekhe), code `1697328` (shared `User`/`AccountRepository.me()`/`ApiClient(token:)`, l10n `login.*`/`account.*`/M4 `error.*`; `/login` + `@client LoginForm`, `/account`, Account link, `/login` e Turnstile CSP, `BffRoute.apiMethod`); `jaspr build` `main.client.dart.js` 172,552 byte (login part 96,486 alada); `make quick-check` green.
- 15 November 2026: M4 step 11: test `588ea32` (`bff_auth_test.dart`, `cache_control_test.dart` e BFF case; fail dekhe), code `8a216dd` (BFF `/_/auth/otp/send`, `verify` (`__Host-sid` set, token body theke muche), `logout` (`Max-Age=0`), `PATCH /_/account` → `/api/me`; `Retry-After`; `private, no-store`); `make quick-check` green.
- 15 November 2026: M4 step 10: test `69ddbc5` (`TestStaffReset2FAIntegration`, `TestRun` er usage; fail dekhe), code `7d319e7` (`shop-api staff reset-2fa`, `auth.ResetStaff2FA`: TOTP/recovery muche, session revoke, audit `staff.reset_2fa` actor CLI); asol Postgres e `-race` pass.
- 15 November 2026: M4 step 9: test `39d824e` (`TestOTPTemplateLength`, `TestJobsCleanupIntegration`; fail dekhe), code `5986261` (`notify.OTPText` bn/en + shop override + customer locale, `auth.otpSMS`, `jobs.cleanup` protidin); asol Postgres e `-race` pass.
- 15 November 2026: M4 step 7: test `e3883ec` (`TestStaffSetupIntegration`, `TestSessionRouteKind`, `TestAdminCookie`; build fail dekhe), code `83ca2d9` (`httpx.AdminSession`, setup_pending gate, `/auth/password`, `/auth/totp/setup`, `/auth/totp/enable`, `/auth/logout`, `GET /api/admin/me`); asol Postgres e `-race` pass.
- 15 November 2026: M4 step 6: test `58e90b3` (`TestStaffLoginIntegration`, `TestPBKDFSemaphore`, `TestLoginIPLimit`, `TestMFAAttemptsIntegration`, `TestTOTPReplayIntegration`, `TestFailedMFAAcrossTokensIntegration`, `TestRecoveryCodeIntegration`; build fail dekhe), code `6f7a86d` (`/api/admin/auth/login`, `/totp`, `/recovery`; `httpx.FailLimit`, `httpx.AdminCookie`); asol Postgres e `-race` pass.
