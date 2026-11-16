# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **M6 (Checkout (COD)) cholche** (plan approve 15 November 2026: Q1–Q13 shob recommendation, design v1.32): step 0 shesh (8.5–8.10 ekhon `docs/design/08-checkout.md` e, M6 er decision 8.19 ar 10.13); step 1 shesh (`internal/shipping`: `Lookup`, `GET /api/delivery-zones`, dev seed zone); step 2 shesh (`internal/coupon`: `Normalize`, `ErrorCode`, `Use`, `Peek`, `Release`); step 3 shesh (`order`: `RiskFlags`, `OTPRequired`, `ConfirmRequired`, `LoadRiskHistory`); porer step 4 (quote). Plan [M6](milestones/M6.md).
- **M0 shesh** (12 November 2026): CI green (run 43151580056), toolchain go1.27.4 (v1.15). **M3 (Website catalog) shesh** (15 November 2026): Lighthouse 16.1 (a) (v1.18); report [M3](milestones/M3.md). Mac e `make check` er `TestSearchRankingIntegration` macOS `C.UTF-8` er karone fail (Linux e green; M3 Report). **M4 (Login) shesh** (15 November 2026, design v1.19–v1.27): customer OTP login + session, `/api/me`, staff login + TOTP/recovery + setup, admin session, `staff reset-2fa` CLI, website `/login` + `/account`, local https (`make https`); report [M4](milestones/M4.md). **M5 (Cart ar wishlist) shesh** (15 November 2026, design v1.28–v1.31): Cart API (guest key, `OptionalCustomer`, normalize, `GET /api/cart/count`), login e merge, wishlist API, `cart.cleanup`, shared `Cart`/repository, website BFF cart/wishlist, `/cart`, `/account/wishlist`, `@client AddToCart`/`WishlistButton`, Cart badge; report [M5](milestones/M5.md). Push baki: `339de22`..HEAD (user: `gh auth switch -u azharcse14`, `git push origin main`).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 85/85 (Postgres 16, C.UTF-8).
- M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M6 step 4: quote, test age** (`docs/milestones/M6.md` step 4; design 7.5, 8.5, 8.19 item 2, 5): age `backend/internal/order/quote_integration_test.go` e `TestQuoteIntegration` (subtotal/discount/delivery/total `pricing` theke, free delivery `>=`, `below_min_order`, `area_not_served`, coupon na khatle coupon chhara total + `coupon_invalid` (14.2 test 39), `otp_required` (`order.OTPRequired` + `LoadRiskHistory`), kichu save na, client dam field `400`); fail dekhao, commit `test(M6): checkout quote`. Tarpor `order/handler.go` + `service.go` (`POST /api/checkout/quote`; `shipping.Lookup`, `coupon.Peek`, `pricing.Calculate`, cart load `internal/cart/store.go` theke), `cmd/api/router.go` + `main_test.go` (`TestSessionRouteKind`), `httpx/message.go` e `area_not_served`, `coupon_invalid`, `coupon_limit_reached` bn/en (`coupon.ErrorCode` theke code); feature doc `docs/features/checkout.md`; `make quick-check`, commit `feat(M6): POST /api/checkout/quote`. Local DB: Homebrew Postgres 16 bondho thakle `/opt/homebrew/opt/postgresql@16/bin/pg_ctl -D /opt/homebrew/var/postgresql@16 -o "-p 5433 -k /tmp" -l <log> start`; role `travela` (postgres role nai): `TEST_DATABASE_URL=postgres://travela@127.0.0.1:5433/postgres?sslmode=disable`.
2. **User: push ar CI**: `git push origin main` (`339de22`..HEAD (M6 er commit shoho); `gh auth switch -u azharcse14`), tarpor CI green kina dekho (Go job, website build, Dart job); Linux e `TestSearchRankingIntegration` (`backend/internal/catalog`) green howa uchit (Mac e macOS `C.UTF-8` e fail, M3/M4/M5 Report). Local browser login er jonno `backend/.env` e `OTP_SECRET`, `TURNSTILE_SECRET`, `TOTP_KEY`, `RECOVERY_SECRET` ar `website/.env` e `TURNSTILE_SITE_KEY` (`env.example` dekho).

## User er kache proshno

- **M12 er age**: client JS er `?v=<git sha>` (10.10 item 10): Jaspr nijei script tag dey, hook nai; recommendation deploy e Cloudflare purge (16.1).
- **Ekhon-i (user er kaj)**: `.claude/settings.json` e `"GOTOOLCHAIN": "go1.27.4"` (hook e shudhu user bodlay); na hole Claude Code er Go command 1.27.3 (GO-2026-6859) e chole.
- **M7 er age**: COD e packing e beshi ojon er shima; packed fulfillment undo (16.1).
- **M10 er age**: SSLCommerz validation er HTTP call DB lock er bhetore naki baire (16.1).
- **M12 er age**: OTP SMS cap shesh hole oi din OTP login bondho (bot DoS): (a) ekhon er moto + monitor/alert, (b) cap er kache extra limit, (c) cap shudhu alert (16.1, v1.26).
- **M12 er age**: production domain (doc e `tomarshop.com` placeholder), server region, SMS provider (16.1).
- **Public launch er age**: IP-bhittik rate limit ar CGNAT; shathe ochena `X-Shop-Host` er rate limit (M1 review); website SSR er API call (badge count, `/cart`) o customer er IP te public 120/minute e gone (M5 review) (16.1).
- **MV er age**: vendor `active` na hole catalog list/detail o lukabe (cart/wishlist er moto), naki cart o na dekhe (16.1, M5 review).
- **M7 er age**: `vendor_owner` admin category list porte parbe kina (11.2, 16.1).

