# Website UI upgrade: plan (khoshra, user er review er jonno)

> Obostha: **prostab**. Code, design doc, STATUS ba milestone e kichu bodlay nai. User approve korle `design-change` skill diye age 9 ar 10 er doc, tarpor code ar test. Notun milestone hole `milestone` skill er niyom: age `docs/milestones/MU.md` plan, approve, tarpor code.
> Pora hoyeche: 1.4 (Rule 4), 9.3–9.6, 10.5, 10.7, 10.10, 15; `website/lib/**`, `website/tailwind.{css,config.js}`, `website/lib/services/theme.dart`, `packages/shared/lib/theme/tokens.dart`; M6 Safari screenshot (scratch `m6/shots/`, commit na). M5 er screenshot nai, tai home/category/product/cart code theke.

## 1. Ekhon ki obostha

Mul kotha: token pipeline (5 rong → `deriveTokens` → `:root` CSS variable → Tailwind) bhalo ar Rule 4 mane. Kintu **token er upore kono design system nai**: shob page nijer moto Tailwind class lekhe (primary button er class 9 jaygay alada alada copy, `bg-primary px-4` / `px-6`), tai "basic" lage.

| Page group | Ki dekhlam | Proman |
|---|---|---|
| Shob page (layout) | Header e logo, Category, English, Cart, Account shob ekoi weight er lekha; active page bojha jay na (`aria-current` nai). Mobile bottom nav shudhu lekha, icon nai, active state nai. Footer shudhu policy link (underline), brand/contact/trust nai. | `components/layout.dart`; `done-360.png`, `account-orders-360.png`, `order-1280.png` |
| Typography | Heading hierarchy ulta: `/track` e card er bhetore "ইতিহাস" (h2) order number er cheye boro. Total ("মোট") baki line er moto, emphasis nai (order page). Choto lekha `text-sm text-muted` beshi, contrast thik kintu sob ek rokom. | `pages/track.dart`, `pages/order.dart`; `track-360.png`, `order-1280.png` |
| Form (checkout, login, track) | Input pill (`rounded` = 14px) kintu `<select>` Safari er native (alada kona, chevron); radio browser er nil (`accent-color` nai); focus ring browser er nil, token na (`focus` class kothao nai). Label ar ager field er majhe faka kom ("অন্য এলাকা" select er gaye lege ache). Field gulo kono group/section e na, ek lomba list. | `pages/checkout.dart`, `components/checkout_form.dart`; `checkout-360.png`, `checkout-1280.png` |
| Button | Primary ekta-i dhoron; destructive (order bathil) ar secondary input er moto dekhay (`order-1280.png`). Disabled shudhu `opacity-50`. | `pages/order.dart`, `pages/cart.dart` |
| Status | Order status plain lekha ("কনফার্ম হয়েছে"), badge/rong nai; itihash plain line, timeline na. | `track-360.png`, `account-orders-360.png`, `pages/status.dart` |
| Empty/success | Done page e success er kono visual nai (icon, summary, porer kaj er button); khali order list/cart/search e shudhu ek line `text-muted`. | `done-360.png`, `components/product_listing.dart` (`py-12 text-center text-muted`) |
| Home | Banner na thakle prothom screen e shudhu announcement + category chip (lekha); category er `imageKey` thakleo tile e chobi nai; "keno amader theke kinben" (COD, delivery, certification) nai. | `pages/home.dart` |
| Product card / listing | Card e chobi + naam + dam; discount %, stock badge, unit, rating nai; out-of-stock shudhu muted lekha. Card er h3 h2 chhara (M3 Lighthouse `heading-order`). | `components/product_card.dart`, `price.dart` |
| Product | Variant, qty, sticky bar kaj kore; kintu trust tottho (origin, certification `accent` badge, delivery er din, COD) chobir pashe nai; notice box (`bg-tint`) ar description ekoi dekhay. | `pages/product.dart` |
| Cart | Line e chobi nai (lekha only), qty `number` input + "update" button; summary card ar mobile sticky total ache kintu visual weight kom. | `pages/cart.dart` |
| Account | Naam + link list; order list e status/tarikh/dam ek line e chape (360 e). | `pages/account.dart`, `account_orders.dart` |

