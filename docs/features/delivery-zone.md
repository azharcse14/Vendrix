# Delivery zone

Design: 7.2, 7.7, 8.6 step 5, 8.19 item 2, 11.7 item 13 vi · Milestone: M6, M7a · Code: `backend/internal/shipping`

## Ki kore

Checkout form `GET /api/delivery-zones` theke district ar area er list pay. Checkout (quote ar order, [checkout.md](checkout.md)) customer er district/area diye zone khuje charge, `min_order`, free delivery ar deadline er `same_city` nay (din `DeadlineDays`: same city 5, na hole 10, 8.15; order ar zone list ekoi function); zone na pele `409 area_not_served`. Owner/admin admin theke zone toiri ar bodlay (`/api/admin/delivery-zones`). Admin screen (`/zones`): [admin-app.md](admin-app.md).

## Kivabe kaj kore

1. Router: `ShopFromHost` → `RateLimit` (public), `GET /api/delivery-zones` → `Handlers.Zones`.
2. `List`: shop er `is_active` zone `district`, `area` er kram e; district e group, `area NULL` zone thakle `district_wide: true` (form e "Onno area" + free text). Area er `name` request er bhashay (`httpx.Locale`), `en` na thakle `bn`. Area na thakle `areas: []`. Protiti area e `days`, district e `district_wide_days` (district-wide na hole `null`) = `DeadlineDays(same_city)` (7.2 v1.35); checkout e order er age delivery er shomoy (10.6).
3. `Lookup` (`order.QuoteCart` ar `order.PlaceOrder` step 5, transaction er age; querier `*sql.DB` ba `*sql.Tx`): ek query te `(district, area)` hubohu ba oi district er `area NULL` zone, `ORDER BY area NULLS LAST` (area age). Bondho zone bad. Na pele `ErrAreaNotServed`.
4. Admin (`admin.go`, owner/admin, 11.2): `GET /api/admin/delivery-zones` (paged, inactive o, `district`, `area NULLS FIRST`, puro `{bn, en}` naam), `POST` (`201` row), `PATCH /{id}` (je field ache; `null` e `free_delivery_over`/`area` muche). `zoneIn.check`: `name.bn` lagbe (shudhu `bn`/`en`), `district`/`area` trim 1–100, `area` khali = `NULL` (puro district), `charge`/`min_order` ≥ 0, `free_delivery_over` > 0 ba `null`, create e `same_city` lagbe; `same_day_cutoff`/`own_rider` (Phase 2) ochena field `400`. Ek tx e row (PATCH e `FOR UPDATE` before) ar audit `zone.create`/`zone.update`. Duplicate (unique `23505`) `409 zone_exists`; delete na (`is_active`).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/cmd/api/router.go` → `newHandler` | Route: public middleware |
| `backend/internal/shipping/handler.go` → `Routes`, `Zones` | HTTP, JSON |
| `backend/internal/shipping/store.go` → `DeadlineDays`, `Lookup`, `List` | Din er niyom, SQL, fallback, group |
| `backend/internal/shipping/admin.go` → `AdminRoutes`, `AdminZones`, `writeZone`, `zoneIn.check`, `insertZone`, `updateZone` | Admin list, create, update, validation, audit |
| `scripts/dev-seed.sql` | Local zone (Dhaka er area + district-wide, Gazipur) |
| `backend/internal/shipping/zone_integration_test.go`, `admin_integration_test.go` | Asol Postgres e test |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Area age, na hole district-wide; bondho bad; string hubohu (8.19 item 2) | `store.go` → `Lookup` | `TestZoneLookupIntegration` |
| Onno shop er zone kokhono na (Rule 3) | `Lookup`, `List` er `shop_id` | `TestZoneLookupIntegration`, `TestDeliveryZonesIntegration` |
| List shape, shudhu active, naam bhashay, area/district-wide er din (7.2 v1.35) | `store.go` → `List` | `TestDeliveryZonesIntegration` |
| Delivery er din: same city 5, na hole 10 (8.15, 12.2) | `store.go` → `DeadlineDays` (order er `delivery_deadline` o) | `TestDeadlineDays` |
| Admin: owner/admin, trim, `area` khali = `NULL`, duplicate `409 zone_exists`, Phase 2 field `400`, delete na, audit (11.7 item 13 vi, v1.45) | `admin.go` | `TestAdminZoneIntegration` |
| Onno shop er zone admin e `404`, list e nai (14.2 test 5) | `admin.go` er `shop_id` | `TestAdminZoneIntegration`, `TestAdminShopIsolationIntegration` |

## Data

`delivery_zones` (001_init.sql): `district`, `area` (NULL = puro district), `charge`, `min_order`, `free_delivery_over` (NULL = nai, `Zone` e 0), `same_city`, `is_active`; `UNIQUE NULLS NOT DISTINCT (shop_id, district, area)`. Admin `/api/admin/delivery-zones` (M7a); local e `scripts/dev-seed.sql` o.

## Bhul hole ki hoy

- District/area e active zone nai: `ErrAreaNotServed` → quote (`order.QuoteCart`) ar order create (`order.PlaceOrder` step 5) e `409 area_not_served`.
- Ochena shop: middleware `404 shop_not_found`.
- Admin: bhul field `400 validation_failed` + `fields`; ekoi (district, area) `409 zone_exists`; onno shop er id `404`; `staff`/vendor role `403`.

## Bodlanor age mone rakho

- Form zone er `district`/`area` string-i pathay; case ba bhasha bodlale lookup district-wide e pore. Admin e mil case-sensitive ("Dhaka" ar "dhaka" duita zone hoy, v1.45); admin form e district list theke bachai bhalo. Naam dekhano `name` JSONB theke.
- `Zone.FreeDeliveryOver` 0 = nai (`pricing.Zone` er mane); bodlale `pricing.md` o.

## Proshno-uttor

- **"Onno area" te ki hoy?** Free text area kono zone er sathe mile na, tai oi district er `area NULL` zone e charge hoy. District-wide zone na thakle form e "Onno area" dekhay na.
