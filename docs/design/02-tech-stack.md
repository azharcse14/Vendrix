> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 2. Tech stack

### 2.1 Stack

| Bhag | Technology | Version/note |
|---|---|---|
| Backend API | Go | Build ar CI **1.27.x**, shob shomoy latest patch. `go.mod` e `go 1.25.1` (minimum: `http.CrossOriginProtection`, `crypto/pbkdf2`, `crypto/rand.Text`) ar `toolchain go1.27.4`. Go 1.25 line 1.25.14 e shesh, ar security patch pabe na. 1.25.0 te CrossOriginProtection er security bug (CVE-2025-47910). v1.5: CI te `GOTOOLCHAIN=go1.27.4` (13.6), tai purono Go te test chole na. v1.7: notun 1.27.x patch ele (11 November 2026 e 1.27.4-i shesh; 1.27.4 e `net/http` er GO-2026-6859 fix, v1.15) `go.mod` er `toolchain`, `.claude/settings.json` ar `scripts/` er `GOTOOLCHAIN` ek PR e barao (12.2 er mashik check) |
| Postgres driver | `github.com/jackc/pgx/v5`, `database/sql` er maddhome (`pgx/v5/stdlib`) | Go er ekmatro bahirer dependency |
| Database | PostgreSQL | 16 ba notun. Extension: `pg_trgm`. Database `C.UTF-8` (ba `en_US.UTF-8`) locale e banao: `C`/`POSIX` e Bangla trigram khali, search kaj kore na (`001` shuru tei check kore) |
| Customer website | Jaspr, server mode | `jaspr: ^0.23.5` (v1.5: 0.23.5 September 2026 e publish, pub.dev e milano), `pubspec.lock` commit. Beta version na: 0.24.0-beta.0 (Dart 3.13 lage) stable hole release notes dekhe tarpor barao |
| Website styling | Tailwind CSS | Class gulo CSS variable ke point korbe. Version pin koro (v3 e `var()` rong e `/50` er moto opacity modifier kaj kore na) |
| Admin panel | Flutter web | `bloc`/`flutter_bloc`, `go_router`, `get_it` |
| Customer app (Phase 2) | Flutter (Android age) | Admin er moto ekoi package |
| Shared code | Pure Dart package | `http`, `intl` |
| File storage | Cloudflare R2 (S3-compatible) | Product chobi, logo, certificate |
| CDN, DDoS, bot | Cloudflare | Turnstile (checkout bot protection) |
| Reverse proxy | Caddy | HTTPS, routing |
| SMS | Bangladesh er SMS gateway (HTTP API) | OTP, order SMS |
| Payment | SSLCommerz (MVP), bKash/Nagad (Phase 2) | Sandbox diye test |
| Courier | Manual (MVP), Steadfast/Pathao/RedX API (Phase 2) | Webhook shoho |

### 2.2 Go: standard library diye ki ki hobe

| Kaj | Ki use korbe |
|---|---|
| Routing | `http.ServeMux`, pattern `"GET /api/products/{slug}"`, `r.PathValue("slug")` |
| Middleware | Nijer `func(http.Handler) http.Handler` chain |
| JSON | `encoding/json`, nijer client er request e decoder `DisallowUnknownFields()`. Third-party webhook e na |
| Login session token | `crypto/rand` diye random token; database e `crypto/sha256` hash |
| Staff password | `crypto/pbkdf2` (SHA-256, 600,000 iteration, 16 byte salt) |
| Staff 2FA | TOTP (RFC 6238) `crypto/hmac` + `crypto/sha1` diye nije; `staff.totp_last_step` diye replay atkano (conditional `UPDATE`, 8.3), `staff.failed_mfa` (temp_token jure bhul gona), recovery code. v1.5: TOTP secret AES-GCM `TOTP_KEY` diye (AAD = `shop_id:staff_id:key_version`, `staff.totp_key_version`), recovery code er HMAC `RECOVERY_SECRET` diye |
| Random public code | `crypto/rand.Text()` |
| Secret encrypt (credential) | `crypto/aes` + `crypto/cipher` (AES-GCM, AAD = `shop_id:provider:key_version`) |
| CSRF | `http.CrossOriginProtection` |
| Log | `log/slog`, JSON handler |
| Migration | `embed` diye `migrations/*.sql`, nijer runner, `schema_migrations` table |
| Background job | Goroutine worker + `jobs` table (`FOR UPDATE SKIP LOCKED`) |
| Bahirer API (SSLCommerz, SMS, courier, R2) | `net/http` client, timeout shoho |
| R2 upload | S3 SigV4 signature `crypto/hmac` diye nije (prai 100 line) |
| CSV export | `encoding/csv` |
| Rate limit | In-memory token bucket (map + `sync.Mutex`), key e client IP (3.2). OTP, staff login ar TOTP er gona database e, restart e-o thake |
| Graceful shutdown | `signal.NotifyContext` + `srv.Shutdown(ctx)` |
| Test | `testing`, `net/http/httptest` |

Standard library jekhane jothesto na, shekhane ei shomadhan:

| Shomossha | Shomadhan |
|---|---|
| WebP banano/resize | Cloudflare image resizing (URL diye size, `format=auto`). Stdlib e resize nai (`image/jpeg` shudhu decode/encode), tai server e resize korbo na |
| Chobi upload | API te 10 MB, oi route e `http.ResponseController` diye read **ar write** deadline 2 minute. Server er `WriteTimeout` (30 second) request header porar shomoy-i boshe, tai shudhu read barale lomba upload er response kete jay. `image.DecodeConfig` diye 4000×4000 er beshi hole reject (decompression bomb), JPEG/PNG decode kore abar encode (EXIF/GPS mucche jay). SVG ar WebP na (SVG e script/XSS; stdlib e WebP decoder nai) |
| PDF invoice | Jaspr e printable HTML invoice page (`/invoice/{token}`), browser e "Save as PDF" |
| JWT | Use korbo na. Opaque token + database session (logout/revoke shohoj) |

### 2.3 Dart package

| Jaygay | Package |
|---|---|
| `packages/shared` | `http`, `intl` (ar kichu na, pure Dart). v1.12: dev e shudhu `test`, `lints`; library tai `pubspec.lock` commit na; CI te `dart-lang/setup-dart` (Flutter 3.47.9 er Dart 3.13.7; action baki action er moto version tag e) |
| `website` (Jaspr) | `jaspr`, `jaspr_router` (dorkar hole), Tailwind integration, `shared` |
| `admin`, `app` (Flutter) | `flutter_bloc`, `bloc`, `go_router`, `get_it`, `shared` |

AI coding agent diye Jaspr code likhle project e `jaspr install-skills` chalao (Claude Code hole `--agent claude-code`). Eta agent ke Jaspr er latest syntax shekhay, karon Jaspr 1.0 hoyni ar purono syntax prai bhul hoy. Jaspr update korle command ta abar chalao.
