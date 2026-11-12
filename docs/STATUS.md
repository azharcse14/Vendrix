# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **M0 shesh** (12 November 2026): CI green (run 43151580056), toolchain go1.27.4 (v1.15). **M3 (Website catalog) cholche**: step 1–10 shesh, porer step 11; plan [M3](milestones/M3.md). Push baki: `339de22`..HEAD (user: `gh auth switch -u azharcse14`, `git push origin main`).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 83/83 (Postgres 16, C.UTF-8).
- M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M3 step 11** ([M3](milestones/M3.md)): SEO (10.4), **test age**: `website/test/seo/meta_test.dart` (`group('Meta ar Open Graph (10.4)')`: protiti page e `<title>`, `meta description` (product `seo`, na hole naam/description), OG `property` attribute (`og:type=product`, `og:image` absolute, `product:price:amount`, `product:price:currency=BDT`), `canonical`, `hreflang` bn/en/x-default (`en` bondho hole shudhu bn + x-default), `seo.noindex` hole puro site `noindex`), `json_ld_test.dart` (`group('JSON-LD escape (10.4, 12.2)')`: `<` → `\u003c`, `aggregateRating` shudhu `review_count > 0`), `sitemap_test.dart` (`group('sitemap ar robots (10.4)')`: category, product (`/api/products` paging), policy, duita bhasha, host er nijer; robots 10.4 er path block). Fail commit `test(M3): SEO`; tarpor `lib/seo/` (`meta.dart`, `open_graph.dart`, `json_ld.dart`, `sitemap.dart`, `hreflang.dart`), page gulo te `Document.head` (search e ekhon shudhu `robots noindex`), `/sitemap.xml` ar `/robots.txt` shelf handler (render er age, `main.server.dart`). Commit `feat(M3): meta, OG, JSON-LD, sitemap, hreflang`; `docs/features/seo.md` notun. Local chalano: `make dev-seed`; API `set -a; . backend/env.example; ADDR=127.0.0.1:8090` diye `go build ./cmd/api` er binary `serve` (`backend/.env` nai); website `website/env.example` load kore `jaspr serve`.

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
- 10 November 2026: M3 step 1: `website/` Jaspr 0.23.5 skeleton, `install-skills`, Tailwind v3.4.19 standalone (`scripts/tailwind.sh`, `jaspr_tailwind` Dart 3.13 e chole na, design v1.14), `make web`.
- 10 November 2026: M3 plan approve (shob recommendation), design v1.13.
