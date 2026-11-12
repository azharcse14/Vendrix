# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **M0 shesh** (12 November 2026): CI green (run 43151580056), toolchain go1.27.4 (v1.15). **M3 (Website catalog) cholche**: step 1–8 shesh, porer step 9; plan [M3](milestones/M3.md).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 83/83 (Postgres 16, C.UTF-8).
- M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M3 step 9** ([M3](milestones/M3.md)): product page `/p/{slug}` (10.1, 10.2, 10.5; Q3, Q7): `pages/product.dart` (`catalog.product(slug)`, `not_found` → `pageError` 404), gallery CSS `scroll-snap` (`components/gallery.dart`, `ResponsiveImage`, prothom ta LCP), `variant_picker` (@client, dam bodlay; data shudhu variant id/dam/stock er shonkhya, lekha server e, `DialogToggle` er moto), description plain text (khali line `<p>`, newline `<br>`, 10.10 item 2), rating/`review_count`, mobile e bottom nav er bodole sticky bar (dam + variant; "Cart e add" M5 e), `app.dart` e route, `catalog_pages_test.dart` e test. Local: `make dev-seed`, API (`backend/env.example`, `ADDR=127.0.0.1:8090`, binary `go build ./cmd/api`), website `jaspr serve` (`website/env.example`), `curl -H 'Host: sobujbazar.localhost' 127.0.0.1:8081/p/red-rice`.

## User er kache proshno

- **Ekhon-i (user er kaj)**: `.claude/settings.json` e `"GOTOOLCHAIN": "go1.27.4"` (hook e shudhu user bodlay); na hole Claude Code er Go command 1.27.3 (GO-2026-6859) e chole.
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
| M0 | Project setup | shesh (12 November 2026) | [M0](milestones/M0.md) |
| M1 | Shop, vendor, settings, catalog API | shesh (10 November 2026) | [M1](milestones/M1.md) |
| M2 | Shared Dart package | shesh (10 November 2026) | [M2](milestones/M2.md) |
| M3 | Website catalog | cholche (10 November 2026 theke) | [M3](milestones/M3.md) |
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

- 12 November 2026: M3 step 8: home, category page, product card, chobi (`srcset`), filter (`<dialog>` + `@client DialogToggle`), header logo; website test 69/69, shared 74/74.
- 12 November 2026: CI green (run 43151580056); M0 shesh (`make check` green, go1.27.4).
- 11 November 2026: CI Go job e govulncheck (GO-2026-6859, `net/http`): toolchain go1.27.4, design v1.15.
- 11 November 2026: M3 step 7: `make dev-seed` (`scripts/dev-seed.sql`, local catalog data, idempotent).
- 11 November 2026: CI fix (ci.yml er Dart job naam e `: ` → YAML parse error, quote); M3 step 6 shesh: `Site`, render (404/503), `/en` route, layout, status page, typography, Bangla okkhor check; local e `/`, `/en` 200, ochena path/host 404.
- 10 November 2026: M3 step 6 er ardhek: theme CSS, self-host font, shared settings getter ar UI key.
- 10 November 2026: M3 step 5: website settings cache (host onujayi, 30 second background version check, API down e purono, suspended e bad), test 11/11.
- 10 November 2026: M3 step 4: BFF skeleton (`services/bff.dart`: CSRF Origin/`Sec-Fetch-Site`, cookie → token, Referer theke bhasha, form `303`, `no-store`), test 14/14.
- 10 November 2026: M3 step 3: `Cache-Control` middleware (14.2 test 24 pass), website server shudhu `127.0.0.1` e (`serveApp` + `shelf_io`).
- 10 November 2026: M3 step 2: website `Config.fromEnv`, `localeOf`, `apiHeaders`/`apiFor` (test 12/12), `docs/features/website.md`.
- 10 November 2026: M3 step 1: `website/` Jaspr 0.23.5 skeleton, `install-skills`, Tailwind v3.4.19 standalone (`scripts/tailwind.sh`, `jaspr_tailwind` Dart 3.13 e chole na, design v1.14), `make web`.
- 10 November 2026: M3 plan approve (shob recommendation), design v1.13.
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
