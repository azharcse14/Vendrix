# Admin app (Flutter web)

Design: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7 item 4–8, 10, 13 vii–viii, 14, 2.3, 4.5, 9.7, 9.8, 13.3 · Milestone: M7b · Code: `admin/`

## Ki kore

Staff browser e admin khole (`admin.<domain>`, local e `https://admin.localhost:8443`): shop ID, email ar password diye login, TOTP ba recovery code; prothom login ba reset er por password bodol, authenticator app e TOTP setup ar 10 ta recovery code ekbar dekhe. Tarpor dashboard (ajker count, tap e oi list), order list/detail (confirm, cancel, pack, undo, note, call, print), packing (pick list, ojon diye pack, ojon beshi hole customer er raji note) ar shipment (carrier, tracking, delivered, parcel ferot); phone e 360px e chole (11.5). Product, inventory, staff ityadi M7b er porer step e. Bhasha bn/en bodlano jay. Role onujayi route lukano; suraksha backend e (11.2).

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
8. Page: `features/auth/pages/login_page.dart`, `totp_page.dart`, `setup_page.dart` (`AuthCard` widget).
9. `di.dart` → `registerServices` (`ApiClient`, `Browser`, repository); client `SessionClient` e mora: login er porer kono API `401` → `AuthBloc` e `SessionExpired` → `Unauthenticated(error.session_expired)` → router `/login` (shudhu `Authenticated` state e; login/TOTP er 401 nijer handler e). `getIt` `core/locator.dart` e (router `package:web` chhara compile hoy, test VM e).
10. `router.dart`: login er porer route `ShellRoute` e `AppShell` (`core/widgets/app_shell.dart`): upore naam, bhasha, logout; menu `menuRoutes` theke `canOpen` diye (11.2), 600px er niche drawer, upore `NavigationRail`. Protiti route e `BlocProvider(create: X(getIt())..load())` (11.3); page shudhu `context.watch`.
11. **Dashboard** (`/`, `DashboardCubit`): `GET /dashboard` er count card; `pending` → `/orders?tab=awaiting_confirm`, `deadline_soon` → `/orders?tab=deadline_soon`, `to_pack` → `/packing`. Bikri `context.taka` (shared `formatTaka`).
12. **Order list** (`/orders?tab=`, `OrderListBloc`): `orderTabs` (`all`, `awaiting_confirm`, `deadline_soon` tab param; baki `display_status`), number/phone khoja (trim), risk chip, ager/porer page (OFFSET paging, tai "aro" na). Card e risk flag chip (`admin.risk.*`), call (`tel:` `Browser.open`), tap → detail.
13. **Order detail** (`/orders/:id`, `OrderDetailCubit`): item snapshot, taka er bhag (vendor e null hole na), history, staff note/call note (shop role). Button `order_actions.dart` → `orderActions`/`fulfillmentActions` (backend er `CanTransitionOrder`/`CanTransitionFulfillment` + `admin.go` er role niyom er moto, shudhu lukano): confirm (pending; COD ba paid online; online unpaid e nai), cancel (owner/admin; pending ba confirmed e shob fulfillment open/packed/cancelled), pack (`/packing/:orderId/:fid`), undo (packed, COD; vendor e method null, backend dekhe; note dialog), ship (`/shipments`), fulfillment cancel (open: staff/vendor_staff chhara; packed: owner/admin). Protiti kaj er por abar load (cancel cascade). Print packing slip, invoice (vendor role e invoice nai, 403).
14. **Print** (11.6): `Browser.openTab(Future<String>)`: tap er bhetore `window.open('', '_blank')` (Safari async `window.open` atkay), `opener = null`, `POST /print-links` er `url` ele `location.href`, bhul e tab bondho.
15. **Packing** (`/packing`, `PickListCubit`): ajker pick list (variant er mot, order list), print (`pick_list`, list er `date`), order er "Pack" → `/packing/:orderId/:fid` (`PackCubit`: `GET /orders/{id}` theke oi fulfillment er item; fixed item shudhu dekhay, ojon-variable e gram field 1–1,000,000, partial na). `409 weight_over_limit` → `PackReady.overLimit` → note dialog → abar `pack(customerOkNote:)`. Batch dekhano nai (11.7 item 14).
16. **Shipment** (`/shipments`, `ShipmentsCubit`): `GET /fulfillments?status=packed` + `?status=shipped` (protiti 100, ponytail): pathano baki = packed + shipped kintu active `shipment` nai (abar pathano); pothe = active shipment shoho (v1.50). Ship dialog (carrier, tracking trim, khali hole na; rider picker nai, 11.7 item 14), delivered (vendor role e nai), parcel ferot (`returned`), booking bhul (`cancelled`); `tracking_code: taken` er lekha.
17. Lekha `admin.*` (bn/en) ar shared `order.status.*`, `order.payment.*`, `print.*`; dialog er title/hint key hishebe pathano (callback e `context.t` `watch` kore, tai na), validator er lekha build e.

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
| `admin/lib/core/auth/session_client.dart` → `SessionClient` | Screen er `401` → `SessionExpired` |
| `admin/lib/core/browser.dart` → `Browser` | Notun tab (print), `tel:` |
| `admin/lib/core/widgets/` → `AppShell`, `showNoteDialog`, `ErrorRetry` | Menu, note dialog, load er bhul |
| `admin/lib/features/dashboard/` → `DashboardCubit` | Ajker count |
| `admin/lib/features/orders/order_actions.dart` → `orderActions`, `fulfillmentActions` | Allowed button |
| `admin/lib/features/orders/` → `OrderListBloc`, `OrderDetailCubit` | Order list ar detail |
| `admin/lib/features/packing/` → `PickListCubit`, `PackCubit` | Pick list, pack |
| `admin/lib/features/shipments/` → `ShipmentsCubit` | Shipment |
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
| Screen er `401` → login (`error.session_expired`); login er age 401 e kichu na (11.3) | `SessionClient`, `SessionExpired` | `admin/test/core/session_test.dart` |
| Menu role onujayi (`canOpen`), 360px drawer, 1280px rail; menu er route router e ache | `AppShell`, `menuRoutes` | `session_test.dart` → `group('menu (11.2, canOpen)')` |
| Dashboard count, bikri taka, 360/1280px (11.7 13 viii) | `DashboardCubit`, `DashboardPage` | `admin/test/features/dashboard/dashboard_test.dart` |
| Order list: tab (status = `display_status`, `awaiting_confirm`/`deadline_soon` tab param), `q` trim, risk, paging, bhul (11.4, 11.7 13 vii) | `OrderListBloc` | `admin/test/features/orders/order_list_bloc_test.dart` |
| Order list 360px (bn/en) ar 1280px overflow nai, risk chip, call `tel:` (11.5) | `OrderListPage` | `admin/test/features/orders/order_list_page_test.dart` |
| Allowed button `CanTransition*` er moto; online unpaid e confirm nai (`payment_required`); staff cancel na; vendor e order kaj na (11.2, 11.4) | `orderActions`, `fulfillmentActions` | `admin/test/features/orders/order_actions_test.dart` |
| Detail: confirm, bhul e order thake, undo note shoho (khali note e POST na), call note, print link notun tab, 360px overflow nai (11.4, 11.6) | `OrderDetailCubit`, `OrderDetailPage` | `admin/test/features/orders/order_detail_test.dart` |
| Pick list ar print; pack e shudhu ojon-variable er gram; `409 weight_over_limit` → note dialog → `customer_ok_note`; gram khali hole submit na; 360px (8.11, 11.7 13 ii) | `PickListCubit`, `PackCubit`, `PackPage` | `admin/test/features/packing/packing_test.dart` |
| Shipment: packed + abar pathano, carrier/tracking (trim), delivered/returned/cancelled, tracking taken, vendor e delivered nai, 360px (11.7 item 5, v1.50) | `ShipmentsCubit`, `ShipmentsPage` | `admin/test/features/shipments/shipments_test.dart` |
| Rong token theke, Bangla lekha shudhu `core/l10n/bn.dart` e (Rule 4, 11.7 item 10) | `buildTheme`, `l10n/` | `scripts/check-rules.sh` |

