# Pick list ar print link

Design: 8.11 item 1, 5, 11.6, 11.7 item 4, 8 (v1.44), 7.7 (Packing), 10.1, 13.5 · Milestone: M7a · Code: `backend/internal/order` (`picklist.go`, `print.go`)

## Ki kore

Packing er staff `GET /api/admin/packing/pick-list` e ek diner (shop er timezone) kon variant koto ta tulte hobe dekhe: mot, area onujayi bhag, ar kon fulfillment gulo. Packing slip, invoice ar pick list print korte admin 10 minute er signed link nay (`POST /api/admin/print-links`); website er `/print/{kind}?token=` page (M7a step 14) `GET /api/print?kind=&token=` theke data ane. Vendor role shudhu nijer vendor er pick list ar packing slip; invoice na.

## Kivabe kaj kore

1. `picklist.go` → `AdminPickList`: `date` `YYYY-MM-DD` (na hole `400 fields.date`), khali = `shopToday` (shop er aj). `LoadPickList`: `open` fulfillment, order `confirmed`, `confirmed_at <` oi diner porer din er shuru (shop timezone) — tai ager diner baki o ashe; `confirmed_at` NULL bad. Shudhu `active` item. Go te variant onujayi `quantity` ar ojon-variable e `ordered_weight_g` jog, `(district, area)` onujayi alada jog, fulfillment `confirmed_at` kram e. Vendor scope `scope(ByStaff)`.
2. `print.go` → `AdminPrintLink`: body `{kind, order_ids | date}`; `packing_slip`/`invoice` e 1–50 alada positive id, `date` na; `pick_list` e `order_ids` na, `date` na dile `shopToday` token e (midnight er por khulleo ekoi list). Vendor role e `invoice` `403`. Token `PrintToken{shop_id, kind, ids|date, staff_id, vendor_id?, exp}`, `SignPrint`; url = primary domain (`shop_domains.is_primary`) `https://<domain>/print/<kind>?token=`; audit `print.link` (`entity_type` = kind, after `{kind, count}` ba `{kind, date}`); `200 {url, expires_at}`.
3. `SignPrint`/`VerifyPrint`: `base64url(json) + "." + base64url(HMAC-SHA256(PRINT_SECRET, "print:" + payload))` (padding chhara). Verify: signature `hmac.Equal` JSON porar age, ochena field na, `exp` (unix second) por na, `shop_id`/`staff_id` > 0, kind er sathe ids|date mile.
4. `print.go` → `Print` (public, `ShopFromHost` er pore) → `LoadPrint`: verify, token `kind` = `?kind=`, token shop = host er shop, invoice token e `vendor_id` na, issuing staff ekhono `is_active`; tarpor data abar scope e: `loadSlips` (fulfillment onujayi, cancelled fulfillment ar removed item bad, vendor e shudhu nijer, `cod_amount` COD e fulfillment `total`), `loadInvoices` (puro order, cancelled order bad, taka order er snapshot theke), `pick_list` e `LoadPickList`. Kichu na thakle ba kono check fail e `404 not_found`. Response `no-store`.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/order/picklist.go` → `AdminPickList`, `LoadPickList`, `shopToday` | Pick list route, query, jog |
| `backend/internal/order/print.go` → `SignPrint`, `VerifyPrint`, `validPrintShape`, `printIDsField` | Token |
| `print.go` → `AdminPrintLink` | Link banano, audit |
| `print.go` → `Print`, `LoadPrint`, `loadSlips`, `loadInvoices` | Public print data |
| `backend/internal/order/admin.go` → `AdminRoutes` | `GET /api/admin/packing/pick-list`, `POST /api/admin/print-links` (shob 5 role) |
| `backend/internal/order/handler.go` → `Routes`, `Guard.PrintSecret` | `GET /api/print` |
| `backend/internal/config/config.go` → `Load` | `PRINT_SECRET` (32+ okkhor, `local-only` shudhu local e) |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Pick list shop timezone e, ager diner baki o, packed/pending/removed bad, variant ar area jog, vendor nijer, bhul date `400` (8.11 item 1, 11.7 item 4) | `LoadPickList`, `AdminPickList` | `TestPickListIntegration` |
| Token er akar, HMAC `print:` prefix, `exp`, bhul signature/secret/payload, kind ar ids|date (11.7 item 8) | `SignPrint`, `VerifyPrint` | `TestPrintToken` |
| Link: kind, max 50, field er mil, vendor invoice `403`, url primary domain, 10 minute, audit; render: kind/shop/expire/staff inactive `404`, vendor scope, data shape, `no-store` (11.7 item 8, v1.44) | `AdminPrintLink`, `LoadPrint` | `TestPrintLinkIntegration` |
| Vendor onno vendor er pick list ba slip pay na (Rule 13) | `scope`, `loadSlips` | `TestAdminVendorIsolationIntegration` |
| Route er role (11.2) | `AdminRoutes` | `TestAdminRouteRoles` |
| `PRINT_SECRET` lagbe (13.5) | `config.Load` | `TestLoadRejects`, `TestRejectExampleSecrets` |

## Data

Kono table e lekha na (link stateless, migration nai). Pore: `order_fulfillments`, `orders` (`confirmed_at`, ship_*), `order_items` (snapshot), `shop_domains` (`is_primary`), `shops.timezone`, `staff.is_active`. Lekha: shudhu `audit_logs` (`print.link`).

## Bhul hole ki hoy

- Pick list `date` bhul: `400 validation_failed` `fields.date`.
- Print link body bhul: `400` `fields.kind` / `order_ids` / `date` / `body`; vendor role e invoice `403 forbidden`.
- `GET /api/print` e token bhul, expire, onno kind, onno shop, staff inactive, ba scope e kichu nai: shob `404 not_found` (karon bola hoy na).
- `PRINT_SECRET` nai ba choto: `serve` start hoy na.

## Bodlanor age mone rakho

- Token er JSON field bodlale purono link (10 minute) bhenge jay; website step 14 er page `kind` query pathay.
- `PRINT_SECRET` bodlale chalu link bondho (10 minute er moddhe kichu na).
- Pick list er din `confirmed_at` diye, `GET /fulfillments` er `date` `placed_at` diye: duita alada.
- Caddy log e `/print*` ar `/api/print*` er query string jabe na (13.3; step 14).

## Proshno-uttor

- **Link kar jonno?** Je staff banalo tar onumoti token e; se deactivate hole tar link o bondho.
- **Raat 12 tar por pick list link khulle?** Token e issue er shomoyer tarikh, tai ekoi list.
- **Vendor invoice keno na?** Invoice e puro order er taka ar customer er tottho; vendor shudhu nijer packing slip.
