# AGENTS.md

Vendrix: ekoi code e single store, marketplace ar multi-tenant ecommerce (Bangladesh, organic food diye shuru). Ei repo te AI coding agent (Claude Code, Codex, Cursor ...) er niyom. Design ar build plan: `docs/design/` (suchipotro ar version: `docs/design/README.md`). Claude Code er kaj er kram: `CLAUDE.md`.

## Tracking: kaj kothay lekha thake
- `docs/STATUS.md`: ekhon kon milestone, porer kaj, user er kache proshno, session log. Kaj er age poro, kaj er shesh e update koro.
- `docs/milestones/M<n>.md`: milestone er plan, checkbox, acceptance er proman.
- `docs/features/`: protiti feature ekhon kivabe kaj kore (code er map, niyom → test, data, proshno-uttor). Code bodlale ek commit e doc o.
- `docs/design/16-open-decisions.md` (16.1): user er decision lage emon jinish.
- Design er bodol `docs/design/README.md` er version table e; code er bodol git commit e.

## Kaj er age
- Kono code likhar age `docs/design/01-overview.md` er Section 1.4 ar je milestone e kaj korcho tar section gulo poro: `docs/design/15-milestones.md` er table er Section column, file niche-r map e. Puro design protibar na. Section 1.4 er niyom gulo kokhono bhangbe na.
- Kaj Section 15 er milestone onujayi, ekta ekta kore. Acceptance criteria pass na hoya porjonto porer milestone e jabe na.
- Business rule er code e age test, tarpor code.
- Design er baire kichu lagle age jiggesh koro; nije decide kore design e likhbe na.

## Repo
| Folder | Ki | Milestone |
|---|---|---|
| `backend/` | Go API, module `vendrix/backend`: `cmd/api` (binary `shop-api`), `internal/*` | M0 theke |
| `backend/migrations/` | SQL migration (`embed.go`), constraint test `tests/`, `DEPLOYED` | M0 |
| `packages/shared/` | Pure Dart: model, API client, l10n, format | M2 |
| `website/` | Jaspr customer website (server mode) | M3 |
| `admin/` | Flutter web admin | M7 |
| `app/` | Flutter customer app | Phase 2 |
| `infra/` | Local Postgres (docker compose), Caddy, systemd | M0, M12 |
| `docs/` | `design/`, `features/`, `milestones/`, `STATUS.md` | shob shomoy |
| `scripts/`, `Makefile` | Check, local run, Claude Code er hook | |
| `.claude/` | Claude Code: settings (permission, hook), rules, skills, agents | |
| `.github/` | CI (`workflows/ci.yml`), PR template | |

## Section → file
| Section | File |
|---|---|
| 1 Overview, decision log, **1.4 niyom**, 1.6 tin mode | `docs/design/01-overview.md` |
| 2 Tech stack, Go stdlib, Dart package | `docs/design/02-tech-stack.md` |
| 3 Architecture, shop/vendor chena, checkout, settings, job | `docs/design/03-architecture.md` |
| 4 Repo ar code structure | `docs/design/04-repo-structure.md` |
| 5 Feature list (phase onujayi) | `docs/design/05-features.md` |
| 6 Database (6.7: `backend/migrations/001_init.sql`) | `docs/design/06-database.md` |
| 7 API | `docs/design/07-api.md` |
| 8 Core flow: phone, OTP, staff login, cart, pricing, checkout, state machine, SSLCommerz, COD, packing, inventory, coupon, review, SMS, job | `docs/design/08-core-flows.md` |
| 9 Theme, design token, localization | `docs/design/09-theme-l10n.md` |
| 10 Website (Jaspr), BFF | `docs/design/10-website.md` |
| 11 Admin (Flutter web), role | `docs/design/11-admin.md` |
| 12 Security, privacy, legal | `docs/design/12-security-legal.md` |
| 13 Infra, deploy, ops | `docs/design/13-infra-ops.md` |
| 14 Testing (14.2: je test obosshoi thakbe), launch | `docs/design/14-testing-launch.md` |
| 15 Milestone (M0–M12, MV, MT) | `docs/design/15-milestones.md` |
| 16 Open decision, assumption, reference | `docs/design/16-open-decisions.md` |

