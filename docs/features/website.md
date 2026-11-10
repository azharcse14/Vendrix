# Website (Jaspr)

Design: 10.1–10.5, 10.9, 10.10, 3.2, 13.5 · Milestone: M3 · Code: `website/`

## Ki kore

Customer er website: Jaspr server mode e protiti page server e render hoy (SSR), dokan chena jay `Host` theke. Bangla root e, English `/en/...` e. Browser kokhono Go API te jay na; page er data Jaspr server API theke ane.
M3 cholche: ekhon skeleton, config, request theke API client ar `Cache-Control`; page, BFF, SEO porer step e ([M3](../milestones/M3.md)).

## Kivabe kaj kore

1. Shuru: `lib/main.server.dart` nijer shelf server `127.0.0.1:$PORT` e (13.5; `jaspr serve`/build `PORT` dey, local default 8081), `Pipeline` e `cacheControl()` tarpor `serveApp` → Jaspr `Document` (`lang`, viewport `viewport-fit=cover`, `/styles.css`) ar `App` (`lib/app.dart`, `jaspr_router`). Env `lib/services/config.dart` → `config` (`Config.fromEnv`): `API_BASE_URL`, `JASPR_INTERNAL_SECRET`, `IMAGE_BASE_URL`, `IMAGE_RESIZE`; bhul hole `FormatException` e variable er naam.
2. Bhasha: `localeOf(path)` (`lib/services/locale.dart`): `/en` ba `/en/...` → `en` ar baki route, na hole `bn`. Browser er `Accept-Language` dekha hoy na (10.3: URL-i bhasha, cache thik thake).
3. API: `apiFor(requestHeaders, locale, config, httpClient)` (`lib/services/api.dart`) protiti request e notun `ApiClient` (shared), ek `http.Client`. Header `apiHeaders`: `X-Shop-Host` = `Host` (API nijei port bad diye normalize kore), Caddy er `X-Real-IP` thakle `X-Client-IP` + `X-Internal-Auth`; local e `X-Real-IP` nai, API connection IP ney.
4. CSS: Tailwind v3.4.19 standalone CLI (`scripts/tailwind.sh`, version ar sha256 pin, binary `website/.dart_tool` e) `tailwind.css` → `web/styles.css`; `tailwind.config.js` e rong, radius, font shudhu CSS variable (9.3). `jaspr_tailwind` Dart 3.13 e resolve hoy na (v1.14).
5. Cache (10.3): `cacheControl()` (`lib/services/cache_control.dart`): BFF `/_/...` → `no-store`; `__Host-sid`, `__Host-cart`, `preview` cookie ba response e `Set-Cookie` → `private, no-store` (page er nijer header o bodle); baki page nije `Cache-Control` dile seta, na dile 2xx e `public, max-age=60`, onno status e `no-store`.
6. Local: `make web` (`scripts/web.sh`): `website/.env` load, Tailwind `--watch`, `jaspr serve` (port `pubspec.yaml` er `jaspr.port` 8081).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `website/lib/main.server.dart` → `main` | Server entry, `Document` |
| `website/lib/main.client.dart` | Browser entry, shudhu `@client` |
| `website/lib/app.dart` → `App` | Route |
| `website/lib/services/config.dart` → `Config.fromEnv`, `config` | Env (13.5) |
| `website/lib/services/locale.dart` → `localeOf` | URL theke bhasha |
| `website/lib/services/cache_control.dart` → `cacheControl` | `Cache-Control` middleware |
| `website/lib/services/api.dart` → `apiHeaders`, `apiFor`, `httpClient` | Request theke `ApiClient` |
| `website/tailwind.config.js`, `scripts/tailwind.sh` | Tailwind token ar build |
| `scripts/web.sh` | `make web` |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| `/en/...` → `en`, root → `bn`; `/english` `bn` (9.8, 10.3) | `localeOf` | `website/test/l10n/locale_route_test.dart` → `group('URL theke bhasha (9.8, 10.3)')` |
| Shop `Host` theke; `X-Client-IP` + `X-Internal-Auth` shudhu Caddy er `X-Real-IP` thakle; `Accept-Language` URL theke (3.2, 10.3) | `apiHeaders`, `apiFor` | `website/test/services/api_test.dart` → `group('API header (3.2, 10.3)')` |
| Personal cookie (`__Host-sid`, `__Host-cart`, `preview`) ba `Set-Cookie` → `private, no-store`; cookie chhara public page `public, max-age=60`; BFF `no-store`; non-2xx `no-store` (10.3, 14.2 test 24) | `cacheControl` | `website/test/server/cache_control_test.dart` → `group('Cache-Control (10.3, 14.2 test 24)')` |
| Env nai ba bhul hole server uthe na (13.5) | `Config.fromEnv` | `website/test/services/api_test.dart` → `group('Config (13.5)')` |

## Data

Nai (website database dhore na; shob Go API theke).

## Bhul hole ki hoy

- Env nai ba bhul: prothom `config` use e `FormatException` (variable er naam shoho).
- `Host` nai: `X-Shop-Host` khali, API `404 shop_not_found`.

## Bodlanor age mone rakho

- `X-Real-IP` shudhu Caddy dey: tai server shudhu loopback e (`main.server.dart`); Jaspr er default `runApp` `0.0.0.0` e bind kore, oita use koro na.
- Notun personal cookie (jemon M4/M5 e) hole `_personal` e jog ar test.
- Jaspr update hole `jaspr install-skills --agent claude-code` abar ar `*.options.dart` (`dart run build_runner build`).

## Proshno-uttor

- **Browser er bhasha keno dekhi na?** Ekoi URL e alada HTML hole Cloudflare cache bhul bhashar page dite pare (10.3); bhasha bodlano shudhu link diye.
