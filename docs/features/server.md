# Server, config ar healthz

Design: 2.2, 4.2, 12.2, 13.4, 13.5 · Milestone: M0 · Code: `backend/cmd/api`, `backend/internal/config`

## Ki kore

`shop-api` binary: `serve` (default) HTTP API chalay, `migrate` database migration chalay ([migration.md](migration.md)), `credentials set` shop er secret rakhe ([credentials.md](credentials.md)), `shop create` notun shop ([shop.md](shop.md)), `staff reset-2fa` staff er TOTP muche ([auth.md](auth.md)), `version` git commit dekhay. Shuru tei env check kore; bhul thakle start-i hoy na. SIGTERM pele cholche emon request shesh kore thame.

## Kivabe kaj kore

1. `main` (`cmd/api/main.go`): SIGINT ba SIGTERM e bondho hoy emon context banay, tarpor `run`.
2. `run`: command bachai (`serve`, `migrate`, `shop`, `staff`, `credentials`, `version`); log JSON e stdout e.
3. `runServe`: `config.Load(lookup, config.Serve)` → `db.Open` (pgx) → `newHandler` → `net.Listen(ADDR)` → job worker goroutine ([jobs.md](jobs.md)) → `serve`; shutdown e worker thamar por database close.
4. `newHandler` (`router.go`): route (`GET /healthz`; public = `ShopFromHost` → `RateLimit(120, minute)` er pore shop ([shop.md](shop.md)), catalog ([catalog.md](catalog.md)) ar delivery zone ([delivery-zone.md](delivery-zone.md)) er route; cart ([cart.md](cart.md)) ar checkout/order ([checkout.md](checkout.md)) public → `OptionalCustomer`, admin (`AdminSession` → `RequireRole`): order ar fulfillment (`order.AdminRoutes`, [order-status.md](order-status.md), [fulfillment.md](fulfillment.md)), inventory ([inventory.md](inventory.md)), zone, coupon, customer + block, staff, audit list, dashboard ([admin-api.md](admin-api.md)), chobi ([media.md](media.md)), wishlist ar `/api/me/orders` public → `RequireCustomer`, `order.Guard` e Turnstile, `OTP_SECRET`, `CONFIRM_SECRET`, `PRINT_SECRET` (print link, [print.md](print.md); `GET /api/print` public); catalog e `newStore` (`STORAGE`: local dir ba R2, [media.md](media.md)); customer auth ([auth.md](auth.md)): `POST /api/auth/otp/send` public er upore `RateLimit(10, ghonta)`, `otp/verify` public, `logout` ar `GET`/`PATCH /api/me` public → `RequireCustomer`; staff login `/api/admin/auth/login`, `/totp`, `/recovery` middleware chhara (shop body/temp_token theke); baki admin route `AdminSession` → `RequireRole` (`/auth/password`, `/totp/setup`, `/totp/enable`, `/auth/logout` shudhu `AdminSession`, `/api/admin/me` `RequireStaff`), CrossOriginProtection e `ADMIN_ORIGINS`, middleware er kram ([http-foundation.md](http-foundation.md)).
5. `serve` (`server.go`): `newServer` er timeout (4.2) e chalay; ctx bondho hole `Shutdown` (25 second, systemd er 30 second er kom).
6. `healthz`: database e 2 second er moddhe ping; thik hole `{"status":"ok"}`, na hole `503 db_unavailable`.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/cmd/api/main.go` → `main`, `run`, `runServe`, `runMigrate`, `runCredentials`, `runShopCreate`, `runStaffReset2FA`, `version` | Command ar shuru |
| `backend/internal/config/config.go` → `Load`, `base64Key`, `TurnstileTestSecret`, `loopback`, `postgresURL`, `origin` | Env pora ar check |
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
| `serve` e secret lagbe: `OTP_SECRET`, `RECOVERY_SECRET`, `CONFIRM_SECRET` (COD confirm link, 8.10), `PRINT_SECRET` (print link, 11.7 item 8) (32+ okkhor), `TURNSTILE_SECRET`, `TOTP_KEY` (base64 32 byte); error e secret na (13.5) | `config.Load`, `base64Key` | `TestLoadServe`, `TestLoadRejects` |
| Turnstile er Cloudflare test secret (`1x`/`2x`/`3x...AA`) local chhara na (13.5 v1.20) | `config.Load` (`turnstileTestSecrets`) | `TestLoadRejects`, `TestRejectExampleSecrets` |
| Router build e panic na (12.2 Server 7); admin route staff chhara `401`, public route shop chhara `404` (v1.10) | `newHandler` | `TestRouterBuild` |
| `STORAGE` default `r2` (`R2_ACCOUNT_ID`, `R2_BUCKET`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY` lagbe), `local` shudhu `APP_ENV=local` e (`UPLOAD_DIR` default `.data/uploads`, process `backend/` e) (13.5, 11.7 item 9) | `config.Load`, `newStore` | `TestLoadStorage` |
| Protiti `/api/admin/*` route × 5 role (11.2): table er role e onumoti, baki `403 forbidden`, cookie chhara `401`, `setup_pending` e `403 setup_required` (setup route chhara); table ar source er route mile (11.7 item 13 i) | `newHandler`, `lookupStaffSession`, catalog `Routes` | `TestAdminRouteRoles` |
| Onno site er browser er POST atkano (12.2 Server 3) | `newHandler` (CrossOriginProtection) | `TestCrossOrigin` |
| Timeout ar graceful shutdown (4.2, 2.2) | `newServer`, `serve` | `TestServeShutdown` |
| Env chhara kono command chole na | `run` | `TestRun` |

## Data

Nai (`healthz` shudhu ping kore).

## Bhul hole ki hoy

- Purono `backend/.env` e M4 er secret (`OTP_SECRET`, `TURNSTILE_SECRET`, `TOTP_KEY`, `RECOVERY_SECRET`) ba `CONFIRM_SECRET`/`PRINT_SECRET` nai, ba `STORAGE=local` nai (default r2 e `R2_*` lage): `make run` start hoy na; `backend/env.example` theke line gulo copy koro.
- Env bhul: shob bhul ek error e, process exit 1; systemd `Restart=always` abar cheshta kore, karon log e.
- Database nai: `serve` start-i hoy na (`database e jete parchi na`); chalu obosthay database gele `/healthz` 503, uptime monitor alert dey (13.8).
- pgx driver register hoyni (`driver.go` er import nai): `db.ErrNoDriver`.

## Bodlanor age mone rakho

- Notun env: `Config` e field, `Load` e check, `backend/env.example`, design 13.5 er table, `config_test.go` e test.
- Notun route: `newHandler` e method shoho pattern; `TestRouterBuild` conflict nijei dhore. Notun `/api/admin/*` route e `main_test.go` er `adminRoutes` e row (role set 11.2 theke), na hole `TestAdminRouteRoles` fail.
- Middleware er kram bodlale log, recover, CORS er test abar dekho.

## Proshno-uttor

- **`run` keno alada function?** Test e env ar output bahir theke dite (`TestRun`); `main` shudhu signal ar exit code.
- **`serve` keno listener ney?** Test e `127.0.0.1:0` er random port e chalate (`TestServeShutdown`).
