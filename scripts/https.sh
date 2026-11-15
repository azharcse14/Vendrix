#!/usr/bin/env bash
# Local https (13.1): Caddy `tls internal` diye https://sobujbazar.localhost:8443 → make web (8081).
# Lage: caddy (brew install caddy); onno shop host: LOCAL_SHOP_HOSTS="a.localhost b.localhost".
set -euo pipefail
cd "$(dirname "$0")/.."
if ! command -v caddy >/dev/null; then
  echo "caddy nai: brew install caddy (ba caddyserver.com/docs/install)" >&2
  exit 1
fi
exec caddy run --config infra/caddy/Caddyfile.local --adapter caddyfile
