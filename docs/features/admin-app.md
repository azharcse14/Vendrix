# Admin app (Flutter web)

Design: 11.1, 11.2, 11.3, 11.7 item 10, 2.3, 4.5, 9.7, 9.8, 13.3 · Milestone: M7b · Code: `admin/`

## Ki kore

Staff browser e admin khole (`admin.<domain>`, local e `https://admin.localhost:8443`): shop ID, email ar password diye login, TOTP ba recovery code; prothom login ba reset er por password bodol, authenticator app e TOTP setup ar 10 ta recovery code ekbar dekhe. Tarpor dashboard (ekhon shudhu shagotom ar logout; screen M7b er porer step e). Bhasha bn/en bodlano jay. Role onujayi route lukano; suraksha backend e (11.2).

## Kivabe kaj kore

1. `lib/main.dart` → `main`: `usePathUrlStrategy()`, `SemanticsBinding.instance.ensureSemantics()` (screen reader, WebDriver), `runApp(await bootstrap())`.
2. `lib/di.dart` → `bootstrap`: `ApiClient(API_BASE_URL ba same-origin, BrowserClient()..withCredentials = true)`, `AdminAuthRepository`, `AuthBloc`, `GoRouter` `get_it` e register; `AuthStarted` diye `GET /api/admin/me`, prothom state er por app (redirect kokhono `AuthUnknown` dekhe na). Session `__Host-asid` cookie e, Dart token dekhe na (11.3 item 4).
3. `lib/core/auth/auth_bloc.dart` → `AuthBloc`: state `Unauthenticated`, `TotpRequired(tempToken)`, `SetupRequired(step)`, `Authenticated(staff)`.
   - `LoginSubmitted` → `login`: `totp_required` → `TotpRequired`; `setup_required` → `_setup`; na hole `_me`.
   - `MfaSubmitted` → `totp` ba `recovery`; `setup_required` hole `_setup`, na hole `_me`. `mfa_invalid`/`validation_failed` e oi temp_token e thake, baki bhul (expire, lock) e `Unauthenticated(error)`.
   - `_setup`: `totp/setup`; `403 password_change_required` → `SetupStep.password`, na hole `SetupStep.totp` (secret, otpauth URL).
   - `PasswordSubmitted` → `password`, tarpor `_me` (TOTP live chhilo hole session active → `Authenticated`; `403 setup_required` → `_setup`).
   - `TotpEnableSubmitted` → `totp/enable` → `SetupStep.recoveryCodes`; `RecoveryCodesSaved` → `_me`.
   - `LogoutRequested` → `logout` (fail holeo) → `Unauthenticated`.
