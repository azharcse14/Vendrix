# MU: Website UI pass

Design: Section 9.3 (v1.38 derived token), 9.4, 9.5, 9.6 (v1.38 font weight, scale, loading), 10.5 (v1.38 UI niyom, budget), 10.7 (v1.38 focus ring, `accent-color`, heading order); shathe 1.4 (Rule 1: discount % shudhu display; Rule 4: `website/` e hex ba Bangla literal na), 3.3 (font size), 15 (MU row), 16.1 (MU Q1–Q8). Plan ar ekhon er obostha er proman: [ui-upgrade-plan.md](../plans/ui-upgrade-plan.md).
Branch: main

**Obostha: cholche** (plan approve 16 November 2026: user shob recommendation nilen, MU M6 er por ar M7 er age; design v1.38). U0 shesh.

## Scope (15 er row theke)
- **Already ache** (M2–M6): `deriveTokens`/`cssVariables` (`packages/shared/lib/theme/tokens.dart`, test `packages/shared/test/theme/tokens_test.dart`), `themeCss` + `_fonts` (`website/lib/services/theme.dart`, test `website/test/services/theme_test.dart`), Tailwind config `var()` (`website/tailwind.config.js`, `website/tailwind.css`, `scripts/tailwind.sh`), page ar component (`website/lib/pages/*`, `website/lib/components/*`), page test (`website/test/pages/*`). 44px tap target, 16px input, safe-area, `srcset`, kom `@client` thakbe.
- **Token** (9.3 v1.38): soft (`primary-soft`, `accent-soft`), `success`/`danger`/`warning` + soft, `focus`, `text-rgb`, `radius-sm`/`radius-lg`, `shadow-1`/`shadow-2`; derive nijei contrast thik kore.
- **Font** (9.6 v1.38): body 400 only, heading 500/600, fallback stack, preload body bn 400; simulate LCP > 2.5s hole body Noto Sans Bengali `local()`.
- **Component class** (10.5 v1.38): `@layer components` (`.btn*`, `.input`, `.card`, `.badge*`, `.notice*`, `.chip`, `.section-title`), base focus ring, `accent-color`, select chevron, reduced-motion; Lucide inline SVG (`components/icon.dart`).
- **Page**: layout (header, bottom nav, footer), product card/price/badge/category tile/listing empty, home (hero fallback, trust strip), product, cart (JS chhara ± stepper), checkout section/radio card/OTP box, order/done/confirm/track/account (status badge map, timeline, empty state). Detail: plan Section 4, 5.
- **Baire**: logic, API, BFF, notun `@client` (JS +0); dark mode, animation, carousel, PWA; asol chobi (M12 er age shop owner); admin theme editor (M8).

## Acceptance criteria → proman
| Criteria | Test ba command | Obostha |
|---|---|---|
| Lighthouse mobile home/category/product: Performance ≥ 95, Accessibility 100 | `npx lighthouse` (devtools throttle) U10 | ☐ |
| LCP < 2.5s, simulate (Lantern) o | `npx lighthouse --throttling-method=simulate` U10 | ☐ |
| Client JS +0 byte (`main.client.dart.js` 176,175 B, gzip 54,444) | `jaspr build` er size, U8 ar U10 e log | ☐ |
| `styles.css` minify gzip ≤ 7 KB (≤ 30 KB) | `website/test` er budget test (U2) | ☐ |
| Prothom view font ≤ 3 file, ≈ 140 KB | `theme_test` (`@font-face` count/weight, U1) + Lighthouse network U10 | ☐ |
| Notun token er contrast AA 3 preset e (lekha 4.5, focus 3.0) + kharap rong | `packages/shared/test/theme/tokens_test.dart` (U1) | ☐ |
| CLS < 0.05 | Lighthouse U10 | ☐ |
| 360/390/412/768/1280 × bn/en screenshot, horizontal scroll nai | Safari `safaridriver` U10 | ☐ |
| Rule 4: `website/` e hex/Bangla literal nai | `make quick-check` (rule check) | ☐ |

## 14.2 er test
MU te 14.2 er notun test nai (UI pass); ager test (24 ar M6 er page test) green thakbe.

## File
- Notun: `website/lib/components/icon.dart`, website test e CSS budget check (U2).
- Bodlabe: `packages/shared/lib/theme/tokens.dart`, `packages/shared/test/theme/tokens_test.dart`, `website/lib/services/theme.dart`, `website/test/services/theme_test.dart`, `website/tailwind.config.js`, `website/tailwind.css`, `website/lib/components/*` (`layout.dart`, `product_card.dart`, `price.dart`, `product_listing.dart`, `filter_sheet.dart`, `gallery.dart`, `variant_picker.dart`, `checkout_form.dart`, `login_form.dart`, `add_to_cart.dart`), `website/lib/pages/*` (`home.dart`, `product.dart`, `cart.dart`, `checkout.dart`, `order.dart`, `confirm.dart`, `track.dart`, `status.dart`, `login.dart`, `account*.dart`), shared l10n (trust strip, empty state, badge er key bn/en), `website/test/pages/*`.

