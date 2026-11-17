# admin (Flutter web)

Admin ar staff panel (Section 11): `bloc`, `go_router`, `get_it`, `shared` path dependency. Kivabe kaj kore: [docs/features/admin-app.md](../docs/features/admin-app.md).

## Lage

Flutter **3.47.9** (Dart 3.13.7; 2.3, CI er sathe ekoi). Homebrew er notun Flutter e website resolve hoy na, tai fvm:

```sh
fvm install 3.47.9
export PATH=$HOME/fvm/versions/3.47.9/bin:$PATH
```

## Check

```sh
cd admin && flutter analyze && flutter test
make quick-check   # admin bodlale Flutter part o (QUICK_PARTS=flutter)
```

## Local e chalano (https, same-origin)

1. `make run` (API; `backend/.env` er `ADDR`, jemon `127.0.0.1:8090`).
2. `make admin` (release build `admin/build/web`, `--no-web-resources-cdn`, `API_BASE_URL` khali = same-origin; `SHOP_SLUG` default `sobujbazar`). Code bodlale abar.
3. `LOCAL_API=127.0.0.1:8090 make https` (Caddy: `admin.localhost` → build + `/api`), browser e `https://admin.localhost:8443`.

Production build: `flutter build web --release --no-web-resources-cdn --pwa-strategy=none --dart-define=API_BASE_URL=https://api.<domain> --dart-define=SHOP_SLUG=<slug>`; CSP 13.3 (`'wasm-unsafe-eval'`).

## Niyom

- Token Dart e ba localStorage e na: `__Host-asid` cookie, `BrowserClient()..withCredentials = true` (11.3).
- `getIt<...>()` shudhu `lib/di.dart` ar `BlocProvider(create:)` e; state `sealed class`.
- Role check UI te shudhu lukano (`lib/core/permissions.dart`), suraksha backend e (11.2).
- Rong theme token theke, lekha `context.t('admin.*')` (`lib/core/l10n/bn.dart`, `en.dart`); `.claude/rules/admin-flutter.md`.
- Font `assets/fonts` (Noto Sans, Noto Sans Bengali; OFL, `OFL.txt`).
