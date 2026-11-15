# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **M0 shesh** (12 November 2026): CI green (run 43151580056), toolchain go1.27.4 (v1.15). **M3 (Website catalog) shesh** (15 November 2026): Lighthouse 16.1 (a) (v1.18); report [M3](milestones/M3.md). Mac e `make check` er `TestSearchRankingIntegration` macOS `C.UTF-8` er karone fail (Linux e green; M3 Report). **M4 (Login) shesh** (15 November 2026, design v1.19–v1.27): customer OTP login + session, `/api/me`, staff login + TOTP/recovery + setup, admin session, `staff reset-2fa` CLI, website `/login` + `/account`, local https (`make https`); report [M4](milestones/M4.md). **M5 (Cart ar wishlist) cholche** (plan approve 15 November 2026: shob recommendation, design v1.28–v1.30; step 0–9 shesh). Push baki: `339de22`..HEAD (user: `gh auth switch -u azharcse14`, `git push origin main`).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 85/85 (Postgres 16, C.UTF-8).
- M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M5 step 10, Feature doc** ([M5](milestones/M5.md) step 10): `docs/features/cart.md` (step 2–5 er handler/store/service, merge, wishlist, `cart.cleanup`) ar `website.md`, `api-client.md`, `l10n.md` code er sathe milao (`feature-doc` skill; code map, niyom → test, data, proshno-uttor; step 9 er proman er link); `make quick-check` (docsync). Tarpor step 11 (`make check`, `security-reviewer` + `design-reviewer`, acceptance, Report).
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

