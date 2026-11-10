# Shop chena ar khola

Design: 3.2, 3.8, 6.7, 7.1, 7.2, 8.3, 9.2, 9.4 · Milestone: M1 · Code: `backend/internal/httpx`, `backend/internal/shop`, `backend/internal/auth`

## Ki kore

Public API er protiti request kon shop er, sheta `X-Shop-Host` header (customer je domain e ache) theke chene ar context e rakhe; tarpor handler shudhu oi shop er data dekhe (Rule 3). Shop na thakle, bondho ba suspended hole handler e jay na. Notun shop khole `shop-api shop create` CLI. Website/app `GET /api/settings` e shop er published settings er public ongsho pay.

## Kivabe kaj kore

1. `httpx.ShopFromHost` (`httpx/shop.go`): `hostname` header ke lowercase kore, port ar shesher `.` bad dey. Faka hole resolver e jay na.
2. Resolver (`shop.Resolver.Resolve`, `shop/resolve.go`): memory cache e 60 second er moddhe pele sheta; na hole `shop_domains` JOIN `shops`. Shudhu pawa domain cache hoy.
3. Middleware: na pele ba `closed` → `404 shop_not_found`; `suspended` → `423 shop_suspended`; resolver er error → log ar `500 internal`; na hole `httpx.Shop` context e.
4. Handler `httpx.ShopFrom(ctx)` theke `ID` niye protiti query te `shop_id = $n`.

Router e public route e jog M1 er router step e (`cmd/api/router.go`).

Public settings (`Handlers.Settings`, `handler.go`): `httpx.ShopFrom` er shop → `publishedSettings` (`store.go`) → `PublicSettings` (whitelist: `branding`, `theme`, `locales`, `features`, `homepage`, `seo`, `integrations`, `checkout.payment_methods`, `checkout.return_window_days`, `marketplace.show_vendor_name`). `GET /api/settings/version` shudhu version.

Shop khola (`runShopCreate` → `shop.Create`): flag check; `DefaultSettings` (preset theke theme, 9.2 er shob block); temporary password `rand.Text()` ar `auth.HashPassword`; tarpor `db.InTx` e `shops`, `shop_domains` (primary), house vendor, `shop_settings` (draft = published), `shop_counters` (`order_number` 0), owner `staff`. Password shudhu stdout e ekbar.

Udahoron (local): `cd backend && set -a && . ./.env && set +a && go run ./cmd/api shop create --slug sobujbazar --prefix SB --domain sobujbazar.localhost --name-bn "সবুজ বাজার" --name-en "Sobuj Bazar" --owner-email owner@example.com --owner-name Owner`.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/httpx/shop.go` → `ShopFromHost`, `ShopFrom`, `hostname`, `Shop`, `ResolveShop` | Middleware ar context |
| `backend/internal/shop/resolve.go` → `NewResolver`, `Resolve` | Domain → shop, cache |
| `backend/internal/shop/settings.go` → `DefaultSettings`, `presets` | Notun shop er settings (9.2, 9.4) |
| `backend/internal/shop/create.go` → `Create`, `Params` | Shop khola (3.8) |
| `backend/internal/shop/handler.go` → `NewHandlers`, `Settings`, `SettingsVersion` | Public settings route |
| `backend/internal/shop/settings.go` → `PublicSettings`, `publicKeys` | Whitelist (7.2) |
| `backend/internal/shop/store.go` → `publishedSettings` | SQL |
| `backend/cmd/api/main.go` → `runShopCreate` | CLI |
| `backend/internal/auth/password.go` → `HashPassword`, `CheckPassword` | Owner staff er password hash (`shop create`) |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| `X-Shop-Host` (lowercase, port chhara) → shop; nai, ochena ba `closed` → `404 shop_not_found`; `suspended` → `423` (3.2, v1.10) | `ShopFromHost`, `hostname` | `TestShopFromHost` |
| Domain → shop, 60 second cache, ochena domain cache na (3.2) | `Resolver.Resolve` | `TestResolverIntegration` |
| Preset er 5 rong, font, radius; 9.2 er shob block; payment shudhu `cod`; `seo.noindex` true (9.4, 14.4) | `DefaultSettings` | `TestDefaultSettings` |
| Ek transaction e shop, primary domain, house vendor, settings, counter, owner (`must_change_password`); bhul slug, `www` domain, naam, email, preset ba duplicate e kono row na (3.8, v1.10) | `Create` | `TestCreateShopIntegration` |
| `shop create` er flag check | `runShopCreate` | `TestRun` |
| Public settings shudhu whitelist; risk, OTP, SMS cap, inventory, commission ar notun block jay na (7.2, 9.2, v1.10) | `PublicSettings` | `TestPublicSettings` |
| Protiti host e nijer shop er published settings ar version; draft na (Rule 3, 7.2) | `Handlers.Settings`, `SettingsVersion` | `TestSettingsIsolationIntegration` |
| Password: pbkdf2-sha256, 600000, 16 byte salt, format `pbkdf2-sha256$600000$<salt>$<hash>`, constant-time tulona, bhul format e false (8.3) | `HashPassword`, `CheckPassword` | `TestPasswordHash` |

## Data

`shop_domains` (`domain` lowercase primary key, `shop_id`, `is_primary`), `shops` (`slug`, `mode`, `status`, `timezone`); shop khola e `vendors`, `shop_settings`, `shop_counters`, `staff` o; `001_init.sql`.

## Bhul hole ki hoy

- Header nai ba ochena domain: `404 shop_not_found`.
- Shop `suspended`: `423 shop_suspended` (website "dokan bondho" page); `closed`: `404`.
- Database error: `500 internal`, log e `shop resolve`.
- Status bodlale 60 second porjonto purono status (cache).
- `shop create` e duplicate slug ba domain, bhul prefix (`^[A-Z]{2,5}$`): database er error, kono row thake na.

## Bodlanor age mone rakho

- Cache process er memory te; ek er beshi API process hole protitir nijer cache, tai bodol 60 second porjonto alada dekhay.
- Ochena domain cache korbe na: random `X-Shop-Host` diye memory bhore jabe.
- Admin route e shop staff session theke (M4), `X-Shop-Host` theke na (3.2).
- Notun setting jog korle public kina `publicKeys` e decide koro; na dile private thake.
- `features` notun shop e faka; organic er `unit_pricing`, `variable_weight` ityadi admin settings (M8) e chalu. SSLCommerz credential set (M10) er por `payment_methods` e `sslcommerz`.
- `auth` package M4 e login, session, TOTP pabe; tokhon password er map `auth` er nijer doc e jabe. Ochena email e dummy hash ar ek shathe 4 tar beshi PBKDF2 na (8.3) login er kaj, M4 e.

## Proshno-uttor

- **`www` keno nai?** Caddy `www` ke apex e `301` kore (3.2 Niyom 6), tai `shop_domains` e `www` lage na.
