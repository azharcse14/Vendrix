# Admin app (Flutter web)

Design: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7 item 1, 2, 4–11, 13 vii–ix, 14, 2.3, 4.5, 9.7, 9.8, 13.3 · Milestone: M7b · Code: `admin/`

## Ki kore

Staff browser e admin khole (`admin.<domain>`, local e `https://admin.localhost:8443`): shop ID, email ar password diye login, TOTP ba recovery code; prothom login ba reset er por password bodol, authenticator app e TOTP setup ar 10 ta recovery code ekbar dekhe. Tarpor dashboard (ajker count, tap e oi list), order list/detail (confirm, cancel, pack, undo, note, call, print), packing (pick list, ojon diye pack, ojon beshi hole customer er raji note) ar shipment (carrier, tracking, delivered, parcel ferot); phone e 360px e chole (11.5). Product (form, variant, chobi upload/kram/delete), category tree, inventory (variant khoja, low stock, stock entry batch, komano wastage/adjust, batch ar movement) ar staff (notun, role, active, reset; temporary password ekbar). Order detail e item kom/bad (edit) ar parcel ferot (return, restock bechhe). Coupon (notun, edit, active; bebohar hole code/dhoron/man lock), delivery zone (notun, edit, inactive), customer (khoja, detail e shesh 50 order, phone block/unblock, block list) ar audit log (filter, before/after JSON): owner/admin. Bhasha bn/en bodlano jay. Role onujayi route lukano; suraksha backend e (11.2).

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
13. **Order detail** (`/orders/:id`, `OrderDetailCubit`): item snapshot, taka er bhag (vendor e null hole na), history, staff note/call note (shop role). Button `order_actions.dart` → `orderActions`/`fulfillmentActions` (backend er `CanTransitionOrder`/`CanTransitionFulfillment` + `admin.go` er role niyom er moto, shudhu lukano): confirm (pending; COD ba paid online; online unpaid e nai), cancel (owner/admin; pending ba confirmed e shob fulfillment open/packed/cancelled), pack (`/packing/:orderId/:fid`), undo (packed, COD; vendor e method null, backend dekhe; note dialog), ship (`/shipments`), fulfillment cancel (open: staff/vendor_staff chhara; packed: owner/admin). Protiti kaj er por abar load (cancel cascade). Print packing slip, invoice (vendor role e invoice nai, 403). **Edit** (`canEdit`: shop role, order pending/confirmed, COD ba paid, `open` fulfillment e active item; `showEditDialog`): protiti item er notun qty 0..ager (barano na), shudhu bodla item, note lagbe, kichu na komale submit na → `editItems`. **Return** (`canReturn`: shop role, fulfillment shipped/delivered; `showReturnDialog`): restock item checkbox (default kono ta na: nosto jinish stock e na jay), note lagbe, taka newa hole refund er lekha → `returnFulfillment`. `403` (staff er limit: vendor er shob item bad/refund) e `errorKey` diye karon er lekha (`admin.edit.forbidden`, `admin.return.forbidden`).
14. **Print** (11.6): `Browser.openTab(Future<String>)`: tap er bhetore `window.open('', '_blank')` (Safari async `window.open` atkay), `opener = null`, `POST /print-links` er `url` ele `location.href`, bhul e tab bondho.
15. **Packing** (`/packing`, `PickListCubit`): ajker pick list (variant er mot, order list), print (`pick_list`, list er `date`), order er "Pack" → `/packing/:orderId/:fid` (`PackCubit`: `GET /orders/{id}` theke oi fulfillment er item; fixed item shudhu dekhay, ojon-variable e gram field 1–1,000,000, partial na). `409 weight_over_limit` → `PackReady.overLimit` → note dialog → abar `pack(customerOkNote:)`. Batch dekhano nai (11.7 item 14).
16. **Shipment** (`/shipments`, `ShipmentsCubit`): `GET /fulfillments?status=packed` + `?status=shipped` (protiti 100, ponytail): pathano baki = packed + shipped kintu active `shipment` nai (abar pathano); pothe = active shipment shoho (v1.50). Ship dialog (carrier, tracking trim, khali hole na; rider picker nai, 11.7 item 14), delivered (vendor role e nai), parcel ferot (`returned`), booking bhul (`cancelled`); `tracking_code: taken` er lekha.
17. **Product** (`/products`, `ProductListCubit`): page (API te khoja nai), inactive chip, notun → `/products/new`. **Form** (`/products/new`, `/products/:id`, `ProductFormCubit`): naam/bornona `{bn, en}` (bn lagbe), slug (`^[a-z0-9]+(-[a-z0-9]+)*$`, max 80, Go `validSlug` er moto), category, unit (6.7), ojon onujayi dam shudhu `kg` e (DB CHECK), khojar shobdo, active; SEO/`attributes` form e nai (PATCH e na pathale bodlay na). Notun save → `createProduct` er row (variant/chobi nai) → `/products/:id`; edit save → PATCH → abar `GET`. Save er por **variant** (`showVariantDialog`: SKU, label bn/en ekta option key e (ager key thake, notun e `size`), dam/ager dam/kena dam taka e lekha → `parseTaka` → poisha, ager dam > dam, ek unit er gram; stock na, 8.12) ar **chobi** (`Browser.pickFile`: lukano `<input type=file accept=image/jpeg,image/png>` `package:web` diye, alt = product er naam; upore/niche → `orderImages` shob id; delete confirm shoho; 10 ta porjonto). Thumbnail `IMAGE_BASE_URL` (`--dart-define`, local `scripts/admin.sh` e `https://img.localhost:8443`) thakle `Image.network` `<img>` element e (`WebHtmlElementStrategy.prefer`: CORS/`connect-src` chhara, CSP `img-src`), na thakle icon.
18. **Category** (`/categories`, `CategoriesCubit`): `categoryTree` flat list ke parent er niche child (indent, API er kram; parent na pele root, cycle e atke na); notun/edit dialog (`showCategoryDialog`: naam bn/en, slug, parent (nije ar `subtree` bad), kram, active; chobi M8 e); delete confirm, `409 category_not_empty` er lekha.
19. **Inventory** (`/inventory?low=1`, `InventoryListCubit`): `q` (trim, search e), low filter chip, page; low chinho `stock <= low_stock_at`; tap → `/inventory/:variantId` (`extra` e row, header er jonno; URL shoja khulle header nai). **Variant stock** (`VariantStockCubit`): batch (FIFO) ar movement (prothom page); stock entry (`showBatchDialog`: qty 1–1,000,000, kena dam taka (`staff`/`vendor_staff` e field nai, API o dey na), batch code trim, harvest/expiry `YYYY-MM-DD` asol din, expiry ≥ harvest); komano (`showReduceDialog`: koto (+), karon `wastage`/`adjust`, note lagbe → `change = -qty`); `409 out_of_stock` e `details.available`. Dashboard er low stock → `/inventory?low=1`.
20. **Staff** (`/staff`, owner, `StaffCubit`): list (prothom page), notun (email, naam, role `owner`/`admin`/`staff`) → temporary password dialog ekbar (copy, bondho korle ar nai; state e rakha hoy na), role dropdown ar active switch (PATCH shudhu oi field), reset confirm → notun temporary password; nijer row e kaj nai (server o `403`).
21. **Coupon** (`/coupons`, `CouponsCubit`, `showCouponDialog`): list page, active switch (PATCH shudhu `is_active`); form puro object pathay (create ar PATCH ekoi): `code` trim + upper `^[A-Z0-9_-]{1,32}$`, percent 1–100 ba fixed taka → poisha, min order, max discount, mot shima (≥ `used_count`), phone prati, shuru/shesh din `YYYY-MM-DD` → `T00:00:00+06:00`/`T23:59:59+06:00` (shop er din, Asia/Dhaka); `used_count > 0` hole code/kind/value field bondho; server er `fields.usage_limit: below_used` ar `fields.<f>: locked` er nijer lekha, `coupon_exists`.
22. **Zone** (`/zones`, `ZonesCubit`, `showZoneDialog`): list (inactive o, "puro jela"), form puro object: `name.bn` lagbe, district lagbe, district/area trim max 100 (area khali → `null`), charge/min order taka, free delivery optional (> 0), same city, active; `zone_exists`.
23. **Customer** (`/customers`, `CustomersCubit`): customer tab (`q` trim, max 64, page) ar block list tab; notun block (phone `normalizePhone`, karon max 200 optional), unblock confirm (phone body te). **Detail** (`/customers/:id`, `CustomerDetailCubit`): profile, shesh 50 order (tap → order), block (karon `showNoteDialog` optional) / unblock confirm, tarpor abar load.
24. **Audit** (`/audit`, `AuditCubit`): filter `action` (trim), `staff_id` (shonkhya), `from`/`to` (`YYYY-MM-DD` asol din, bhul hole GET na), page; row khulle before/after JSON (indent).
25. Lekha `admin.*` (bn/en) ar shared `order.status.*`, `order.payment.*`, `print.*`; dialog er title/hint key hishebe pathano (callback e `context.t` `watch` kore, tai na), validator er lekha build e.

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
| `admin/lib/features/products/` → `ProductListCubit`, `ProductFormCubit`, `showVariantDialog` | Product, variant, chobi |
| `admin/lib/features/categories/` → `CategoriesCubit`, `categoryTree`, `showCategoryDialog` | Category tree |
| `admin/lib/features/inventory/` → `InventoryListCubit`, `VariantStockCubit`, `showBatchDialog`, `showReduceDialog` | Variant stock, batch, komano |
| `admin/lib/features/staff/` → `StaffCubit`, `StaffPage` | Staff |
| `admin/lib/features/orders/widgets/edit_dialogs.dart` → `showEditDialog`, `showReturnDialog` | Order edit, return |
| `admin/lib/features/coupons/` → `CouponsCubit`, `showCouponDialog` | Coupon |
| `admin/lib/features/zones/` → `ZonesCubit`, `showZoneDialog` | Delivery zone |
| `admin/lib/features/customers/` → `CustomersCubit`, `CustomerDetailCubit` | Customer, block |
| `admin/lib/features/audit/` → `AuditCubit`, `AuditPage` | Audit log |
| `admin/lib/core/widgets/common.dart` → `confirmDialog`, `PageBar`, `ErrorLine`, `takaInput`, `validDate` | Confirm, page, kaj er bhul, taka/din form |
| `admin/web/index.html`, `manifest.json` | Tab er title "Vendrix Admin" |
| `scripts/admin.sh`, `make admin` | Release build (`--no-web-resources-cdn`, same-origin) |
| `infra/caddy/Caddyfile.local` → `admin.localhost` | `admin/build/web` + `/api` → `LOCAL_API`, CSP |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Role onujayi route: 11.2 er table (staff e product/category/coupon na, credential shudhu owner), ochena route na | `canOpen` | `admin/test/core/permissions_test.dart` → `canOpen ${e.key} (11.2)` (protiti route er row), `ochena role kichui khulte pare na` |
| Redirect: setup, TOTP, login, role (11.3) | `authRedirect` | `admin/test/router_test.dart` → `group('authRedirect (11.3)')` |
| AuthBloc bodlale router nijei redirect (`refreshListenable`) | `GoRouterRefreshStream` | `admin/test/router_test.dart` → `AuthBloc bodlale router nijei redirect kore` |
| Login → TOTP (bhul code e temp_token thake) → dashboard; recovery code; bhul password; temp_token expire (8.3) | `AuthBloc` | `admin/test/features/auth/auth_bloc_test.dart` |
| Setup: password (bhul `fields.old`) → TOTP (bhul code e ekoi secret) → recovery code → dashboard; TOTP live + `must_change_password`; reset er por shoja TOTP (8.3 step 4) | `AuthBloc._setup` | `auth_bloc_test.dart` → `prothom login: setup_pending → password → TOTP setup/enable → recovery code → dashboard`, `TOTP live + must_change_password: password er por shoja dashboard`, `reset er por (password bodol lage na): shoja TOTP setup` |
| Shuru te session nai ba network bhul → login (app atke na) | `AuthBloc` `AuthStarted` | `auth_bloc_test.dart` → `shuru: session nai → Unauthenticated (error chhara)`, `shuru: network bhul o Unauthenticated (app atke na)` |
| Logout API fail holeo local logout | `LogoutRequested` | `auth_bloc_test.dart` → `logout: API fail holeo Unauthenticated` |
| temp_token, secret, recovery code state er `toString` e na (log) | `TotpRequired`, `SetupRequired` | `auth_bloc_test.dart` → `prothom login: setup_pending → password → TOTP setup/enable → recovery code → dashboard` (recovery code; temp_token/secret er `toString` code e) |
| Screen er `401` → login (`error.session_expired`); login er age 401 e kichu na (11.3) | `SessionClient`, `SessionExpired` | `admin/test/core/session_test.dart` |
| Menu role onujayi (`canOpen`; `staff` e product/category/coupon/zone/customer/staff/audit nai, inventory ache), 360px drawer, 1280px rail (lomba hole scroll); menu er route router e ache | `AppShell`, `menuRoutes` | `session_test.dart` → `group('menu (11.2, canOpen)')` |
| Dashboard count, bikri taka, 360/1280px (11.7 13 viii) | `DashboardCubit`, `DashboardPage` | `admin/test/features/dashboard/dashboard_test.dart` |
| Order list: tab (status = `display_status`, `awaiting_confirm`/`deadline_soon` tab param), `q` trim, risk, paging, bhul (11.4, 11.7 13 vii) | `OrderListBloc` | `admin/test/features/orders/order_list_bloc_test.dart` |
| Order list 360px (bn/en) ar 1280px overflow nai, risk chip, call `tel:` (11.5) | `OrderListPage` | `admin/test/features/orders/order_list_page_test.dart` |
| Allowed button `CanTransition*` er moto; online unpaid e confirm nai (`payment_required`); staff cancel na; vendor e order kaj na (11.2, 11.4) | `orderActions`, `fulfillmentActions` | `admin/test/features/orders/order_actions_test.dart` |
| Detail: confirm, bhul e order thake, undo note shoho (khali note e POST na), call note, print link notun tab, 360px overflow nai (11.4, 11.6) | `OrderDetailCubit`, `OrderDetailPage` | `admin/test/features/orders/order_detail_test.dart` |
| Pick list ar print; pack e shudhu ojon-variable er gram; `409 weight_over_limit` → note dialog → `customer_ok_note`; gram khali hole submit na; 360px (8.11, 11.7 13 ii) | `PickListCubit`, `PackCubit`, `PackPage` | `admin/test/features/packing/packing_test.dart` |
| Shipment: packed + abar pathano, carrier/tracking (trim), delivered/returned/cancelled, tracking taken, vendor e delivered nai, 360px (11.7 item 5, v1.50) | `ShipmentsCubit`, `ShipmentsPage` | `admin/test/features/shipments/shipments_test.dart` |
| Product list page/inactive (360/1280); form: khali naam/bhul slug e POST na, `{bn, en}` body, save er por variant/chobi; edit PATCH, `slug_taken`; variant taka → poisha, ager dam ≤ dam e POST na, ager option key thake, ager dam muchle `null`; chobi upload/kram/delete confirm, pick bondho e upload na (7.7, 11.7 item 9) | `ProductListCubit`, `ProductFormCubit`, `showVariantDialog` | `admin/test/features/products/products_test.dart` → `notun product: khali/bhul slug e POST na; thik hole {bn, en} body`, `variant edit: ager option key thake, compare_at muchle null`, `chobi: upload (multipart), kram bodol, delete confirm shoho` |
| Category tree (child parent er niche), notun/edit body, parent e nije/child nai, delete confirm + `category_not_empty` | `categoryTree`, `subtree`, `CategoriesCubit` | `admin/test/features/categories/categories_test.dart` → `flatten: parent er niche child, depth shoho`, `edit: parent list e nije ar nijer child nai; PATCH`, `delete: confirm er por; category_not_empty er lekha` |
| Inventory khoja/low, stock entry body (bhul din/qty e na), komano note lagbe ar `change` negative, `out_of_stock` e available; `staff` e kena dam nai (11.7 item 1, 2) | `InventoryListCubit`, `VariantStockCubit`, stock dialog | `admin/test/features/inventory/inventory_test.dart` → `staff: kena dam nai, batch dialog e cost_price field nai`, `stock entry (batch): bhul din/qty e POST na; body`, `komano: note chhara POST na; change negative; out_of_stock e available` |
| Staff: temporary password ekbar, `staff_exists`, reset confirm, role/active PATCH shudhu oi field, nijer row e kaj nai (11.7 item 11) | `StaffCubit`, `StaffPage` | `admin/test/features/staff/staff_test.dart` → `create: temp password ekbar dekhay, bondho korle ar nai`, `reset: confirm er por notun temp password`, `role bodol ar active bondho: PATCH shudhu oi field` |
| Order edit: shudhu kom (`0` = bad), note lagbe, bodla item shudhu; staff `403` er lekha; return: restock bechhe, note lagbe, staff refund `403` er lekha (11.7 item 6, 7) | `canEdit`, `canReturn`, `showEditDialog`, `showReturnDialog` | `order_actions_test.dart` → `group('canEdit (11.7 item 7: shudhu kom, open fulfillment)')`, `group('canReturn (11.7 item 6)')`; `order_detail_test.dart` → `order edit: shudhu kom, note lagbe, bodla item shudhu`, `order edit: staff e 403 hole limit er lekha`, `return: restock item bechhe, note lagbe`, `return: staff e refund 403 er lekha` |
| Coupon: code upper/trim, percent ≤ 100, fixed taka → poisha, shesh din 23:59:59 +06:00, bebohar hole code/kind/value lock ar limit < used na, `below_used`/`locked`/`coupon_exists` er lekha, active switch (11.7 13 v) | `CouponsCubit`, `showCouponDialog` | `admin/test/features/coupons/coupons_test.dart` → `create: code upper + trim, percent, tarikh shop er din`, `bebohar hoyeche: code/kind/value lock, limit used er niche na`, `server er below_used ar locked er lekha` |
| Zone: `name.bn` lagbe, trim, max 100, area khali `null`, taka poisha, inactive list e, `zone_exists` (11.7 13 vi) | `ZonesCubit`, `showZoneDialog` | `admin/test/features/zones/zones_test.dart` → `create: name.bn lagbe, trim, taka poisha, area khali`, `district 100 er beshi e submit na`, `zone_exists er lekha` |
| Customer `q`, block list, block (normalize phone, karon), unblock confirm, detail e order ar block/unblock (11.7 item 12) | `CustomersCubit`, `CustomerDetailCubit` | `admin/test/features/customers/customers_test.dart` → `khoja: q trim kore GET`, `blocked list, notun block (reason), unblock confirm er por`, `detail block: reason optional, tarpor abar load` |
| Audit filter (`action`, `staff_id`, `from`, `to`), bhul din e GET na, before/after JSON (11.7 item 11) | `AuditCubit`, `AuditPage` | `admin/test/features/audit/audit_test.dart` → `filter: action, staff_id, from, to`, `row khulle before/after JSON` |
| Admin tab er title Flutter default na | `web/index.html` | `admin/test/web_shell_test.dart` → `web/index.html ar manifest e admin er naam` |
| Rong token theke, Bangla lekha shudhu `core/l10n/bn.dart` e (Rule 4, 11.7 item 10) | `buildTheme`, `l10n/` | `scripts/check-rules.sh` |

