# API client ar model (shared)

Design: 7.1, 7.2, 7.3, 7.4, 7.5, 7.7, 8.19, 11.7, 4.3 · Milestone: M2, M4, M5, M6, M7a · Code: `packages/shared/lib/api`, `packages/shared/lib/models`

## Ki kore

Website, admin ar app Go API er JSON ekoi Dart class e pore: product list ar detail, category tree, public settings, paging; M7a theke admin API (order, inventory, staff ...). Server er shape bodlale chupchap bhul data na, `FormatException`. Shob API bhul ekta `ApiError` e, app `code` dekhe kaj kore.

## Kivabe kaj kore

1. `ApiClient(baseUrl, client, locale:, headers:, timeout:, token:, cartKey:)` (`api/api_client.dart`): `send` protiti request e `Accept-Language`, constructor er `headers` (jemon `X-Shop-Host`), `send(headers:)` shudhu oi request er (M6: `Idempotency-Key`), `token` thakle `Authorization: Bearer <token>` (M4, 10.11 item 4), `cartKey` thakle `X-Cart-Key` (M5, 7.1; website e `__Host-cart` theke), body thakle JSON; `204` e `null`. Bhul: network → `ApiError(0, 'network_error')`, `timeout` (default 15 second) → `ApiError(0, 'timeout')`, JSON na (proxy er HTML 502) → asol status ar `internal`, 7.1 er `{"error":{...}}` → `code`, `message`, `fields`, `details`, `429` e `retryAfter` (second). Body er shape bhul holeo `ApiError` (`code` na thakle `internal`, shudhu string `fields`); 2xx khali body `null`. Auto retry na (v1.12).
2. Protiti model er `fromJson` `field<T>` (`models/json.dart`) diye pore: type na mille `FormatException`; `number` JSON er `0` (int) ke `double` kore; `objects` list.
3. `Page.fromJson(json, item)` (`models/page.dart`): 7.1 er `{"items","page","limit","total"}`; `hasNext`.
4. `ProductCard`, `Product`, `Variant`, `ProductImage`, `Category` (`models/catalog.dart`): M1 er public response (lekha age-i locale e resolve, 7.1); dam poisha `int`.
5. `PublicSettings` (`models/settings.dart`): `raw` JSON ar dorkari getter (`name`, `colors`, `defaultLocale`, `enabledLocales`, `banglaDigits`, `paymentMethods`, font, radius, branding, `announcement`, `banners` (`Banner`: `imageKey`, `alt`, `link`; `image_key` string na hole item bad), `featuredCategorySlugs`, `featuredProductSlugs` (string na hole bad), `seoTitle`, `seoDescription`, `ogImageKey`, `noindex` (na thakle false), MU U5: `tagline`, `features` (string na hole bad), `returnWindowDays` (int ≥ 1 na hole null)); block na thakle default.
6. `CatalogRepository` (`categories`, `products` filter shoho, `product`, `policy` (v1.13: `body`)) ar `SettingsRepository` (`settings`, `version`, `translations`) (`api/repositories.dart`): 7.2 er route → model. `AccountRepository.me()` (M4): `GET /api/me` → `User` (`models/user.dart`: `phone`, `name`, `email`, `locale`; id nai, 7.3); token na thakle Go `401`.
7. M5 (7.4, 8.4): `CartRepository` (`cart`, `add(variantId, quantity)`, `setQuantity`, `remove`) shob route e puro `Cart`; `count()` (`GET /api/cart/count`, badge, v1.31) shudhu `int` (`models/cart.dart`: `items` (`CartItem`: `variantId`, `productSlug`, `name`, `variantLabel`, `image`, `unitPrice`, `compareAtPrice`, `variableWeight`, `quantity`, `maxQuantity`, `inStock`, `lineTotal`), `count`, `subtotal`, `notices` (`CartNotice`: `code`, `name`), notun guest cart e `cartKey`, na hole null). Dam/total client e hishab na (Rule 1). Bhul `ApiError` e: `out_of_stock` (`details.max_quantity`), `cart_full`, `not_found`. `WishlistRepository` (`wishlist`, `add`, `remove`, `merge(productIds)`; token lage) shob route e `List<ProductCard>` (Go er catalog card); 200 e `wishlist_full`. Verify er `cart_notices` website BFF `CartNotice.fromJson` diye pore (step 7).
8. M6 (7.2, 7.3, 7.5, 8.10, 8.19): `CheckoutRepository` (`zones()` → `List<DeliveryZone>` (`district`, `areas` (`DeliveryArea`: `area`, `name`, `days`), `districtWide`, `districtWideDays` (`int?`); din server er, 7.2 v1.35); `quote(body)` → `Quote` (`subtotal`, `discount`, `deliveryCharge`, `total`, `otpRequired`, `problems` (`error.<code>`; Go er `null` → khali)); `placeOrder(body, idempotencyKey:)` → `PlacedOrder` (`orderNumber`, `publicToken`, `status`, `paymentUrl`), header `Idempotency-Key`). Body 7.5 er JSON `Map` (BFF browser er body hubohu forward kore; typed input na), dam/total er field nai (Rule 1). `409 otp_required` (`details.phone_masked`), `price_changed` (`details.total` ...) shadharon `ApiError`. `OrderRepository` (`byToken` → `Order`, `cancel(token, reason:)` (body `{}` ba `{reason}`) ar `confirm` → notun status `String`, `track(number, phone)` (POST body) → `TrackedOrder` (`history`: `OrderEvent` `status`, `at`; `tracking` server ekhono `null` dey, pora hoy na), `confirmView` → `ConfirmSummary`, `myOrders(page:, limit:)` → `Page<OrderSummary>`). Model `models/order.dart`: `ConfirmSummary` (mask kora: `phoneMasked`, `district`, `area`; `items` `OrderItem` `name`/`variantLabel` `LocalizedText`), `Order extends ConfirmSummary` (Go `View` `ConfirmSummary` embed kore: `recipientName`, `recipientPhoneMasked`, taka, `deliveryDeadline?`, `canCancel`), `OrderSummary` (`orderNumber`, `publicToken`, `displayStatus`, `total`, `placedAt`); shomoy `DateTime.parse` (bhul hole `FormatException`). Token path e `Uri.encodeComponent`.

