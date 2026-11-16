# Delivery zone

Design: 7.2, 8.6 step 5, 8.19 item 2 · Milestone: M6 · Code: `backend/internal/shipping`

## Ki kore

Checkout form `GET /api/delivery-zones` theke district ar area er list pay. Checkout (quote ar order, [checkout.md](checkout.md)) customer er district/area diye zone khuje charge, `min_order`, free delivery ar deadline er `same_city` nay; zone na pele `409 area_not_served`.

## Kivabe kaj kore

1. Router: `ShopFromHost` → `RateLimit` (public), `GET /api/delivery-zones` → `Handlers.Zones`.
2. `List`: shop er `is_active` zone `district`, `area` er kram e; district e group, `area NULL` zone thakle `district_wide: true` (form e "Onno area" + free text). Area er `name` request er bhashay (`httpx.Locale`), `en` na thakle `bn`. Area na thakle `areas: []`.
3. `Lookup` (`order.QuoteCart` ar `order.PlaceOrder` step 5, transaction er age; querier `*sql.DB` ba `*sql.Tx`): ek query te `(district, area)` hubohu ba oi district er `area NULL` zone, `ORDER BY area NULLS LAST` (area age). Bondho zone bad. Na pele `ErrAreaNotServed`.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/cmd/api/router.go` → `newHandler` | Route: public middleware |
| `backend/internal/shipping/handler.go` → `Routes`, `Zones` | HTTP, JSON |
| `backend/internal/shipping/store.go` → `Lookup`, `List` | SQL, fallback, group |
| `scripts/dev-seed.sql` | Local zone (Dhaka er area + district-wide, Gazipur) |
| `backend/internal/shipping/zone_integration_test.go` | Asol Postgres e test |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Area age, na hole district-wide; bondho bad; string hubohu (8.19 item 2) | `store.go` → `Lookup` | `TestZoneLookupIntegration` |
| Onno shop er zone kokhono na (Rule 3) | `Lookup`, `List` er `shop_id` | `TestZoneLookupIntegration`, `TestDeliveryZonesIntegration` |
| List shape, shudhu active, naam bhashay (7.2) | `store.go` → `List` | `TestDeliveryZonesIntegration` |

## Data

`delivery_zones` (001_init.sql): `district`, `area` (NULL = puro district), `charge`, `min_order`, `free_delivery_over` (NULL = nai, `Zone` e 0), `same_city`, `is_active`; `UNIQUE NULLS NOT DISTINCT (shop_id, district, area)`. M6 e zone shudhu seed; admin (`/api/admin/delivery-zones`) M7 e.

## Bhul hole ki hoy

- District/area e active zone nai: `ErrAreaNotServed` → quote (`order.QuoteCart`) ar order create (`order.PlaceOrder` step 5) e `409 area_not_served`.
- Ochena shop: middleware `404 shop_not_found`.

## Bodlanor age mone rakho

- Form zone er `district`/`area` string-i pathay; case ba bhasha bodlale lookup district-wide e pore. Naam dekhano `name` JSONB theke.
- `Zone.FreeDeliveryOver` 0 = nai (`pricing.Zone` er mane); bodlale `pricing.md` o.

## Proshno-uttor

- **"Onno area" te ki hoy?** Free text area kono zone er sathe mile na, tai oi district er `area NULL` zone e charge hoy. District-wide zone na thakle form e "Onno area" dekhay na.