## Data

Nai (API: [auth.md](auth.md); session cookie `__Host-asid`, Dart e ba localStorage e kichu na).

## Bhul hole ki hoy

- API bhul: page e `error.<code>` (shared lekha); setup password e `fields.old`/`fields.new` field er niche.
- Session expire (`401`, shuru te ba je kono screen e): login page e `error.session_expired` (`SessionClient`).
- Screen er kaj er bhul (`payment_required`, `invalid_transition`, `forbidden`): page e `error.<code>` (order edit/return er `403` e karon er lekha, coupon er `fields` e lock/below_used er lekha), data thake; load er bhul e "abar cheshta" (`ErrorRetry`).
- Font e na thaka glyph: CanvasKit `fonts.gstatic.com` chay, CSP atkay (tofu); tai Latin (Noto Sans) ar Bangla (Noto Sans Bengali) duitai asset.

## Bodlanor age mone rakho

- Notun route: `router.dart` e ar `permissions.dart` e 11.2 er role; `permissions_test` e row; menu e hole `menuRoutes` ar `admin.nav.*`.
- Page e `getIt` na: route er `BlocProvider(create:)` e; `context.t` shudhu build e (callback e na).
- `getIt<...>()` shudhu `di.dart` ar `BlocProvider(create:)` e (4.5).
- Flutter 3.47.9 (2.3; local e fvm), CI Flutter job ekoi version.
- Admin CSP (13.3) bodlale `Caddyfile.local` er `admin.localhost` o.

