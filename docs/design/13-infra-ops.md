> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 13. Infrastructure, deployment ar operations

### 13.1 Environment

| Environment | Domain | Database | Payment | SMS |
|---|---|---|---|---|
| Local | `localhost` | Docker Compose e Postgres | SSLCommerz sandbox | Log e (pathay na) |
| Staging | `staging.tomarshop.com`, `api.staging...`, `admin.staging...` | Alada | Sandbox | Shudhu team er number |
| Production | `tomarshop.com`, `api.tomarshop.com`, `admin.tomarshop.com` | Asol | Live | Asol |

### 13.2 Server

MVP er jonno ekta VPS jothesto (shuru te 2 vCPU, 4 GB RAM, SSD), tate Go API, Jaspr server, PostgreSQL ar Caddy. **Region thik korar age** data protection law er localization niyom (12.1) lawyer er sathe confirm koro: Bangladesh er bhetore hosting lagbe, naki Singapore/Mumbai er moto kache-r region cholbe. Order barle database alada server ba managed Postgres e.

### 13.3 Caddy

v1.8: file `infra/caddy/Caddyfile`.

```
{
  servers {
    trusted_proxies static {$CF_IP_RANGES}   # Cloudflare er IPv4/IPv6 range (13.5)
    client_ip_headers CF-Connecting-IP
  }
}
(origin_tls) {
  tls /etc/caddy/origin.pem /etc/caddy/origin.key   # Cloudflare origin certificate
}
(app_proxy) {
  header_up X-Real-IP {client_ip}    # Cloudflare theke ashle CF-Connecting-IP, na hole connection er IP
  header_up -X-Client-IP             # client nije boshate parbe na (3.2)
  header_up -X-Internal-Auth
}
www.tomarshop.com {
  import origin_tls
  redir https://tomarshop.com{uri} permanent
}
tomarshop.com {                       # shop er nijer domain (jemon sobujbazar.com) holeo ekoi
  import origin_tls
  reverse_proxy 127.0.0.1:8081 {      # Jaspr server
    import app_proxy
  }
}
api.tomarshop.com {
  import origin_tls
  reverse_proxy 127.0.0.1:8080 {      # Go API
    import app_proxy
  }
}
admin.tomarshop.com {
  import origin_tls
  header {
    Content-Security-Policy "frame-ancestors 'none'"
    X-Content-Type-Options nosniff
  }
  @app path / /index.html /flutter_bootstrap.js /main.dart.js
  header @app Cache-Control "no-cache"   # deploy er por purono JS na
  root * /srv/admin                   # flutter build web er output
  try_files {path} /index.html
  file_server
}
```

Cloudflare proxy on, SSL mode "Full (strict)", Caddy te Cloudflare origin certificate. Server er firewall e 80/443 shudhu Cloudflare er IP range theke (ba Authenticated Origin Pulls), na hole Cloudflare er WAF ar DDoS shurokkha edano jay. Cloudflare er range Caddy-i dekhe (`trusted_proxies`, `client_ip_headers`); Jaspr ar Go API shudhu loopback theke asha `X-Real-IP` mane (3.2).

**Multi-tenant (Phase 3)**: shop er nijer domain er jonno duita upay: Cloudflare for SaaS (custom hostname), ba Caddy on-demand TLS jar `ask` endpoint shudhu `shop_domains.verified_at` thaka domain allow kore. Domain verify (DNS TXT record) na hoya porjonto certificate na, na hole keu je kono domain point kore certificate banate parbe.

### 13.4 Service (systemd)

v1.8: file `infra/systemd/`.