9. M7a (7.7, 11.7; step 13): `ApiClient.multipart(path, fields:, files:, timeout:)` `POST` `MultipartRequest`, header ar bhul `send` er moto (ekoi `_send`), timeout default 2 minute (server er upload deadline). Admin model `models/admin.dart`: `StaffProfile` (`/me`), `Staff` (`tempPassword` shudhu create e), `AdminOrder` (vendor role e shop view er field null), `OrderDetail extends AdminOrder` (`Fulfillment` → `AdminItem`, `HistoryEntry`, `OrderNote`; vendor e `notes` khali), `FulfillmentRow` (v1.50: `shipment` active shipment ba null), `Shipment` (PATCH e shudhu `id`, `status`, `displayStatus`; row er `shipment` e `displayStatus` null), `PickList` (`PickVariant`, `PickArea`, `PickFulfillment`), `InventoryVariant` (`variantLabel` = `options`: `Map<String, LocalizedText>`), `Batch`/`Movement` (`costPrice` `staff`/`vendor_staff` e null), `Coupon`, `Zone`, `Customer` (list e `orderCount`/`orderTotal`, detail e `orders` `CustomerOrder`), `BlockedPhone`, `AuditEntry` (`before`/`after` je kono JSON), `Dashboard`, `AdminImage`; M7b step 4: `AdminCategory` (`parentId`, `name` `LocalizedText`, `isActive`), `AdminProduct` (list row e `variants`/`images` khali, detail e `AdminVariant`/`AdminImage`; SEO/`attributes` model e nai), `AdminVariant` (`options` `Map<String, LocalizedText>`, `price`/`compareAtPrice`/`costPrice` poisha). Taka `int`, din (`YYYY-MM-DD`, shop timezone) `String`, shomoy `time`/`timeOrNull` (`models/json.dart`; offset `+06:00` holeo UTC `DateTime`). Repository `api/admin_repositories.dart`: `AdminAuthRepository` (`login` → record `totpRequired`/`tempToken`/`setupRequired`, `totp`/`recovery` → `setup_required`, `changePassword`, `totpSetup`, `totpEnable` → recovery code, `logout`, `me`), `AdminOrderRepository` (`orders` filter, `order`, `transition`, `addNote`, `editItems(id, {itemId: qty}, note)`, `fulfillments`, `transitionFulfillment` (`restockItemIds`), `pack(id, {itemId: gram}, customerOkNote:)`, `ship`, `updateShipment`, `pickList`, `printLink` → record `url`/`expiresAt`), `InventoryRepository` (`variants`, `batches`, `createBatch`, `adjust` → notun stock, `movements`), `AdminCatalogRepository` (`categories` (array, paging na), `createCategory`/`updateCategory(id, body)`/`deleteCategory`, `products` (page), `product(id)`, `createProduct`/`updateProduct`, `createVariant(productId, body)`/`updateVariant`; body 6.7 er field `Map`, PATCH e `null` = muche; chobi `uploadImage` multipart `file`/`alt_bn`/`alt_en`, `orderImages`, `deleteImage`), `AdminZoneRepository`/`AdminCouponRepository` (list, `create(body)`, `update(id, body)`; body 6.7 er field `Map`), `AdminCustomerRepository` (`customers`, `customer`, `blockedPhones`, `block`, `unblock` phone body te), `StaffRepository` (`staff`, `create` → `tempPassword`, `update`, `reset` → temp password), `AuditRepository.logs`, `DashboardRepository.dashboard`. Na deya filter query te jay na (`_query`). Staff session cookie (11.3): admin `BrowserClient` `withCredentials`, `token` na.