Bhalo ja ache (rakhbo): 44px tap target, input 16px, safe-area, mobile-first grid, `tint` placeholder, `srcset`, kom `@client`, contrast-safe `text-muted`.

## 2. Lokkho

"Bhalo" mane ei shop e: **bishshash** (COD, nirdishto delivery din, certification, asol phone number chokher shamne), **tajatar onubhuti** (asol chobi boro, kolapata sobuj + tint, shada jayga), **Bangla age** (Bangla heading/number shundor, kar-fola kata na), **sosta Android e druto**.

| Metric | Ekhon | Lokkho (protiti phase er por) |
|---|---|---|
| Lighthouse mobile (home/category/product, devtools throttle) | Perf 100/99/99, A11y 100/98/100 | Perf ≥ 95, A11y 100 (category o), LCP < 2.5s (simulate o, M3 er 2.7s thik hobe font kom kore) |
| `main.client.dart.js` | 176,175 B (gzip 54,444) | **+0**: UI pass e notun `@client` na; icon, state shob SSR/CSS |
| `styles.css` (minify) | 12.9 KB (gzip 3.5 KB) | ≤ 30 KB, gzip ≤ 7 KB |
| Font (prothom view) | 6 file ~270 KB | ≤ 3 file, ≤ 140 KB (Section 3.3) |
| Contrast | 6 jora 4.5:1 (9.3) | + notun token er jora (status, focus 3:1, badge) 3 preset e test e |
| CLS | ≤ 0.002 | < 0.05 |

## 3. Design system upgrade (token)

Niyom: admin age er moto **5 rong, 2 font, radius** dey (9.3); notun token shob `packages/shared/lib/theme/tokens.dart` er `deriveTokens`/`cssVariables` hishab kore, website `<head>` e `:root` e jay, Tailwind config shudhu `var()` (Rule 4). Fixed semantic rong (success/danger) shudhu shared e (Rule 4 check `website/` dekhe; shared e preset hex already ache).

### 3.1 Rong (notun derived token)

| Token | CSS variable | Hishab (shared) | Kothay |
|---|---|---|---|
| primary-soft | `--color-primary-soft` | `mix(background, primary, 10%)` | Active nav, selected chip/radio card, info notice |
| on-primary-soft | (text e `primary`) | `primary`/`primary-soft` ≥ 4.5 check | |
| accent-soft | `--color-accent-soft` | `mix(background, accent, 18%)` | Offer strip, discount badge er halka version |
| success, danger, warning | `--color-success` ... | Fixed (jemon success `#1E6B3A`, danger `#B42318`, warning `#8A5A00`), `text`/`bg` er upore 4.5 na hole text er dike shorano (muted er moto loop) | Status badge, form error, stock |
| *-soft | `--color-success-soft` ... | `mix(background, X, 12%)` | Badge/notice er pichon |
| focus | `--color-focus` | `primary`; `bg` er upore 3:1 na hole `text` | `:focus-visible` ring (WCAG 2.4.7/1.4.11) |
| text-rgb | `--color-text-rgb` | `59 42 26` (triplet) | Shadow: `rgb(var(--color-text-rgb) / .08)`; Safari/purono Chrome e `color-mix` lage na |

Contrast test: `packages/shared/test/theme/` e 3 preset × notun jora (badge lekha/soft pichon 4.5, focus/bg 3.0). Admin warning list (`_pairs`) bodlabe na (admin er dewa rong er jora-i), notun jora shudhu derive er bhetore auto-fix.

### 3.2 Radius, shadow, spacing, motion

