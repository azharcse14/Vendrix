#!/usr/bin/env bash
# Local admin (11.7 item 10): release build (production er moto CSP, CanvasKit nijer server theke), API same-origin.
# Dekhte: alada terminal e make https (Caddy admin.localhost → admin/build/web ar /api), make run.
# https://admin.localhost:8443. Shop slug login form e age theke: SHOP_SLUG (default sobujbazar).
# Lage: Flutter 3.47.9 (2.3; jemon fvm: PATH=$HOME/fvm/versions/3.47.9/bin:$PATH). Code bodlale abar chalao.
set -euo pipefail
cd "$(dirname "$0")/../admin"
flutter pub get >/dev/null
flutter build web --release --no-web-resources-cdn --pwa-strategy=none \
  --dart-define=API_BASE_URL= --dart-define=SHOP_SLUG="${SHOP_SLUG:-sobujbazar}"
echo "build hoyeche: make https chalu thakle https://admin.localhost:8443"
