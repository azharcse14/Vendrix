#!/usr/bin/env bash
# Local website: website/.env (website/env.example theke) load, Tailwind --watch ar `jaspr serve` (127.0.0.1:8081).
# API alada: make run. Lage: dart (Flutter er) ar jaspr_cli 0.23.5 (dart pub global activate jaspr_cli 0.23.5).
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root/website"
if [ ! -f .env ]; then
  echo "website/.env nai: cp website/env.example website/.env" >&2
  exit 1
fi
set -a
# shellcheck disable=SC1091
. ./.env
set +a
dart pub get >/dev/null
bash "$root/scripts/tailwind.sh" --watch=always &
trap 'kill $! 2>/dev/null' EXIT
jaspr serve
