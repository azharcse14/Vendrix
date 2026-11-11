#!/usr/bin/env bash
# Puro check: milestone shesh e ar CI te. quick-check (shob part) + database test + race test (DB test shoho).
# PG* env na thakle local er make db-up (127.0.0.1:5433, postgres user).
set -euo pipefail
cd "$(dirname "$0")/.."
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.27.4}"
export PGHOST="${PGHOST:-127.0.0.1}" PGPORT="${PGPORT:-5433}" PGUSER="${PGUSER:-postgres}" PGPASSWORD="${PGPASSWORD:-postgres}"
QUICK_ALL=1 bash scripts/quick-check.sh
# db-test shop_app role banay; Go er DB test (jobs, shop, migrate) e shop_app er onumoti o dekhe
bash scripts/db-test.sh
export TEST_DATABASE_URL="${TEST_DATABASE_URL:-postgres://$PGUSER:$PGPASSWORD@$PGHOST:$PGPORT/${PGMAINTDB:-postgres}?sslmode=disable}"
if [ -d backend ]; then (cd backend && go test -race -count=1 ./...); fi
