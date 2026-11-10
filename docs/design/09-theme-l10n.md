> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 9. Theme, design system ar localization

### 9.1 Mul niyom

Design code e na, **data te**. Layout ar component ekoi thake, shudhu theme (rong, font, kona, logo) ar lekha bodlay. Flutter e jemon `ThemeData` bodlale puro app bodlay. Kono component e hex color ba user-facing lekha hardcode korbe na.

Theme shop er. Marketplace e vendor er shudhu logo ar description (`vendors.profile`), vendor nijer rong ba font dey na, jate puro website ek rokom dekhay. Multi-tenant e protiti shop er nijer theme.

### 9.2 Settings JSON (`shop_settings.draft` / `published`)

```json
{
  "branding": {
    "name": { "bn": "সবুজ বাজার", "en": "Sobuj Bazar" },
    "tagline": { "bn": "কৃষকের কাছ থেকে সরাসরি", "en": "Straight from the farm" },
    "logo_key": "shops/1/logo.png",
    "favicon_key": "shops/1/favicon.png",
    "phone": "+8801XXXXXXXXX",
    "whatsapp": "+8801XXXXXXXXX",
    "email": "hello@example.com",
    "address": { "bn": "...", "en": "..." },
    "social": { "facebook": "https://facebook.com/...", "instagram": null },
    "dbid_number": "...",
    "bfsa_number": "..."
  },
  "theme": {
    "preset": "organic",
    "colors": {
      "primary": "#1F5A32", "accent": "#F2C230", "tint": "#E3EED3",
      "text": "#3B2A1A", "background": "#FBFBF6"
    },
    "fonts": { "heading": "Baloo Da 2", "body": "Hind Siliguri" },
    "radius": 14
  },
  "locales": { "enabled": ["bn", "en"], "default": "bn", "bangla_digits": true },
  "features": ["unit_pricing", "variable_weight", "batch_expiry", "certification"],
  "checkout": {
    "payment_methods": ["cod", "sslcommerz"],
    "otp_mode": "risky",
    "cod_confirmation": "risky",
    "auto_cancel_unconfirmed_hours": 24,
    "risk": { "cod_amount_over": 300000, "new_phone_amount_over": 200000, "max_orders_per_hour": 3 },
    "sms_daily_cap": 500,
    "return_window_days": 3
  },
  "inventory": { "expiry_alert_days": 7 },
  "marketplace": { "delivery_charge_mode": "per_order", "default_commission_bps": 0, "show_vendor_name": false },
  "homepage": {
    "announcement": { "bn": "৳১,০০০ বা তার বেশি অর্ডারে ফ্রি ডেলিভারি", "en": "Free delivery on orders of ৳1,000 or more" },
    "banners": [ { "image_key": "shops/1/banner1.jpg", "alt": { "bn": "...", "en": "..." }, "link": "/c/chal" } ],
    "featured_category_slugs": ["chal", "modhu"],
    "featured_product_slugs": []
  },
  "seo": { "title": { "bn": "...", "en": "..." }, "description": { "bn": "...", "en": "..." }, "og_image_key": null, "noindex": true },
  "integrations": { "facebook_pixel_id": null, "ga4_id": null },
  "policies": { "terms": { "bn": "...", "en": "" }, "return": {}, "refund": {}, "delivery": {}, "privacy": {} }
}
```

