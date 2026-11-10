# Catalog: category, product, search

Design: 6.4, 6.7, 7.1, 7.2, 7.7 · Milestone: M1 · Code: `backend/internal/catalog`

## Ki kore

Website/app shop er category tree, product list (filter, sort, paging, search) ar product detail pay; lekha `Accept-Language` er bhasha e (na thakle bn). Protiti query shudhu oi shop er (Rule 3).

## Kivabe kaj kore

1. `httpx.ShopFromHost` shop context e rakhe (shop.md); handler `httpx.ShopFrom` er `ID` diye query kore.
2. `GET /api/categories` (`Handlers.Categories`): `categories` (active, `sort_order`) → `tree` (parent → children; inactive parent er child o bad), naam `Text.In(httpx.Locale(r))`.
3. `GET /api/products` (`Handlers.Products`): `httpx.Page`, query check (`min_price`/`max_price` poisha ≥ 0, `in_stock` bool, `sort` `newest|price_asc|price_desc|popular`, `q` max 100 okkhor) → `searchTerm` → `listProducts`. Shudhu active product jar ontoto ekta active variant ache; card er `price` sobcheye kom active variant er dam, `in_stock` kono active variant e `stock > 0`.
4. Search (6.4): `q` 2 okkhorer kom hole search na (list cholbe). Go te `\ % _` escape kore `ILIKE '%' || normalize(q, NFC) || '%'` (`search_text` generated, NFC); `q` thakle ar `sort` na dile `word_similarity` diye relevance.
5. `GET /api/products/{slug}` (`Handlers.Product`): `productBySlug`: active product, active variant (`sort_order`), chobi (`sort_order`); `seo` locale er ongsho; `rating` = `rating_sum/review_count` (ek dosomik).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/catalog/handler.go` → `NewHandlers`, `Categories`, `Products`, `Product`, `tree`, `rating`, `seoIn` | Public HTTP |
| `backend/internal/catalog/service.go` → `Text`, `In`, `searchTerm`, `validSlug`, `makesCycle` | Niyom |
| `backend/internal/catalog/store.go` → `categories`, `listProducts`, `productBySlug` | SQL |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Public lekha `Accept-Language` er bhasha e, khali hole `bn` (7.1) | `Text.In` | `TestLocalizedText` |
| Category tree: shudhu active, `sort_order`, naam locale e (7.2) | `Categories`, `tree` | `TestCategoryTreeIntegration` |
| List: filter, sort (`popular` = `review_count`, `rating_sum`), paging; inactive ba active variant chhara product nai; bhul query `400` (7.1, 7.2, v1.10) | `Products`, `listProducts` | `TestListProductsIntegration` |
| Detail: active variant ar chobi `sort_order` e; inactive ba nai → `404 not_found` (7.2) | `Product`, `productBySlug` | `TestGetProductIntegration` |
| Search: Bangla, juktakkhor, English, Banglish keyword, NFC, `%`/`_` escape, 2 okkhorer kom e search na (6.4) | `searchTerm`, `listProducts` | `TestSearchIntegration`, `TestSearchTerm` |
| Protiti host e shudhu nijer shop er category ar product (Rule 3) | protiti query te `shop_id` | `TestShopIsolationIntegration` |
| Onno shop er category, vendor, parent ba product e reference → DB error (14.2 test 13) | `001_init.sql` er composite FK | `TestTenantForeignKeyIntegration` |
| Category cycle atkano (7.7) | `makesCycle` | `TestCategoryCycle` |
| Slug `^[a-z0-9]+(-[a-z0-9]+)*$`, max 80 (7.7, v1.10) | `validSlug` | `TestValidSlug` |

## Data

`categories` (`parent_id`, `slug`, `name` JSONB, `sort_order`, `is_active`), `products` (`name`, `description`, `seo` JSONB, `search_keywords`, `search_text` generated NFC, `rating_sum`, `review_count`, `is_active`), `product_variants` (`price` poisha, `compare_at_price`, `stock`, `options`, `is_active`), `product_images`; index `products_search_trgm`; `001_init.sql`.

## Bhul hole ki hoy

- Bhul `page`, `limit`, `sort`, dam, `in_stock` ba lomba `q`: `400 validation_failed` (`fields` e naam).
- Ochena category ba vendor slug: khali list (error na).
- Product inactive, active variant nai, ba onno shop er: `404 not_found`.
- Database error: `500 internal`, log e `catalog handler`.

## Bodlanor age mone rakho

- `total` `count(*) OVER ()` theke, tai shesh page er porer page e `total` 0 (`listProducts` er `ponytail:` comment).
- Stock er sonkhya public e jay na, shudhu `in_stock`.
- `search_text` e notun field lagle migration (generated column), ar `TestSearchIntegration` e case.
- Router e route jog M1 er router step e (`cmd/api/router.go`).

## Proshno-uttor

- **Slug Bangla te na keno?** Admin Banglish slug dey (`lal-chal`); Bangla theke auto slug bhalo hoy na (v1.10).
