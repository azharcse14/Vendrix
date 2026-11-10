# Shop chena

Design: 3.2, 6.7, 7.1, 8.3 · Milestone: M1 · Code: `backend/internal/httpx`, `backend/internal/shop`, `backend/internal/auth`

## Ki kore

Public API er protiti request kon shop er, sheta `X-Shop-Host` header (customer je domain e ache) theke chene ar context e rakhe; tarpor handler shudhu oi shop er data dekhe (Rule 3). Shop na thakle, bondho ba suspended hole handler e jay na.

## Kivabe kaj kore

1. `httpx.ShopFromHost` (`httpx/shop.go`): `hostname` header ke lowercase kore, port ar shesher `.` bad dey. Faka hole resolver e jay na.
2. Resolver (`shop.Resolver.Resolve`, `shop/resolve.go`): memory cache e 60 second er moddhe pele sheta; na hole `shop_domains` JOIN `shops`. Shudhu pawa domain cache hoy.
3. Middleware: na pele ba `closed` → `404 shop_not_found`; `suspended` → `423 shop_suspended`; resolver er error → log ar `500 internal`; na hole `httpx.Shop` context e.
4. Handler `httpx.ShopFrom(ctx)` theke `ID` niye protiti query te `shop_id = $n`.

Router e public route e jog M1 er router step e (`cmd/api/router.go`).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/httpx/shop.go` → `ShopFromHost`, `ShopFrom`, `hostname`, `Shop`, `ResolveShop` | Middleware ar context |
| `backend/internal/shop/resolve.go` → `NewResolver`, `Resolve` | Domain → shop, cache |
| `backend/internal/auth/password.go` → `HashPassword`, `CheckPassword` | Owner staff er password hash (`shop create`) |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| `X-Shop-Host` (lowercase, port chhara) → shop; nai, ochena ba `closed` → `404 shop_not_found`; `suspended` → `423` (3.2, v1.10) | `ShopFromHost`, `hostname` | `TestShopFromHost` |
| Domain → shop, 60 second cache, ochena domain cache na (3.2) | `Resolver.Resolve` | `TestResolverIntegration` |
| Password: pbkdf2-sha256, 600000, 16 byte salt, format `pbkdf2-sha256$600000$<salt>$<hash>`, constant-time tulona, bhul format e false (8.3) | `HashPassword`, `CheckPassword` | `TestPasswordHash` |

## Data

`shop_domains` (`domain` lowercase primary key, `shop_id`, `is_primary`), `shops` (`slug`, `mode`, `status`, `timezone`); `001_init.sql`.

## Bhul hole ki hoy

- Header nai ba ochena domain: `404 shop_not_found`.
- Shop `suspended`: `423 shop_suspended` (website "dokan bondho" page); `closed`: `404`.
- Database error: `500 internal`, log e `shop resolve`.
- Status bodlale 60 second porjonto purono status (cache).

## Bodlanor age mone rakho

- Cache process er memory te; ek er beshi API process hole protitir nijer cache, tai bodol 60 second porjonto alada dekhay.
- Ochena domain cache korbe na: random `X-Shop-Host` diye memory bhore jabe.
- Admin route e shop staff session theke (M4), `X-Shop-Host` theke na (3.2).
- `auth` package M4 e login, session, TOTP pabe; tokhon password er map `auth` er nijer doc e jabe. Ochena email e dummy hash ar ek shathe 4 tar beshi PBKDF2 na (8.3) login er kaj, M4 e.

## Proshno-uttor

- **`www` keno nai?** Caddy `www` ke apex e `301` kore (3.2 Niyom 6), tai `shop_domains` e `www` lage na.
