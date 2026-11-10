> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 4. Repository ar code structure

### 4.1 Monorepo

```
Vendrix/                    → repo (github.com/azharcse14/Vendrix), v1.8
  README.md                 → porichoy, local e chalano, Claude Code diye kaj
  AGENTS.md                 → AI agent er niyom: tracking, repo, section → file map, command
  CLAUDE.md                 → Claude Code: @AGENTS.md + kaj er kram (4.7)
  .claude/                  → settings (permission, hook), rules, skills, agents (4.7)
  Makefile, scripts/        → quick-check, check, db-test, rule check, local run, hook script
  docs/design/              → ei document, section onujayi file (README.md e suchi ar version)
  docs/features/            → protiti feature ekhon kivabe kaj kore, code er map (4.7)
  docs/milestones/          → milestone er plan ar progress (milestone skill)
  docs/STATUS.md            → ekhon kothay, porer kaj, proshno, session log (4.7)
  backend/                  → Go API (module vendrix/backend)
  packages/shared/          → Pure Dart: model, API client, l10n, format, validation (M2)
  website/                  → Jaspr customer website, server mode (M3)
  admin/                    → Flutter web admin panel (M7)
  app/                      → Flutter customer app (Phase 2)
  infra/                    → docker-compose (local Postgres), Caddyfile, systemd unit
  .github/                  → workflows/ci.yml, pull_request_template.md
```

### 4.2 Backend (Go)

```
backend/
  go.mod                    → module vendrix/backend; dependency shudhu github.com/jackc/pgx/v5 (M0)
  cmd/api/                  → main.go (serve, migrate, credentials set, version; pore shop create: 3.8),
                              router.go (route, middleware, healthz), server.go (timeout, shutdown)
  internal/
    config/                 → env theke config (os.Getenv), validation
    httpx/                  → middleware (recover, log, shop, auth, rate limit, client IP 3.2),
                              JSON read/write, error response, pagination
    db/                     → connection, transaction helper, migration runner
    auth/                   → OTP, session, staff password, TOTP
    shop/                   → shop resolve (3.2), provisioning, settings, translations, credentials
    vendor/                 → vendor, house vendor, vendor scope (Phase 3: KYC, ledger, payout)
    catalog/                → category, product, variant, image, search
    inventory/              → batch, movement, stock alert, expiry
    cart/                   → cart, wishlist
    pricing/                → subtotal, discount, delivery charge (pure function)
    coupon/
    order/                  → checkout, order ar fulfillment state machine, tracking, risk, confirmation
    payment/                → sslcommerz client, IPN, refund
    shipping/               → delivery zone, shipment, COD settlement, courier (Phase 2)
    review/
    customer/               → customer, block list, privacy request
    notify/                 → SMS template, sender
    storage/                → R2 (SigV4) upload
    jobs/                   → worker, job type
    report/
    audit/
    platform/               → Phase 3: platform admin, shop signup, domain, billing
    docsync/                → test: docs/design ar docs/features er link, size, code block, section ar package map (4.7)
  migrations/
    embed.go                → //go:embed *.sql (v1.6: shudhu migration; tests/ subfolder embed e ashe na)
    001_init.sql
    tests/001_init_test.sql → constraint test, psql diye (migration na)
```

Protiti domain folder e tinta layer:

| File | Kaj |
|---|---|
| `handler.go` | HTTP request porbe, validate korbe, service dakbe, JSON likhbe. Business niyom na |
| `service.go` | Asol niyom (pricing, state change). HTTP ba SQL jane na. Test ekhanei beshi |
| `store.go` | SQL query, shob `shop_id` diye filter |

`pricing` package ekdom pure function hobe (input diye output, database na), jate unit test shohoj hoy.

Router er udahoron:

```go
mux := http.NewServeMux()

// Public: shop ashe X-Shop-Host theke (3.2)
pub := httpx.ShopFromHost(shops)
mux.Handle("GET /api/products", pub(http.HandlerFunc(catalogH.List)))
mux.Handle("GET /api/products/{slug}", pub(http.HandlerFunc(catalogH.Get)))
mux.Handle("POST /api/orders", pub(http.HandlerFunc(orderH.Create)))

// Admin: shop, vendor ar role ashe staff session theke
adm := httpx.StaffSession(sessions)
mux.Handle("POST /api/admin/products",
	adm(httpx.RequireRole("owner", "admin", "vendor_owner")(http.HandlerFunc(catalogH.Create))))

// Payment: shop ashe tran_id theke (handler er bhetore)
mux.HandleFunc("POST /api/payments/sslcommerz/ipn", paymentH.IPN)
mux.HandleFunc("GET /api/payments/sslcommerz/return/{result}", paymentH.Return)
mux.HandleFunc("POST /api/payments/sslcommerz/return/{result}", paymentH.Return)
mux.HandleFunc("GET /healthz", healthH)

cop := http.NewCrossOriginProtection()
for _, o := range cfg.AdminOrigins { // ADMIN_ORIGINS, jemon https://admin.tomarshop.com
	if err := cop.AddTrustedOrigin(o); err != nil {
		log.Fatal(err)
	}
}
// SSLCommerz customer er browser theke cross-site POST kore return URL e
cop.AddInsecureBypassPattern("POST /api/payments/sslcommerz/return/{result}")

// v1.8: cmd/api/router.go er moto; Log Recover er bairer, tai panic er 500 o log e
handler := httpx.Chain(mux, httpx.RequestID, httpx.ClientIP(cfg.JasprInternalSecret),
	httpx.Log(log, mux), httpx.Recover(log), httpx.CORS(cfg.AdminOrigins), cop.Handler)
srv := &http.Server{Addr: cfg.Addr, Handler: handler,
	ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
	WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
```

