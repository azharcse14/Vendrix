# Server, config ar healthz

Design: 2.2, 4.2, 12.2, 13.4, 13.5 · Milestone: M0 · Code: `backend/cmd/api`, `backend/internal/config`

## Ki kore

`shop-api` binary: `serve` (default) HTTP API chalay, `migrate` database migration chalay ([migration.md](migration.md)), `credentials set` shop er secret rakhe ([credentials.md](credentials.md)), `version` git commit dekhay. Shuru tei env check kore; bhul thakle start-i hoy na. SIGTERM pele cholche emon request shesh kore thame.

## Kivabe kaj kore

1. `main` (`cmd/api/main.go`): SIGINT ba SIGTERM e bondho hoy emon context banay, tarpor `run`.
2. `run`: command bachai (`serve`, `migrate`, `shop` ([shop.md](shop.md)), `credentials`, `version`); log JSON e stdout e.
3. `runServe`: `config.Load(lookup, config.Serve)` → `db.Open` (pgx) → `newHandler` → `net.Listen(ADDR)` → job worker goroutine ([jobs.md](jobs.md)) → `serve`; shutdown e worker thamar por database close.
4. `newHandler` (`router.go`): route (`GET /healthz`), CrossOriginProtection e `ADMIN_ORIGINS`, middleware er kram ([http-foundation.md](http-foundation.md)).
5. `serve` (`server.go`): `newServer` er timeout (4.2) e chalay; ctx bondho hole `Shutdown` (25 second, systemd er 30 second er kom).
6. `healthz`: database e 2 second er moddhe ping; thik hole `{"status":"ok"}`, na hole `503 db_unavailable`.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/cmd/api/main.go` → `main`, `run`, `runServe`, `runMigrate`, `version` | Command ar shuru |
| `backend/internal/config/config.go` → `Load`, `loopback`, `postgresURL`, `origin` | Env pora ar check |
| `backend/cmd/api/router.go` → `newHandler`, `healthz` | Route ar middleware |
| `backend/cmd/api/server.go` → `newServer`, `serve` | Timeout, graceful shutdown |
| `backend/cmd/api/main_test.go`, `backend/internal/config/config_test.go` | Test |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| API shudhu loopback e (13.5) | `config.loopback` | `TestLoadRejects` |
| Env bhul hole start na; error e password na (13.5) | `config.Load`, `postgresURL` | `TestLoadRejects`, `TestLoadServe` |
| `ADMIN_ORIGINS` https; local e `http://localhost` (13.5) | `config.origin` | `TestLoadServe`, `TestLoadRejects` |
| Migrate e shudhu `MIGRATE_DATABASE_URL` (6.1) | `config.Load` | `TestLoadMigrate` |
| env.example er `local-only` secret local chhara na (13.5) | `config.Load` | `TestRejectExampleSecrets` |
| Router build e panic na (12.2 Server 7) | `newHandler` | `TestRouterBuild` |
| Onno site er browser er POST atkano (12.2 Server 3) | `newHandler` (CrossOriginProtection) | `TestCrossOrigin` |
| Timeout ar graceful shutdown (4.2, 2.2) | `newServer`, `serve` | `TestServeShutdown` |
| Env chhara kono command chole na | `run` | `TestRun` |

## Data

Nai (`healthz` shudhu ping kore).

## Bhul hole ki hoy

- Env bhul: shob bhul ek error e, process exit 1; systemd `Restart=always` abar cheshta kore, karon log e.
- Database nai: `serve` start-i hoy na (`database e jete parchi na`); chalu obosthay database gele `/healthz` 503, uptime monitor alert dey (13.8).
- pgx driver register hoyni (`driver.go` er import nai): `db.ErrNoDriver`.

## Bodlanor age mone rakho

- Notun env: `Config` e field, `Load` e check, `backend/env.example`, design 13.5 er table, `config_test.go` e test.
- Notun route: `newHandler` e method shoho pattern; `TestRouterBuild` conflict nijei dhore.
- Middleware er kram bodlale log, recover, CORS er test abar dekho.

## Proshno-uttor

- **`run` keno alada function?** Test e env ar output bahir theke dite (`TestRun`); `main` shudhu signal ar exit code.
- **`serve` keno listener ney?** Test e `127.0.0.1:0` er random port e chalate (`TestServeShutdown`).
