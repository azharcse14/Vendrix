> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 15. Build order: milestone onujayi

Protiti milestone e AI agent ke: (1) ei document er kon section porte hobe, (2) scope, (3) acceptance criteria dao. Business rule er code e **age test, tarpor code** likhte bolo. Protiti milestone shesh e tumi nije diff dekho ar acceptance check koro. Claude Code e ei template `/milestone M<n>` skill (`.claude/skills/milestone`): skill nijei ei table er row ar Section column er file gulo pore (map: `AGENTS.md`), plan `docs/milestones/` e rakhe (v1.6).

AI agent er jonno prompt er template:

```
docs/design/01-overview.md er Section 1.4 (niyom) ar Section <X, Y> poro (kon file: AGENTS.md er map).
Milestone <M#>: <naam>.
Scope: <niche-r table theke>.
Acceptance criteria pass na howa porjonto shesh bolbe na.
Business rule er jonno age test likho. Design er baire kichu lagle age amake jiggesh koro.
```

| # | Milestone | Section | Scope | Acceptance criteria |
|---|---|---|---|---|
| M0 | Project setup | 2, 4, 6, 8.17, 13 | Monorepo, Docker Compose Postgres (`C.UTF-8`), Go module (shudhu pgx, `go 1.25.1` + `toolchain go1.27.4`), migration runner (`backend/migrations/embed.go`, protiti file ek transaction e, `pg_advisory_xact_lock`, `set_config`; 6.1 v1.6; v1.7: `schema_migrations.sha256` checksum), `001_init.sql` + `tests/001_init_test.sql`, DB role, config, logging, server timeout, graceful shutdown, `/healthz`, job worker + `notify` (dev e log), `credentials set` CLI, router build test, docsync test, CI (`make check`, `govulncheck`, `GOTOOLCHAIN=go1.27.4`); v1.8: shuru er code ache, baki kaj ar proman `docs/milestones/M0.md` | `go test ./...` ar CI green, `/healthz` 200, migration duibar chalale-o thik, `001_init_test` 83/83, docsync pass, router e panic nai, embed kora migration list e kono test file nai (runner test), 14.2 er test 42 (v1.7) |
| M1 | Shop, vendor, settings, catalog API | 1.6, 3.2, 3.8, 6, 7.2, 9.2 | `httpx` (middleware, error format, paging, client IP 3.2), route onujayi shop resolve (`X-Shop-Host`, session, `tran_id`; v1.10: M1 e `X-Shop-Host` ar staff context er type, session M4, `tran_id` M10), `shop create` command (house vendor shoho), public settings, category, product list/detail/search, admin product/variant/category CRUD (v1.10: `/api/translations` M1 e; settings preview M8, chobi M7, vendor endpoint MV) | Shop isolation test, 14.2 er test 13, search Bangla/English/Banglish e kaj kore (v1.7: juktakkhor wala shobdo shoho, ranking o dekho; 6.4), error format 7.1 er moto |
| M2 | Shared Dart package | 4.3, 9.3, 9.8 | Model, `ApiClient`, error, translator (fallback shoho), money/tarikh format, phone validation, theme token derive + contrast | Unit test: format (bn/en: `৳১,২৫,০০০`, `৳1,25,000`; `intl` er default currency pattern na), fallback, contrast, phone |
| M3 | Website catalog | 9, 10.1–10.5, 10.9 | Jaspr server mode, BFF skeleton (Origin check, `X-Client-IP`, Cache-Control), settings cache, CSS variable, Tailwind token, layout (header, bottom nav), home, category, product, search, policy page, bhasha (bn root, `/en`), SEO (meta, OG, JSON-LD, sitemap, hreflang) | 360px ar 1280px e design thik, Lighthouse lokkho, Facebook debugger e preview, `@client` shudhu dorkari jaygay, 14.2 er test 24 |
| M4 | Login | 8.2, 8.3, 11.3 | OTP send/verify (Turnstile, atomic attempt, SMS cap), session, cookie (website `__Host-sid`, admin `__Host-asid`), staff login + temp_token + TOTP + `setup_pending` + recovery code + `failed_mfa`, OTP er dine cap, rate limit, client IP (Caddy `X-Real-IP`) | 14.2 er test 10, 18, 19; cookie flag thik |
| M5 | Cart ar wishlist | 8.4, 7.4 | Guest cart (`X-Cart-Key`), login cart, merge, wishlist (login + cookie merge) | Merge test, bondho product cart theke bad, dam fresh |
| M6 | Checkout (COD) | 8.5–8.8, 8.10, 8.13 | Delivery zone, quote, `POST /api/orders` (puro 8.6), risk + OTP, coupon, idempotency, fulfillment, duita state machine + `Summary`, COD confirm, tracking, order view, cancel, SMS job, auto-cancel job | 14.2 er test 1–5, 7, 8, 11, 15, 16, 22, 23, 26, 27, 31, 33, 39, 40 pass |
| M7 | Admin panel v1 | 11 | Flutter web setup (get_it, bloc, go_router, guard), login + TOTP, dashboard, product/variant/chobi (R2), category, inventory (batch, adjust), order list/detail/transition/edit, packing (fulfillment dhore, FIFO, ojon, `Reprice`), shipment (manual), print link, zone, coupon, customer + block | Staff role onujayi menu ar backend 403, packing e batch ar ojon thik (14.2 er test 9, 21, 29, 32, 34, 35, 36, 41), print page khole |
| M8 | Store settings | 9, 3.5, 11.4 | Branding, theme editor (preset, color, font, radius, live preview mobile/desktop, contrast), draft/preview/publish/history/rollback, translation editor, homepage, checkout setting, feature on/off, credential (owner) | Publish er 1 minute e website e bodol, rollback kaj kore, contrast warning, 14.2 er test 37 |
| M9 | Review | 8.14 | Login ar token diye review, rating hishab, moderation | Delivered na hole review na, hide e rating thik |
| M10 | SSLCommerz | 8.9 | Session (mandatory field), validation (IPN + success return), reconcile job, duplicate payment refund, retry `/pay`, unpaid auto-cancel, refund record, settlement mela (`store_amount`), Cloudflare WAF rule (Bot Fight Mode off ba Super Bot Fight Mode skip) | 14.2 er test 6, 30, sandbox e shob case (14.3 er 2) |
| M11 | Taka, support, compliance | 12, 8.15 | COD settlement, complaint (72 ghonta `due_at` ar alert), privacy request (export/delete, anonymize), `data.retention`, supply na korte parle refund alert, audit log screen, deadline alert, inventory alert, footer e DBID/BFSA, consent | Deadline alert, privacy export file thik, audit e shob admin kaj |
| M12 | Ops ar launch | 13, 14 | Staging/production, Caddy (www redirect, origin cert, `trusted_proxies` + `X-Real-IP`, admin JS `no-cache`), firewall (shudhu Cloudflare), systemd hardening, Cloudflare (WAF rule, cache bypass, Bot Fight Mode off), WAL archiving + restore test, monitoring, security header, Playwright E2E, manual checklist | 14.3 er shob tick, restore test hoyeche |

