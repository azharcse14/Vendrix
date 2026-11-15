# infra

| File | Ki | Design |
|---|---|---|
| `docker-compose.yml` | Local Postgres 16 (C.UTF-8): `make db-up`, `make db-down` | 13.1 |
| `postgres/init.sql` | Local er `shop_owner` (migration), `shop_app` (API) role ar `shop` database; duibar chalaleo kichu bhange na | 6.1, 13.5 |
| `caddy/Caddyfile` | Production/staging er reverse proxy: Cloudflare range, `X-Real-IP`, client er `X-Client-IP`/`X-Internal-Auth` muche fela, www redirect, admin static | 3.2, 13.3 |
| `caddy/Caddyfile.local` | Local https (`make https`): `tls internal`, `https://sobujbazar.localhost:8443` → `make web` (8081), production er moto header; `__Host-` cookie browser e thake. Ekbar `caddy trust` (nije chalao) browser warning er jonno; M7 e `admin.localhost` | 13.1, 13.3 |
| `systemd/shop-api.service`, `systemd/shop-web.service` | API ar Jaspr er service, hardening, restart | 13.4 |

Local: `make db-up`, `cp backend/env.example backend/.env`, `make migrate`, `make run`; website `make web`, browser e login er jonno alada terminal e `make https` (`brew install caddy`, ekbar `caddy trust`) ar `https://sobujbazar.localhost:8443`. Integration test er `TEST_DATABASE_URL`: `postgres://postgres:postgres@127.0.0.1:5433/postgres?sslmode=disable` (shudhu local; host port 5433, karon 5432 e onno project er Postgres thakte pare).

Server e (M12): env file `/etc/shop/api.env`, `/etc/shop/web.env` (secret repo te na), firewall e 80/443 shudhu Cloudflare er range, API ar Jaspr shudhu `127.0.0.1` e (13.2–13.5).
