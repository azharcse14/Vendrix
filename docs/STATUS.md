# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **M0 shesh** (12 November 2026): CI green (run 43151580056), toolchain go1.27.4 (v1.15). **M3 (Website catalog) shesh** (15 November 2026): Lighthouse 16.1 (a) (v1.18); report [M3](milestones/M3.md). Mac e `make check` er `TestSearchRankingIntegration` macOS `C.UTF-8` er karone fail (Linux e green; M3 Report). **M4 (Login) cholche** (plan approve 15 November 2026, design v1.20; step 0-1 shesh). Push baki: `339de22`..HEAD (user: `gh auth switch -u azharcse14`, `git push origin main`).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 83/83 (Postgres 16, C.UTF-8).
- M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M4 step 2, OTP primitive** ([M4](milestones/M4.md), 8.2): `backend/internal/auth` e `doc.go` (package comment → `docs/features/auth.md`) ar `otp.go`: `crypto/rand` 6 digit code (samne zero thake), `HMAC-SHA256(OTP_SECRET, phone:purpose:code)`, compare `hmac.Equal`. Test age: `otp_test.go` e `TestOTPCode`, `TestOTPHash`, fail dekhe commit `test(M4): OTP code ar hash`, tarpor `feat(M4): OTP code ar hash` (`docs/features/auth.md` shoho, `feature-doc` skill). Local `backend/.env` e `env.example` er notun 4 line (`OTP_SECRET`, `RECOVERY_SECRET`, `TOTP_KEY`, `TURNSTILE_SECRET`) lagbe, na hole `make run` start hoy na.
2. Push er por CI green kina dekho (`ci(M3): website build` ar Go job; Linux e `TestSearchRankingIntegration` green howa uchit).

## User er kache proshno

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
- 12 November 2026: M3 step 9: product page (gallery `scroll-snap`, variant radio + `@client VariantSwitch`, sticky bar, plain text description, rating); website test 71/71, shared 75/75; local API e `/p/red-rice` 200.
- 12 November 2026: `QUICK_ALL=1` quick-check e website `dart format` dhorlo (`layout.dart`), thik (`11ce92c`); shob green.
- 12 November 2026: M3 step 8: home, category page, product card, chobi (`srcset`), filter (`<dialog>` + `@client DialogToggle`), header logo; website test 69/69, shared 74/74.