## Feature doc
- Bodlabe: `docs/features/theme.md` (notun token, font), `docs/features/website.md` (UI component class, icon, page er map), `docs/features/l10n.md` (notun key).

## Open question (design e nai, user ke jiggesh; 16.1 ar STATUS e o)
- Ekhon nai (MU Q1–Q8 16.1 e decided, v1.38).
- U10 proman er shima: ei Mac e shudhu Safari (`safaridriver`); Android Chrome ar Facebook in-app browser shudhu user device dile. Lighthouse `npx lighthouse` + headless Chrome thakle; na thakle Report e note.

## Progress
Protiti phase alada commit (test + code), por `make quick-check`. Logic thaka jaygay (token derive, contrast, discount %, status → badge map, ± stepper er form) test age, fail dekhiye `test(MU): ...`, tarpor `feat(MU): ...`. Page test structure ar lekha assert kore, class string na; golden HTML na (M3 Q9).

- [x] **U0. Design doc** (9.3, 9.6, 10.5, 10.7, 15, 16.1, README v1.38, 1.5 row 20). Commit `f31d5d6` `docs(MU): plan approve, design v1.38`; milestone file ar STATUS `docs(MU): milestone file, STATUS`.
- [ ] **U1. Token ar font: test age** (9.3, 9.6): `tokens_test.dart` e 3 preset × notun jora (badge lekha/soft 4.5, `primary`/`primary-soft` 4.5, `focus`/bg ar surface 3.0, kharap halka primary), `cssVariables` e notun variable (`--color-primary-soft` ... `--color-text-rgb`, `--radius-sm`, `--radius-lg`, `--shadow-1`, `--shadow-2`); `theme_test.dart` e body family shudhu 400, heading 500/600, file count, fallback stack, `:root` e notun var. Tarpor `tokens.dart`, `theme.dart`, `tailwind.config.js`. Commit `test(MU): derived token ar font weight`, `feat(MU): derived token, font kom`.
- [ ] **U2. Component class ar icon** (10.5, 10.7): website test e `styles.css` minify gzip ≤ 7 KB (`scripts/tailwind.sh --minify` er por); `tailwind.css` `@layer components` + base (focus, `accent-color`, select chevron, reduced-motion); `components/icon.dart` (Lucide ~12, ISC notice). Commit `test(MU): CSS budget`, `feat(MU): component class ar icon`.
- [ ] **U3. Layout** (header, bottom nav icon + active, footer): `catalog_pages_test` e `aria-current="page"`, landmark ekbar, ekta `h1`; 360/1280 screenshot. Commit `test(MU): layout landmark`, `feat(MU): header, bottom nav, footer`.
- [ ] **U4. Product card, price, badge, category tile, listing empty, heading order**: test age: discount % shared format (`round((old − new)/old × 100)`, < 5% e null), badge lekha (l10n key), out-of-stock lekha (rong only na), card heading order. Commit `test(MU): discount badge ar card`, `feat(MU): product card ar listing`.
- [ ] **U5. Home**: test age: banner chhara text hero, trust item settings onujayi ase/jay (COD, din, certification, return). Commit `test(MU): home hero ar trust strip`, `feat(MU): home`.
- [ ] **U6. Product page** (gallery, variant chip, price block, sticky bar, description `<details>`): product test (ache) + `details` structure. Commit `test(MU): product details`, `feat(MU): product page`.
- [ ] **U7. Cart** (thumbnail, JS chhara ± stepper, summary, empty): test age: `cart_test` e ± form er `qty` value ar JS chhara POST path. Commit `test(MU): cart stepper`, `feat(MU): cart line ar summary`.
- [ ] **U8. Checkout** (section card, `.input`, radio card, OTP box): `checkout_test` (ache) green; JS size log +0. Commit `feat(MU): checkout section`.
- [ ] **U9. Order/done/confirm/track/account**: test age: status → badge variant map (shared, protiti status), `order_test`/`track_test` e heading order ar timeline. Commit `test(MU): status badge map`, `feat(MU): order, track, account`.
- [ ] **U10. Proman**: 360/390/412/768/1280 × bn/en Safari screenshot, Lighthouse simulate + devtools (home/category/product), size table (JS, CSS, font); feature doc (`theme.md`, `website.md`, `l10n.md`) milano; `make check`; `design-reviewer`. Commit `docs(MU): proman ar report`.

## Report