- Radius: admin er `radius` = `--radius` (card, button). Derive: `--radius-sm` = `round(r/2)` (input chip, badge), `--radius-lg` = `min(r*1.5, 24)` (hero, sheet), `rounded-full` pill. Input ar select ekoi `--radius-sm` (ekhon pill vs native mix).
- Shadow: duita matro, `--shadow-1` (card hover/sticky bar) ar `--shadow-2` (sheet, dialog), text-rgb diye. Beshi shadow na: sosta screen e border-i bhalo.
- Spacing: Tailwind er 4px scale thake; niyom lekha: section majhe `space-y-10` (mobile 8), card bhetore `p-3`/`p-4`, form field majhe `space-y-4`, label-input `gap-1.5`.
- Motion: shudhu CSS `transition` 150ms (`color`, `background`, `box-shadow`, `transform` ≤ 2px); `@media (prefers-reduced-motion: reduce)` e off. JS animation na.

### 3.3 Typography ar font

- Scale (9.6 er upor, rem): 14 (choto, min, Bangla 12px e kar-fola bhange) / 16 body / 18 lead, price / 20 h3 / `clamp` h2, h1 (9.6 hubohu). Price `font-heading` 500–600, `tabular-nums` (Latin digit e; Bangla digit e font nijei fixed width na, tai price column right-align).
- Font weight kom: body font **shudhu 400**; emphasis (price, total, label er bold) heading font 500/600 diye. Organic e: Hind Siliguri bn 400 (71 KB) + Baloo Da 2 bn 500 (49 KB) + latin 2 ta (~34 KB, English page e ba Latin lekha thakle) ≈ 120–155 KB, ekhon er ~270 KB er ordhek. `theme.dart` er `_fonts` e weight list bodle (9.6 er "400 ar 500" niyom bodol, design-change).
- `<link rel=preload>` shudhu body bn 400 (M3 e preload e LCP bhalo hoy ni; tai Phase 1 e mapa, kaj na korle bad).
- Option (user): body `Noto Sans Bengali` hole `src: local("Noto Sans Bengali"), url(...)`: Android e system font, download 0 byte. Brand bhab kom, speed beshi.
- `font-display: swap` thake; fallback stack e `"Noto Sans Bengali", "Nirmala UI", sans-serif` jog (swap er shomoy layout kom nore).

### 3.4 Component class (Tailwind `@layer components`)

Notun Dart abstraction na. `website/tailwind.css` e choto class set, Tailwind token diye: `.btn`, `.btn-primary`, `.btn-secondary`, `.btn-ghost`, `.btn-danger`, `.input` (input/select/textarea ekoi; select e `appearance-none` + CSS chevron `background-image` SVG `currentColor`), `.card`, `.badge` (+ `-success/-danger/-accent/-muted`), `.notice` (+ variant), `.chip`, `.section-title`. Base e `:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px }`, `accent-color: var(--color-primary)` (radio/checkbox). Ei-i Rule 4 safe, HTML choto rakhe, page er duplicate class muche.

Icon: ~12 ta inline SVG (Lucide/Tabler, ISC/MIT, license file shoho) ekta Dart file e path string hishebe (`components/icon.dart`), `stroke="currentColor"`, `aria-hidden`. Home, Category, Cart, Account, Search, Leaf, Truck, Cash, Shield/Certificate, Check, Phone, Chevron. Font icon ba JS na.

## 4. Component list

