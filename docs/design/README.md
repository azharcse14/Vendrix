# Vendrix: puro system design ar build plan

Version 1.9, 10 November 2026

Ei document e ekta Bangladesh-based ecommerce platform er puro design ache: ki banano hobe, kon technology te, database kemon, API kemon, kon niyom mante hobe, ar kon order e banate hobe. Shuru hobe ekta organic food er dokan diye (single store), kintu ekoi code pore fashion, electronics er moto onno business eo chalano jabe. Design emon je pore ekoi code e **marketplace** (ek website e onek seller) ar **multi-tenant** (ek platform e onek alada dokan) chalano jay, kono boro rewrite chhara. Tin ta mode Section 1.6 e.

## Ei document ta kivabe use korbe

v1.6 theke design ekta file na: `docs/design/` e section onujayi 16 ta file, ar ei README te suchipotro ar version. Ager `DESIGN.md` (287 KB, prai 80K token, 40% er beshi code) Claude Code er Read tool er shima (ek call e 25,000 token, 256 KB er beshi file ek bare na) periye jeto, tai agent tukro tukro porto. Ekhon protiti file 60 KB er niche; `go test ./...` er `backend/internal/docsync` test eta, link, code block er size ar section map dhore rakhe.

Repo er root e `AGENTS.md` ache (Claude Code `CLAUDE.md` er `@AGENTS.md` import diye pore): kono code likhar age Section 1.4 ([01-overview.md](01-overview.md)) ar je milestone e kaj korcho tar section gulo (map `AGENTS.md` e) poro, puro design protibar na. Section 1.4 er niyom gulo kokhono bhangbe na.

AI ke kaj dewar shomoy Section 15 er milestone gulo **ekta ekta kore** dao. Protiti milestone e bolo kon section gulo porte hobe, ar acceptance criteria pass na hoya porjonto porer milestone e jeo na; Claude Code e `/milestone M<n>` (`.claude/skills/milestone`) eta kore. Design e kono bodol hole age ei document update koro, tarpor code (`.claude/skills/design-change`). v1.8: kaj er obostha `docs/STATUS.md` e, feature ekhon kivabe kaj kore `docs/features/` e, Claude Code er kaj er kram `CLAUDE.md` e (4.7).

Ei design er shathe ready file. v1.6 theke egulo-i asol spec: doc e copy nai, shudhu link ar niyom er lekha.

| File | Section |
|---|---|
| [`backend/migrations/001_init.sql`](../../backend/migrations/001_init.sql) | 6.7 (puro schema) |
| [`backend/migrations/tests/001_init_test.sql`](../../backend/migrations/tests/001_init_test.sql) | 6, 14.2 er test 25 (83 ta constraint test; `make db-test`, M0 theke CI te) |
| [`backend/internal/phone`](../../backend/internal/phone/phone.go) | 8.1 |
| [`backend/internal/pricing`](../../backend/internal/pricing/pricing.go) | 8.5 |
| [`backend/internal/order`](../../backend/internal/order/order.go) | 8.8 |

Niyom er lekha (8.x) ar code e kono parthokko dekhle age doc thik koro, tarpor code ar test.

## Suchipotro