## Milestone

| # | Naam | Obostha | Plan |
|---|---|---|---|
| M0 | Project setup | shesh (12 November 2026) | [M0](milestones/M0.md) |
| M1 | Shop, vendor, settings, catalog API | shesh (10 November 2026) | [M1](milestones/M1.md) |
| M2 | Shared Dart package | shesh (10 November 2026) | [M2](milestones/M2.md) |
| M3 | Website catalog | shesh (15 November 2026) | [M3](milestones/M3.md) |
| M4 | Login | shesh (15 November 2026) | [M4](milestones/M4.md) |
| M5 | Cart ar wishlist | shesh (15 November 2026) | [M5](milestones/M5.md) |
| M6 | Checkout (COD) | cholche | [M6](milestones/M6.md) |
| M7 | Admin panel v1 | baki | |
| M8 | Store settings | baki | |
| M9 | Review | baki | |
| M10 | SSLCommerz | baki | |
| M11 | Taka, support, compliance | baki | |
| M12 | Ops ar launch | baki | |
| MV | Marketplace (Phase 3) | baki | |
| MT | Multi-tenant (Phase 3) | baki | |

## Session log

- 16 November 2026: M6 step 3 shesh: test `dec9a39`, code `c2944af`: `order/risk.go` (`RiskFlags`, `OTPRequired` login e na, `ConfirmRequired` risky = flag ar `otp_verified` false; ochena mode fail closed), `order/store.go` `LoadRiskHistory`, package comment `doc.go` e, `docs/features/checkout.md` shuru; `TestRiskFlags`, `TestOTPRequired`, `TestConfirmRequired`, `TestRiskHistoryIntegration` green (`-race`, asol Postgres), quick-check green. Porer kaj step 4 (quote).
- 16 November 2026: M6 step 2 shesh: `internal/coupon` (`Normalize` trim + upper max 32, `ErrorCode`, `Use` atomic `used_count + 1` tarpor phone count, karon `coupon_invalid`/`coupon_limit_reached`, read-only `Peek`, `Release` `GREATEST`), `docs/features/coupon.md`; `TestCouponNormalize`, `TestCouponUseIntegration`, `TestCouponPeekIntegration` green, quick-check green. Porer kaj step 3 (risk ar OTP).
- 16 November 2026: M6 step 1 shesh: `internal/shipping` (`Lookup` area → district-wide, `List`, `GET /api/delivery-zones`), router, `scripts/dev-seed.sql` e zone, `docs/features/delivery-zone.md`; `TestZoneLookupIntegration`, `TestDeliveryZonesIntegration` green, quick-check green. Porer kaj step 2 (coupon).
- 15 November 2026: M6 plan approve (user: shob recommendation, Q1–Q13); M6 step 0: `96d58f9` (8.5–8.10 `docs/design/08-checkout.md` e, AGENTS.md map, README, rule link) ar design v1.32 (notun 8.19, 10.13; 7.1, 7.2, 7.3, 7.5, 7.8, 8.6–8.8, 8.10, 8.13, 8.15, 8.16, 10.1, 10.6, 10.9, 10.11, 12.2, 16.1 perishable row); M6.md e uttor. Porer kaj step 1 (delivery zone).
- 15 November 2026: M6 plan draft `docs/milestones/M6.md` (scope, Already ache: `001` e shob table, migration lage na; acceptance → test, 14.2 test 1–5, 7, 8, 11, 15, 16, 22, 23, 26, 27, 31, 33, 39, 40; 13 ta Open question recommendation shoho; step 0–16); user approve baki, code na.
- 15 November 2026: **M5 shesh**: step 11 `make check` (gofmt, vet, docsync, rule green; DB 85/85; shared 86, website 173; `-race` e shudhu jana `TestSearchRankingIntegration` macOS locale, `catalog` `-skip` diye green), JS `main.client.dart.js` 174,827 byte (gzip 54 KB), acceptance ☑, [M5](milestones/M5.md) Report. Porer kaj: push + CI, tarpor M6 plan.
- 15 November 2026: M5 step 11 review fix (user decision D1/D2): design v1.31 `750f161` (7.4 `GET /api/cart/count`, 8.4 item 9, 8.17 Rule 3/5 er chhar, 10.12 item 5, 7–9, 12.2, 16.1 duita thik holo + MV er age vendor `active` + CGNAT row e note), test `6df9511`, fix `65c3604` (count route, qty overflow `409`), `4ef530a` (Layout count, session `401` e `/login?next=` + `__Host-sid` `Max-Age=0`, `validSid`, login e `cart_key` cookie na, approx bracket); M5.md review line. Rakha: security L3/L5 (12.2), L1 (16.1), design L4 (16.1 MV).
- 15 November 2026: M5 step 10 (feature doc): `cart.md`, `website.md`, `api-client.md`, `l10n.md`, `auth.md`, `jobs.md`, `http-foundation.md` code er sathe milano (function ar test naam grep e); jog: `PATCH` `404`, `parseCookies` shikkha, `/cart` public cache, `wl` 50/merge fail, verify e merge er bhul; README (M5). Code-doc mismatch nai. Porer kaj step 11.
- 15 November 2026: M5 step 9 (manual proman): scratch stack (HEAD `d6ca8db` er `shop-api` 8091, `jaspr build` 8082, Caddy copy 8444; chalu 8090/8081/8443 e hat na), headless Chromium: guest add → `/cart` (qty `303`, bad), JS chhara form `303`, SQL dam bodol → notun dam, product bondho → `item_removed` notice (ekbar), OTP login → merge (2+3=5), `__Host-cart` `Max-Age=0`, guest `wl` → `/account/wishlist`, `wl` muche; cookie flag, `Cache-Control`, 360/1280 screenshot (scratch), console error 0. Bug nai. Porer kaj step 10.
- 15 November 2026: M5 step 8 (website UI): test `e27b4fb` (14 fail), `440367d` (`wl` er koma e Jaspr `context.cookies` bhange), `6904327` (`wlIds`/`wlCookie`); code `7069873`: `/cart` (SSR, JS chhara qty/bad form, notice, checkout nai), `@client AddToCart` + `WishlistButton`, Cart link + badge (cookie e ek `GET`, `CartLoad`), `/account/wishlist`, login e `cart_notices`; `website.md`; website 163 pass; JS `main.client.dart.js` 174,785 byte (gzip 54 KB, +2.2 KB). Browser proman baki (stack restart onumoti na); porer kaj step 9.
- 15 November 2026: M5 step 7 (website BFF): test `db1bb2e` (13 fail), code `df83e9f`: cart route (JSON + JS chhara qty/delete form, `303 /cart`), wishlist route, `__Host-cart` (Go er `cart_key`, body theke muche, sliding), `X-Cart-Key` shudhu valid format ar login chhara, verify e `__Host-cart`/`wl` `Max-Age=0` + wishlist merge (fail e `wl` thake), `cache_control_test` cart case; `website.md`; website 148 pass. Porer kaj step 8 (UI).
- 15 November 2026: M5 step 6 (shared): test `fdef5aa` (compile fail; `messages_test` e Go er cart error ar notice key nai), code: `models/cart.dart` (`Cart`, `CartItem`, `CartNotice`), `CartRepository`, `WishlistRepository`, `ApiClient(cartKey:)` → `X-Cart-Key`, l10n `nav.cart`, `cart.*`, `cart.notice.<code>`, `wishlist.*`, `error.out_of_stock`/`cart_full`/`wishlist_full`; `api-client.md`, `l10n.md`; shared 85, website 123 pass. Porer kaj step 7 (BFF).
- 15 November 2026: M5 step 5 (`cart.cleanup`): test `411eec2` (compile fail, schedule e key nai), code: `cart.CleanupJob` (30 din er purono guest cart ek `DELETE` e, shob shop, item CASCADE; login cart na), `jobs.schedule` e `cart.cleanup:<UTC din>`, `cmd/api/main.go` e handler; `cart.md`, `jobs.md`; asol Postgres e `-race` pass. Porer kaj step 6 (shared).
- 15 November 2026: M5 step 4 (wishlist): test `c0a222e` (compile fail), code: `cart` e `WishlistRoutes` (`GET`, `POST`/`DELETE /api/wishlist/{product_id}`, merge), `AddWish` (user row lock, 200 e `409 wishlist_full`, onno shop/bondho `404`), `MergeWish` (prothom 50 ID, active product, mot 200), router `RequireCustomer`, `wishlist_full` bn/en; asol Postgres e `-race` pass. Porer kaj step 5 (`cart.cleanup`).
- 15 November 2026: M5 step 3 (merge): design v1.30 `bbb5178` (user: `item_dropped` verify response er `cart_notices` e; 7.3, 8.4, 10.12, 16.1), test `0183a14` (fail dekhe), code: `cart.Merge` (guest `FOR UPDATE` → account upsert, jog `min(jog, 50, stock)`, 50 variant e `item_dropped`, guest muche), `auth.CartMerge` router theke (import cycle), verify `{token, user, cart_notices}`; asol Postgres e `-race` pass. Porer kaj step 4 (wishlist).
