#!/usr/bin/env bash
# 001_init.sql + tests/001_init_test.sql ekta notun, khali C.UTF-8 database e (6, 14.2 test 25).
# Lage: Postgres 16+ er psql, createdb, dropdb; libpq env (PGHOST, PGPORT, PGUSER, PGPASSWORD).
# PGUSER er CREATEDB lage; shop_app role na thakle CREATEROLE o (13.5: CI ar local e shop_app thakbe,
# na hole privilege test kichu dekhe na). Shesh e test database muche fele.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
mig="$root/backend/migrations"
db="esd_test_$$"
psql=(psql -X -q -v ON_ERROR_STOP=1)

"${psql[@]}" -d "${PGMAINTDB:-postgres}" \
  -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'shop_app') THEN CREATE ROLE shop_app LOGIN; END IF; END \$\$;"
createdb -E UTF8 --locale=C.UTF-8 -T template0 "$db"
trap 'dropdb --if-exists "$db" >/dev/null 2>&1 || true' EXIT

# Migration runner er moto: ek transaction, shurute set_config('app.env', ...) (6.1, v1.6).
{ echo "BEGIN;"; echo "SELECT set_config('app.env', 'local', true);"; cat "$mig/001_init.sql"; echo "COMMIT;"; } \
  | "${psql[@]}" -d "$db" >/dev/null

if ! out=$("${psql[@]}" -d "$db" -f "$mig/tests/001_init_test.sql" 2>&1); then
  printf '%s\n' "$out" | grep -v '^ok ' | tail -n 30 >&2
  exit 1
fi
printf '%s\n' "$out" | grep -E '^[0-9]+ / [0-9]+ ' || true