```ini
# /etc/systemd/system/shop-api.service
[Unit]
Description=Shop Go API
After=network-online.target postgresql.service
Wants=network-online.target
# v1.5: barbar fail holeo restart chere dey na (default 10 second e 5 bar er por thame)
StartLimitIntervalSec=0

[Service]
User=shop
EnvironmentFile=/etc/shop/api.env
ExecStart=/srv/api/shop-api
Restart=always
# v1.5: DB restart er shomoy druto crash loop na
RestartSec=2
KillSignal=SIGTERM
# graceful shutdown er context 25 second, TimeoutStopSec er kom
TimeoutStopSec=30
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Jaspr er jonno ekoi rokom `shop-web.service` (ekoi `RestartSec`, `StartLimitIntervalSec`): `ExecStart=/srv/web/app`, ar `build/jaspr/web` folder ta executable er pashei (`/srv/web/web`).

### 13.5 Config (env)

| Variable | Ki |
|---|---|
| `APP_ENV` | `local`, `staging`, `production`. Migration runner transaction e `set_config('app.env', $1, true)` diye dey (6.1, v1.6; v1.7: `local` chhara, khali holeo, `shop_app` na thakle `001` fail); `local` chhara OTP log mode chalu thakle server start hobe na (8.2) |
| `ADDR` | `127.0.0.1:8080` (v1.5: shudhu loopback, Caddy theke; Jaspr o `127.0.0.1:8081`). `:8080` dile firewall bhul hole API Cloudflare ar Caddy chhara-i khola thake |
| `DATABASE_URL` | Postgres connection, `shop_app` role (DDL nai, 6.1) |
| `MIGRATE_DATABASE_URL` | Table er malik role, shudhu `shop-api migrate` e |
| `OTP_SECRET` | OTP HMAC er secret (32 byte random) |
| `CREDENTIALS_KEY` | AES-GCM key (32 byte, base64). Rotation: notun key notun version e, purono row `key_version` dekhe pore re-encrypt |
| `ADMIN_ORIGINS` | Admin panel er origin, comma diye (CrossOriginProtection ar CORS), jemon `https://admin.tomarshop.com` |
| `PREVIEW_SECRET`, `PRINT_SECRET` | Signed preview/print link |
| `TURNSTILE_SECRET` | Cloudflare Turnstile |
| `R2_ACCOUNT_ID`, `R2_BUCKET`, `R2_PUBLIC_BASE_URL`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY` | File storage (platform er, Rule 15) |
| `CF_IP_RANGES` | Cloudflare er IP range (space diye), shudhu Caddy er `trusted_proxies` e (13.3). Go API ar Jaspr eta dekhe na, shudhu loopback theke asha `X-Real-IP` mane (3.2) |
| `JASPR_INTERNAL_SECRET` | Jaspr → API er `X-Internal-Auth` (API ar website duitatei) |
| `CONFIRM_SECRET` | v1.4: COD confirm token er HMAC (32 byte random, 8.10). v1.7: rotate korleo ager pathano link kaj kore (hash diye lookup); bhange pending order er SMS resend, tai rotation e pending COD order er `confirm_token_hash` abar likho (8.10 step 5) |
| `TOTP_KEY` | v1.5: staff TOTP secret er AES-GCM key (32 byte, base64), AAD = `shop_id:staff_id:key_version` (8.3). Rotation: notun key notun version e (`staff.totp_key_version`), purono row pore re-encrypt |
| `RECOVERY_SECRET` | v1.5: staff recovery code er HMAC (32 byte random, 8.3). Rotate korle shob staff ke notun code dite hobe |
| `ALERT_EMAIL`, `ALERT_WEBHOOK_URL` | Admin alert |
| `IMAGE_BASE_URL`, `IMAGE_RESIZE` | v1.13, website (Jaspr): chobir base URL ar `cloudflare` ba `none` (local) (10.10) |
| Website: `API_BASE_URL` | Jaspr server, `http://127.0.0.1:8080` (Cloudflare diye na). Shop ashe `Host` theke; local e `localhost` ke `shop_domains` e ekta row hishebe rakho |

