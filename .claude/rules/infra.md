---
paths:
  - "infra/**"
  - ".github/**"
---
# Infra ar CI (13)
- Caddy: client er `X-Client-IP` ar `X-Internal-Auth` muche, `X-Real-IP` boshay (3.2, 13.3); `trusted_proxies` e shudhu Cloudflare.
- API ar Jaspr shudhu loopback e (`127.0.0.1`, 13.5); firewall e 80/443 shudhu Cloudflare.
- systemd hardening (13.4) kombe na; secret shudhu env file ba secret manager e, repo te na.
- CI: action ar tool er version pin (`@v7`, `@v1.9.0`); `GOTOOLCHAIN` `go.mod` er sathe ek; job permission `contents: read`.
- Local Postgres C.UTF-8 (2.1), role `shop_owner` ar `shop_app` (6.1).