- 15 November 2026: M5 step 9 (manual proman): scratch stack (HEAD `d6ca8db` er `shop-api` 8091, `jaspr build` 8082, Caddy copy 8444; chalu 8090/8081/8443 e hat na), headless Chromium: guest add → `/cart` (qty `303`, bad), JS chhara form `303`, SQL dam bodol → notun dam, product bondho → `item_removed` notice (ekbar), OTP login → merge (2+3=5), `__Host-cart` `Max-Age=0`, guest `wl` → `/account/wishlist`, `wl` muche; cookie flag, `Cache-Control`, 360/1280 screenshot (scratch), console error 0. Bug nai. Porer kaj step 10.
- 15 November 2026: M5 step 8 (website UI): test `e27b4fb` (14 fail), `440367d` (`wl` er koma e Jaspr `context.cookies` bhange), `6904327` (`wlIds`/`wlCookie`); code `7069873`: `/cart` (SSR, JS chhara qty/bad form, notice, checkout nai), `@client AddToCart` + `WishlistButton`, Cart link + badge (cookie e ek `GET`, `CartLoad`), `/account/wishlist`, login e `cart_notices`; `website.md`; website 163 pass; JS `main.client.dart.js` 174,785 byte (gzip 54 KB, +2.2 KB). Browser proman baki (stack restart onumoti na); porer kaj step 9.
- 15 November 2026: M5 step 7 (website BFF): test `db1bb2e` (13 fail), code `df83e9f`: cart route (JSON + JS chhara qty/delete form, `303 /cart`), wishlist route, `__Host-cart` (Go er `cart_key`, body theke muche, sliding), `X-Cart-Key` shudhu valid format ar login chhara, verify e `__Host-cart`/`wl` `Max-Age=0` + wishlist merge (fail e `wl` thake), `cache_control_test` cart case; `website.md`; website 148 pass. Porer kaj step 8 (UI).
- 15 November 2026: M5 step 6 (shared): test `fdef5aa` (compile fail; `messages_test` e Go er cart error ar notice key nai), code: `models/cart.dart` (`Cart`, `CartItem`, `CartNotice`), `CartRepository`, `WishlistRepository`, `ApiClient(cartKey:)` → `X-Cart-Key`, l10n `nav.cart`, `cart.*`, `cart.notice.<code>`, `wishlist.*`, `error.out_of_stock`/`cart_full`/`wishlist_full`; `api-client.md`, `l10n.md`; shared 85, website 123 pass. Porer kaj step 7 (BFF).
- 15 November 2026: M5 step 5 (`cart.cleanup`): test `411eec2` (compile fail, schedule e key nai), code: `cart.CleanupJob` (30 din er purono guest cart ek `DELETE` e, shob shop, item CASCADE; login cart na), `jobs.schedule` e `cart.cleanup:<UTC din>`, `cmd/api/main.go` e handler; `cart.md`, `jobs.md`; asol Postgres e `-race` pass. Porer kaj step 6 (shared).
- 15 November 2026: M5 step 4 (wishlist): test `c0a222e` (compile fail), code: `cart` e `WishlistRoutes` (`GET`, `POST`/`DELETE /api/wishlist/{product_id}`, merge), `AddWish` (user row lock, 200 e `409 wishlist_full`, onno shop/bondho `404`), `MergeWish` (prothom 50 ID, active product, mot 200), router `RequireCustomer`, `wishlist_full` bn/en; asol Postgres e `-race` pass. Porer kaj step 5 (`cart.cleanup`).
- 15 November 2026: M5 step 3 (merge): design v1.30 `bbb5178` (user: `item_dropped` verify response er `cart_notices` e; 7.3, 8.4, 10.12, 16.1), test `0183a14` (fail dekhe), code: `cart.Merge` (guest `FOR UPDATE` → account upsert, jog `min(jog, 50, stock)`, 50 variant e `item_dropped`, guest muche), `auth.CartMerge` router theke (import cycle), verify `{token, user, cart_notices}`; asol Postgres e `-race` pass. Porer kaj step 4 (wishlist).
- 15 November 2026: M5 step 2 (Cart API): design v1.29 `e2c6b6b` (user: merge e 50 er beshi variant `item_dropped`, wishlist 200 e `409 wishlist_full`; 7.1, 7.4, 8.4, 16.1); test `79748ca` (compile fail), code: `cart` handler/store/service (`View`, `Add`, `SetQuantity`, `Remove`), `httpx.OptionalCustomer`, router, `out_of_stock`/`cart_full` message; asol Postgres e `-race` pass. Porer kaj step 3 (merge).
- 15 November 2026: M5 step 1: notun `backend/internal/cart` (guest key `NewKey`/`KeyHash`, `CheckQuantity`, `CheckRoom`, `NormalizeQuantity`), test age `c29251f`, `docs/features/cart.md`. Porer kaj step 2 (Cart API).
- 15 November 2026: M5 plan approve (user: shob recommendation, Q1–Q11), design v1.28 (3.2, 7.1, 7.4, 7.8, 8.2, 8.4, 8.17, 10.1–10.3, 10.5, 10.9, 10.10, notun 10.12, 12.2); M5 step 0.
- 15 November 2026: M5 plan draft `docs/milestones/M5.md` (scope, Already ache, acceptance → test, 11 ta Open question recommendation shoho, step 0–11); user approve baki, code na.
- 15 November 2026: **M4 shesh**: step 15 `make check` (gofmt, vet, docsync, rule green; DB 85/85; shared 78, website 123; `-race` e shudhu jana `TestSearchRankingIntegration` macOS locale), `dart format` fix `7b90d9b` (`account.dart`), acceptance ☑, [M4](milestones/M4.md) Report. Porer kaj: push + CI, tarpor M5 plan.
- 15 November 2026: 16.1 local `__Host-` cookie: user (a), design v1.27 `bf33e68` (13.1, 13.3); `make https` (Caddy `tls internal`, `infra/caddy/Caddyfile.local`, `scripts/https.sh`); Chromium e `https://sobujbazar.localhost:8443` OTP login → `/account` e `__Host-sid` thake. User: `brew install caddy`, ekbar `caddy trust`.
- 15 November 2026: M4 step 15 design review baki finding: design v1.26 `bb57e10` (error code, response, rate limit note, audit action, 16.1 SMS cap DoS), test `7f765fe`, fix `4bc4b88` (OTP lock alert key HMAC, account phone placeholder), feature doc `868f489`; M4.md test 23 ☑.
- 15 November 2026: M4 step 15 TOTP pending (user decision, security-reviewer M2): design v1.25 `d35a877`, test `aea4dcb` (fail dekhe), fix `2cf0ede` (setup er secret `totp_pending_*` e, enable e live; TOTP live + `must_change_password` e password bodlale session `active`); `make db-test` 85/85, `-race` green. `TestStaffLockRaceIntegration` er flake: shombhoboto Mac sleep (wall clock 5 minute er beshi agay), code race na; 22 bar `-race` pass.