## Proshno-uttor

- **Return e restock default kono ta na keno?** Ferot asha jinish (fol, shobji) nosto hote pare; staff bechhe na dile stock e bhul jinish jay na (history note e baki).
- **Staff edit/return e button keno dekhay, pore `403`?** Staff er limit (vendor er shob item bad, refund er poriman) server hishab kore; UI shudhu lukay (11.2), karon `403` er lekha e.
- **Coupon er tarikh shudhu din keno?** Shop er din (Asia/Dhaka): shuru 00:00, shesh 23:59:59; ghonta lagle pore.
- **Audit e staff naam diye filter na keno?** `GET /staff` shudhu owner, audit admin o dekhe; tai staff ID.
- **Product e SEO ar attributes field nai keno?** M7b step 4 e lean form (advisor decision, M7b.md); API ache, PATCH e na pathale ager value thake; pore form e.
- **Chobi pick e package nai keno?** `package:web` er `<input type=file>` (already dependency); type/size server abar dekhe (11.7 item 9).
- **Inventory variant URL shoja khulle naam nai keno?** Ek variant er GET nai; list theke `extra` e ashe. Batch/movement/kaj thik chole.
- **Rider bechhe neya jay na keno?** `GET /staff` shudhu owner; ekhon rider chhara shipment (11.7 item 14, 16.1).
- **List e "aro dekhao" na keno, page keno?** API OFFSET paging; notun order ele row shore jay, "aro" te duplicate.

- **Shop ID keno lage?** Admin domain e shop nai; login body te shop slug (8.3). Build e `SHOP_SLUG` dile field age theke bhora.
- **QR code nai keno?** Package na; secret copy kore authenticator app e "manual key" diye dewa jay.
- **Login er por ager URL e ferot jay na keno?** Ekhon shob somoy `/`; dorkar hole `from` parameter pore.
