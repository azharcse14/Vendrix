# M7b: Flutter admin

Design: Section 11 (11.1 route, 11.2 role, 11.3 code er niyom, 11.4 screen, 11.5 mobile, 11.6 print, 11.7 item 10 Flutter setup); shathe 1.4 (Rule 4, 13, 15), 2.3 (Dart package), 4.5, 4.6, 9.7 (Flutter theme), 9.8 (l10n), 13.3 (admin CSP `'wasm-unsafe-eval'`, `no-cache`) (file: [01](../design/01-overview.md), [02](../design/02-tech-stack.md), [04](../design/04-repo-structure.md), [09](../design/09-theme-l10n.md), [11](../design/11-admin.md), [13](../design/13-infra-ops.md), [15](../design/15-milestones.md))
Branch: main

**Obostha: baki (M7a er por)**. Plan user approve korechen 16 November 2026 (M7 plan er step 15–19, launch er screen age; decision 11.7). [M7a](M7a.md) shesh na hole shuru na (API ar shared repository M7a te).

## Scope (15 er row theke)
- **Already ache** (M7a shesh hole): shob admin API (`/api/admin/*`, role ar vendor scope), shared admin model ar repository (`ApiClient.multipart`; catalog e shudhu chobi, product/variant/category CRUD er repository ar model step 4 e, M7a step 13 decision), print page, `img.localhost`, dev seed (batch, `staff` user, coupon). Flutter 3.47.9 (Dart 3.13.7) Mac e `/opt/homebrew/share/flutter/bin/flutter`; `admin/` e shudhu `README.md`.
- **Setup** (11.3, 11.7 item 10): `flutter create --platforms web`, 2.3 er package + `http` + `flutter_localizations`, `di.dart`, `buildRouter` + `GoRouterRefreshStream`, `AuthBloc`, `ThemeCubit` (organic preset `buildTheme`), `LocaleCubit`, admin l10n (`admin.*`), Noto Sans Bengali asset, semantics on, `flutter build web --release --no-web-resources-cdn` (3.47.9 e flag confirm), JS build; `scripts/admin.sh` + `make admin`, local Caddy `admin.localhost` (same-origin, CSP `'wasm-unsafe-eval'`), CI Flutter job.
- **Screen** (11.1, 11.4, 11.5), ei kram e:
  1. Login, TOTP/recovery, setup (password, TOTP, recovery code); dashboard; order list/detail; packing (pick list, pack, ojon, `weight_over_limit` note, undo, print); shipment (360px).
  2. Product/variant/chobi, category; inventory (variant list, batch, adjust/wastage, movement); staff (create, role/active, reset).
  3. Zone, coupon, customer + block; audit (shesh e, optional: API M7a te ache, screen baki thakle Report e).
- **Baire**: settings/theme/translation editor (M8), review (M9), `/money/*`, complaint, privacy (M11), vendor (MV), report (Phase 2), Android app (Phase 2).

## Acceptance criteria → proman
| Criteria | Test ba command | Obostha |
|---|---|---|
| Staff role onujayi menu: `Staff.canOpen(route)` 11.2 er sathe mile; router guard `redirect` (setup, login, role) | `admin/test/core/permissions_test.dart`, `admin/test/router_test.dart` | ☐ |
| Login → TOTP → dashboard; setup flow (password, TOTP, recovery code); logout | `admin/test/features/auth/auth_bloc_test.dart` (`MockClient`); step 3 Safari | ☐ |
| Order/packing/shipment 360px e (overflow nai), detail e allowed button (`payment_required` e confirm nai), pack `weight_over_limit` e note dialog, undo note, print link notun tab | widget test (`tester.view.physicalSize`); step 3 Safari | ☐ |
| Product + chobi, category, inventory, staff screen | widget test; step 5 Safari | ☐ |
| Zone, coupon, customer + block (audit optional) | widget test; step 7 Safari | ☐ |
| Lekha translation key, rong theme (Rule 4); `--no-web-resources-cdn` e bairer domain na; CSP e console error nai | `flutter analyze`, `scripts/check-rules.sh` (admin e hex/literal), step 3 network log | ☐ |
| `make check` green (Flutter shoho, CI Flutter job); `security-reviewer` ar `design-reviewer` e high nai | `make check`, review (step 9) | ☐ |

## File
- Admin (notun): `admin/pubspec.yaml`, `analysis_options.yaml`, `web/index.html`, `lib/main.dart`, `lib/di.dart`, `lib/router.dart`, `lib/core/{auth,theme,l10n,permissions,widgets}/`, `lib/features/{auth,dashboard,orders,packing,shipments,products,categories,inventory,staff,zones,coupons,customers,audit}/{bloc,pages,widgets}/`, `assets/fonts/` (Noto Sans Bengali, OFL), `test/**`, `README.md`.
- Infra/CI: `infra/caddy/Caddyfile.local` (`admin.localhost`, CSP), `scripts/admin.sh`, `Makefile` `admin` target, `.github/workflows/ci.yml` (Flutter job, 3.47.9 pin), `scripts/check-rules.sh` (admin e hex/literal), `scripts/quick-check.sh` (admin e `pubspec.yaml` hole Flutter, dorkar hole).
- Doc: `docs/features/admin-app.md` (notun), `docs/features/README.md`, `docs/STATUS.md`.

