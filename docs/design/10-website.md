> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 10. Customer website (Jaspr)

### 10.1 Page ar route

English version e shob route er samne `/en`.

| Route | Page | Login | Note |
|---|---|---|---|
| `/` | Home | Na | Banner, category, featured |
| `/c/{slug}` | Category | Na | Filter, sort, paging (`?page=`) |
| `/p/{slug}` | Product | Na | Gallery, variant, review, SEO |
| `/search?q=` | Search | Na | `noindex` |
| `/v/{slug}` | Vendor store page | Na | Shudhu marketplace mode e, single e `404` |
| `/cart` | Cart | Na | Guest cart. v1.28: SSR, `@client` na, JS chhara form (10.12) |
| `/checkout` | Checkout | Na | Guest checkout |
| `/order/{token}` | Order dekha | Na (token) | SMS link; phone/thikana mask, `noindex` |
| `/order/{token}/done` | Order er por | Na | Payment status check |
| `/confirm/{confirm_token}` | COD confirm (v1.4: alada token, shudhu SMS e, 8.10) | Na | `noindex`, `no-referrer` |
| `/order/{token}/review` | Review | Na | Delivered hole |
| `/invoice/{token}` | Printable invoice | Na | Print CSS |
| `/track` | Order tracking | Na | Order number + phone (POST form) |
| `/login` | OTP login | | v1.19: `@client LoginForm`, Turnstile (10.11) |
| `/account`, `/account/orders`, `/account/addresses`, `/account/wishlist`, `/account/profile`, `/account/privacy` | Account | Hya | v1.19: M4 e shudhu `/account` (naam, bhasha, logout); baki tar milestone e (10.11). v1.28: `/account/wishlist` M5 e (10.12) |
| `/policy/{terms,return,refund,delivery,privacy}` | Policy | Na | Bangla badhyotamulok |
| `/contact`, `/faq`, `/complaint` | Support | Na | DBID, BFSA number footer e |
| `/print/{kind}?token=` | Staff er packing slip, pick list | Signed token (10 min) | Admin theke khole; `Referrer-Policy: no-referrer` |
| `/sitemap.xml`, `/robots.txt` | SEO | Na | |

v1.5: token page (`/order/*`, `/confirm/*`, `/invoice/*`, `/print/*`) e Facebook Pixel, GA4 ba onno third-party script load hoy na. Egulo page er puro URL pathay, tai `confirm_token`/`public_token` onno company er kache jeto, ar v1.4 er alada `confirm_token` er uddeshyo bhenge jeto.

### 10.2 Jaspr er niyom

| Kaj | Ki use korbe |
|---|---|
| Page er data | `AsyncStatelessComponent`, server e Go API theke |
| Title, description, OG | `Document.head` (protiti page e) |
| Interactive | Shudhu `@client` component: variant picker (dam bodlay), quantity, cart button, filter sheet, checkout form er live hishab, v1.19: login form (10.11), v1.28: `AddToCart` (quantity + cart button), `WishlistButton`; `/cart` page e `@client` na (10.12) |
| Chobi swipe | CSS `scroll-snap`, JavaScript na |
| Filter sheet | Browser er `<dialog>` |
| Layout bodlano (mobile/desktop) | Shudhu CSS (Tailwind prefix), server e device detect na |
| Form | JS chhara-o kaj kore emon (normal form POST Jaspr server er BFF endpoint e, 10.9, tarpor `303`), tarpor JS diye shundor |

Jaspr version 0.23.x e pin (2.1: `^0.23.5`, `pubspec.lock` commit). Update er age release notes, karon 1.0 er age breaking change hoy (0.24.0-beta.0 ber hoyeche, Dart 3.13 lage).

### 10.3 Cookie

| Cookie | Ki | Flag |
|---|---|---|
| `__Host-sid` | Customer session token | `HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=30 din` |
| `__Host-cart` | Guest cart key | `HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=30 din`. v1.28: key Go banay, BFF set kore, protiti cart write e abar (sliding 30 din, 10.12) |
| `wl` | Guest wishlist (product ID list) | `Secure; SameSite=Lax`. v1.28: `Path=/; Max-Age=1 bochor`, HttpOnly na (`@client WishlistButton` lekhe); koma diye decimal ID, max 50 (10.12) |
| `lang` | `bn`/`en` | `Secure; SameSite=Lax; Max-Age=1 bochor` |
| `preview` | Admin er draft theme preview token | `HttpOnly; Secure; SameSite=Lax; Max-Age=30 min` |

Jaspr server cookie theke token/key niye Go API te `Authorization: Bearer` ar `X-Cart-Key` header e pathay. Browser kokhono Go API ke shorasori token dey na.