| Section | File | Bhetore |
|---|---|---|
| 1. Project overview ar decision | [01-overview.md](01-overview.md) | 1.1 Lokkho · 1.2 Launch strategy · 1.3 Mul decision · 1.4 Je niyom gulo kokhono bhangbe na · 1.5 Decision log (alochonar shar-songkhep) · 1.6 Tin ta mode: single store, marketplace, multi-tenant |
| 2. Tech stack | [02-tech-stack.md](02-tech-stack.md) | 2.1 Stack · 2.2 Go: standard library diye ki ki hobe · 2.3 Dart package |
| 3. System architecture | [03-architecture.md](03-architecture.md) | 3.1 Component gulo · 3.2 Shop ar vendor chena · 3.3 Product page request (SSR) · 3.4 Checkout ar online payment · 3.5 Settings publish ar cache · 3.6 Background job · 3.7 Order, fulfillment ar vendor · 3.8 Notun shop khola |
| 4. Repository ar code structure | [04-repo-structure.md](04-repo-structure.md) | 4.1 Monorepo · 4.2 Backend (Go) · 4.3 Shared Dart package · 4.4 Website (Jaspr) · 4.5 Admin (Flutter web) · 4.6 Dart package er jora · 4.7 Doc ar tracking |
| 5. Feature list (phase onujayi) | [05-features.md](05-features.md) | 5.1 Customer website · 5.2 Admin panel · 5.3 Business module (config diye on/off) · 5.4 Marketplace ar multi-tenant (Phase 3) |
| 6. Database design | [06-database.md](06-database.md) | 6.1 Database er niyom · 6.2 Mul shomporko · 6.3 Table gulor kaj · 6.4 Guruttopurno query (test kora) · 6.5 JSONB column er format · 6.6 Pore-r migration (Phase 2/3) · 6.7 Puro schema: `001_init.sql` |
| 7. API design | [07-api.md](07-api.md) | 7.1 Convention · 7.2 Public ar system · 7.3 Customer login ar account · 7.4 Cart ar wishlist · 7.5 Checkout ar order · 7.6 Payment ar webhook · 7.7 Admin API (`/api/admin`, staff login lagbe) · 7.8 Rate limit · 7.9 Platform API (Phase 3, multi-tenant) |
| 8. Core flow ar business rule | [08-core-flows.md](08-core-flows.md) | 8.1 Phone number · 8.2 Customer login (OTP) · 8.3 Staff login · 8.4 Cart · 8.5 Dam er hishab (`pricing` package) · 8.6 Checkout: `POST /api/orders` · 8.7 Risk ar OTP · 8.8 Order ar fulfillment er status (state machine) · 8.9 SSLCommerz payment · 8.10 COD confirmation · 8.11 Packing, batch ar ojon · 8.12 Inventory · 8.13 Coupon · 8.14 Review · 8.15 Delivery deadline · 8.16 SMS · 8.17 Background job · 8.18 Vendor er taka (marketplace, Phase 3) |
| 9. Theme, design system ar localization | [09-theme-l10n.md](09-theme-l10n.md) | 9.1 Mul niyom · 9.2 Settings JSON (`shop_settings.draft` / `published`) · 9.3 Design token · 9.4 Preset · 9.5 Organic er design nirdeshona · 9.6 Font ar typography · 9.7 Flutter e theme · 9.8 Localization |
| 10. Customer website (Jaspr) | [10-website.md](10-website.md) | 10.1 Page ar route · 10.2 Jaspr er niyom · 10.3 Cookie · 10.4 SEO checklist · 10.5 Responsive ar mobile · 10.6 Checkout form · 10.7 Accessibility · 10.8 Phase 2: PWA · 10.9 BFF: Jaspr server er nijer endpoint |
| 11. Admin panel (Flutter web) | [11-admin.md](11-admin.md) | 11.1 Module ar route · 11.2 Role ar onumoti · 11.3 Code er niyom · 11.4 Guruttopurno screen · 11.5 Mobile e admin · 11.6 Print |
| 12. Security, privacy ar legal compliance | [12-security-legal.md](12-security-legal.md) | 12.1 Legal checklist (Bangladesh) · 12.2 Security checklist |
| 13. Infrastructure, deployment ar operations | [13-infra-ops.md](13-infra-ops.md) | 13.1 Environment · 13.2 Server · 13.3 Caddy · 13.4 Service (systemd) · 13.5 Config (env) · 13.6 Deploy (GitHub Actions) · 13.7 Backup · 13.8 Monitoring · 13.9 Shomossha hole (runbook) |
| 14. Testing ar limited launch plan | [14-testing-launch.md](14-testing-launch.md) | 14.1 Test er stor · 14.2 Je test gulo obosshoi thakbe · 14.3 Launch er age manual checklist · 14.4 Limited launch |
| 15. Build order: milestone onujayi | [15-milestones.md](15-milestones.md) |  |
| 16. Open decision, assumption ar reference | [16-open-decisions.md](16-open-decisions.md) | 16.1 Ekhono thik hoyni · 16.2 Assumption · 16.3 Reference |

## Version

*Ei document bodlale version number bariye niche ekta line jog koro: tarikh, ki bodlalo, keno.*

