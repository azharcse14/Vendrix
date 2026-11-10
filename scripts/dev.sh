#!/usr/bin/env bash
# Local e API: backend/.env (backend/env.example theke) load kore `go run ./cmd/api <command>`.
# make run → serve, make migrate → migrate. Database: make db-up (infra/docker-compose.yml).
set -euo pipefail
cd "$(dirname "$0")/../backend"
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.27.3}"
if [ ! -f .env ]; then
  echo "backend/.env nai: cp backend/env.example backend/.env" >&2
  exit 1
fi
set -a
# shellcheck disable=SC1091
. ./.env
set +a
exec go run ./cmd/api "$@"