**Cache**: session, cart ba `preview` cookie thaka response e `Cache-Control: private, no-store`; Cloudflare e ei cookie gulo thakle HTML cache bypass rule. Na hole ek customer er page (naam, cart) ba admin er draft theme onno jon pete pare. v1.28: `wl` er page public thake: server `wl` diye kichu render kore na (10.12). Public page (product, category) e `Cache-Control: public, max-age=60` shudhu jodi page e personal kichu na thake. Jaspr Go API te `Accept-Language` pathay URL er bhasha theke (`/en/` hole `en`, na hole `bn`), browser er header theke na; na hole ekoi URL e alada bhasha cache hoy. Deploy e `main.dart.js` er naam bodlay na, tai URL e `?v=<git sha>` dao ba deploy e Cloudflare cache purge koro, na hole notun HTML purono JS diye hydrate hoy.

### 10.4 SEO checklist

1. Protiti page e alada `<title>` ar `meta description` (product er `seo` field, na thakle naam ar description theke).
2. Open Graph: `og:title`, `og:description`, `og:image`, `og:url`, `og:type=product`, `product:price:amount`, `product:price:currency=BDT`, **`property` attribute diye**. Launch er age Facebook Sharing Debugger e check.
3. Product page e JSON-LD `Product` (`name`, `image`, `offers.price`, `priceCurrency: BDT`, `availability`, `aggregateRating`). JSON-LD er bhetore `<` ke `\u003c` (naam e `</script>` thakle XSS).
4. `canonical` URL, `hreflang` (bn, en, x-default).
5. `sitemap.xml` (category, product, policy; duita bhasha; protiti shop er nijer domain e nijer sitemap), `robots.txt` (`/search`, `/cart`, `/checkout`, `/account`, `/order`, `/confirm`, `/print` block).
6. Porishkar URL (`/p/cotton-panjabi-black`), `?id=` na.
7. Search page `noindex`; token page (`/order/*`, `/confirm/*`, `/invoice/*`, `/print/*`) e `X-Robots-Tag: noindex`. Limited launch e `seo.noindex` (puro site).

### 10.5 Responsive ar mobile

**Mobile-first**: base design 360px er jonno, `md:` (768px) tablet, `lg:` (1024px) desktop, content max 1280px.

| Page | Mobile | Tablet | Desktop |
|---|---|---|---|
| Home | Upore search bar, category side-scroll, 2 column | 3 column | Header e category menu, 4–5 column |
| Category | Sticky "Filter" ar "Sort", filter bottom sheet e | 3 column | Bame filter sidebar, 3–4 column |
| Product | Full-width gallery, niche sticky "Cart e add" bar | Pashapashi | 2 column, button inline |
| Cart | List, niche sticky total + checkout (v1.28: checkout button M6 porjonto lukano) | Mobile er moto | Bame list, dane sticky summary |
| Checkout | Ek column, header e shudhu logo | Mobile er moto | Bame form, dane summary |

```
Mobile home                      Mobile product
+---------------------------+    +---------------------------+
| Logo              Cart(2) |    | <-  Logo      Search Cart |
| [ Search korun...       ] |    | +-----------------------+ |
| (Chal)(Dal)(Tel)(Modhu) > |    | |  Chobi (swipe)  o o o | |
| +-----------------------+ |    | +-----------------------+ |
| |     Offer banner      | |    | Product naam              |
| +-----------------------+ |    | Tk 180 / kg               |
| +---------+ +---------+   |    | [1 kg] [5 kg]   [-] 1 [+] |
| | Product | | Product |   |    | Description            v  |
| +---------+ +---------+   |    | Review (24)            v  |
+---------------------------+    +---------------------------+
| Home  Category  Cart  Ami |    | Tk 180   [ Cart e add ]   |
+---------------------------+    +---------------------------+
```

Mobile niyom:

1. Bottom nav (Home, Category, Cart, Account). Product page e bottom nav er bodole sticky "Cart e add" bar, duita ek shathe na.
2. Tap kora jinish kom-poksha 44px uchu, majhe faka.
3. Hover er upor kono kaj nirbhor na.
4. Input font 16px er kom na (na hole iPhone zoom kore).
5. Phone: `type="tel" inputmode="numeric" autocomplete="tel"`. OTP: `inputmode="numeric" autocomplete="one-time-code"`. Naam: `autocomplete="name"`. Thikana: `autocomplete="street-address"`. District/area dropdown search shoho.
6. Viewport: `width=device-width, initial-scale=1, viewport-fit=cover`. `maximum-scale` dibe na. Bottom bar e `padding-bottom: env(safe-area-inset-bottom)`.
7. Chobi: 320, 640, 960, 1280px size (Cloudflare resizing), `srcset` + `sizes`, `width`/`height` ba `aspect-ratio`, prothom screen er chobi chhara baki `loading="lazy"`. Prothom screen er boro chobi (LCP) te `fetchpriority="high"`.