4. `lib/router.dart` → `buildRouter(AuthBloc)`: `refreshListenable: GoRouterRefreshStream(auth.stream)`, `redirect` = `authRedirect(state, matchedLocation)`: setup → `/login/setup`, TOTP → `/login/totp`, login chhara → `/login`, login hoye `/login*` → `/`, `canOpen` na hole `/`. Ochena route `errorBuilder`.
5. `lib/core/permissions.dart` → `StaffPermissions.canOpen` (`StaffProfile` er extension): route er prothom bhag → 11.2 er role.
6. `lib/core/theme/theme_cubit.dart` → `ThemeCubit`: `buildTheme(deriveTokens(organicColors))` (9.7, shared er organic preset); font `NotoSans` + fallback `NotoSansBengali` (`assets/fonts`, OFL).
7. `lib/core/l10n/locale_cubit.dart` → `LocaleCubit` (`bn` default, memory te), `context.t(key)` = shared er `Translator` (`error.*`) + `admin.*` (`bn.dart`, `en.dart`); `context.errorText(ApiError)` = `error.<code>`.
8. Page: `features/auth/pages/login_page.dart`, `totp_page.dart`, `setup_page.dart` (`AuthCard` widget), `features/dashboard/pages/dashboard_page.dart`.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `admin/lib/main.dart` → `main` | URL strategy, semantics, app |
| `admin/lib/di.dart` → `bootstrap` | `get_it` register, session check |
| `admin/lib/app.dart` → `AdminApp` | `AuthBloc`, `ThemeCubit`, `LocaleCubit`, `MaterialApp.router` (bn/en) |
| `admin/lib/core/auth/auth_bloc.dart` → `AuthBloc` | Login, TOTP, setup, logout |
| `admin/lib/router.dart` → `buildRouter`, `authRedirect`, `GoRouterRefreshStream` | Route ar guard |
| `admin/lib/core/permissions.dart` → `canOpen` | 11.2 role → route |
| `admin/lib/core/theme/theme_cubit.dart` → `buildTheme` | Token → `ThemeData` |
| `admin/lib/core/l10n/` → `LocaleCubit`, `L10n.t` | Bhasha, `admin.*` lekha |
| `admin/lib/features/auth/` | Login, TOTP, setup page |
| `scripts/admin.sh`, `make admin` | Release build (`--no-web-resources-cdn`, same-origin) |
| `infra/caddy/Caddyfile.local` → `admin.localhost` | `admin/build/web` + `/api` → `LOCAL_API`, CSP |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Role onujayi route: 11.2 er table (staff e product/category/coupon na, credential shudhu owner), ochena route na | `canOpen` | `admin/test/core/permissions_test.dart` → protiti route e `canOpen /coupons (11.2)` er moto test, `ochena role kichui khulte pare na` |
| Redirect: setup, TOTP, login, role (11.3) | `authRedirect` | `admin/test/router_test.dart` → `group('authRedirect (11.3)')` |
| AuthBloc bodlale router nijei redirect (`refreshListenable`) | `GoRouterRefreshStream` | `admin/test/router_test.dart` → `AuthBloc bodlale router nijei redirect kore` |
| Login → TOTP (bhul code e temp_token thake) → dashboard; recovery code; bhul password; temp_token expire (8.3) | `AuthBloc` | `admin/test/features/auth/auth_bloc_test.dart` |
| Setup: password (bhul `fields.old`) → TOTP (bhul code e ekoi secret) → recovery code → dashboard; TOTP live + `must_change_password`; reset er por shoja TOTP (8.3 step 4) | `AuthBloc._setup` | `auth_bloc_test.dart` → `prothom login: ...`, `TOTP live + must_change_password ...`, `reset er por ...` |
| Shuru te session nai ba network bhul → login (app atke na) | `AuthBloc` `AuthStarted` | `auth_bloc_test.dart` → `shuru: ...` |
| Logout API fail holeo local logout | `LogoutRequested` | `auth_bloc_test.dart` → `logout: ...` |
| temp_token, secret, recovery code state er `toString` e na (log) | `TotpRequired`, `SetupRequired` | `auth_bloc_test.dart` → `prothom login: ...` |
| Rong token theke, Bangla lekha shudhu `core/l10n/bn.dart` e (Rule 4, 11.7 item 10) | `buildTheme`, `l10n/` | `scripts/check-rules.sh` |

## Data

Nai (API: [auth.md](auth.md); session cookie `__Host-asid`, Dart e ba localStorage e kichu na).

## Bhul hole ki hoy

- API bhul: page e `error.<code>` (shared lekha); setup password e `fields.old`/`fields.new` field er niche.
- Session expire (`401`): login page e `error.session_expired`. Porer step e screen er `401` o `AuthBloc` e jabe.
- Font e na thaka glyph: CanvasKit `fonts.gstatic.com` chay, CSP atkay (tofu); tai Latin (Noto Sans) ar Bangla (Noto Sans Bengali) duitai asset.

## Bodlanor age mone rakho

- Notun route: `router.dart` e ar `permissions.dart` e 11.2 er role; `permissions_test` e row.
- `getIt<...>()` shudhu `di.dart` ar `BlocProvider(create:)` e (4.5).
- Flutter 3.47.9 (2.3; local e fvm), CI Flutter job ekoi version.
- Admin CSP (13.3) bodlale `Caddyfile.local` er `admin.localhost` o.

## Proshno-uttor

- **Shop ID keno lage?** Admin domain e shop nai; login body te shop slug (8.3). Build e `SHOP_SLUG` dile field age theke bhora.
- **QR code nai keno?** Package na; secret copy kore authenticator app e "manual key" diye dewa jay.
- **Login er por ager URL e ferot jay na keno?** Ekhon shob somoy `/`; dorkar hole `from` parameter pore.