## Niyom (Section 1.4 er pashe)
- Database: `backend/migrations/001_init.sql` (schema er asol spec, 6.7). Test: `backend/migrations/tests/001_init_test.sql` (psql diye; migration na, embed e ashe na). Deploy er por kono migration file edit na, notun `00N_*.sql` jog; deploy hoya file er hash `backend/migrations/DEPLOYED` e.
- Migration runner (`backend/internal/db`): protiti file ek transaction e; shurute `pg_advisory_xact_lock`, `SELECT set_config('app.env', $1, true)` (`SET LOCAL` e string jora na, Rule 12); embed `backend/migrations/embed.go` theke. `schema_migrations` e file er sha256; applied file bodlale ba out-of-order file e fail (6.1).
- `backend/internal/phone`, `pricing`, `order` er code-i oi niyom er asol spec (8.1, 8.5, 8.8 e link ar niyom er lekha). Niyom bodlate hole age doc er lekha, tarpor code ar test (`design-change` skill).
- `order_items` theke taka-r hishab er line shudhu `pricing.LineFromItem` diye; nije `Line` banabe na (8.5).
- Go toolchain 1.27.x (`go.mod` er `toolchain`); Go 1.25 e ar security patch ashe na. Script, CI ar `.claude/settings.json` `GOTOOLCHAIN=go1.27.4` dey; notun patch ele tin jaygay ek shathe (2.1).
- Feature doc: protiti feature `docs/features/<naam>.md` (template `docs/features/_template.md`), code er sathe ek commit e; `backend` er protiti package er package comment e tar feature doc er link. `backend/internal/docsync` test heading, link, test er naam ar package map dhore (4.7).
- Dart: `packages/shared` path dependency diye (`shared: {path: ../packages/shared}`); website er nijer resolution ar `pubspec.lock`; admin ar app chaile ek pub workspace (4.6).
- Jaspr code likhar age `website/` e `jaspr install-skills --agent claude-code` chalao.
- Design doc bodlale: file 60 KB er niche, 80 line er beshi code block na, notun file hole ei map ar README; `backend/internal/docsync` test eta dhore.
- Secret ba `.env` file porbe na, bodlabe na, commit korbe na; local er udahoron `backend/env.example`.

## Command
- `make quick-check` (ba `bash scripts/quick-check.sh`): gofmt, go vet, go test (docsync shoho), rule check; Dart/Flutter shudhu je folder e `pubspec.yaml` ache ar bodleche (`packages/shared` bodlale shob app). Protiti kaj shesh e; Claude Code er Stop hook o chalay.
- `make check`: upore (shob part) + `make db-test` + `go test -race` asol Postgres shoho (`TEST_DATABASE_URL` PG env theke; PG env na thakle local `make db-up`, 5433). C compiler lage. Milestone shesh e ar CI te.
- `make db-test`: notun C.UTF-8 database e `001_init.sql` + `tests/001_init_test.sql` (83/83). Lage: Postgres 16+ er `psql`, `createdb`, `dropdb`; libpq env (`PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`); role ar database banano onumoti.
- `make db-up` / `make db-down`: local Postgres 16 (`infra/docker-compose.yml`; `shop_owner`, `shop_app` role ar `shop` database).
- `make migrate`, `make run`: `backend/.env` (`cp backend/env.example backend/.env`) diye migration ar API (`127.0.0.1:8080`, `GET /healthz`).
- DB lage emon Go test `TEST_DATABASE_URL` na thakle `t.Skip` kore, tai `quick-check` DB chhara chole.
