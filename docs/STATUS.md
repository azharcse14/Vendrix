# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **Milestone M0 (Project setup): cholche.** Plan ar progress: [M0](milestones/M0.md).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 83/83 (Postgres 16, C.UTF-8).
- M0 er code shesh; baki shudhu push er por CI green (step 6). M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M0 step 6, CI**: user GitHub e push korle `ci` workflow er result (`gh run list --limit 3`); red hole log dekhe thik koro. `database` job e ekhon Go DB test o (`TEST_DATABASE_URL`).
2. **M3 plan approve** ([M3](milestones/M3.md)): plan likha; 13 ta open question er uttor user er kache. Uttor pele step 0 (design-change v1.13), tarpor step 1 (`jaspr install-skills --agent claude-code`, skeleton).

## User er kache proshno

- **M3 er age (ekhon)**: [M3](milestones/M3.md) er 13 ta open question (Jaspr/Tailwind setup, policy page er lekha, description format, chobi URL, settings cache, BFF route, cart button, CSP, test, CI build, install-skills commit, Facebook debugger tunnel, choto behavior).
- **M4 er age**: OTP SMS er code `jobs.payload` e plain text thake; AES-GCM naki job chhara pathano (16.1).
- **M4 er age**: `jobs` er purono row: `jobs.cleanup` (30 din) naki done hole payload khali (16.1).
- **M7 er age**: COD e packing e beshi ojon er shima; packed fulfillment undo (16.1).
- **M10 er age**: SSLCommerz validation er HTTP call DB lock er bhetore naki baire (16.1).
- **M12 er age**: production domain (doc e `tomarshop.com` placeholder), server region, SMS provider (16.1).
- **Public launch er age**: IP-bhittik rate limit ar CGNAT; shathe ochena `X-Shop-Host` er rate limit (M1 review) (16.1).
- **M7 er age**: `vendor_owner` admin category list porte parbe kina (11.2, 16.1).

## Milestone

| # | Naam | Obostha | Plan |
|---|---|---|---|
| M0 | Project setup | cholche (10 November 2026 theke) | [M0](milestones/M0.md) |
| M1 | Shop, vendor, settings, catalog API | shesh (10 November 2026) | [M1](milestones/M1.md) |
| M2 | Shared Dart package | shesh (10 November 2026) | [M2](milestones/M2.md) |
| M3 | Website catalog | baki | |
| M4 | Login | baki | |
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

- 10 November 2026: M3 plan likha (13 ta open question), approve er opekkhay.
- 10 November 2026: M2 step 10: `make check` green, design review fix, acceptance ☑, report; M2 shesh.
- 10 November 2026: M2 step 1–9: `packages/shared` er shob part, CI Dart job, local API te dekha.
- 10 November 2026: M2 plan approve (shob recommendation), design v1.12.
- 10 November 2026: M2 plan likha (9 ta open question), approve er opekkhay.
- 10 November 2026: M1 step 20: `make check` green, design/security review fix, acceptance ☑, report; M1 shesh.
- 10 November 2026: M1 step 9–19: catalog (public, search, admin category/product/variant), audit, router; design v1.11 (`sku_taken`); local curl; step 20 er check ar review cholche.
- 10 November 2026: M1 step 8b: `GET /api/translations` (shop er override).
- 10 November 2026: M1 step 8: public settings (whitelist), settings version; `shop` e handler/store.
- 10 November 2026: M1 step 7: `shop.DefaultSettings`, `shop.Create`, `shop-api shop create`; local e sobujbazar.localhost banano.
- 10 November 2026: M1 step 6: `internal/auth` password hash (pbkdf2, 8.3).
- 10 November 2026: M1 step 5: `httpx.RateLimit` (7.8, `shop_id:ip`, minute e 120).
- 10 November 2026: M1 step 4: `httpx.Staff`, `RequireStaff` (fail-closed 401), `RequireRole` (403).
- 10 November 2026: M1 step 3: `httpx.ShopFromHost` ar `shop.Resolver` (60 second cache, shudhu pawa domain); feature doc `shop.md`.
- 10 November 2026: M1 step 2: error message `{bn, en}` map (`httpx/message.go`), `WriteError(w, r, ...)`.
- 10 November 2026: M1 step 1: `httpx.JSONErrors` (404/405 JSON), `Page`, `NewPaged`.
- 10 November 2026: M1 plan approve (user 12 ta recommendation niyeche); design v1.10 (3.2, 3.8, 7.1, 7.2, 7.7, 9.2, 15).
- 10 November 2026: M0 review (design + security): batch e churi kora lock er job bad, handler 1 minute timeout, shutdown e baki job ferot, `run_at` DB er `now()`, concurrency test, env.example `local-only` secret local chhara reject (design v1.9); `jobs` cleanup 16.1 e.
- 10 November 2026: M0 step 6 (local ongsho): CI `database` job e Go DB test, `check.sh` e DB shoho race, PG env default 5433; `make check` green (83/83), govulncheck, staticcheck clean.
- 10 November 2026: M0 step 5: `shop-api credentials set` (config `CREDENTIALS_KEY`, `internal/shop` AES-GCM + upsert); test ar asol CLI e dekha.
- 10 November 2026: M0 step 4: `internal/jobs` (6.4 claim, backoff, failed_at + log alert, stale lock guard, `jobs.sweep` 5 minute slot), `internal/notify` (local log, phone mask), `internal/dbtest`; serve e worker. Asol Postgres e test pass, end-to-end SMS job done.
- 10 November 2026: M0 step 1 (pgx v5.12.0, `driver.go`, CI go cache), step 2 (local Postgres 5433 e, migrate duibar, `/healthz` ok), step 3 (`TestMigrateIntegration` pass, shop_app grant check jog); `check-rules.sh` macOS bash 3.2 e chole na (mapfile) thik. quick-check green.
- 10 November 2026: Repo setup (design v1.8): docs, Claude Code er puro setup, CI, infra; M0 er shuru code (config, httpx, db runner, cmd/api) ar 7 ta feature doc. Go test green, DB test 83/83.