| Component | Ki bodlabe |
|---|---|
| Header | Mobile: logo bame, Cart (badge) dane, niche full-width search (icon shoho). Desktop: logo · search (majhe, boro) · bhasha · Account · Cart icon+badge; niche category nav row (top-level, `featured_category_slugs`). `aria-current="page"`. Sticky na (screen kom), shudhu product page er sticky bar. |
| Bottom nav | Icon + choto lekha, active item `primary` + `primary-soft` pill, badge icon er kone. |
| Search | `.input` + icon button; khali result e empty state (category chip suggestion). |
| Product card | 1:1 chobi (tint + leaf icon placeholder), discount badge (`compare_at` theke % shudhu API er dam diye dekhano, hishab na: % shared format e `round((old-new)/old*100)`, Rule 1 er "taka hishab" na, display; review e confirm), out-of-stock badge + chobi `opacity`, unit ("/ kg", "৫০০ গ্রাম"), rating (thakle) choto star + shonkhya. Naam 2 line clamp, h3 → `p`/`h2` thik heading order. |
| Category tile | `imageKey` thakle 1:1 gol/rounded chobi + naam (mobile side-scroll, desktop grid 6); na thakle `primary-soft` er upor icon. |
| Button | `.btn-*` 4 dhoron; loading e `aria-busy` + lekha bodol (JS component e already ache, shudhu class). |
| Form/input | Label upore `font-medium`, hint/error niche (`danger`, icon, `aria-describedby`), required/optional chinho ekoi rokom; select custom chevron; radio "card" (payment method: border, `primary-soft` selected). |
| Price | Notun dam `text-lg font-heading`, purono `line-through text-muted`, discount badge pashe; total line `text-xl font-heading`. |
| Badge | Stock (`in` success-soft, `kom` warning-soft, `shesh` danger-soft, lekha + rong, 10.7), discount (`accent`), certification (`accent`, shudhu certificate thakle, 9.5). |
| Cart line | Thumbnail 64px, naam + variant, `−/+` stepper (JS chhara form: duita submit button `qty±1`, `@client` na), line total dane, remove ghost button. |
| Checkout | Section card: 1 Contact, 2 Thikana, 3 Payment, (OTP); numbered heading (step indicator na, ek page). Summary card desktop e sticky, mobile e upore collapse (`<details>`, JS chhara). Turnstile summary er niche, submit full-width. |
| Order status | `status.dart` er status → badge variant map (shared, test shoho); itihash vertical timeline (dot + line, CSS only), shesh step highlight. |
| Notice/toast | `.notice` info/success/warning/danger (icon + lekha, `role="status"`/`alert`). Toast na (JS lage); `AddToCart` er existing message `.notice` class pabe. |
| Empty state | Icon + ek line title + ek line help + ekta button (cart khali → "Kenakata shuru", order khali → home, search 0 → category chip). |
| Footer | 3 column (desktop)/stack: brand + tagline, contact (phone `tel:`, WhatsApp), policy link; DBID/BFSA; "COD available" trust line. Puro compliance M11 e thake. |

## 5. Page-by-page

- **Home** (`pages/home.dart`): banner na thakle text hero (branding name + tagline + "Shob ponno" CTA; chobi na, tai LCP lekha, druto). Banner thakle `--radius-lg`, prothomta `fetchpriority=high` (ache). Trust strip (3–4 item icon + lekha): COD, delivery din (zone theke min `days` thakle, na hole bad), certification (`features` e `certification` thakle), return window (`checkout.return_window_days`). Category tile grid; featured product grid; announcement `accent-soft` strip header er upore.
- **Category/Search** (`product_listing.dart`, `filter_sheet.dart`): breadcrumb (`text-sm`), title + result count, sticky filter/sort bar `shadow-1`, chip er active state, empty state, pagination button `.btn-secondary`.
- **Product** (`pages/product.dart`, `gallery.dart`, `variant_picker.dart`): gallery thumbnail row (desktop), variant chip `primary-soft` selected, price block (badge shoho), stock badge, "delivery X diner moddhe · COD" line, certification/origin badge, description `<details>` section (JS chhara). Sticky bar e price + button, `shadow-1`.
- **Cart** (`pages/cart.dart`): line component, notice `.notice-warning`, summary card + free-delivery progress lekha shudhu API dile (Rule 1: client hishab na; na thakle bad), empty state.
- **Checkout** (`pages/checkout.dart`, `checkout_form.dart`): section card, select/radio fix, error inline, OTP box `.card` + boro `inputmode=numeric` input, `price_changed` `.notice-warning`. Logic/JS bodol na, shudhu class (JS size +0).
- **Order/done/confirm/track** (`order.dart`, `confirm.dart`, `track.dart`): done page e success icon + order number boro + porer kaj (SMS confirm) `.notice-info` + "Order dekhun" `.btn-primary` + "Kenakata chaliye jan" secondary; order page e status badge + timeline, total emphasis, cancel `.btn-danger` (secondary er moto outline, danger rong) ar reason field `<details>` e; track e heading order thik (h1 → h2 result → timeline).
- **Login/Account** (`login.dart`, `login_form.dart`, `account*.dart`): login card (max-w-sm, center), phone input e `+880` prefix chip; account e menu card list (icon + chevron); order list card (number, badge, tarikh, total; 360 e 2 line).