```html
<img src="https://img.tomarshop.com/cdn-cgi/image/width=640,format=auto/shops/1/rice.jpg"
     srcset="https://img.tomarshop.com/cdn-cgi/image/width=320,format=auto/shops/1/rice.jpg 320w,
             https://img.tomarshop.com/cdn-cgi/image/width=640,format=auto/shops/1/rice.jpg 640w,
             https://img.tomarshop.com/cdn-cgi/image/width=960,format=auto/shops/1/rice.jpg 960w"
     sizes="(min-width: 1024px) 25vw, 50vw"
     width="640" height="640" loading="lazy" alt="অর্গানিক লাল চাল">
```

**Speed er lokkho** (majhari Android phone, slow 4G): LCP 2.5 second er kom, INP 200 ms er kom, CLS 0.1 er kom. Protiti boro bodoler por Lighthouse mobile (Slow 4G + CPU throttling). v1.18: acceptance er number devtools (asol) throttle e (`--throttling-method=devtools`); simulate (Lantern) er number Report e note; font kom korar kaj M12 er age, Lighthouse CI er sathe (16.1). `build/jaspr/web/main.dart.js` er size protibar maapo, hothat barle karon khojo. `@client` kom rakho.

**Test er device**: sosta Android er Chrome, **Facebook/Messenger er in-app browser** (login, OTP, cart, SSLCommerz redirect shob), iPhone Safari, desktop Chrome. Width: 360, 390, 412, 768, 1280. Duita bhashay-i.

### 10.6 Checkout form

Field: naam, phone, email (optional; SSLCommerz e na dile shop er email jay), "onno kauke pathacchi" (on korle recipient er naam ar phone), district, area (dropdown, zone er list theke), puro thikana, note, coupon, payment method, privacy/terms consent checkbox. Lukano honeypot `website`. Turnstile widget.

Flow: form → (jhuki thakle) OTP box → order → `/order/{token}/done`. Done page e order number, SMS er kotha (COD confirmation lagle "SMS e pathano link e confirm korun"; done page e confirm button nai, 8.10), ar choto kore "Porer bar druto order korte account khulun (shudhu OTP)". Live hishab (`POST /api/checkout/quote`, BFF diye) area ba coupon bodlale; quote er total `expected_total` hishebe order e jay. OTP box theke submit e notun Turnstile lage na (8.6). Order er age delivery er shomoy dekhao (perishable hole aro kom).

### 10.7 Accessibility

Shob chobir `alt` (product naam), protiti input er `<label>`, keyboard diye shob kaj, focus dekha jay, rong-er upor ekmatro nirbhor kore kono tottho na (stock shesh = lekha + rong), contrast 4.5:1.

### 10.8 Phase 2: PWA

`manifest.json` (naam, icon, theme rong settings theke), service worker (static file cache, offline page `/offline`), web push (Android). Flutter app ashar age app er moto kaj kore. PWA-o BFF (10.9) diye, Go API te shorasori na.

### 10.9 BFF: Jaspr server er nijer endpoint

Browser kokhono Go API te jay na (CORS shudhu admin), tai `@client` component ar JS chhara form Jaspr server er ei endpoint e jay. Jaspr eigulo shelf handler hishebe render handler er age rakhe ar Go API te forward kore.

