# Website (Jaspr)

Design: 10.1–10.5, 10.9, 10.10, 3.2, 13.5 · Milestone: M3 · Code: `website/`

## Ki kore

Customer er website: Jaspr server mode e protiti page server e render hoy (SSR), dokan chena jay `Host` theke. Bangla root e, English `/en/...` e. Browser kokhono Go API te jay na; page er data Jaspr server API theke ane.
M3 cholche: ekhon skeleton, config, request theke API client, `Cache-Control`, BFF skeleton, settings cache, theme/font, layout (header, bottom nav, footer), 404/bondho/error page, home, category page, product card, chobi, filter, product page; search, policy, SEO porer step e ([M3](../milestones/M3.md)).

## Kivabe kaj kore

1. Shuru: `lib/main.server.dart` nijer shelf server `127.0.0.1:$PORT` e (13.5; `jaspr serve`/build `PORT` dey, local default 8081), `Pipeline` e `cacheControl()`, `bff(...)`, tarpor `serveApp` → `renderPage` (`lib/services/render.dart`). Env `lib/services/config.dart` → `config` (`Config.fromEnv`): `API_BASE_URL`, `JASPR_INTERNAL_SECRET`, `IMAGE_BASE_URL`, `IMAGE_RESIZE`; bhul hole `FormatException` e variable er naam.
2. Bhasha: `localeOf(path)` (`lib/services/locale.dart`): `/en` ba `/en/...` → `en` ar baki route, na hole `bn`. Browser er `Accept-Language` dekha hoy na (10.3: URL-i bhasha, cache thik thake).
3. API: `apiFor(requestHeaders, locale, config, httpClient)` (`lib/services/api.dart`) protiti request e notun `ApiClient` (shared), ek `http.Client`. Header `apiHeaders`: `X-Shop-Host` = `Host` (API nijei port bad diye normalize kore), Caddy er `X-Real-IP` thakle `X-Client-IP` + `X-Internal-Auth`; local e `X-Real-IP` nai, API connection IP ney.
4. CSS: Tailwind v3.4.19 standalone CLI (`scripts/tailwind.sh`, version ar sha256 pin, binary `website/.dart_tool` e) `tailwind.css` → `web/styles.css`; `tailwind.config.js` e rong, radius, font shudhu CSS variable (9.3). `jaspr_tailwind` Dart 3.13 e resolve hoy na (v1.14).
5. Cache (10.3): `cacheControl()` (`lib/services/cache_control.dart`): BFF `/_/...` → `no-store`; `__Host-sid`, `__Host-cart`, `preview` cookie ba response e `Set-Cookie` → `private, no-store` (page er nijer header o bodle); baki page nije `Cache-Control` dile seta, na dile 2xx e `public, max-age=60`, onno status e `no-store`.
6. BFF (10.9): `bff(config, httpClient, bffRoutes)` (`lib/services/bff.dart`) shudhu `/_/...` dhore, baki page e jay. POST/PATCH/DELETE e CSRF check: `Origin` thakle tar host:port = `Host`, na thakle `Sec-Fetch-Site: same-origin`, na hole `403 forbidden`. `bffRoutes` (`BffRoute`: method, path regex → Go API path) e na mille `404 not_found`; M3 e list khali, prothom route M4 e. Forward: `apiHeaders` (shop, IP), cookie `__Host-sid` → `Authorization: Bearer`, `__Host-cart` → `X-Cart-Key`, `Idempotency-Key`, bhasha nijer page er `Referer` path theke (`localeOf`; nai ba onno host hole `bn`); browser er cookie ar `Accept-Language` jay na. Go API er status, body ar `content-type` hubohu. JS chhara form (`x-www-form-urlencoded`) JSON hoye jay, uttor `303`: 2xx e `Referer` er page e, error e oi page e `?error=<code>` (onno host er Referer hole `/`). API e na pele `502 network_error`, 15 second e `504 timeout`. Shob uttor `no-store`.
7. Settings cache (3.5, 10.10 item 4): `settingsCache.get(host)` (`lib/services/settings_cache.dart` → `SettingsCache`, global `lib/services/api.dart` e) `ShopSettings` (settings, version, `translations` bn/en) dey. Prothom bar `version`, `settings`, duita `translations` (ek host e ekshathe ekta load). 30 second por prothom request purono data dey ar background e `version` check (host e ekta); bodlale shob abar. API down ba `429`: purono rakhe, log, 30 second por abar. `shop_not_found`/`shop_suspended`: cache theke bad, porer request error. Faka cache e bhul `get` theke `ApiError` (error page); ochena host cache hoy na.
8. Page (10.10 item 10): `resolvePage(host, path, settingsCache.get)` bhasha (`localeOf`) ar shop er settings niye `Site` (`lib/components/site.dart`, InheritedComponent: `locale`, `settings`, `Translator` (default + override), `catalog` (ei request er `CatalogRepository`: `apiFor` diye `Host`, bhasha, IP), `images` (`ImageUrls`), `t`, `taka` (shop er `bangla_digits`), `href`) banay. Shop nai (`shop_not_found`) → `404`; bondho (`shop_suspended`) → `503` + `Retry-After: 3600` "dokan bondho" page; onno bhul (API down) → log, `503` error page; ei tin page e shop er theme ar layout nai. `locales.enabled` e `en` na thakle `/en/*` → `404` (lekha Bangla). `renderPage` Jaspr `Document` (`lang` = bhasha, viewport `viewport-fit=cover`, `/styles.css`, `<style>` e `themeCss`) render kore status ar header boshay; non-2xx `cacheControl` e `no-store`.
9. Route ar layout: `App` (`lib/app.dart`, `jaspr_router`) protiti route `/x` ar `/en/x` duitai; na mille `_NotFound` (`setStatusCode(404)`). `Layout` (`lib/components/layout.dart`, 10.5): header (shop er naam → home, desktop e "Category" `/#categories`, `en` chalu thakle bhasha bodlanor link ekoi path e, GET search form `/search?q=`, input 16px), content max 1280px, footer (phone, email, DBID, BFSA thakle), mobile e fixed bottom nav (product page `/p/*` e na, sticky bar; Home, Category; Cart/Account M4/M5 porjonto lukano, 10.10 item 6) `env(safe-area-inset-bottom)` shoho. Tap 44px (`min-h-11`).
10. Theme (9.3, 9.6): `themeCss` (`lib/services/theme.dart`) `:root` e settings er rong, radius, font, ar shudhu settings er duita font er `@font-face` (`web/fonts/`, bengali/latin subset `unicode-range`, `font-display: swap`). `tailwind.css` er base layer e 9.6 er typography (body 16px/1.7, h1–h3 `clamp`, heading font 500); `tailwind.config.js` e font `var(--font-*)`, fallback `sans-serif`; `text-sm` 14px/1.6.
11. Home (`lib/pages/home.dart`, `AsyncStatelessComponent`, 10.1, 10.5): `categories()` ar featured ekshathe. Announcement (thakle); banner (`settings.banners`, prothom ta LCP; link `safeLink` (`lib/services/link.dart`) diye abar check: `/...` (na `//`, `\`, control char) → bhashar prefix, `https://` + host hubohu, baki link chhara); category `section#categories` (`featured_category_slugs` dile oigulo oi kram e, na hole shob top-level; mobile e side-scroll); featured product: `featured_product_slugs` er prothom 12 ta protiti slug e `product(slug)` ekshathe, `not_found` bad, card er dam variant der sobcheye kom ta (hishab na, Rule 1). API er onno bhul → `pageError` (`lib/pages/status.dart`): `not_found` 404, baki 503 error page + log.
12. Header e `branding.logo_key` thakle logo chobi (`ImageUrls.url` 320, `alt` = naam), na hole naam. Card ar chobi: `ProductCardView` (`lib/components/product_card.dart`): `/p/{slug}` link, chobi, naam, `Price`, stock na thakle "স্টকে নেই". `Price` (`lib/components/price.dart`): `site.taka(price)`, `compare_at_price` thakle kete (`<s>`), `variable_weight` e "/ kg". `ResponsiveImage` (`lib/components/responsive_image.dart`, 10.5 niyom 7): `ImageUrls.url/srcset` (`lib/services/image.dart`: cloudflare e `<base>/cdn-cgi/image/width=W,format=auto/<key>` 320/640/960/1280, `none` e `<base>/<key>`; key er segment encode, `..`/khali segment e `null`), `sizes`, `width`/`height`, LCP e `fetchpriority="high"`, baki `loading="lazy"`; key na thakle `bg-tint` placeholder (`role="img"`, `aria-label`).
13. Category (`lib/pages/category.dart`, `/c/{slug}`): `categories()` (tree e slug khoje naam; na pele 404) ar `products(category, sort, in_stock, min/max, page, limit 24)` ekshathe. Query `ListQuery.parse` (`lib/services/list_query.dart`): `sort` shudhu 7.2 er 4 ta, `in_stock=1`, `min`/`max` URL e taka (0–1 koti) → API te poisha, `page` 1–10000; bhul value bad (API `400` na). Koyta product `plural('category.count')` (Bangla digit `site.number`). Paging link `withPage` (filter thake, page 1 e `page` na; `rel=prev/next`). Filter: `FilterForm` (`lib/components/filter_sheet.dart`, GET form, JS chhara-o chole) desktop e bame sidebar; mobile e `FilterSheet`: "Filter" button ar `<dialog id="filters">` server e, `@client DialogToggle` (data shudhu dialog id, 10.2) browser e `[data-dialog-open]` → `showModal`, `[data-dialog-close]` → `close`.
14. Local: `make web` (`scripts/web.sh`): `website/.env` load, Tailwind `--watch`, `jaspr serve` (port `pubspec.yaml` er `jaspr.port` 8081). Catalog data: `make dev-seed` (`scripts/dev-seed.sql`, docker er Postgres e `shop_owner` diye; `sobujbazar.localhost` shop lage; 3 category, 5 product, variant, stock batch + movement shoho; abar chalale bad; chobi nai).
15. Product (`lib/pages/product.dart`, `/p/{slug}`, 10.1, 10.5): `product(slug)`; `not_found` → 404 (`pageError`). `Gallery` (`lib/components/gallery.dart`): chobi gulo CSS `scroll-snap` er side-scroll (JS na, `tabindex=0` keyboard e), `ResponsiveImage` (API er `width`/`height`, `alt` = product naam, prothom ta LCP); chobi na thakle placeholder. `review_count > 0` hole "★ rating (count)" (`formatRating`, `plural('product.rating')`). Variant (`lib/components/variant_picker.dart`): shuru te prothom stock e thaka variant (`defaultVariant`); `VariantPrices` protiti variant er dam (`Price`, API er, Rule 1) ar stock na thakle lekha `[data-variant=<id>]` e render kore, bachai chhara baki `hidden`; 2+ variant hole `VariantPicker`: radio `name=variant` (lekha `options` er value, server e), `@client VariantSwitch` (data kichu na) radio bodlale shudhu oi `data-variant` dekhay. JS chhara prothom dam dekha jay. Description `plainText` (`lib/components/plain_text.dart`, 10.10 item 2): `text` e escape, khali line → `<p>`, newline → `<br>`. Mobile e niche fixed sticky bar (variant naam + dam, `md:hidden`); "Cart e add" M5 e.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `website/lib/main.server.dart` → `main` | Server entry, `Document` |
| `website/lib/main.client.dart` | Browser entry, shudhu `@client` |
| `website/lib/services/render.dart` → `renderPage`, `resolvePage` | Request → page, status (404/503), `Document` |
| `website/lib/components/site.dart` → `Site` | Request er shop, bhasha, lekha |
| `website/lib/app.dart` → `App` | Route (`/en` shoho), router 404 |
| `website/lib/components/layout.dart` → `Layout` | Header, bottom nav, footer |
| `website/lib/pages/status.dart` → `StatusPage`, `StatusKind`, `pageError` | 404, bondho, error page; page er API bhul |
| `website/lib/pages/home.dart` → `Home` | Home page |
| `website/lib/components/product_card.dart` → `ProductCardView`; `price.dart` → `Price` | Product card, dam |
| `website/lib/components/responsive_image.dart` → `ResponsiveImage`; `lib/services/image.dart` → `ImageUrls` | Chobi, `srcset` |
| `website/lib/services/link.dart` → `safeLink` | Admin er link check |
| `website/lib/pages/category.dart` → `CategoryPage`; `lib/services/list_query.dart` → `ListQuery` | Category page, query parse |
| `website/lib/components/filter_sheet.dart` → `FilterForm`, `FilterSheet`, `DialogToggle` (@client) | Filter form, mobile `<dialog>` |
| `website/lib/pages/product.dart` → `ProductPage`; `lib/components/gallery.dart` → `Gallery`; `variant_picker.dart` → `VariantPrices`, `VariantPicker`, `VariantSwitch` (@client); `plain_text.dart` → `plainText` | Product page, gallery, variant, plain text |
| `website/lib/services/theme.dart` → `themeCss`, `fontFiles` | `:root` variable ar `@font-face` |
| `website/lib/services/config.dart` → `Config.fromEnv`, `config` | Env (13.5) |
| `website/lib/services/locale.dart` → `localeOf` | URL theke bhasha |
| `website/lib/services/cache_control.dart` → `cacheControl` | `Cache-Control` middleware |
| `website/lib/services/settings_cache.dart` → `SettingsCache`, `ShopSettings`; `api.dart` → `settingsCache` | Host onujayi settings ar translation cache |
| `website/lib/services/bff.dart` → `bff`, `BffRoute`, `bffRoutes`, `parseCookies` | BFF: CSRF, forward, form `303` |
| `website/lib/services/api.dart` → `apiHeaders`, `apiFor`, `httpClient` | Request theke `ApiClient` |
| `website/tailwind.config.js`, `scripts/tailwind.sh` | Tailwind token ar build |
| `scripts/web.sh` | `make web` |
| `scripts/dev-seed.sql` | `make dev-seed`: local catalog data |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| `/en/...` → `en`, root → `bn`; `/english` `bn` (9.8, 10.3) | `localeOf` | `website/test/l10n/locale_route_test.dart` → `group('URL theke bhasha (9.8, 10.3)')` |
| Shop `Host` theke; `X-Client-IP` + `X-Internal-Auth` shudhu Caddy er `X-Real-IP` thakle; `Accept-Language` URL theke (3.2, 10.3) | `apiHeaders`, `apiFor` | `website/test/services/api_test.dart` → `group('API header (3.2, 10.3)')` |
| Personal cookie (`__Host-sid`, `__Host-cart`, `preview`) ba `Set-Cookie` → `private, no-store`; cookie chhara public page `public, max-age=60`; BFF `no-store`; non-2xx `no-store` (10.3, 14.2 test 24) | `cacheControl` | `website/test/server/cache_control_test.dart` → `group('Cache-Control (10.3, 14.2 test 24)')` |
| BFF: Origin nai/onno/`null`/onno port ba `Sec-Fetch-Site` cross-site → `403`, API te jay na; header forward; Go API er status/error hubohu; ochena route `404`; form `303`; `no-store` (10.9) | `bff` | `website/test/bff/bff_test.dart` → `group('BFF (10.9)')` |
| Settings cache: 30 second er moddhe API te na; por version check, na bodlale shudhu version; bodlale background e notun; ekshathe ekta; host onujayi; API down/429 e purono + log; faka cache e error; ochena host cache na; shop nai/suspended e bad (3.5, 10.10) | `SettingsCache` | `website/test/services/settings_cache_test.dart` → `group('Settings cache (3.5)')` |
| Shop nai → `404`; bondho → `503` + `Retry-After: 3600`; API down/onno bhul → `503` error page; `en` bondho hole `/en/*` `404`; bhasha URL theke, override shoho (10.10 item 10) | `resolvePage` | `website/test/server/render_test.dart` → `group('Page status (10.10 item 10)')` |
| `:root` e settings er rong/radius/font; shudhu settings er font er `@font-face`, `swap`; 9.6 er protiti font file `web/fonts/` e (9.3, 9.6) | `themeCss`, `fontFiles` | `website/test/services/theme_test.dart` → `group('Theme CSS (9.3, 9.6)')` |
| Chobi: cloudflare e width/format URL ar `srcset` 320–1280, `none` e `<base>/<key>` (srcset na); key er segment encode, `..`/khali segment/`/` shuru → `null` (10.5, 10.10 item 3) | `ImageUrls` | `website/test/services/image_test.dart` → `group('Chobi URL (10.5, 10.10 item 3)')` |
| Admin er link: `/...` bhashar prefix shoho, `https://` + host hubohu; `//`, `\`, control char, `javascript:`, `http://` → link na (9.2, 12.2) | `safeLink` | `website/test/services/link_test.dart` → `group('Admin er link (9.2, 12.2)')` |
| List er query: `sort` whitelist, `in_stock=1`, dam taka → poisha (0–1 koti), `page` ≥ 1; bhul value bad; paging link e filter thake (7.2, 10.1) | `ListQuery` | `website/test/services/list_query_test.dart` → `group('List er query (7.2, 10.1)')` |
| Category page: naam (tree), koyta (Bangla digit), card, dam, `srcset`, LCP, filter `<dialog>`, paging link; API te dam poisha, bhul `sort` bad; ochena category 404. Home: category, na pawa featured bad, banner er `//` link e link na (10.1, 10.5, 12.2) | `CategoryPage`, `Home` | `website/test/pages/catalog_pages_test.dart` → `group('Category page (10.1, 10.5)')`, `group('Home (10.1)')` |
| Product page: gallery `scroll-snap` (prothom LCP, baki lazy, `alt` = naam), stock e thaka variant shuru te, dam API theke (onno variant `hidden`), sticky bar e naam + dam, rating, description escape ar `<p>`/`<br>`, bottom nav na; ochena product 404 (10.1, 10.5, 10.10, 12.2) | `ProductPage`, `VariantPrices`, `plainText` | `website/test/pages/catalog_pages_test.dart` → `group('Product page (10.1, 10.5)')` |
| UI te Bangla okkhor shorasori na, lekha key theke (Rule 4, 9.8) | `scripts/check-rules.sh` | `bash scripts/check-rules.sh` |
| Env nai ba bhul hole server uthe na (13.5) | `Config.fromEnv` | `website/test/services/api_test.dart` → `group('Config (13.5)')` |

## Data

Nai (website database dhore na; shob Go API theke).

## Bhul hole ki hoy

- Env nai ba bhul: prothom `config` use e `FormatException` (variable er naam shoho).
- `Host` nai: `X-Shop-Host` khali, API `404 shop_not_found` → 404 page.
- API down ar cache faka: `503` error page (`no-store`), log `page: ...`; cache e purono thakle purono dekhay.
- Router e route nai: `404` page (layout shoho).
- BFF: CSRF check fail `403 forbidden`; ochena `/_/` route `404 not_found`; API down `502 network_error`, dhir `504 timeout` (client `ApiError` er code er moto).

## Bodlanor age mone rakho

- `X-Real-IP` shudhu Caddy dey: tai server shudhu loopback e (`main.server.dart`); Jaspr er default `runApp` `0.0.0.0` e bind kore, oita use koro na.
- BFF e notun route `bffRoutes` e (M4 theke) ar test; OTP verify er `Set-Cookie` (`__Host-sid`) M4 e jog hobe, ekhon API er `Set-Cookie` forward hoy na.
- Notun personal cookie (jemon M4/M5 e) hole `_personal` e jog ar test.
- Jaspr update hole `jaspr install-skills --agent claude-code` abar ar `*.options.dart` (`dart run build_runner build`).

## Proshno-uttor

- **Admin publish korle website e kokhon?** Max ~30 second + ek request: TTL er por prothom request purono dey ar refresh shuru kore, porer request notun pay (3.5: 1 minute er moddhe).
- **Translation override save (M8) e version bare na hole?** Purono lekha porer publish porjonto; tai M8 e override save e `version` barabe (3.5 item 6).

- **Browser er bhasha keno dekhi na?** Ekoi URL e alada HTML hole Cloudflare cache bhul bhashar page dite pare (10.3); bhasha bodlano shudhu link diye.