## Data

Nai (API: [auth.md](auth.md); session cookie `__Host-asid`, Dart e ba localStorage e kichu na).

## Bhul hole ki hoy

- API bhul: page e `error.<code>` (shared lekha); setup password e `fields.old`/`fields.new` field er niche.
- Session expire (`401`, shuru te ba je kono screen e): login page e `error.session_expired` (`SessionClient`).
- Screen er kaj er bhul (`payment_required`, `invalid_transition`, `forbidden`): page e `error.<code>`, data thake; load er bhul e "abar cheshta" (`ErrorRetry`).
- Font e na thaka glyph: CanvasKit `fonts.gstatic.com` chay, CSP atkay (tofu); tai Latin (Noto Sans) ar Bangla (Noto Sans Bengali) duitai asset.

## Bodlanor age mone rakho

- Notun route: `router.dart` e ar `permissions.dart` e 11.2 er role; `permissions_test` e row; menu e hole `menuRoutes` ar `admin.nav.*`.
- Page e `getIt` na: route er `BlocProvider(create:)` e; `context.t` shudhu build e (callback e na).
- `getIt<...>()` shudhu `di.dart` ar `BlocProvider(create:)` e (4.5).
- Flutter 3.47.9 (2.3; local e fvm), CI Flutter job ekoi version.
- Admin CSP (13.3) bodlale `Caddyfile.local` er `admin.localhost` o.

## Proshno-uttor

- **Order edit, return (restock + refund) screen nai keno?** Step 2 er list e chhilo na, dialog boro; M7b er porer step e (M7b.md e baki line).
- **Rider bechhe neya jay na keno?** `GET /staff` shudhu owner; ekhon rider chhara shipment (11.7 item 14, 16.1).
- **List e "aro dekhao" na keno, page keno?** API OFFSET paging; notun order ele row shore jay, "aro" te duplicate.

- **Shop ID keno lage?** Admin domain e shop nai; login body te shop slug (8.3). Build e `SHOP_SLUG` dile field age theke bhora.
- **QR code nai keno?** Package na; secret copy kore authenticator app e "manual key" diye dewa jay.
- **Login er por ager URL e ferot jay na keno?** Ekhon shob somoy `/`; dorkar hole `from` parameter pore.