| Method | Jaspr path | Go API |
|---|---|---|
| POST, PATCH, DELETE | `/_/cart/items`, `/_/cart/items/{variant_id}` | `/api/cart/items...` (v1.28: notun `cart_key` e `__Host-cart` set, body theke muche; JS chhara form e `303 /cart`) |
| POST | v1.28: `/_/cart/items/{variant_id}/delete` | `DELETE /api/cart/items/{variant_id}` (JS chhara form) |
| POST | `/_/checkout/quote` | `POST /api/checkout/quote` |
| POST | `/_/checkout` | `POST /api/orders` (JS e random `Idempotency-Key`; JS chhara form e page render er shomoy hidden field) |
| POST | `/_/auth/otp/send`, `/_/auth/otp/verify`, `/_/auth/logout` | `/api/auth/...` (verify e `__Host-sid` cookie set; v1.26: logout e cookie `Max-Age=0`, JS chhara form e `303`; v1.28: verify `200` e `__Host-cart` `Max-Age=0`, `wl` thakle `POST /api/wishlist/merge` tarpor `wl` `Max-Age=0`, 10.12) |
| POST, DELETE | `/_/wishlist/{product_id}` | `/api/wishlist/{product_id}` (v1.28: shudhu login e; guest e `wl` cookie) |
| POST | `/_/orders/{token}/cancel`, `pay` | `/api/orders/{token}/...` |
| POST | `/_/confirm/{confirm_token}` | `POST /api/confirm/{confirm_token}` (v1.4, 8.10) |
| POST | `/_/track`, `/_/complaint`, `/_/review` | `POST /api/orders/track`, `/api/complaints`, `/api/reviews` |
| PATCH, POST | `/_/account` | v1.26: `PATCH /api/me` (JS chhara form er POST o PATCH hoye jay, `apiMethod`) |
| PATCH, POST, DELETE | `/_/account/...` | `/api/me/...` (M6) |

Niyom:

1. Protiti POST, PATCH, DELETE e `Origin` (ba `Sec-Fetch-Site`) check: nijer domain na hole `403` (CSRF; `SameSite=Lax` er upor extra).
2. Cookie theke token ar key niye `Authorization: Bearer`, `X-Cart-Key`; shathe `X-Shop-Host`, `X-Client-IP`, `X-Internal-Auth`, `Accept-Language` (3.2).
3. Go API er status ar error JSON hubohu ferot; client `code` diye logic kore.
4. JS chhara form POST e `303` redirect (POST-redirect-GET), error hole form abar dekhay. v1.19: login form chhara (Turnstile JS chhara chole na, 10.11).
5. Jaspr kono dam, stock, discount hishab kore na (Rule 1), shudhu forward kore.
6. Response e `Cache-Control: no-store`.

### 10.10 M3 er decision (v1.13)