## 6. Phase (protiti alada commit/PR, ager ta chhara-o ship hoy)

| # | Kaj | Test / proman | Size | User decision |
|---|---|---|---|---|
| U0 | Design doc: 9.3 notun token table, 9.6 font weight/scale, 10.5 UI niyom + CSS/font budget, 10.7 focus; README version | docsync | S | Q1–Q4 er uttor |
| U1 | `tokens.dart` derive (soft, semantic, focus, text-rgb, radius-sm/lg); `theme.dart` font weight kom + fallback stack; Tailwind config e notun token | shared: 3 preset × notun jora contrast; `theme_test`: `@font-face` er weight/count, `:root` e notun var; Lighthouse font file count | S | Q2 (font) |
| U2 | `tailwind.css` component class + base (focus, accent-color, select chevron, reduced-motion); `components/icon.dart` | `website/test` e ekta check: `styles.css` minify gzip ≤ 7 KB (tailwind.sh por); a11y: Lighthouse A11y 100 | S | Q5 (icon set) |
| U3 | Layout: header, bottom nav (icon, active), footer | `catalog_pages_test`: `aria-current`, landmark (`header/main/footer/nav` ekbar), ekta `h1`; 360/1280 screenshot | M | — |
| U4 | Product card, price, badge, category tile, listing empty state, heading-order fix | page test: badge lekha (key), out-of-stock lekha (rong only na), discount % (shared format test) | M | Q6 (discount %) |
| U5 | Home: hero fallback, trust strip, tile | home test: banner chhara hero, trust item settings onujayi ase/jay | M | Q3 (trust copy), Q7 (chobi) |
| U6 | Product page, gallery, variant chip, sticky bar | product test (ache) + `details` structure | M | — |
| U7 | Cart line (thumbnail, JS chhara ± stepper), summary, empty | `cart_test`: ± form er `qty` value, JS chhara POST path | M | — |
| U8 | Checkout section, form class, radio card, OTP box | `checkout_test` (ache) green; JS size log +0 | S–M | — |
| U9 | Order/done/confirm/track/account: status badge map, timeline, empty state | `order_test`, `track_test`: badge variant per status, heading order | M | — |
| U10 | Proman: 360/390/412/768/1280 × bn/en, Safari + Android Chrome + FB in-app, Lighthouse simulate + devtools, size table | Report | S | — |

Golden HTML test nai (M3 Q9: Jaspr update e bhange); structure/lekha assertion-i (ekhon er `website/test/pages/*` er moto). Class string assert na (bhongur).

**User er kache proshno** (16.1 e row hobe, recommendation shoho):

