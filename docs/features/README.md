# Feature doc

Protiti feature ekhon kivabe kaj kore: code er kothay, kon niyom kon test dhore, data, bhul hole ki hoy. Design (`docs/design/`) bole ki ar keno; ekhane kivabe ar kothay (4.7). Shudhu ekhon er obostha; itihash git e.

Notun feature: [_template.md](_template.md) copy, tarpor ei index ar niche-r code map e row (`feature-doc` skill). `backend/internal/docsync` test dekhe: dorkari heading, link, test er naam code e ache kina, ar `backend` er protiti package ei map e ache kina.

## Index

| Feature | Doc | Design | Milestone | Obostha |
|---|---|---|---|---|
| Server, config, healthz | [server.md](server.md) | 2.2, 4.2, 13.5 | M0 | cholche (pgx driver baki) |
| HTTP er bhitti: client IP, log, error | [http-foundation.md](http-foundation.md) | 3.2, 7.1, 12.2 | M0, M1 | chalu |
| Migration ar database | [migration.md](migration.md) | 6.1, 6.7, 13.6 | M0 | cholche (integration test baki) |
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
| `backend/internal/pricing` | Taka er hishab (pure) | [pricing.md](pricing.md) |
| `backend/internal/order` | Status er state machine (pure) | [order-status.md](order-status.md) |
| `backend/internal/phone` | Phone normalize | [phone.md](phone.md) |
| `backend/internal/docsync` | Doc er test | [doc-sync.md](doc-sync.md) |

`packages/shared` (M2), `website` (M3), `admin` (M7) er map oi milestone e ekhane jog hobe.

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