v1.9: `backend/env.example` er secret e `local-only` thake; `APP_ENV` local na hole config ei value nile start hoy na (copy kora example secret production e na). Shop-specific secret (SSLCommerz, SMS, courier) database er `shop_credentials` e, env e na. Database banao: `createdb -E UTF8 --locale=C.UTF-8 -T template0 -O shop_owner shop`. CI ar local e-o `shop_app` role banao (`CREATE ROLE shop_app LOGIN`), na hole `001_init_test` er privilege test kichu dekhe na. v1.8: local e `make db-up` (`infra/docker-compose.yml`; `infra/postgres/init.sql` `shop_owner`, `shop_app` ar `shop` database banay), tarpor `cp backend/env.example backend/.env`, `make migrate`, `make run`.

### 13.6 Deploy (GitHub Actions)

1. Protiti push (v1.8: `.github/workflows/ci.yml`; `GOTOOLCHAIN=go1.27.4`, v1.5: tai CI ar local e ekoi Go): Go `vet` + `test` (docsync shoho) + `govulncheck` (Postgres service container, `C.UTF-8`, `shop_app` role), `make check` (`tests/001_init_test.sql` shoho, `scripts/db-test.sh`), router build test, Dart `analyze` + `test` (shared, website, admin).
2. `main` e merge: build: Go `CGO_ENABLED=0 GOOS=linux go build`, `jaspr build`, `flutter build web --release`.
3. Staging e deploy → migration (`shop-api migrate`, `MIGRATE_DATABASE_URL`) → service restart → `/healthz` check → Playwright checkout test.
4. Hate approve korle production e ekoi step. `/healthz` fail hole ager binary te ferot (rollback). v1.7: production e notun migration chalanor por deploy job oi file er `sha256sum` line `backend/migrations/DEPLOYED` e jog kore PR khole (merge manush kore); tarpor Claude Code hook ar CI hash check oi file pahara dey. Asol pahara runner er checksum (6.1): applied file er ek byte bodlaleo `migrate` fail.
5. Migration shob shomoy samner dike compatible (column jog age, purono column bad pore-r deploy e).

### 13.7 Backup

1. WAL archiving (wal-g ba pgBackRest → alada private R2 bucket, encrypt): point-in-time restore, RPO kichu minute. Shathe protirat 2 ta (Asia/Dhaka) `pg_dump`, encrypt, oi bucket e. v1.5: oi bucket e R2 er bucket lock (retention rule, jemon 35 din), tai VPS hack hole-o tar token diye backup muche ba overwrite kora jay na.
2. 30 ta daily ar 12 ta monthly rakho.
3. **Mashe ekbar staging e restore kore dekho.** Restore na korle backup kaj kore kina jana jay na.
4. R2 chobir bucket e versioning on.

### 13.8 Monitoring

1. Uptime monitor protiti minute e `/healthz` (API) ar home page (website).
2. `log/slog` JSON log, protiti request e `request_id`, latency, status. Personal data na.
3. 5xx barle, job `failed_at`, payment validation fail, 30 minute er beshi purono `initiated` payment, IPN na asha, SMS daily cap er 80% hole admin alert (email/webhook).
4. Disk, CPU, memory alert (VPS provider er tool).
5. Dashboard e business number: ajker order, conversion, COD refuse rate.

### 13.9 Shomossha hole (runbook)

| Shomossha | Ki korbe |
|---|---|
| SSLCommerz down | Settings e `payment_methods` theke `sslcommerz` shoriye publish (1 minute e checkout e lukabe) |
| SMS gateway down | OTP mode `never` kore dao, COD confirmation staff phone e |
| Deploy er por error | Ager binary te rollback, migration samner-compatible bole database thik thake |
| Disk bhore geche | Log rotate, purono backup local theke muche fela |
| Bhul theme publish | Settings history theke rollback |
| Fake order er dhel | OTP mode `all_cod`, phone block, Cloudflare e IP block |
| IPN ashche na | Cloudflare WAF ar firewall log, Bot Fight Mode off kina (skip kora jay na, 8.9), SSLCommerz panel e IPN URL, reconcile job chalu kina (8.9) |
| SMS er khoroch hothat bare | Turnstile chalu kina, daily cap, OTP mode `risky` ba `never` |