1. **Brand rong**: organic preset (`#1F5A32`, `#F2C230`, ...) rakhbo? Rec: **hya**, 9.5 er palette bhalo; shudhu derived token jog.
2. **Font**: (a) Hind Siliguri 400 + Baloo Da 2 500/600 (weight kom), (b) body Noto Sans Bengali (`local()`, Android e 0 byte) + Baloo heading, (c) ekhon er moto. Rec: **(a)**, Lighthouse e simulate LCP 2.5 er beshi thakle (b).
3. **Trust strip er lekha** (COD, delivery, certification, return): l10n key er default copy ami likhbo, user/shop owner dekhe. Rec: settings theke shudhu ja shotto (certification shudhu feature on + certificate), faka dabi na (9.5).
4. **Logo**: ekhon naam-text; asol logo (PNG, 2.2) ache? Rec: na thakle heading font e naam + leaf icon.
5. **Icon set**: Lucide (ISC) ba Tabler (MIT), ~12 ta inline SVG. Rec: **Lucide**.
6. **Discount %** card e dekhabo? Rec: **hya**, `compare_at_price` theke shudhu display (order/taka hishab na); 5% er kom hole bad.
7. **Photography**: asol product/krishok chobi kobe? (9.5) Rec: UI pass chobi chhara-o bhalo dekhabe (tint + icon placeholder); chobi M12 er age shop owner dibe.
8. **Milestone**: Section 7 er "MU" naki M12 e? Rec: **MU, M7 er por, M8 er age**.

## 7. Kothay boshe

Rec: **alada milestone "MU: Website UI pass"**, **M7 er por, M8 er age**. Karon: M8 (Store settings) admin e theme editor + preview banay, ekoi `deriveTokens` use kore; token age thik hole M8 er preview ekbar-i bananor kaj. M9 (review UI) ar M11 (footer compliance, account address) MU er component use korbe. M7 (admin, Flutter) block hoy na; website ar admin alada folder.

- M7 e mishale M7 er acceptance (staff role, packing) er sathe sompoko nai, review kothin. M12 e rakhle launch er age jhuki (Lighthouse CI ar font kom-o M12 e, 10.5), ar M8/M9/M11 er notun page purono style e banbe, pore abar bodlate hobe.
- Niyom: notun milestone, tai `docs/design/15-milestones.md` e MU row (design-change, version table), tarpor `docs/milestones/MU.md` plan (ei file theke), **user approve** kore, tarpor code. Protiti phase e `design-change` → test → code, `feature-doc` (`docs/features/website.md` e UI component er map), `make quick-check`, STATUS.
- Choto ar ekhon-i kora jay (MU chhara, `fix` skill diye): focus ring + `accent-color` (a11y bug, 10.7 "focus dekha jay"), category `heading-order`. User chaile age.

## 8. Jhuki

- **Performance**: icon inline SVG protiti page e HTML barabe (~2–3 KB, gzip e kom); `<symbol>` sprite ek bar `<body>` e rakhle kom. Notun `@client` ashle JS bare: niyom MU e `@client` 0, CI er JS size log dekhe. CSS barbe, gzip budget test atkabe. Font weight kom korle heading e nakol bold na hoy sheta dekhte hobe (9.6: Tiro Bangla 400 only).
- **Bangla rendering**: Safari/iOS ar sosta Android e kar-fola/juktakkhor kata (line-height 1.7 komano na, `line-clamp` e niche kata dekho); Bangla digit + `৳` price column align; English page e Latin subset load ase kina. Screenshot proman duita bhashay.
- **Contrast/theme**: notun derived token admin er je kono rong e kaj korte hobe (M8 e shop owner rong bodlabe): derive function nijei 4.5/3.0 thik kore, test 3 preset + kharap rong (halka primary) diye.
- **Maintenance**: `@layer components` class choto rakho (~12); page e aabar long class string ashle review e dhoro. Go draft save (M8) shudhu 9.3 er warning jora mile (notun token Go te lage na), tai Dart ar Go er hishab alada hoye jabe na.
- **Scope creep**: dark mode, animation, carousel, PWA (Phase 2) ei pass e na.