Router build er ekta test rakho (M0): shob route register kore dekho. Go er ServeMux pattern conflict e panic kore; v1.1 e `GET /api/orders/by-token/{token}` ar `GET /api/orders/{token}/invoice` er conflict e server start-i hoto na (7.5 e thik kora).

### 4.3 Shared Dart package

```
packages/shared/lib/
  shared.dart
  models/        → Product, Variant, Category, Cart, Order, Settings, Theme ... (fromJson/toJson)
  api/           → ApiClient (http), ApiError, repository gulo
  l10n/          → bn.dart, en.dart (const Map, v1.12), Translator (key → lekha, override shoho)
  theme/         → token derive, contrast (9.3, v1.12)
  format/        → Money (poisha → "৳১,২৫০"), tarikh, phone
  validation/    → phone, naam, thikana
```

v1.12 (M2): model, repository ar validation API er sathe sathe ashe: M2 te shudhu M1 er public API (settings, translations, category, product list/detail, `Page`, `LocalizedText`); cart, order, user, admin ar naam/thikana tader milestone e (M4–M7). `ApiClient` `package:http` er `Client` inject kore; sob bhul `ApiError` (`code`): server er 7.1 body, network fail `status 0` `network_error`, timeout (default 15 second) `timeout`, JSON na (HTML 502) `internal` asol status shoho; auto retry na. Phone er Dart validation Go (`backend/internal/phone`, 8.1) er test case hubohu copy kore test kore; Go bodlale Dart o.

### 4.4 Website (Jaspr)

```
website/lib/
  main.server.dart / main.client.dart   → Jaspr CLI je entrypoint banay
  app.dart                               → router, layout
  pages/        → home, category, product, search, cart, checkout, order_done, cod_confirm,
                  track, order_view, account/*, policy, contact, invoice, not_found
  components/   → header, bottom_nav, product_card, price, gallery (@client),
                  variant_picker (@client), qty (@client), filter_sheet (@client)
  services/     → settings cache, session cookie, api (shared ApiClient), bff (10.9: Origin check, X-Client-IP)
  seo/          → meta, open_graph, json_ld, sitemap, hreflang
web/
  fonts/        → self-host kora Bangla font
  styles.css    → Tailwind output
```

### 4.5 Admin (Flutter web)

```
admin/lib/
  main.dart          → get_it setup, usePathUrlStrategy(), runApp
  di.dart            → ApiClient (BrowserClient, withCredentials), repository, router registration
  router.dart        → buildRouter(AuthBloc): go_router, setup/login/role guard (GoRouterRefreshStream, 11.3)
  core/theme/        → ThemeCubit (settings JSON → ThemeData)
  core/l10n/         → LocaleCubit
  features/
    auth/ dashboard/ products/ inventory/ categories/ orders/ packing/
    shipments/ cod/ coupons/ reviews/ customers/ zones/ settings/ vendors/
    translations/ staff/ audit/ reports/ complaints/
      → bloc/, pages/, widgets/
```

Niyom: `getIt<...>()` shudhu `di.dart` ar `BlocProvider(create: ...)` e. Widget er bhetore na. State `sealed class` diye. Simple state e `Cubit`, onek event e `Bloc`.

### 4.6 Dart package er jora (v1.8)

`packages/shared` pure Dart (Rule 10); website, admin ar app eta **path dependency** diye ney: `shared: {path: ../packages/shared}`. Website (Jaspr) er nijer resolution ar `pubspec.lock` (2.1): ek pub workspace e shob member ek Dart SDK e resolve hoy, tai website Flutter er Dart e atke jeto, ar Jaspr 1.0 er age druto bodlay (0.24.0-beta te Dart 3.13 lage). `admin` ar `app` (duitai Flutter) chaile ek workspace e. `scripts/quick-check.sh` je folder e `pubspec.yaml` ache ar bodleche shudhu shetai chalay (`packages/shared` bodlale shob app); `make check` ar CI te shob. Kono package pub.dev e publish hoy na, tai melos lage na.

### 4.7 Doc ar tracking (v1.8)

| Kothay | Ki | Ke bodlay |
|---|---|---|
| `docs/design/` | Ki ar keno (spec) | `design-change` skill: age doc, tarpor code |
| `docs/features/` | Feature ekhon kivabe kaj kore: code er map, niyom → test, data, proshno-uttor | Code er sathe ek commit e (`feature-doc` skill) |
| `docs/milestones/M<n>.md` | Milestone er plan, checkbox, proman | `milestone` skill |
| `docs/STATUS.md` | Ekhon kothay, porer kaj, user er kache proshno, session log | Protiti session: SessionStart hook dekhay, Stop hook dekhe |
| Code comment | Shudhu "keno"; itihash na | Code er sathe |
| Git commit | Ki bodlalo | `<type>(<milestone ba section>): ...` |

Pahara: `backend/internal/docsync` (feature doc er heading, link, test er naam, package map, STATUS er gothon), Stop hook (`quick-check`, ar code bodlale STATUS), `design-reviewer` agent (doc ar code mile kina). Claude Code er kram `CLAUDE.md` e; protected file (deploy hoya migration, hook, permission, secret) hook atkay.