Website e (M3, [website.md](website.md)): protiti page request e notun `ApiClient` (`apiFor`, ek `http.Client`): `X-Shop-Host` = `Host`, `Accept-Language` URL er bhasha, Caddy er `X-Real-IP` thakle `X-Client-IP` + `X-Internal-Auth`. Settings er `SettingsRepository` process er cache e (`settingsCache`, 3.5). M4: `/account` `__Host-sid` cookie er token diye `ApiClient(token:)`; `@client LoginForm` browser e `ApiClient(Uri.base, http.Client())` diye shudhu BFF `/_/...` e (Go API te na, 10.9).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `packages/shared/lib/api/api_client.dart` → `ApiClient`, `send`, `get`, `multipart`, `_send`, `ApiError` | HTTP, header, bhul |
| `packages/shared/lib/api/repositories.dart` → `CatalogRepository`, `SettingsRepository`, `AccountRepository`, `CartRepository`, `WishlistRepository`, `CheckoutRepository`, `OrderRepository` | Route → model |
| `packages/shared/lib/api/admin_repositories.dart` → `AdminAuthRepository`, `AdminOrderRepository`, `InventoryRepository`, `AdminCatalogRepository`, `AdminZoneRepository`, `AdminCouponRepository`, `AdminCustomerRepository`, `StaffRepository`, `AuditRepository`, `DashboardRepository` | Admin route → model (M7a) |
| `packages/shared/lib/models/json.dart` → `field`, `number`, `objects`, `time`, `timeOrNull` | Typed JSON pora |
| `packages/shared/lib/models/page.dart` → `Page` | Paging |
| `packages/shared/lib/models/catalog.dart` → `ProductCard`, `Product`, `Variant`, `ProductImage`, `Category` | Catalog |
| `packages/shared/lib/models/settings.dart` → `PublicSettings` | Settings |
| `packages/shared/lib/models/user.dart` → `User` | Customer profile |
| `packages/shared/lib/models/cart.dart` → `Cart`, `CartItem`, `CartNotice` | Cart (M5) |
| `packages/shared/lib/models/order.dart` → `DeliveryZone`, `DeliveryArea`, `Quote`, `PlacedOrder`, `ConfirmSummary`, `OrderItem`, `Order`, `TrackedOrder`, `OrderEvent`, `OrderSummary`, `BadgeTone`, `orderStatusTones`, `orderStatusTone` | Checkout ar order (M6); status badge er rong (MU U9) |
| `packages/shared/lib/models/admin.dart` → `StaffProfile`, `Staff`, `AdminOrder`, `OrderDetail`, `Fulfillment`, `AdminItem`, `HistoryEntry`, `OrderNote`, `FulfillmentRow`, `Shipment`, `PickList`, `InventoryVariant`, `Batch`, `Movement`, `Coupon`, `Zone`, `Customer`, `CustomerOrder`, `BlockedPhone`, `AuditEntry`, `Dashboard`, `AdminImage` | Admin API er response (M7a) |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| M1 er list, detail, category JSON pora; paging; bhul type e `FormatException` (7.1, 7.2) | `Page`, `ProductCard`, `Product`, `Category` | `test/models/catalog_test.dart` → `group('Catalog model (7.1, 7.2, M1 er response)')` |
| Header, query, JSON body, `204`; 7.1 error, `fields`, `429 Retry-After`, HTML 502, network, timeout → `ApiError` (7.1, v1.12) | `ApiClient.send` | `test/api/api_client_test.dart` → `group('ApiClient (7.1, v1.12)')` |
| Repository er query (na deya filter jay na) ar model (7.2) | `CatalogRepository`, `SettingsRepository` | `test/api/api_client_test.dart` → `group('Repository (7.2)')` |
| `token` thakle `Authorization: Bearer`, na thakle header nai; `me()` → `User`, bhul shape `FormatException` (7.3, 10.11 item 4) | `ApiClient.send`, `AccountRepository`, `User` | `test/api/api_client_test.dart` → `group('Account (7.3, 10.11 item 4)')` |
| `cartKey` thakle `X-Cart-Key`, na thakle nai; cart route er method/path/body; `Cart`/`CartItem`/`CartNotice` parse, `cart_key` shudhu notun cart e; `count()` path ar `int`; wishlist card list ar merge body; `out_of_stock` `max_quantity`, `cart_full`, `wishlist_full`; bhul shape `FormatException` (7.4, 8.4) | `ApiClient.send`, `CartRepository`, `WishlistRepository`, `Cart` | `test/api/api_client_test.dart` → `group('Cart ar wishlist (7.4, 8.4, M5)')` |
| Status badge: `order.status.*` er protiti key er `BadgeTone` (beshi na), `delivered` success, `cancelled` danger, `pending` warning, `confirmed` = `open`, ochena `muted` (10.5 v1.38, 8.8) | `orderStatusTones`, `orderStatusTone` | `test/models/order_test.dart` → `group('Status badge (10.5 v1.38, 8.8)')` |
| Checkout ar order er Go JSON pora (quote, placed, confirm summary, order view, track, me/orders card, delivery zone); `problems` null, `delivery_deadline` null, khali `variant_label`; bhul type/tarikh `FormatException` (7.2, 7.3, 7.5, 8.19) | `models/order.dart` | `test/models/order_test.dart` → `group('Checkout ar order model (7.2, 7.3, 7.5, 8.19, M6)')` |
| Checkout/order route er method, path, body, query; `Idempotency-Key` header, Bearer, `X-Cart-Key`; `409 otp_required`/`price_changed` er `details`, `404` (7.5, 8.6, 8.10) | `ApiClient.send(headers:)`, `CheckoutRepository`, `OrderRepository` | `test/api/api_client_test.dart` → `group('Checkout ar order (7.2, 7.3, 7.5, 8.6, 8.10, M6)')` |
| Admin Go JSON pora (step 2–12 er shape): vendor role e shop view null, `+06:00` shomoy UTC, nullable field, `options` er label, `cost_price` na thakle null, `temp_password` shudhu create e; bhul type `FormatException` (7.1, 7.7, 11.7) | `models/admin.dart` | `test/models/admin_test.dart` → `group('Admin model (7.1, 7.7, 11.7, M7a step 13)')` |
| Catalog admin row: category parent/inactive, product list row e variant/chobi khali, detail e variant (taka poisha, `options`) ar chobi (7.7, 6.7) | `AdminCategory`, `AdminProduct`, `AdminVariant` | `test/models/admin_test.dart` → `group('Catalog admin (7.7, 6.7, 11.7 item 13 ix)')` |
| Admin login/TOTP/me route ar body (7.7, 8.3) | `AdminAuthRepository` | `test/api/admin_repositories_test.dart` → `group('AdminAuthRepository (7.7, 8.3)')` |
| Order list filter query, detail, transition, note, edit, fulfillment, return restock, pack, shipment, pick list, print link (7.7, 8.8, 8.11, 11.7) | `AdminOrderRepository` | `test/api/admin_repositories_test.dart` → `group('AdminOrderRepository (7.7, 8.8, 8.11, 11.7)')` |
| Inventory variant/batch/movement query, batch ar adjust body (8.12, 11.7 item 1, 2) | `InventoryRepository` | `test/api/admin_repositories_test.dart` → `group('InventoryRepository (8.12, 11.7 item 1, 2)')` |
| Chobi multipart (`file`, `alt_bn`; na deya alt jay na), order, delete (11.7 item 9) | `ApiClient.multipart`, `AdminCatalogRepository` | `test/api/admin_repositories_test.dart` → `group('AdminCatalogRepository: chobi (11.7 item 9)')` |
| Category list (array), create, update, delete; product page, detail (variant), create, update; variant create/update (`null` pathay); `409 sku_taken` `ApiError` (7.7) | `AdminCatalogRepository` | `test/api/admin_repositories_test.dart` → `group('AdminCatalogRepository: product, variant, category (7.7)')` |
| Zone/coupon list, create, update; `409 coupon_exists` `ApiError` (11.7 item 13 v, vi) | `AdminZoneRepository`, `AdminCouponRepository` | `test/api/admin_repositories_test.dart` → `group('Zone, coupon (11.7 item 13 v, vi)')` |
| Customer, block; unblock e phone body te, URL e na (11.7 item 12, 12.2) | `AdminCustomerRepository` | `test/api/admin_repositories_test.dart` → `group('AdminCustomerRepository (11.7 item 12)')` |
| Staff list, create (`temp_password`), update (shudhu deya field), reset (11.7 item 11) | `StaffRepository` | `test/api/admin_repositories_test.dart` → `group('StaffRepository (11.7 item 11)')` |
| Audit filter query, dashboard (11.7 item 11, 13 viii) | `AuditRepository`, `DashboardRepository` | `test/api/admin_repositories_test.dart` → `group('Audit, dashboard (11.7 item 11, 13 viii)')` |
| Public settings er getter ar default (7.2, 9.2) | `PublicSettings` | `test/models/catalog_test.dart` → `group('PublicSettings (7.2, 9.2)')` |
| Theme, branding, homepage (banner, featured), tagline, features, return window er getter; bhul type e default ba item bad (9.2, 10.5 v1.38) | `PublicSettings`, `Banner` | `test/models/settings_test.dart` → `group('PublicSettings (9.2)')` |

