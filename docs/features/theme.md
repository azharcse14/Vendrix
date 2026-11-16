# Theme token ar contrast (shared)

Design: 9.3, 9.4, 9.7 · Milestone: M2, MU · Code: `packages/shared/lib/theme`

## Ki kore

Admin shudhu 5 ta rong dey (primary, accent, tint, text, background); baki token (hover, card, border, halka lekha, button er lekha) ei code nijei banay. Lekha porte kothin emon rong er jora pele admin e warning dekhay. Website (CSS variable), admin ar app (Flutter theme) ekoi token pay.

## Kivabe kaj kore

1. `deriveTokens(colors)` (`theme/tokens.dart`): 5 rong (`colorKeys`) `#RRGGBB` na hole ba na thakle `ArgumentError`.
2. Hishab sRGB channel e `mixColor` (v1.12): `primary-hover` = primary 8% kalo, `surface` = background 50% shada, `border` = background theke text er dike 12%.
3. `text-muted`: background theke text er dike 65% theke shuru, 1% kore barano jotokkhon `background` ar `surface` duitar upore `contrastRatio` ≥ `minContrast` (4.5).
4. `on-primary`, `on-accent`: shada ba `text`, je ta contrast beshi.
5. v1.38 (MU) derived rong: `primary-soft` = `mix(background, primary, 10%)`, `accent-soft` = `mix(background, accent, 18%)`; lekha (`primary`, `text`) er sathe 4.5 na hole % 1 kore komano (0 te `background`). `success`/`danger`/`warning` fixed (`#1E6B3A`/`#B42318`/`#8A5A00`) theke shuru, `background` ar nijer `-soft` (`mix(background, X, 12%)`) e 4.5 na hole `text` er dike 1% kore. `focus` = `primary`, `background` ba `surface` e 3:1 na hole `text`. Ei jora admin warning e na (derive nijei thik kore).
6. `contrastWarnings(tokens)`: `text`/`background`, `text`/`surface`, `text`/`tint`, `on-primary`/`primary`, `on-accent`/`accent`, `primary`/`background` e 4.5 er kom hole `ContrastWarning` (lekha key `theme.low_contrast`).
7. `cssVariables(tokens, radius:, headingFont:, bodyFont:)`: 9.3 er CSS variable naam (`background` → `--color-bg`, notun rong `--color-primary-soft` ... `--color-focus`), `--color-text-rgb` (`text` er triplet, jemon `59 42 26`), `--radius` px, `--radius-sm` = `round(r/2)`, `--radius-lg` = `min(round(r × 1.5), 24)`, `--shadow-1`/`--shadow-2` (text-rgb er rgb alpha), font `"..."`; font e shudhu harf/shonkhya/space (admin er lekha `<style>` e dhukano na), radius 0–64, na hole `ArgumentError`. Website `<head>` er `:root` (M3).

Website e (M3, [website.md](website.md)): `themeCss` (`website/lib/services/theme.dart`) shop er settings theke `deriveTokens` + `cssVariables` diye `<head>` er `:root` `<style>` banay; Tailwind config (9.3) shudhu `var(--...)` token (MU: `primary-soft`, `accent-soft`, `success`/`danger`/`warning` + `-soft`, `focus`, `rounded-sm`/`rounded-lg`, `shadow-1`/`shadow-2`), component e hex rong na (`scripts/check-rules.sh` Rule 4). Notun token gulo `website/tailwind.css` er component class (`.btn*`, `.badge*`, `.notice*`, `.chip`) ar base focus ring e (MU U2, [website.md](website.md)).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `packages/shared/lib/theme/tokens.dart` → `deriveTokens`, `mixColor`, `contrastRatio`, `contrastWarnings`, `ContrastWarning`, `cssVariables` | Token, contrast, CSS variable |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| WCAG contrast `(L_halka + 0.05) / (L_gaḍho + 0.05)` (9.3) | `contrastRatio` | `test/theme/tokens_test.dart` → `group('contrastRatio (9.3 WCAG)')` |
| sRGB mix, round (9.3, v1.12) | `mixColor` | `test/theme/tokens_test.dart` → `group('mixColor (9.3, v1.12 sRGB)')` |
| Tin preset er token golden; muted background/surface e ≥ 4.5; preset e warning nai; bhul hex error; v1.38 notun jora (status/soft 4.5, `primary`/`primary-soft` 4.5, `text`/`accent-soft` 4.5, `focus` 3.0) 3 preset, halka primary, rongin background (9.3, 9.4) | `deriveTokens` | `test/theme/tokens_test.dart` → `group('deriveTokens (9.3, 9.4, v1.12)')` |
| CSS variable er naam, radius, font; v1.38 notun variable, `radius-sm`/`radius-lg` er hishab; font e CSS dhukano atkano (9.3) | `cssVariables` | `test/theme/tokens_test.dart` → `group('cssVariables (9.3)')` |
| Kom contrast er jora dhora (9.3, v1.12) | `contrastWarnings` | `test/theme/tokens_test.dart` → `group('contrastWarnings (9.3, v1.12)')` |

## Data

`shop_settings.published.theme.colors` (5 rong, 9.2); preset er rong `backend/internal/shop/settings.go` er `presets` (9.4).

## Bhul hole ki hoy

- Rong nai ba `#RRGGBB` na: `ArgumentError` (admin form e age check).
- Kharap jora: token bane, sathe warning; save atkay na (9.3).

## Bodlanor age mone rakho

- Formula bodlale 9.3 (design-change), golden test er hex, ar M8 e Go er draft save er warning ekoi hishab rakhte hobe.
- Preset er rong bodlale Go er `presets` ar ei test er `presets` duitai.

## Proshno-uttor

- **Shadow e `color-mix` na keno?** `rgb(59 42 26 / .08)` purono Safari/Chrome e o chole (v1.38).
- **Notun jora admin warning e nai keno?** Egulo admin er dewa rong na; derive nijei thik kore, tai M8 er Go draft save e notun hishab lage na (9.3 v1.38).
- **OKLCH na keno?** sRGB mix choto, CSS `color-mix(in srgb)` er sathe mile (v1.12).
