#!/usr/bin/env bash
# Puro check: milestone shesh e ar CI te. quick-check (shob part) + race test + database test.
set -euo pipefail
cd "$(dirname "$0")/.."
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.27.3}"
QUICK_ALL=1 bash scripts/quick-check.sh
if [ -d backend ]; then (cd backend && go test -race -count=1 ./...); fi
bash scripts/db-test.sh
