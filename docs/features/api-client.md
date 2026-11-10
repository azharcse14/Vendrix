# API client ar model (shared)

Design: 7.1, 7.2, 4.3 · Milestone: M2 · Code: `packages/shared/lib/src/api`, `packages/shared/lib/src/models`

## Ki kore

Website, admin ar app Go API er JSON ekoi Dart class e pore: product list ar detail, category tree, public settings, paging. Server er shape bodlale chupchap bhul data na, `FormatException`. Shob API bhul ekta `ApiError` e, app `code` dekhe kaj kore.

## Kivabe kaj kore

0. `ApiClient(baseUrl, client, locale:, headers:, timeout:)` (`api/api_client.dart`): `send` protiti request e `Accept-Language`, constructor er `headers` (jemon `X-Shop-Host`), body thakle JSON; `204` e `null`. Bhul: network → `ApiError(0, 'network_error')`, `timeout` (default 15 second) → `ApiError(0, 'timeout')`, JSON na (proxy er HTML 502) → asol status ar `internal`, 7.1 er `{"error":{...}}` → `code`, `message`, `fields`, `details`, `429` e `retryAfter`. Auto retry na (v1.12).
1. Protiti model er `fromJson` `field<T>` (`models/json.dart`) diye pore: type na mille `FormatException`; `number` JSON er `0` (int) ke `double` kore; `objects` list.
2. `Page.fromJson(json, item)` (`models/page.dart`): 7.1 er `{"items","page","limit","total"}`; `hasNext`.
3. `ProductCard`, `Product`, `Variant`, `ProductImage`, `Category` (`models/catalog.dart`): M1 er public response (lekha age-i locale e resolve, 7.1); dam poisha `int`.
5. `CatalogRepository` (`categories`, `products` filter shoho, `product`) ar `SettingsRepository` (`settings`, `version`, `translations`) (`api/repositories.dart`): 7.2 er route → model.
4. `PublicSettings` (`models/settings.dart`): `raw` JSON ar dorkari getter (`name`, `colors`, `defaultLocale`, `enabledLocales`, `banglaDigits`, `paymentMethods`); block na thakle default.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `packages/shared/lib/src/api/api_client.dart` → `ApiClient`, `send`, `get`, `ApiError` | HTTP, header, bhul |
| `packages/shared/lib/src/api/repositories.dart` → `CatalogRepository`, `SettingsRepository` | Route → model |
| `packages/shared/lib/src/models/json.dart` → `field`, `number`, `objects` | Typed JSON pora |
| `packages/shared/lib/src/models/page.dart` → `Page` | Paging |
| `packages/shared/lib/src/models/catalog.dart` → `ProductCard`, `Product`, `Variant`, `ProductImage`, `Category` | Catalog |
| `packages/shared/lib/src/models/settings.dart` → `PublicSettings` | Settings |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| M1 er list, detail, category JSON pora; paging; bhul type e `FormatException` (7.1, 7.2) | `Page`, `ProductCard`, `Product`, `Category` | `test/models/catalog_test.dart` → `group('Catalog model (7.1, 7.2, M1 er response)')` |
| Header, query, JSON body, `204`; 7.1 error, `fields`, `429 Retry-After`, HTML 502, network, timeout → `ApiError` (7.1, v1.12) | `ApiClient.send` | `test/api/api_client_test.dart` → `group('ApiClient (7.1, v1.12)')` |
| Repository er query (na deya filter jay na) ar model (7.2) | `CatalogRepository`, `SettingsRepository` | `test/api/api_client_test.dart` → `group('Repository (7.2)')` |
| Public settings er getter ar default (7.2, 9.2) | `PublicSettings` | `test/models/catalog_test.dart` → `group('PublicSettings (7.2, 9.2)')` |

## Data

Nai (API er JSON; [catalog.md](catalog.md), [shop.md](shop.md)).

## Bhul hole ki hoy

- Field er type mile na ba nai: `FormatException` (field er naam shoho).
- Server/network er bhul: `ApiError`; dekhate `Translator.t('error.${code}')`.

## Bodlanor age mone rakho

- Go er response e field jog/bodlale ekhane model ar `catalog_test.dart` er JSON o (M1 er local API theke copy).
- Timeout e request bondho hoy na, shudhu opekkha thame; POST e app nije abar pathabe na (idempotency, 8.6).
- Model shudhu je API ache tar (v1.12); cart, order, user tader milestone e.
- `attributes`, `seo` vendor er lekha: HTML/JSON-LD e boshate escape (M3).

## Proshno-uttor

- **Codegen (`json_serializable`) na keno?** 2.3 e runtime package shudhu `http`, `intl`; hand-written choto.
