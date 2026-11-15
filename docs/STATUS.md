# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **M0 shesh** (12 November 2026): CI green (run 43151580056), toolchain go1.27.4 (v1.15). **M3 (Website catalog) shesh** (15 November 2026): Lighthouse 16.1 (a) (v1.18); report [M3](milestones/M3.md). Mac e `make check` er `TestSearchRankingIntegration` macOS `C.UTF-8` er karone fail (Linux e green; M3 Report). **M4 (Login) cholche** (plan approve 15 November 2026, design v1.21; step 0-4 ar 8 shesh: `POST /api/auth/otp/send`). Push baki: `339de22`..HEAD (user: `gh auth switch -u azharcse14`, `git push origin main`).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 83/83 (Postgres 16, C.UTF-8).
- M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M4 step 5, OTP verify ar customer session** ([M4](milestones/M4.md), 8.2 verify step 1-7, 3.2, Rule 16, 7.3): test age `backend/internal/auth/otp_integration_test.go` (package `auth_test`, `newOTPFixture` reuse) e `TestOTPAttemptsIntegration` (5 bhul er por `otp_too_many_attempts`, error path e attempt commit, parallel e 5 er beshi na; 24 ghontay 10 bhul e lock ar `alert.security` ekbar, `lockAlert` reuse), `TestOTPExpiredIntegration`, `TestOTPParallelVerifyIntegration` (ekoi code e duita verify → ekta session), `TestGuestOrdersLinkIntegration` (SQL row diye), `session_integration_test.go` e `TestCustomerSessionIntegration` (logout revoke, expire/revoke `401`, `last_used_at` ghontay ekbar) ar `TestSessionShopIsolationIntegration`; fail dekhe commit `test(M4): OTP verify ar session`. Tarpor `POST /api/auth/otp/verify` (`{phone, code}` → `{token, user}`, Turnstile lage na), `users` upsert (`locale` = `Accept-Language`), guest order jog, `session.go` (`rand.Text()`, `sha256`, 30 din, `ip` = `httpx.IP`), `httpx/session.go` `RequireCustomer`, `POST /api/auth/logout`, `GET`/`PATCH /api/me`; commit `feat(M4): OTP verify, session, /api/me` (`docs/features/auth.md`, `http-foundation.md` shoho). Integration: `TEST_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:5433/postgres?sslmode=disable go test -race ./internal/auth ./cmd/api`. `make run` e local `backend/.env` e `env.example` er 4 line (`OTP_SECRET`, `RECOVERY_SECRET`, `TOTP_KEY`, `TURNSTILE_SECRET`) lagbe.
2. Push er por CI green kina dekho (`ci(M3): website build` ar Go job; Linux e `TestSearchRankingIntegration` green howa uchit).

## User er kache proshno

- **M4 step 12 er age**: Cloudflare Turnstile er test secret siteverify e `hostname` `example.com` dey, tai 10.11 er hostname check local login e fail kore (16.1, v1.21). Recommendation (a): test secret hole (shudhu local e dewa jay, v1.20) hostname check bad.
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

- 15 November 2026: M4 step 4 (ar 8): test `3681ea1`, code `453dae4` (`POST /api/auth/otp/send`: Turnstile, phone limit/lock, SMS cap, payload AES-GCM, IP limit); asol Postgres e pass; 16.1 e Turnstile test secret er hostname proshno (v1.21).
- 15 November 2026: M4 step 3: AES-GCM helper `auth.Seal`/`Open` e `08023df`, test `3384d7a` (`TestTOTP`, `TestTOTPSecretAAD`, `TestRecoveryCode`), code `d8ea20c` (`totp.go`, `recovery.go`).
- 15 November 2026: M4 step 2: OTP primitive: test `8c3cfaf` (`TestOTPCode`, `TestOTPHash`), code `b9f6ec2` (`NewOTPCode`, `HashOTP`, `CheckOTP`), `docs/features/auth.md` notun.
- 15 November 2026: M4 step 1: design v1.20 (Turnstile test secret local chhara config error, user decision; 13.5, 16.1) `898b41b`, test `ff652ec`, config e `OTP_SECRET`/`TURNSTILE_SECRET`/`TOTP_KEY`/`RECOVERY_SECRET` `58252c7`.
- 15 November 2026: M4 plan approve (shob recommendation), design v1.19; M4 step 0.
- 15 November 2026: M3 shesh: Lighthouse row ☑, step 16, Report; `bd32992`/`11df782` choto review (finding nai); `make check`: quick-check, DB 83/83 green, race e shudhu `TestSearchRankingIntegration` (macOS `C.UTF-8`, Linux e green).
- 15 November 2026: 16.1 M3 Lighthouse LCP er uttor (a) (user): devtools throttle er number acceptance, simulate Report e, font kom + Lighthouse CI M12 er age (10.5, 15); design v1.18.
- 15 November 2026: M4 plan draft (`docs/milestones/M4.md`): scope, acceptance → test, 14.2 test 10/18/19, 15 step, 10 ta open question (recommendation shoho); code na.
- 15 November 2026: M3 step 15: 30 ta responsive page 200, tap target 44px; bug fix `bd32992` (`<base>`/CSP e hydration), `11df782` (favicon); Lighthouse simulate LCP 2.3/2.7/2.7s, devtools 1.3/1.7/1.6s (16.1 e proshno); Facebook Q12 (b).
- 12 November 2026: M3 step 16: `make check` green; design/security review e high nai; finding fix (BFF Origin null/scheme, open redirect, body limit, host normalize, sitemap, renderPage test); website 93/93. `?v=` 16.1 e.
- 12 November 2026: M3 step 14: `l10n.md`, `theme.md`, `api-client.md` e website er ongsho.
- 12 November 2026: M3 step 13: CI Dart job e website `jaspr build` + JS size (local 141 KB, gzip 45 KB).
- 12 November 2026: M3 step 12: security header ar CSP middleware, `@client` list test; website 85/85.
- 12 November 2026: M3 step 11: SEO (`SeoHead`, JSON-LD escape, sitemap/robots, hreflang, `noindex`); test age (`c75c2b7`); website 83/83, shared 76/76.
- 12 November 2026: M3 step 10: `GET /api/policies/{kind}` (Go test, Postgres e integration), search (`noindex`), policy page, footer link, `ProductListing`; website test 74/74, shared 75/75.
