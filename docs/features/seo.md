# SEO (website)

Design: 10.4, 12.2 · Milestone: M3 · Code: `website/lib/seo`

## Ki kore

Protiti page e Google ar Facebook er jonno `<title>`, description, Open Graph, canonical ar hreflang; product page e JSON-LD `Product`. Protiti shop er domain e nijer `sitemap.xml` ar `robots.txt`. Search page ar `seo.noindex` chalu shop `noindex`.

## Kivabe kaj kore

1. Absolute URL: `originOf` (`lib/services/api.dart`): Caddy er `X-Forwarded-Proto: https` hole `https`, na hole `http`, + `Host`. `renderPage` eta `Site.origin` e dey.
2. `SeoHead` (`lib/seo/meta.dart`): page er bhetore boshe, Jaspr `Document.head` diye `<head>` e jay. `title` `null` hole home: shop er `seo.title` (na hole naam); na hole "`title` | shop er naam" (naam khali hole shudhu `title`). Description: page er (product `seo.description`, na hole description; policy er lekha), na hole shop er `seo.description`; ek line, 160 okkhor. `canonical` ar `og:url` = origin + ei bhashar path (query chhara). `hreflang`: `bn` (root), `en` (`/en`, shudhu `locales.enabled` e `en` thakle), `x-default` = `bn`. OG `property` attribute diye: `og:title`, `og:description`, `og:type` (`website`/`product`), `og:url`, `og:site_name`, `og:image` (`ImageUrls.url(key, 1280)`; key: page er, na hole `seo.og_image_key`, na hole logo), product e `product:price:amount` (shuru te bachai kora variant er dam, `amount`) ar `product:price:currency=BDT`. `noindex` (page) ba `settings.noindex` hole `<meta name="robots" content="noindex">`.
3. Page: `Home` `SeoHead()`; `ProductListing` (category, search) `SeoHead(title, noindex)`; search er khali/choto `q` e o `noindex`; `ProductPage` (product `seo.title`/naam, chobi, dam, JSON-LD); `PolicyPage`; `StatusPage` (404, bondho, error) `noindex`.
4. JSON-LD (`lib/seo/json_ld.dart`): `productJsonLd` = `Product` (`name`, `description`, `image` absolute, protiti variant ekta `Offer`: `sku`, `price` (`amount`: poisha → `850.00`, hishab na), `priceCurrency: BDT`, `availability` InStock/OutOfStock, `url`), `aggregateRating` shudhu `review_count > 0`. `jsonLd` JSON encode kore `<` ke `<` kore, tai naam e `</script>` thakleo script bhange na; `script(content:)` raw boshay.
5. `seoFiles` middleware (`lib/seo/sitemap.dart`, `main.server.dart` e page render er age): `/robots.txt` → `robotsTxt` (`/search`, `/cart`, `/checkout`, `/account`, `/order`, `/confirm`, `/print` ar tader `/en` `Disallow`, `Sitemap:` link); `/sitemap.xml` → `sitemapXml`: home, category tree er shob, product (`/api/products` `limit=100` page kore shesh porjonto), 5 ta policy; `en` chalu hole protiti tar `/en`; `<loc>` XML escape. Host er shop `settingsCache` theke.

## Code er map

| File → function | Ki kore |
|---|---|
| `website/lib/seo/meta.dart` → `SeoHead` | Title, description, OG, canonical, hreflang, robots, JSON-LD script |
| `website/lib/seo/json_ld.dart` → `productJsonLd`, `jsonLd`, `amount` | Product JSON-LD, escape, taka lekha |
| `website/lib/seo/sitemap.dart` → `seoFiles`, `sitemapXml`, `robotsTxt` | `sitemap.xml`, `robots.txt` |
| `website/lib/services/api.dart` → `originOf`; `services/locale.dart` → `localePath` | Absolute URL, bhashar path |
| `packages/shared/lib/models/settings.dart` → `seoTitle`, `seoDescription`, `ogImageKey`, `noindex` | Shop er `seo` block |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Protiti page e title/description (product `seo`), OG `property` attribute, `og:image` absolute, product er dam ar BDT, canonical, hreflang bn/en/x-default; `en` bondho hole shudhu bn + x-default; `seo.noindex` e puro site `noindex`, title ekta (10.4) | `SeoHead` | `website/test/seo/meta_test.dart` → `group('Meta ar Open Graph (10.4)')` |
| JSON-LD e `<` na (`</script>` XSS), offer protiti variant, `aggregateRating` shudhu review thakle, taka lekha (10.4, 12.2) | `jsonLd`, `productJsonLd`, `amount` | `website/test/seo/json_ld_test.dart` → `group('JSON-LD escape (10.4, 12.2)')` |
| Sitemap: category, product (shob page), policy, duita bhasha, host er nijer; robots e 10.4 er path; ochena shop 404 (10.4) | `sitemapXml`, `robotsTxt`, `seoFiles` | `website/test/seo/sitemap_test.dart` → `group('sitemap ar robots (10.4)')` |
| Search `noindex` (10.4 item 7) | `SearchPage`, `ProductListing` | `website/test/pages/catalog_pages_test.dart` → `group('Search ar policy (10.1, 10.4)')` |

## Data

Nai (API: `GET /api/settings` er `seo` block, product er `seo`, `/api/categories`, `/api/products`).

## Bhul hole ki hoy

- Shop nai: `sitemap.xml`/`robots.txt` `404`; shop bondho ba API bhul: `503` ar log (`seo file:`), `no-store` (`cacheControl`).
- Sitemap banate API bhul hole puro sitemap `503` (adhek sitemap na).
- Chobi nai: `og:image` nai.

## Bodlanor age mone rakho

- Notun public page e `SeoHead` dao; token page (`/order/*`, `/confirm/*`, ...) e `noindex` ar `X-Robots-Tag` (tader milestone e) ar robots e path.
- Sitemap ek file e (50k URL er shima); product onek hole sitemap index lagbe (`ponytail:` comment).
- Production e Caddy `X-Forwarded-Proto` na dile canonical `http://` hobe: 13.3 er Caddy config dekho.

## Proshno-uttor

- **Category page 2 er canonical page 1 keno?** Canonical query chhara path; paging/filter er URL index e alada na ashuk.
- **Limited launch e Google e ashbe?** Na: `seo.noindex` default `true` (14.4), admin bondho korle index hoy.