**Phase 2 milestone**: Flutter customer app (Android age; route website er moto, deep link), courier API + webhook (`shipment_events`), bKash/Nagad, refund API, delivery slot, pre-order (advance 10% ba escrow, 12.1), combo, "stock ashle janao", search upgrade (Meilisearch), Redis/CDN cache (Redis client notun Go dependency, decision log e likho), PWA + push, Facebook Pixel/GA4 (consent shoho, token page e na; 14.2 er test 38), abandoned cart, return flow, supplier/purchase, bistarito report, CSV bulk upload, load test (k6).

**Phase 3 milestone**: AI shopping assistant (Go te `net/http` diye Claude API call, product search tool), recommendation, subscription basket, flash sale (concurrency test), loyalty (age lawyer), ar niche-r duita.

| # | Milestone | Section | Scope | Acceptance criteria |
|---|---|---|---|---|
| MV | Marketplace | 1.6, 3.7, 5.4, 8.18, 11.2 | `009`, `010` migration, vendor invite/KYC, seller center (vendor role), product approval, vendor store page, `per_vendor` delivery, ledger + payout, vendor rating | 14.2 er test 14–17; ek order e duita vendor er COD parcel alada collect hoy ar ledger mile |
| MT | Multi-tenant | 1.6, 3.2, 3.8, 7.9, 13.3 | `011` migration, platform admin, shop signup (3.8 er command er UI), custom domain verify + HTTPS, suspend, billing, shop onujayi job/rate limit, optional RLS (`012`) | Duita shop duita domain e alada theme, payment, customer; test 5 ar 13 pass; suspend kora shop `423` |
