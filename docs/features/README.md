# Feature doc

Protiti feature ekhon kivabe kaj kore: code er kothay, kon niyom kon test dhore, data, bhul hole ki hoy. Design (`docs/design/`) bole ki ar keno; ekhane kivabe ar kothay (4.7). Shudhu ekhon er obostha; itihash git e.

Notun feature: [_template.md](_template.md) copy, tarpor ei index ar niche-r code map e row (`feature-doc` skill). `backend/internal/docsync` test dekhe: dorkari heading, link, test er naam code e ache kina, ar `backend` er protiti package ei map e ache kina.

## Index

| Feature | Doc | Design | Milestone | Obostha |
|---|---|---|---|---|
| Server, config, healthz | [server.md](server.md) | 2.2, 4.2, 13.5 | M0 | cholche |
| HTTP er bhitti: client IP, log, error | [http-foundation.md](http-foundation.md) | 3.2, 7.1, 12.2 | M0, M1 | chalu |
| Migration ar database | [migration.md](migration.md) | 6.1, 6.7, 13.6 | M0 | chalu |
| Shop chena ar khola (`X-Shop-Host`, `shop create`) | [shop.md](shop.md) | 3.2, 3.8, 9.2 | M1 | cholche |
| Catalog: category, product, search | [catalog.md](catalog.md) | 6.4, 7.2, 7.7 | M1 | cholche |
| Audit log | [audit.md](audit.md) | 12 | M1 | cholche (category, product, variant) |
| Bhasha, taka ar tarikh (shared Dart) | [l10n.md](l10n.md) | 9.8 | M2 | cholche (taka, tarikh, translator) |
| Theme token ar contrast (shared Dart) | [theme.md](theme.md) | 9.3, 9.4 | M2 | cholche |
| API client ar model (shared Dart) | [api-client.md](api-client.md) | 7.1, 7.2 | M2 | cholche |
| Website (Jaspr): route, bhasha, API, layout | [website.md](website.md) | 10, 3.2, 13.5 | M3 | cholche (layout, home, category, product, search, policy) |
| SEO: meta, OG, JSON-LD, sitemap, robots | [seo.md](seo.md) | 10.4 | M3 | cholche |
| Shop er credential | [credentials.md](credentials.md) | 2.2, 3.8, 12.2 | M0 | chalu (pora M4 e) |
| Background job ar SMS | [jobs.md](jobs.md) | 6.4, 8.17 | M0 | chalu (SMS provider M4, alert M12) |
| Dam er hishab | [pricing.md](pricing.md) | 8.5 | checkout (M6), packing (M7) e use | chalu (code-i spec) |
| Order ar fulfillment er status | [order-status.md](order-status.md) | 3.7, 8.8 | M6, M7 e use | chalu (code-i spec) |
| Phone number | [phone.md](phone.md) | 8.1 | M4, M6 e use | chalu (code-i spec) |
| Doc er pahara (docsync) | [doc-sync.md](doc-sync.md) | 4.7, 14.2 test 28 | M0 | chalu |

## Code map (backend)

| Package | Ki | Feature doc |
|---|---|---|
| `backend/cmd/api` | Binary `shop-api`: serve, migrate, version; router, healthz | [server.md](server.md) |
| `backend/internal/config` | Env theke config, validation | [server.md](server.md) |
| `backend/internal/httpx` | Middleware, client IP, log, JSON error | [http-foundation.md](http-foundation.md) |
| `backend/internal/db` | Connection, transaction, migration runner | [migration.md](migration.md) |
| `backend/migrations` | SQL migration (embed), constraint test | [migration.md](migration.md) |
| `backend/internal/dbtest` | Asol Postgres e test er database | [migration.md](migration.md) |
| `backend/internal/shop` | Shop er data: resolve, credential encrypt ar save | [shop.md](shop.md), [credentials.md](credentials.md) |
| `backend/internal/catalog` | Category, product, variant: public read, search, admin CRUD | [catalog.md](catalog.md) |
| `backend/internal/audit` | Admin kaj er log, kaj er transaction e | [audit.md](audit.md) |
| `backend/internal/auth` | Staff password hash (login M4) | [shop.md](shop.md) |
| `backend/internal/jobs` | Job queue, worker, sweep | [jobs.md](jobs.md) |
| `backend/internal/notify` | SMS sender, `sms.send` | [jobs.md](jobs.md) |
| `backend/internal/pricing` | Taka er hishab (pure) | [pricing.md](pricing.md) |
| `backend/internal/order` | Status er state machine (pure) | [order-status.md](order-status.md) |
| `backend/internal/phone` | Phone normalize | [phone.md](phone.md) |
| `backend/internal/docsync` | Doc er test | [doc-sync.md](doc-sync.md) |

## Code map (packages/shared)

| Folder | Ki | Doc |
|---|---|---|
| `packages/shared/lib/format` | Taka ar tarikh format | [l10n.md](l10n.md) |
| `packages/shared/lib/l10n` | Translator, default lekha | [l10n.md](l10n.md) |
| `packages/shared/lib/validation` | Phone (Go er sathe ekoi) | [phone.md](phone.md) |
| `packages/shared/lib/theme` | Token derive, contrast | [theme.md](theme.md) |
| `packages/shared/lib/api` | `ApiClient`, `ApiError`, repository | [api-client.md](api-client.md) |
| `packages/shared/lib/models` | API er model (`LocalizedText` [l10n.md](l10n.md) e) | [api-client.md](api-client.md) |

## Code map (website)

| Folder | Ki | Doc |
|---|---|---|
| `website/lib/services` | Env, URL theke bhasha, request theke `ApiClient` | [website.md](website.md) |
| `website/lib` (`main.*.dart`, `app.dart`, `pages/`, `components/`) | Entry, route, page | [website.md](website.md) |
| `website/lib/seo` | Meta, OG, JSON-LD, sitemap, robots | [seo.md](seo.md) |

`admin` (M7) er map oi milestone e ekhane jog hobe.

## Shobdo

| Shobdo | Mane | Code e |
|---|---|---|
| Shop (tenant) | Ekta website ba brand: domain, theme, customer, payment | `shops`, `shop_id` |
| Vendor | Shop er bhetore seller; single store e shop nijei (house vendor) | `vendors`, `vendors.is_house` |
| Order | Customer er ek bar er kena ar payment | `orders`, `order.Status` |
| Fulfillment | Ek order e ek vendor er pack-ship-deliver | `order_fulfillments`, `order.FStatus` |
| Poisha | Taka × 100; shob jaygay integer | `int64`, `BIGINT` |
| Ojon-variable | Packing e asol ojon diye dam | `variable_weight`, `pricing.Line.WeightG` |
| Snapshot | Order er shomoyer naam, dam, coupon er copy | `order_items`, `orders.coupon_*` |
| Reprice | Packing ba edit er por notun hishab | `pricing.Reprice` |
