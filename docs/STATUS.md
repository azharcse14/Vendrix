# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **M0 shesh** (12 November 2026): CI green (run 43151580056), toolchain go1.27.4 (v1.15). **M3 (Website catalog) cholche**: step 1–12 shesh, porer step 13; plan [M3](milestones/M3.md). Push baki: `339de22`..HEAD (user: `gh auth switch -u azharcse14`, `git push origin main`).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 83/83 (Postgres 16, C.UTF-8).
- M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M3 step 13** ([M3](milestones/M3.md)): CI te website build (Q10 (a), 10.5 "JS size maapo"): `.github/workflows/ci.yml` er Dart job e `dart pub global activate jaspr_cli 0.23.5`, Tailwind binary (`scripts/tailwind.sh`, sha256 pin), `cd website && jaspr build`, `build/jaspr/web/main.client.dart.js` (ba build er asol path, local e `jaspr build` chaliye dekho) er size log e (`wc -c`). Local e age `jaspr build` chaliye path ar shomoy dekho. Commit `ci(M3): website build`. CI chalano/green dekha user er push er por.
2. Tarpor step 14 (feature doc: `l10n.md`, `theme.md` baki kina dekho), step 15 (manual: 360/390/412/768/1280 duita bhashay, Lighthouse mobile, browser e CSP shoho filter dialog ar variant bodlano, Facebook debugger (Q12)), step 16 (`make check`, `design-reviewer`, `security-reviewer` BFF/header er jonno).

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

- 12 November 2026: M3 step 12: security header ar CSP middleware, `@client` list test; website 85/85.
- 12 November 2026: M3 step 11: SEO (`SeoHead`, JSON-LD escape, sitemap/robots, hreflang, `noindex`); test age (`c75c2b7`); website 83/83, shared 76/76.
- 12 November 2026: M3 step 10: `GET /api/policies/{kind}` (Go test, Postgres e integration), search (`noindex`), policy page, footer link, `ProductListing`; website test 74/74, shared 75/75.
- 12 November 2026: M3 step 9: product page (gallery `scroll-snap`, variant radio + `@client VariantSwitch`, sticky bar, plain text description, rating); website test 71/71, shared 75/75; local API e `/p/red-rice` 200.
- 12 November 2026: `QUICK_ALL=1` quick-check e website `dart format` dhorlo (`layout.dart`), thik (`11ce92c`); shob green.
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
