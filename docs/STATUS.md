# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **M0 shesh** (12 November 2026): CI green (run 43151580056), toolchain go1.27.4 (v1.15). **M3 (Website catalog) shesh** (15 November 2026): Lighthouse 16.1 (a) (v1.18); report [M3](milestones/M3.md). Mac e `make check` er `TestSearchRankingIntegration` macOS `C.UTF-8` er karone fail (Linux e green; M3 Report). **M4 (Login) cholche** (plan approve 15 November 2026, design v1.22; step 0-13 shesh: OTP send/verify, customer session, `/api/me`, staff login + TOTP/recovery, setup + admin session, `sms.otp` template, `jobs.cleanup`, `staff reset-2fa` CLI, BFF auth route, website `/login` + `/account`, manual proman). Push baki: `339de22`..HEAD (user: `gh auth switch -u azharcse14`, `git push origin main`).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 83/83 (Postgres 16, C.UTF-8).
- M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M4 step 14, Feature doc** ([M4](milestones/M4.md)): `docs/features/auth.md`, `website.md` ar baki M4 doc code er sathe mile kina dekho (step 13 er finding: local http e `__Host-` cookie, 16.1 v1.23); `make quick-check`. Commit `docs(M4): feature doc`. Tarpor step 15 (`make check`, `security-reviewer`, `design-reviewer`, Report).
2. Push er por CI green kina dekho (`ci(M3): website build` ar Go job; Linux e `TestSearchRankingIntegration` green howa uchit).

## User er kache proshno

- **M7 er age**: local http e `__Host-sid`/`__Host-asid` Chromium e thake na (https lage), tai `make web` e browser login hoy na; (a) local https (Caddy `tls internal`) recommendation, (b) local e prefix chhara naam, (c) kichu na (16.1, v1.23).
- **M12 er age**: client JS er `?v=<git sha>` (10.10 item 10): Jaspr nijei script tag dey, hook nai; recommendation deploy e Cloudflare purge (16.1).
- **Ekhon-i (user er kaj)**: `.claude/settings.json` e `"GOTOOLCHAIN": "go1.27.4"` (hook e shudhu user bodlay); na hole Claude Code er Go command 1.27.3 (GO-2026-6859) e chole.
- **M7 er age**: COD e packing e beshi ojon er shima; packed fulfillment undo (16.1).
- **M10 er age**: SSLCommerz validation er HTTP call DB lock er bhetore naki baire (16.1).
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
| M4 | Login | cholche (15 November 2026 theke) | [M4](milestones/M4.md) |
| M5 | Cart ar wishlist | baki | |
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