| Version | Tarikh | Bodol |
|---|---|---|
| 1.0 | 27 October 2026 | Prothom puro design |
| 1.1 | 28 October 2026 | Marketplace ar multi-tenant ready: `vendors`, `order_fulfillments`, tenant-safe foreign key, pricing e vendor onujayi bhag + `Reprice` + commission, route onujayi shop chena, duita state machine (1.6, 3.2, 3.7, 3.8, 8.5, 8.8, 8.18). Shathe review er fix: coupon phone limit er kram, CrossOriginProtection (admin origin, SSLCommerz return), random `tran_id`, packing e ojon komle total bare na, ojon-variable e `WeightG` = mot ojon |
| 1.2 | 28 October 2026 | Review er fix: client IP forward ar BFF (3.2, 10.9), payment dekhe confirm ar cascade cancel (8.8), refund shudhu fulfillment e, IPN validation/reconcile/duplicate payment (8.9), checkout er kram (8.6), OTP/TOTP attempt (8.2, 8.3), `001` e tenant-safe FK order er child e, `phone_bd`, anonymize, DB role, locale check, jobs `failed_at` (6), edit e coupon onupate (8.5), route conflict thik (7.5), Caddy/backup/firewall (13), legal update (12.1), test ar milestone (14, 15) |
| 1.3 | 28 October 2026 | Review er fix: order cancel er niche-r cascade `CancelCascade` (8.8), client IP Caddy te (3.2, 13.3), TOTP `failed_mfa` + `setup_pending` + soft lock (8.3, 7.8), OTP er dine cap (8.2), Turnstile skip e OTP verify ar idempotency hash e token na (8.6), `audit_logs.ip` grant ar production e role check (6.1), Go 1.27 toolchain (2.1), job sweeper (6.4, 8.17), payment lock er kram ar local status (8.9), Bot Fight Mode (8.9), pricing input validation (8.5), Bangla NFC search (6.4), upload write deadline (2.2), `refunds.kind`, `payments.store_amount`, complaint 72 ghonta (12.1), phone input (8.1), cache (10.3), Jaspr pin (2.1), docsync test, 59 ta DB test (14) |
| 1.4 | 29 October 2026 | Review er fix: idempotency hash e server er cart na, `expected_total` badhyotamulok (8.6); COD confirm alada `confirm_token`, shudhu SMS e (8.10, `CONFIRM_SECRET`); `Reprice` e je vendor er line bodlay nai tar taka freeze, percent e delta niyom, ৳0 er line er vendor er delivery thake (8.5); coupon er shorto (`orders.coupon_*`) ar `order_items.commission_bps` snapshot, snapshot chhara `Reprice` error (Rule 8); reconcile `reconcile_after` diye local `cancelled` payment o (8.9); state machine golden-table, pricing boundary ar guard test, 68 ta DB test (14) |
| 1.5 | 29 October 2026 | Review er fix: edit e vendor er shob item bad hole refund ekbar-i, kram shoho (8.8, 8.11), `staff` role e edit diye cancel/refund na (11.2); `order_items.variable_weight` snapshot, removed item e taka 0, `pricing.LineFromItem`/`EditWeightG`, `Reprice` e Original er jog-biyog check, fixed line e `WeightG` 0 (8.5, 6.7); coupon chhara discount na, coupon snapshot er check (6.7); TOTP `TOTP_KEY` + `totp_key_version` + AAD, `RECOVERY_SECRET`, conditional `totp_last_step` (8.3, 13.5); systemd `RestartSec`/`StartLimitIntervalSec`, `ADDR` loopback, backup bucket lock, `GOTOOLCHAIN` (13); token page e third-party script na, CSP hash, banner link parse, `intl` currency, admin heading font (9, 10, 12); BFSA 1 November ar Jaspr `^0.23.5` milano (2.1, 12.1); mutation test er fak bondho, docsync parser shokto, 78 ta DB test (protiti reject e constraint er naam) (14) |
| 1.6 | 29 October 2026 | Design ke section onujayi `docs/design/` e bhag (ager file 287 KB, Claude Code er Read er 256 KB shima periye jeto); schema, phone, pricing, order er code er copy bad, file-i asol (6.7, 8.1, 8.5, 8.8 e link); docsync ekhon link, file size (60 KB), code block (80 line), section er file ar index dekhe (14.2 test 28); migration runner: test `tests/` e (age embed e migration hishebe cholto), `embed.go`, `pg_advisory_xact_lock`, `set_config` (4.2, 6.1, 13.5, 15); Flutter card e `surface` (9.7); Claude Code setup: `CLAUDE.md`, `.claude/` (rules, hooks, skill, agent), `Makefile`, `scripts/` (4.1); review er 6 ta open decision (16.1) |
| 1.7 | 10 November 2026 | Review er fix: payment validation e shudhu response er `tran_id` er payment, VALID na hole kichu bodlay na (8.9, 12.2); `CONFIRM_SECRET` rotation er bhul dabi thik (8.10, 13.5); checkout OTP er consume order er transaction e, ar consume conditional (8.2, 8.6); risk flag e `expected_total` (8.6, 8.7); `pricing.ErrCouponNotApplicable`: coupon er eligible subtotal 0 ba `min_order` er kom hole `409 coupon_invalid` (8.5, 8.6; `TestCalculate` er duita case `TestCouponNotApplicable` e, error ar coupon chhara total duitai dekhe); `staff` role e fulfillment cancel ar refund wala return na (7.7, 8.8, 11.2); migration checksum (`schema_migrations.sha256`) ar `DEPLOYED` er update (6.1, 13.6); cancel e `GREATEST(used_count - 1, 0)` (8.8, 8.13); `001` er role check fail-closed (6.1); hasanta (juktakkhor) search er shima (6.4); SMS cap shudhu OTP (7.8); Go patch bump (2.1, 12.2); hook matcher e `MultiEdit`; edit → pack random test, 83 ta DB test (14); review er 2 ta open decision (16.1) |
| 1.8 | 10 November 2026 | Vendrix repo (`github.com/azharcse14/Vendrix`) er setup: Go module `vendrix/backend`; M0 er shuru code (`internal/config`, `internal/httpx`, `internal/db` er migration runner checksum shoho, `cmd/api` er serve/migrate/version, `/healthz`, graceful shutdown) ar test; `001` e `schema_migrations` e `shop_app` shudhu SELECT (6.1); `docs/features/` (feature doc, template, docsync e pahara) ar `docs/STATUS.md` (tracking) (4.7); Claude Code: SessionStart e STATUS, PostToolUse e format, Stop e STATUS check, `next`/`fix`/`feature-doc` skill, `security-reviewer` agent, notun rule (4.7); Dart: `packages/shared` path dependency, website alada resolution (4.6); `infra/` (docker compose, Caddyfile, systemd), CI workflow, `make db-up`/`migrate`/`run` (13) |
| 1.9 | 10 November 2026 | M0 review: `credentials set` M0 e (4.2 er map, 15 er moto); env.example er `local-only` secret local chhara reject (13.5) |