1. **Setup**: `jaspr ^0.23.5`, `jaspr_router` (`/en` prefix), Tailwind **v3.4.x standalone CLI** (Node chhara; version ar sha256 `scripts/tailwind.sh` e pin, script download kore; v1.14: `jaspr_tailwind` Dart 3.13 e resolve hoy na (`build_modules` er SDK shima), tai builder chhara CLI shorasori `tailwind.css` → `web/styles.css`); `tailwind.config.js` 9.3 er hubohu; dev e `jaspr_test`. `jaspr install-skills --agent claude-code` er file commit hoy (Jaspr update e abar).
2. **Admin er lekha** (product description, policy): plain text, shob escape; khali line `<p>`, newline `<br>`; link na (12.2).
3. **Chobi**: website env `IMAGE_BASE_URL` (platform er) ar `IMAGE_RESIZE=cloudflare|none`; `srcset` = `<base>/cdn-cgi/image/width=W,format=auto/<key>` (`none` e `<base>/<key>`); chobi na thakle `tint` placeholder (13.5).
4. **Settings cache** (3.5): host onujayi `{settings, version, translations[bn,en], checkedAt}`; 30 second por prothom request background e version check (request atkay na, ekbar e ekta); version bodlale settings ar duita translation abar; API down hole purono cache ar log; cache faka hole error page; ochena host cache na.
5. **BFF** M3 e shudhu skeleton (Origin check, cookie → header, forward, `303`, `no-store`) ar test; prothom asol route M4 e (`/_/auth/otp/*`).
6. **Cart/Account**: page M4/M5 er age link lukano (layout ekoi; v1.19: Account link M4 e abar); product page er sticky bar e dam ar variant, "Cart e add" button M5 e. v1.28: Cart link M5 e abar (badge 10.12).
7. **Security header** Jaspr middleware e (12.2): `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, CSP `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' <IMAGE_BASE_URL> data:; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'`. Turnstile/payment domain M4/M10 e; HSTS M12 e (Cloudflare/Caddy).
8. **Test**: logic (cache, BFF, SEO, locale, header) `package:test`; page `jaspr_test` (title, OG, `srcset`, lekha); 360/1280 ar Lighthouse manual, Report e; Playwright M12 e. CI er Dart job e `jaspr build` ar `main.client.dart.js` er size log e.
9. **Facebook debugger** M3 e `cloudflared` quick tunnel diye (install na thakle M12 e staging e).
10. **Choto niyom**: `locales.enabled` e `en` na thakle `/en/*` `404`, hreflang shudhu `bn` ar `x-default`; suspended shop ba API 5xx/network e page `503` (`Retry-After: 3600` suspended e), `no-store`; root `/` `lang` cookie dekhe redirect kore na; client JS `?v=<git sha>`; featured product protiti slug e alada detail call (max 12), na pele bad.

### 10.11 M4 er decision (v1.19)

1. **Login form**: `@client LoginForm`: phone → BFF `/_/auth/otp/send` (JSON) → code box (phone memory te) → `/_/auth/otp/verify` → `/account` ba `?next=`. `next` shudhu `/` diye shuru, `//` ba `\` na (M3 er `_sameOriginPath` er moto). JS chhara form na: Turnstile nijei JS chhara chole na, tai phone rakhar notun cookie ba URL query (log, referrer) na.
2. **Turnstile**: platform er ekta widget, site key website env `TURNSTILE_SITE_KEY` (shop er hostname widget e jog; Rule 15); implicit render, `data-response-field-name="turnstile_token"`; Go siteverify e `hostname` = `X-Shop-Host` o mile. Local e Cloudflare er test key (13.5), tai asol code path chole. v1.22: `TURNSTILE_SECRET` Cloudflare er test secret (`1x`/`2x`/`3x...AA`) hole `hostname` check bad (test secret shob shomoy `example.com` dey); asol secret e local eo check. Test secret local chhara config e atke (v1.20, 13.5), tai production e check bad hoy na. Per-shop key MT e.
3. **CSP**: `script-src` ar `frame-src` e `https://challenges.cloudflare.com` shudhu Turnstile thaka page e (M4 e `/login`); baki page 10.10 item 7.
4. **BFF auth**: verify e Go er response er `token` theke `__Host-sid` (10.3) set, body theke `token` muche (JS kokhono token dekhe na); `/_/auth/logout` revoke ar cookie `Max-Age=0`. `ApiClient` e optional Bearer (`/account` SSR e `/api/me`).
5. **Account**: M4 e `/account` (naam, bhasha, logout; login chhara `/login` e `303`); `/account/orders`, `addresses` M6, `wishlist` M5, `privacy` M11 e. Bottom nav/header e Account link abar.

### 10.12 M5 er decision (v1.28)

1. **Guest cart cookie**: key Go banay (8.4 item 1); BFF Go er response er `cart_key` theke `__Host-cart` (`HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=2592000`) set kore, body theke `cart_key` muche (`_login` er moto). Protiti cart write e BFF cookie abar set kore (sliding 30 din, DB er 30 din `updated_at` er sathe mile).
2. **Merge e cookie**: BFF er verify `200` hole `__Host-cart` `Max-Age=0` (Go already merge koreche, 8.2 step 6). v1.30: BFF verify er body te `cart_notices` rekhe dey (token muche); login form `cart_notices` khali na hole ekta line dekhay (`item_dropped`: "Cart e jayga na thakay kichu item bad gelo", l10n key) ar tarpor redirect.
3. **Guest wishlist (`wl`)**: koma diye product ID (decimal), max 50, `Path=/; Secure; SameSite=Lax; Max-Age=1 bochor`, HttpOnly na. Guest e `@client WishlistButton` cookie nijei lekhe (server e jay na); login e BFF `/_/wishlist/{id}` (Go). Server `wl` diye kichu render kore na, tai `_personal` e lage na ar product page public cache thake; guest er wishlist page nai (`/account/wishlist` login chay). Merge: verify `200` er por `wl` thakle notun token diye `POST /api/wishlist/merge`, tarpor `wl` `Max-Age=0`; merge fail hole login fail na (log). Login kora user er heart er shuru obostha SSR e `GET /api/wishlist` theke (oi page `__Host-sid` er jonno already `private, no-store`).
4. **`@client` er poriman**: product page e `@client AddToCart` (quantity stepper + button; JSON e BFF, uttor e badge/lekha; JS chhara `<form method=post action=/_/cart/items>` → `303 /cart`; sticky bar e o). `/cart` page e `@client` na: protiti item e JS chhara form (qty `PATCH`, bad `DELETE`; BFF er `apiMethod` ar `POST /_/cart/items/{id}/delete`), `303` e page abar render, total server er (Rule 1). Checkout button M6 porjonto lukano. Pore dorkar hole `@client CartQty` jog kora shohoj.
5. **Cart badge**: SSR: `__Host-sid` ba `__Host-cart` cookie thakle Layout `GET /api/cart` er `count` (alada line, 7.4) dekhay (page er data call er shathe parallel); ei page already `private, no-store` (10.3). Cookie chhara page e badge nai, cache thake. JS na.
6. **Notice**: cart response er `notices` (`item_removed`, `quantity_reduced`, 8.4 item 4) `/cart` e l10n key diye dekhay; ojon-variable line e "approx".