## Feature doc
- Notun: `docs/features/admin-app.md` (di, router guard, `AuthBloc`, `canOpen`, l10n, theme, build, CSP, local Caddy, screen map).
- Bodlabe: `l10n.md` (admin key), `theme.md` (Flutter `buildTheme`), `README.md`.

## Open question
Nai (11.7 e decided). `--no-web-resources-cdn` 3.47.9 e na thakle step 1 e 16.1 e row ar user ke jiggesh.

## Progress
Protiti step ek commit (ba test + code duita), por `make quick-check`. Widget/bloc test age, fail dekhiye alada commit (`test(M7b): ...`), tarpor code (`feat(M7b): ...`). Safari proman (safaridriver): scratch stack alada port e (M7a step 15 er moto; `flutter build web --dart-define=API_BASE_URL=` same-origin, Caddy copy e `admin.localhost`), semantics on: element `flt-semantics` (`role`, `aria-label`) ba `Semantics(identifier:)` er attribute (3.47 e naam confirm); text field focus hole `<input>`, `/element/active` e keys; route bodol er por element abar khoja; `acceptInsecureCerts: true`; `window.onerror`/`console.error` hook; TOTP stdlib Python. 360 ar 1280, bn/en screenshot; console error 0; `__Host-asid` cookie flag, localStorage e token nai.

- [x] **0. Plan approve** (16 November 2026, M7 plan er step 15–19 theke; design v1.41 11.7). Shuru [M7a](M7a.md) shesh hole.
- [ ] **1. Setup ar login: test age** (11.3, 11.7 item 10): `flutter create`, pubspec, `--no-web-resources-cdn` 3.47.9 e confirm, `di.dart`, `router.dart`, `AuthBloc` (login → TOTP/recovery → `setup_pending` password → TOTP setup/enable → recovery code), `Staff.canOpen`, `ThemeCubit`/`LocaleCubit`, admin l10n, font, semantics; test: `test/router_test.dart`, `test/core/permissions_test.dart`, `test/features/auth/auth_bloc_test.dart`; `scripts/admin.sh`, `make admin`, Caddy `admin.localhost` (CSP), CI Flutter job, `admin/README.md`. Commit `test(M7b): admin router ar auth`, `feat(M7b): admin Flutter setup ar login`.
- [ ] **2. Dashboard, order, packing, shipment: test age** (11.4, 11.5): order list tab/filter/chip, detail allowed button, pack form ojon + `weight_over_limit` note dialog, undo note, shipment form, print link `window.open`; 360px overflow nai; tarpor screen. Commit `test(M7b): order ar packing screen`, `feat(M7b): dashboard, order, packing, shipment`.
- [ ] **3. Safari proman 1**: owner login → setup → dashboard; website e COD order → order list → confirm → pick list → print → pack (+10%+1g → note) → undo → pack → shipment → delivered; 360/1280, bn/en; network e bairer domain nai; cookie flag. Report e.
- [ ] **4. Product/chobi, category, inventory, staff: test age**: shared e `AdminCatalogRepository` e product/variant/category CRUD ar model (Go `to_jsonb` row er shudhu screen er field; `packages/shared/test` e age test), form validation, `{bn, en}` field, chobi upload/order/delete, category tree, inventory list + batch/adjust dialog (`staff` e `cost_price` field dekhay na), staff create/reset (temp password ekbar dekhano); `staff` role e product/staff menu nai; tarpor screen. Commit `test(M7b): catalog, inventory, staff screen`, `feat(M7b): product, category, inventory, staff`.
- [ ] **5. Safari proman 2**: owner staff (`staff` role) banay → staff login: menu e product/coupon/zone/customer/staff nai, `/coupons` URL e `/` e redirect; chobi upload (website e dekha), batch entry staff diye, wastage; reset → staff abar setup. Report e.
- [ ] **6. Zone, coupon, customer, audit: test age**; tarpor screen (audit shesh e, shomoy na thakle Report e baki). Commit `test(M7b): zone, coupon, customer screen`, `feat(M7b): zone, coupon, customer, audit`.
- [ ] **7. Safari proman 3**: zone, coupon (website checkout e kaj kore), block (`phone_blocked`), audit list. Report e.
- [ ] **8. Feature doc**: `admin-app.md` notun, baki milano. Commit `docs(M7b): feature doc`.
- [ ] **9.** `make check` green (Flutter shoho); `security-reviewer` (cookie/CORS, CSP, token localStorage e na, role UI vs backend) ar `design-reviewer` (11, 9.7, 9.8, 1.4 Rule 4); finding thik ba karon; acceptance ☑. Commit `fix(M7b): review fix` (dorkar hole).
- [ ] **10. Report** ar `docs/STATUS.md` (M7b shesh, porer kaj M8). Commit `docs(M7b): report`.

## Report