`PUT /api/admin/settings/draft` e backend validate korbe: rong `#RRGGBB`, font allowed list theke, `radius` 0 theke 24, `features` jana list theke, `locales.enabled` e `bn` thakbei, taka integer poisha, banner `link` shudhu `/` diye shuru (kintu `//` na: protocol-relative URL onno site e jay; v1.5: `\`, tab, newline ba onno control character o na, karon browser `/\evil.com` ke `//evil.com` dhore; string na dekhe URL parse kore host khali kina dekho) ba `https://`, social link `https://` (`javascript:` na), logo PNG/JPEG (SVG ar WebP na, 2.2), `marketplace.delivery_charge_mode` `per_order` ba `per_vendor` (single mode e `marketplace` block use hoy na). Public `GET /api/settings` e `checkout.risk` er moto vitorer setting jabe na; v1.10: whitelist (7.2 er list), notun setting default e private. `seo.noindex` limited launch e `true` (14.4).

v1.13: `policies` block (policy page er plain text, 10.10) public whitelist e na; `GET /api/policies/{kind}` alada (settings payload choto rakhte); notun shop e Bangla placeholder, admin editor M8 e.

### 9.3 Design token

Admin shudhu 5 ta rong, 2 ta font ar radius dey. Baki token `packages/shared` er ekta function hishab kore, jate admin preview ar website **hubohu ekoi** dekhay.

| Token | CSS variable | Kivabe ashe | Kothay use |
|---|---|---|---|
| primary | `--color-primary` | Admin | Button, link, dam |
| primary-hover | `--color-primary-hover` | primary 8% gaḍho | Button hover/press |
| on-primary | `--color-on-primary` | Shada ba text rong, je ta contrast beshi | Button er lekha |
| accent | `--color-accent` | Admin | Shudhu offer, badge, certification |
| on-accent | `--color-on-accent` | Contrast diye | Badge er lekha |
| tint | `--color-tint` | Admin | Chobir pichone, selected chip |
| text | `--color-text` | Admin | Lekha |
| text-muted | `--color-text-muted` | text 65% + background 35%; 4.5:1 na hole text er dike shorano (derive function nijei) | Choto tottho |
| background | `--color-bg` | Admin | Page |
| surface | `--color-surface` | background er cheye ektu halka/shada | Card |
| border | `--color-border` | text 12% | Line, input |
| radius | `--radius` | Admin | Button, card, chip |
| heading font | `--font-heading` | Admin | Heading, logo text |
| body font | `--font-body` | Admin | Baki shob |

Contrast check (WCAG): rong ke sRGB theke linear kore `L = 0.2126 R + 0.7152 G + 0.0722 B`, ratio = `(L_halka + 0.05) / (L_gaḍho + 0.05)`. Lekha ar tar background er ratio 4.5 er kom hole admin e warning: "Ei rong e lekha porte kothin hobe." Backend-o draft save e ei warning ferot dey. Derived token (muted) er contrast-o function nijei 4.5:1 rakhe: v1.1 er "35% mix" e organic preset er muted lekha 2.0:1 (ba ulto mane 4.46:1) chilo.

v1.12 (M2): derive er hishab sRGB channel e linear mix (`mix(a, b, t) = round(a + (b − a)·t)`, CSS `color-mix(in srgb, ...)` er moto): `primary-hover = mix(primary, #000000, 8%)`, `surface = mix(background, #FFFFFF, 50%)`, `border = mix(background, text, 12%)`, `text-muted` `mix(background, text, 65%)` theke shuru, 1% kore text er dike jotokkhon `background` ar `surface` duitar upore 4.5 na hoy; `on-primary`/`on-accent` shada ba `text`, je ta contrast beshi. Contrast warning ei jora gulote, shob 4.5: `text`/`background`, `text`/`surface`, `text`/`tint`, `on-primary`/`primary`, `on-accent`/`accent`, `primary`/`background` (link, dam); warning er lekha key `theme.low_contrast`. Code `packages/shared` er `theme/` (Go draft save M8 e ekoi hishab).

Tailwind e class gulo variable ke point kore, kokhono hex na:

```js
// tailwind.config.js (website)
theme: { extend: {
  colors: {
    primary: 'var(--color-primary)', 'primary-hover': 'var(--color-primary-hover)',
    'on-primary': 'var(--color-on-primary)', accent: 'var(--color-accent)',
    'on-accent': 'var(--color-on-accent)', tint: 'var(--color-tint)',
    ink: 'var(--color-text)', muted: 'var(--color-text-muted)',
    page: 'var(--color-bg)', surface: 'var(--color-surface)', line: 'var(--color-border)',
  },
  borderRadius: { DEFAULT: 'var(--radius)' },
  fontFamily: { heading: 'var(--font-heading)', body: 'var(--font-body)' },
}}
```

Jaspr server protiti page er `<head>` e `:root { --color-primary: #1F5A32; ... }` boshay, published settings theke. Tailwind v3 e `bg-primary/50` er moto opacity modifier `var()` rong e kaj kore na; dorkar hole alada token.

### 9.4 Preset

| Preset | primary | accent | tint | text | background | Heading font | Body font | radius |
|---|---|---|---|---|---|---|---|---|
| `organic` | `#1F5A32` kolapata | `#F2C230` shorshe ful | `#E3EED3` kochi dhan | `#3B2A1A` khejur gur | `#FBFBF6` chal | Baloo Da 2 | Hind Siliguri | 14 |
| `fashion` | `#2A3480` nil | `#B83A5E` gamchha lal | `#ECEDF7` | `#1C1E33` | `#FFFFFF` | Noto Serif Bengali | Hind Siliguri | 2 |
| `electronics` | `#0A5BB5` | `#0B7F65` | `#E4EEFA` | `#14202E` | `#F4F7FB` | Anek Bangla | Hind Siliguri | 10 |

Preset beche nile 5 rong, font ar radius bhore jay, tarpor admin bodlate pare.

### 9.5 Organic er design nirdeshona

Rong Bangladesh er khet theke: kolapata sobuj main rong, shorshe holud **shudhu** offer/badge/certification e (shob jaygay dile tar mane hariye jay), kochi dhaner halka sobuj chobir pichone, ar kalo er bodole khejur gurer badami lekha. Beshirbhag organic site cream ar terracotta use kore, tai shob ek rokom lage. Ei palette tomar ta alada rakhe.

Chobi-i asol design: stock photo na, nijer product ar krishok er asol chobi, prakritik aloy, ekoi ratio (4:3 ba 1:1) ar ekoi dhoroner background. Lekha shohoj ar shotto: "নওগাঁর কৃষকদের কাছ থেকে সরাসরি" er moto nirdishto kotha, "best quality" er moto faka dabi na. "Organic" shudhu certificate thakle (Section 12).

### 9.6 Font ar typography

Shudhu ei font gulo allowed (shob Bangla ar Latin duitai shapport kore), ar segulo nijer server e (`website/web/fonts/`), Google theke protibar load na:

| Font | Dhoron |
|---|---|
| Hind Siliguri | Body er jonno shobcheye porishkar |
| Baloo Da 2 | Gol, bondhuttopurno heading |
| Tiro Bangla | Serif, traditional (shudhu 400 weight; 500 dile browser nakol bold banay) |
| Noto Sans Bengali | Neutral sans |
| Noto Serif Bengali | Elegant serif |
| Anek Bangla | Modern, condensed |

| Jinish | Mobile | Desktop | Line-height |
|---|---|---|---|
| Body | 16px | 16–17px | 1.7 (Bangla kar-fola er jonno) |
| Choto lekha | 14px | 14px | 1.6 |
| h3 | 20px | 22px | 1.35 |
| h2 | `clamp(1.5rem, 4vw, 2rem)` | | 1.3 |
| h1 | `clamp(1.75rem, 6vw, 2.75rem)` | | 1.25 |

Font weight shudhu 400 ar 500 (heading font e 600 thakle sheta). `font-display: swap`. Line er dairgho 70 okkhor er beshi na. Button ar chip e fixed width na, karon English lekha prai Bangla er cheye lomba.

### 9.7 Flutter e theme

Admin preview ar Phase 2 app ekoi settings JSON theke `ThemeData` banay (`ThemeCubit`):

```dart
ThemeData buildTheme(ThemeTokens t) {
  final heading = TextStyle(fontFamily: t.headingFont);   // v1.5: heading font o, website er moto (9.3)
  return ThemeData(
    colorScheme: ColorScheme.light(
      primary: t.primary, onPrimary: t.onPrimary,
      secondary: t.accent, onSecondary: t.onAccent,
      surface: t.background, onSurface: t.text,
    ),
    fontFamily: t.bodyFont,               // font asset hishebe bundle kora
    textTheme: TextTheme(
      displaySmall: heading, headlineLarge: heading, headlineMedium: heading,
      headlineSmall: heading, titleLarge: heading,
    ),
    cardTheme: CardThemeData(color: t.surface,   // v1.6: card e surface token (9.3)
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(t.radius))),
  );
}
```

`ThemeTokens` toiri hoy `packages/shared` er ekoi derive function diye (9.3).

### 9.8 Localization

Duitar lekha, duita alada bebostha:

| Dhoron | Udahoron | Kothay thake | Ke bodlay |
|---|---|---|---|
| UI lekha | Button, menu, error, SMS | `packages/shared/lib/l10n/bn.dart`, `en.dart` (v1.12: `const Map<String, String>`, pure Dart e JSON file pora jay na) + `translations` table (override) | Developer (default), admin (override) |
| Content | Product er naam, description, category, policy, banner | Database `JSONB {"bn","en"}` | Admin |

**Key er naam**: `bhag.jinish` ba `bhag.jinish.obostha`, jemon `cart.add`, `checkout.place_order`, `product.out_of_stock`, `error.out_of_stock`, `sms.order_placed`. Placeholder `{name}`, `{amount}`. English plural er jonno `.one` ar `.other` (`cart.items.one`, `cart.items.other`).

```json
{ "cart.add": "কার্টে যোগ করুন", "checkout.place_order": "অর্ডার করুন",
  "error.out_of_stock": "{name} এখন স্টকে নেই", "sms.order_placed": "{shop}: অর্ডার {number} পেয়েছি। দেখুন {link}" }
```

**Lekha khojar kram**: `override[locale][key]` → `file[locale][key]` → `override[bn][key]` → `file[bn][key]` → key nijei (ar log e "missing translation"). Content e `value[locale] ?? value["bn"]`.

**Shonkhya, taka, tarikh**: database e shob shomoy integer poisha. Dekhano `intl` diye: `bangla_digits` on hole `bn` locale (Bangla shonkhya, lakh-style comma, `৳১,২৫,০০০`), off hole `en_IN` (`৳1,25,000`). v1.5: `intl` er `bn` currency pattern `#,##,##0.00¤` (৳ pichone, duita doshomik), `en_IN` er `¤#,##,##0.00`; tai `NumberFormat.currency` na, `NumberFormat('#,##,##0', locale)` (poisha thakle `#,##,##0.00`) diye shonkhya ar ৳ samne nije boshao. Test e `৳১,২৫,০০০`, `৳১৮০.৫০`, `৳1,25,000` (M2). Poisha na thakle doshomik dekhabe na (`৳১৮০`, `৳১৮০.৫০` na hole). Tarikh `DateFormat('d MMMM y', locale)`: "২৭ অক্টোবর ২০২৬". `packages/shared/format` e eder unit test likho.

v1.12 (M2): Bangla digit shudhu UI bhasha `bn` **ar** `bangla_digits` on hole (`en` UI te shob shomoy English digit); mash er naam UI bhasha theke (`bn` + off = "27 অক্টোবর 2026"). Minus taka `-৳১৮০` (chinho ৳ er age). API UTC dey; tarikh shop er timezone e: pure Dart e tz database nai, tai `packages/shared` e choto offset map (`Asia/Dhaka` = +6:00, DST nai), ochena zone e error; onno timezone er shop (MT) ele package ba design-change. Backend er SMS template er default (`sms.*`) Go te alada, M4 e decision (Go `embed` `packages/` e pouchay na).

**URL**: Bangla default root e (`/p/organic-red-rice`), English `/en/` er niche (`/en/p/organic-red-rice`). Protiti page e `hreflang` (`bn`, `en`, `x-default`). `<html lang>` bhasha onujayi. Bhasha bodlale `lang` cookie, login thakle `users.locale`. SMS customer er bhashay, invoice `orders.locale` e.

**Mone rakho**: policy ar terms er Bangla version shob shomoy thakbe (DBID er jonno), English chalu thakleo.
