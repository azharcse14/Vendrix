# Migration ar database

Design: 6.1, 6.6, 6.7, 13.5, 13.6 · Milestone: M0 · Code: `backend/internal/db`, `backend/migrations`

## Ki kore

`shop-api migrate` binary te embed kora `backend/migrations/*.sql` kram onujayi database e chalay, protiti file ekbar. Deploy er por kono file bodlale ba bhul kram e notun file dile kichu na chaliye fail kore. API `db.Open` diye pool ney ar `db.InTx` diye transaction (deadlock e ekbar retry).

## Kivabe kaj kore

1. `migrations.FS` (`embed.go`): shudhu top-level `*.sql`; `tests/` er test SQL embed e ashe na.
2. `db.LoadMigrations`: naam `00N_naam.sql` kina, ekoi number duibar kina, protitir sha256.
3. `db.Migrate`:
   1. Lock (`pg_advisory_xact_lock`) er bhetore `schema_migrations (version, sha256, applied_at)` banay.
   2. `db.Pending`: applied file er hash mile kina, applied file ache kina, notun file shesh applied er porer number kina.
   3. Protiti notun file nijer transaction e: lock → abar dekha (onno runner chaliye thakle skip) → `set_config('app.env', $1, true)` → file → `schema_migrations` e insert.
4. `001_init.sql` er shesh block: `shop_app` role er onumoti (history te DELETE nai, `schema_migrations` e shudhu SELECT); role na thakle `app.env` `local` chhara fail.
5. `scripts/db-test.sh`: notun C.UTF-8 database e `001` ar `tests/001_init_test.sql` (83 ta constraint test).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/migrations/embed.go` → `FS` | Migration file embed |
| `backend/internal/db/migrate.go` → `LoadMigrations`, `Pending`, `Migrate` | Runner |
| `backend/internal/db/db.go` → `Open`, `InTx`, `retryable` | Pool ar transaction |
| `backend/internal/db/driver.go` | pgx driver register |
| `backend/migrations/001_init.sql` | Puro schema (6.7) |
| `backend/migrations/tests/001_init_test.sql` | Constraint test |
| `backend/migrations/DEPLOYED` | Production e chola file er hash (13.6) |
| `scripts/db-test.sh`, `scripts/check-rules.sh`, `scripts/hooks/guard-files.sh` | Test ar pahara |
| `infra/postgres/init.sql` | Local er role ar database |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Deploy er por file bodlale fail (6.1) | `Pending` | `TestPending`, `TestMigrateIntegration` |
| Ager number er notun file e fail (6.1) | `Pending` | `TestPending`, `TestMigrateIntegration` |
| Test SQL embed e na (6.1) | `LoadMigrations` | `TestLoadMigrations`, `TestEmbeddedMigrations` |
| `DEPLOYED` er hash = embed kora file | `migrations` | `TestEmbeddedMigrations` |
| Duibar chalale-o thik, `app.env` set (M0) | `Migrate` | `TestMigrateIntegration` |
| `shop_app` `schema_migrations` e shudhu SELECT (6.1) | `001_init.sql` | `TestMigrateIntegration` |
| Deadlock ba serialization e ekbar retry (8.6) | `InTx`, `retryable` | `TestRetryable` |
| pgx chhara spashto error | `Open` | `TestOpenWithoutDriver` |
| Schema er constraint (6.4) | `001_init.sql` | `tests/001_init_test.sql` (`make db-test`, 83/83) |

## Data

`schema_migrations` (runner banay, `001` er age). Baki shob table `001_init.sql` e (6.3).

## Bhul hole ki hoy

- `deploy er por bodleche`: file ager moto koro (`git checkout`), bodol notun `00N_*.sql` e.
- `choto number`: notun file e shesh applied er porer number dao.
- `shop_app role nai`: role banao (local e `infra/postgres/init.sql` nijei banay; 13.5).
- Kono file er moddhe error: oi file er transaction ROLLBACK, ager file gulo thake.

## Bodlanor age mone rakho

- Deploy hoya file (`DEPLOYED` e naam) edit na: hook atkay, CI hash dekhe, runner checksum dekhe.
- Notun table hard-delete-nishiddho ba history hole oi file e `REVOKE` (6.1).
- Notun CHECK e naam ar `tests/` e `'reject:<naam>'` test (`.claude/rules/migrations.md`).
- Runner er SQL bodlale `TestMigrateIntegration` asol Postgres e chalao.

## Proshno-uttor

- **Session lock na keno, `pg_advisory_xact_lock`?** `database/sql` er pool e session lock onno connection e pore jete pare; transaction lock transaction er sathe-i chole jay (6.1).
- **`schema_migrations` `001` e na keno?** Eta runner er nijer table; `001` chalanor age-i lage.
