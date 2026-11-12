# Vendrix: ekhon kothay

<!-- Claude Code: protiti session er shuru te ei file poro (SessionStart hook "Ekhon", "Porer kaj" ar
"User er kache proshno" dekhay), ar code bodlale kaj shesh er age update koro (Stop hook dekhe).
Format: "Ekhon" 3-6 line. "Porer kaj" numbered, protiti item eto spashto je notun session ekhan theke
shuru korte pare (file, command, milestone file er step). "User er kache proshno" e shudhu uttor baki
proshno, kon milestone er age lagbe shoho; uttor pele 16.1 ba design e likhe ekhan theke muche dao.
"Milestone" table e obostha: baki, cholche, shesh (tarikh). "Session log" e notun line upore, shesh 15 ta
(purono gulo git e). Tarikh Asia/Dhaka. Heading bodlabe na (docsync dekhe). -->

## Ekhon

- **M0 shesh** (12 November 2026): CI green (run 43151580056), toolchain go1.27.4 (v1.15). **M3 (Website catalog) cholche**: step 1–14 shesh, step 16 er review ar `make check` shesh; baki step 15 (manual, user); plan [M3](milestones/M3.md). Push baki: `339de22`..HEAD (user: `gh auth switch -u azharcse14`, `git push origin main`).
- Repo setup shesh (design v1.8): design, feature doc, Claude Code (hook, skill, agent, rule), CI, infra, ar M0 er shuru code: config, `httpx`, migration runner (checksum shoho), server, `/healthz`.
- Go test ar `go test -race` green; database constraint test 83/83 (Postgres 16, C.UTF-8).
- M1 shesh (v1.11). **M2 shesh** (v1.12): `packages/shared` (format, phone, theme token/CSS variable, translator, model, `ApiClient`), CI Dart job; report [M2](milestones/M2.md). Local Postgres host port **5433**; local API `ADDR=127.0.0.1:8090` (8080 e onno project er `hb-api`).
- `make check` e `psql` lage: `brew install libpq` korechi; PATH e `/opt/homebrew/opt/libpq/bin` (keg-only).

## Porer kaj

1. **M3 step 15** (user er manual proman, "User er kache proshno" dekho): result pele [M3](milestones/M3.md) er acceptance (360/1280, Lighthouse, Facebook) ☑ ar "Report" likho, STATUS e M3 shesh, porer kaj M4 (`milestone` skill, age plan, user approve; M4 er age 16.1 er OTP payload ar `jobs` cleanup proshno). CI (`ci(M3): website build`) push er por green kina dekho.
2. User manual na korle/korte deri hole: M4 er plan (docs/design/15-milestones.md er M4 row) pora shuru kora jay, kintu M3 er acceptance pass na hole M4 er code na (AGENTS.md).

## User er kache proshno

- **M12 er age**: client JS er `?v=<git sha>` (10.10 item 10): Jaspr nijei script tag dey, hook nai; recommendation deploy e Cloudflare purge (16.1).
- **M3 shesh korte (user er kaj)**: step 15 manual proman: machine e Chrome nai. Chrome DevTools e 360/390/412/768/1280 (home, category, product; duita bhasha), filter dialog ar variant bodlano (CSP shoho), Lighthouse mobile (`jaspr build` er binary), Facebook Sharing Debugger (`cloudflared` lage, Q12; na hole 14.3 e). Result Report e likhte ami pore help korbo.
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

- 12 November 2026: M3 step 16: `make check` green; design/security review e high nai; finding fix (BFF Origin null/scheme, open redirect, body limit, host normalize, sitemap, renderPage test); website 93/93. `?v=` 16.1 e.
- 12 November 2026: M3 step 14: `l10n.md`, `theme.md`, `api-client.md` e website er ongsho.
- 12 November 2026: M3 step 13: CI Dart job e website `jaspr build` + JS size (local 141 KB, gzip 45 KB).
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
