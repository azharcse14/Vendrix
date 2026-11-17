# Admin: customer, phone block ar dashboard

Design: 7.7, 11.2, 11.7 item 12, 13 viii, 8.19 item 6, 12.2 · Milestone: M7a · Code: `backend/internal/customer`, `backend/internal/order/dashboard.go`

## Ki kore

Owner/admin shop er customer (account) er list (order count, mot kena) ar detail (profile, shesh order) dekhe, ar phone block/unblock kore. Block kora phone e COD checkout ar OTP send `403 phone_blocked` (M6 er niyom). Dashboard e shob role ajker order, bikri ar kaj er count dekhe (vendor nijer). Zone ar coupon admin [delivery-zone.md](delivery-zone.md) ar [coupon.md](coupon.md) e; staff [auth.md](auth.md), audit list [audit.md](audit.md) e. Admin screen (dashboard, `/customers`, block list): [admin-app.md](admin-app.md).

## Kivabe kaj kore

1. Router: `AdminSession` → `RequireRole("owner", "admin")` (11.2; customer data PII, `staff` `403`), `customer.Handlers.AdminRoutes`.
2. `GET /api/admin/customers?q=`: `q` trim, max 64; `phone.Normalize` hole phone hubohu, na hole naam er ongsho (`ILIKE`, `%`/`_` escape). `listCustomers`: deleted chhara user, `order_count` (shob order `user_id`), `order_total` (cancelled bad), `blocked`; paged, notun age. Guest order (user nai) order list er phone search e.
3. `GET /api/admin/customers/{id}`: `customerDetail` profile + `blocked` + shesh 50 order (`id`, `order_number`, `display_status`, `payment_method`, `payment_status`, `total`, `placed_at`); onno shop ba deleted `404`.
4. `GET /api/admin/blocked-phones`: paged, notun age (`phone`, `reason`, `staff_id`, `created_at`).
5. `POST /api/admin/blocked-phones {phone, reason?}`: phone normalize (Rule 6), reason trim max 200 (khali = `NULL`). `block`: `INSERT ... ON CONFLICT DO NOTHING`; notun hole `201` + audit `customer.block`, already thakle `200` ager row (write/audit na).
6. `DELETE /api/admin/blocked-phones {phone}` (phone URL/access log e na): `unblock` muchle audit `customer.unblock`, na thakleo `204`.
7. Audit (`Handlers.audit`): `entity_type` `customer`, `entity_id` oi phone er user (na thakle 0), `before`/`after` e `{reason, phone_masked}` (`phone.Mask`, shesh 3 digit; guest phone er unblock o chena jay); puro phone kokhono na (12.2, 11.7 item 13 x).
8. `GET /api/admin/dashboard` (shob role; `order.AdminDashboard` → `LoadDashboard`, ek query, v1.46): `date` shop er timezone e aj (Rule 5); `orders`/`sales` aj placed, cancelled bad, `sales` e online unpaid bad (COD ba `paid`/`partially_refunded`/`refunded`); `pending` = COD `pending` (je kono din); `to_pack` = confirmed order er `open` fulfillment; `low_stock` = `stock <= low_stock_at` (`/inventory/variants?low=1` er moto); `expiring` = `batch_expiry` on e `quantity_remaining > 0` ar `expiry_date` aj … aj + `inventory.expiry_alert_days` (default 7), off e 0; `deadline_soon` order list er tab er moto. Vendor role (`scope`): nijer fulfillment ache emon order, `sales` nijer fulfillment `total` (cancelled fulfillment bad), nijer variant.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/cmd/api/router.go` → `newHandler` | Route: admin session |
| `backend/internal/customer/handler.go` → `AdminRoutes`, `Customers`, `Customer`, `Blocked`, `Block`, `Unblock`, `decodeBlock`, `audit`, `auditJSON` | HTTP, validation, tx, audit |
| `backend/internal/customer/store.go` → `listCustomers`, `customerDetail`, `listBlocked`, `block`, `unblock`, `userByPhone` | SQL (protiti query `shop_id`) |
| `backend/internal/customer/customer_integration_test.go` | Asol Postgres e test |
| `backend/internal/order/dashboard.go` → `AdminDashboard`, `LoadDashboard`, `Dashboard` | Dashboard er count |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| List: deleted na, count shob, total cancelled bad, guest na, `q` phone/naam (wildcard escape) (11.7 item 12, v1.45) | `store.go` → `listCustomers` | `TestAdminCustomerIntegration` |
| Detail: profile, `blocked`, shesh 50 order; onno shop `404` (Rule 3) | `store.go` → `customerDetail` | `TestAdminCustomerIntegration`, `TestAdminShopIsolationIntegration` |
| Block e COD checkout `403 phone_blocked`, unblock e abar chole (8.19 item 6) | `block`/`unblock`; effect `order.PlaceOrder` | `TestAdminCustomerIntegration` |
| Abar block `200` (write na), na thaka unblock `204`; onno shop er block muche na (v1.45) | `handler.go` → `Block`, `Unblock` | `TestAdminCustomerIntegration`, `TestAdminShopIsolationIntegration` |
| Audit e puro phone na, `{reason, phone_masked}` (12.2) | `handler.go` → `audit`, `auditJSON` | `TestAdminCustomerIntegration` |
| Shudhu owner/admin (11.2) | `AdminRoutes` | `TestAdminCustomerIntegration`, `TestAdminRouteRoles` |
| Dashboard: shop timezone e aj, bikri e cancelled/online unpaid na, pending/pack/deadline je kono din, expiry `expiry_alert_days`, off e 0; vendor nijer (11.7 item 13 viii, v1.46, 14.2 test 14) | `order/dashboard.go` → `LoadDashboard` | `TestDashboardIntegration`, `TestAdminVendorIsolationIntegration` |

## Data

`users` (`phone`, `name`, `email`, `locale`, `deleted_at`), `orders` (`user_id`, `total`, `status`), `blocked_phones` (PK `(shop_id, phone)`, `reason`, `staff_id`, `created_at`), `audit_logs`; dashboard `orders`, `order_fulfillments`, `product_variants`, `inventory_batches`, `shop_settings.published` (`features`, `inventory.expiry_alert_days`). Migration lage na (`001`).

## Bhul hole ki hoy

- Bhul phone `400 fields.phone`, reason 200+ `fields.reason`, `q` 64+ `fields.q`, ochena field `fields.body`.
- Onno shop er ba deleted customer `404`; `staff`/vendor role `403`.
- Block kora customer er purono order ba session e kichu hoy na (v1.45); online payment e block lage na (8.19 item 6).

## Bodlanor age mone rakho

- Audit ba log e phone dile 12.2 bhange; `TestAdminCustomerIntegration` audit e phone khoje.
- Block er prabhab `order` (`phoneBlocked`) ar `auth` (`isBlocked`) e; ekhane bodlale oi duita o dekho.
- `q` e phone thake: API log shudhu route pattern (`httpx.Log`), query na; production Caddy log M12 e (13.3).

## Proshno-uttor

- **Dashboard e list na keno?** List ache: order list er tab ar `/inventory/variants?low=1`; dashboard shudhu count (v1.46, Claude decide, user bodlate paren).

- **Staff keno customer dekhe na?** 11.2 te customer block shudhu owner/admin; customer data PII, tai list/detail o (v1.45, Claude decide, user bodlate paren).
- **Abar block e error na keno?** Form duibar submit e bhul dekhano lage na; ager row (reason shoho) ferot ashe, audit hoy na.