## Data

Nai (API er JSON; [catalog.md](catalog.md), [shop.md](shop.md)).

## Bhul hole ki hoy

- Field er type mile na ba nai: `FormatException` (field er naam shoho).
- Server/network er bhul: `ApiError`; dekhate `Translator.t('error.${code}')`.

## Bodlanor age mone rakho

- Go er response e field jog/bodlale ekhane model ar `catalog_test.dart` er JSON o (M1 er local API theke copy).
- Timeout e request bondho hoy na, shudhu opekkha thame; POST e app nije abar pathabe na (idempotency, 8.6).
- Model shudhu je API ache tar (v1.12). Cart er Go response (`cart/service.go` er `Cart`, `Item`) bodlale `cart.dart` ar test er JSON o; order er (`order/service.go` `Quote`/`Placed`, `confirm.go`, `view.go`, `shipping/store.go`) bodlale `order.dart` ar `order_test.dart` er JSON o; admin er (`order/store.go`, `inventory/store.go`, `auth/staff_admin.go`, `customer/store.go`, `audit/handler.go`, `*/admin.go`) bodlale `admin.dart` ar `admin_test.dart` er JSON o.
- `attributes`, `seo` vendor er lekha: HTML/JSON-LD e boshate escape (M3).

## Proshno-uttor

- **Codegen (`json_serializable`) na keno?** 2.3 e runtime package shudhu `http`, `intl`; hand-written choto.
- **Admin product model e SEO/`attributes` nai keno?** Admin form e ekhon nai (M7b step 4 decision); PATCH e na pathale bodlay na. Form e ele model e field.
- **One-off uttor (login, TOTP setup, print link) record keno?** Class er dorkar nai; `field<T>` diye kora pora tobu-o.