- 15 November 2026: M4 step 13 manual proman: customer (Chromium 131, local https proxy) phone → Turnstile test key → log er OTP → `/account` (naam, bhasha) → logout, `__Host-sid` flag, CSP, console error 0, 360/1280 screenshot; staff (curl, shop `qa13`) password → TOTP → recovery → `/api/admin/me` → logout, `reset-2fa` er por abar setup. Bug nai; finding: http e `__Host-` cookie `InvalidPrefix` (16.1 v1.23).
- 15 November 2026: M4 step 12: test `ed916bd` (`login_test.dart`, Turnstile CSP, `client_components_test` e `login_form.dart`, account form `POST` → `PATCH`, `TURNSTILE_SITE_KEY`, shared `Account`; fail dekhe), code `1697328` (shared `User`/`AccountRepository.me()`/`ApiClient(token:)`, l10n `login.*`/`account.*`/M4 `error.*`; `/login` + `@client LoginForm`, `/account`, Account link, `/login` e Turnstile CSP, `BffRoute.apiMethod`); `jaspr build` `main.client.dart.js` 172,552 byte (login part 96,486 alada); `make quick-check` green.
- 15 November 2026: M4 step 11: test `588ea32` (`bff_auth_test.dart`, `cache_control_test.dart` e BFF case; fail dekhe), code `8a216dd` (BFF `/_/auth/otp/send`, `verify` (`__Host-sid` set, token body theke muche), `logout` (`Max-Age=0`), `PATCH /_/account` → `/api/me`; `Retry-After`; `private, no-store`); `make quick-check` green.
- 15 November 2026: M4 step 10: test `69ddbc5` (`TestStaffReset2FAIntegration`, `TestRun` er usage; fail dekhe), code `7d319e7` (`shop-api staff reset-2fa`, `auth.ResetStaff2FA`: TOTP/recovery muche, session revoke, audit `staff.reset_2fa` actor CLI); asol Postgres e `-race` pass.
- 15 November 2026: M4 step 9: test `39d824e` (`TestOTPTemplateLength`, `TestJobsCleanupIntegration`; fail dekhe), code `5986261` (`notify.OTPText` bn/en + shop override + customer locale, `auth.otpSMS`, `jobs.cleanup` protidin); asol Postgres e `-race` pass.
- 15 November 2026: M4 step 7: test `e3883ec` (`TestStaffSetupIntegration`, `TestSessionRouteKind`, `TestAdminCookie`; build fail dekhe), code `83ca2d9` (`httpx.AdminSession`, setup_pending gate, `/auth/password`, `/auth/totp/setup`, `/auth/totp/enable`, `/auth/logout`, `GET /api/admin/me`); asol Postgres e `-race` pass.
- 15 November 2026: M4 step 6: test `58e90b3` (`TestStaffLoginIntegration`, `TestPBKDFSemaphore`, `TestLoginIPLimit`, `TestMFAAttemptsIntegration`, `TestTOTPReplayIntegration`, `TestFailedMFAAcrossTokensIntegration`, `TestRecoveryCodeIntegration`; build fail dekhe), code `6f7a86d` (`/api/admin/auth/login`, `/totp`, `/recovery`; `httpx.FailLimit`, `httpx.AdminCookie`); asol Postgres e `-race` pass.
- 15 November 2026: 16.1 Turnstile test secret er hostname: user (a), design v1.22 `c559ca6` (10.11, 13.5); test `1f86d07`, code `9b03b5b` (`config.TurnstileTestSecret`, test secret e `Turnstile.Verify` hostname check na).
- 15 November 2026: M4 step 5: test `67aa30c` (OTP attempt/expire/parallel verify, guest order jog, customer session, shop isolation; build fail dekhe), code `12aad8b` (`POST /api/auth/otp/verify`, `RequireCustomer` + `LookupSession`, logout, `GET`/`PATCH /api/me`); asol Postgres e `-race` pass.
- 15 November 2026: M4 step 4 (ar 8): test `3681ea1`, code `453dae4` (`POST /api/auth/otp/send`: Turnstile, phone limit/lock, SMS cap, payload AES-GCM, IP limit); asol Postgres e pass; 16.1 e Turnstile test secret er hostname proshno (v1.21).
- 15 November 2026: M4 step 3: AES-GCM helper `auth.Seal`/`Open` e `08023df`, test `3384d7a` (`TestTOTP`, `TestTOTPSecretAAD`, `TestRecoveryCode`), code `d8ea20c` (`totp.go`, `recovery.go`).
- 15 November 2026: M4 step 2: OTP primitive: test `8c3cfaf` (`TestOTPCode`, `TestOTPHash`), code `b9f6ec2` (`NewOTPCode`, `HashOTP`, `CheckOTP`), `docs/features/auth.md` notun.
- 15 November 2026: M4 step 1: design v1.20 (Turnstile test secret local chhara config error, user decision; 13.5, 16.1) `898b41b`, test `ff652ec`, config e `OTP_SECRET`/`TURNSTILE_SECRET`/`TOTP_KEY`/`RECOVERY_SECRET` `58252c7`.
- 15 November 2026: M4 plan approve (shob recommendation), design v1.19; M4 step 0.
- 15 November 2026: M3 shesh: Lighthouse row ☑, step 16, Report; `bd32992`/`11df782` choto review (finding nai); `make check`: quick-check, DB 83/83 green, race e shudhu `TestSearchRankingIntegration` (macOS `C.UTF-8`, Linux e green).
