# HTTP er bhitti: client IP, log, error

Design: 3.2, 7.1, 11.3, 12.2, 13.8 · Milestone: M0 (M1 e 404/405 er JSON ar rate limit) · Code: `backend/internal/httpx`

## Ki kore

Shob request er common kaj: request id, asol client IP, ek line log, panic e 500, admin er CORS, JSON ar 7.1 er error format. Business niyom na.

## Kivabe kaj kore

Router (`cmd/api/router.go`) bairer theke ei kram e mure:

1. `RequestID`: notun random id, `X-Request-Id` header ar context e.
2. `ClientIP`: loopback + thik `X-Internal-Auth` hole Jaspr er `X-Client-IP`; loopback hole Caddy er `X-Real-IP`; na hole connection er IP. Pore `IP(ctx)`.
3. `Log`: request shesh e ek line JSON: request_id, method, route (ServeMux er pattern), status, bytes, duration.
4. `Recover`: panic hole `500 internal`, details shudhu log e.
5. `CORS`: shudhu `ADMIN_ORIGINS`, credentials shoho.
6. `http.CrossOriginProtection` (router e): onno site er browser theke POST 403.

Handler `WriteJSON`, `WriteError` (7.1) ar `DecodeJSON` (1 MB, ochena field e error) use kore.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/httpx/httpx.go` → `Chain`, `RequestID`, `Recover`, `WriteJSON`, `WriteError`, `DecodeJSON` | Chain, id, panic, JSON |
| `backend/internal/httpx/clientip.go` → `ClientIP`, `IP`, `resolveIP`, `parseIP` | Client IP (3.2) |
| `backend/internal/httpx/log.go` → `Log`, `statusWriter` | Request log |
| `backend/internal/httpx/cors.go` → `CORS` | Admin er CORS |
| `backend/internal/httpx/httpx_test.go` | Test |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Client er boshano `X-Client-IP`/`X-Real-IP` mane na (3.2 Niyom 5) | `resolveIP` | `TestClientIP` |
| Log e path er token, query, IP na (12.2, 13.8) | `Log` (route pattern) | `TestLogRouteNotPath` |
| Panic e client e stack na (12.2 Server 4) | `Recover` | `TestRecover` |
| CORS shudhu admin origin, credentials shoho (11.3) | `CORS` | `TestCORS` |
| Ochena field, duita JSON, 1 MB er beshi → error (7.1) | `DecodeJSON` | `TestDecodeJSON` |
| Error format (7.1) | `WriteError` | `TestWriteError` |
| Middleware er kram | `Chain` | `TestChainOrder` |
| Client er pathano request id mane na | `RequestID` | `TestRequestID` |

## Data

Nai. `IP(ctx)` pore `orders.ip`, `sessions.ip`, rate limit e jabe (M4, M6).

## Bhul hole ki hoy

- `JASPR_INTERNAL_SECRET` website ar API te alada hole website er shob request `127.0.0.1` hoye rate limit e atke (14.2 test 18).
- ServeMux er nijer 404 ar 405 ekhono plain text; 7.1 er JSON e ana M1 er kaj.

## Bodlanor age mone rakho

- `statusWriter` er `Unwrap` muchbe na: upload er deadline `http.ResponseController` diye (2.2).
- Log e notun field dile personal data (phone, thikana, IP, token) na (12.2).
- Rate limit (7.8) ei package e ashbe (M1, M4); key `shop_id:ip`.

## Proshno-uttor

- **Log e keno route, path na?** Path e `public_token` ba `confirm_token` thake; log e gele token faash hoy.
- **Recover keno Log er bhetore?** Tahole panic er 500 o log e ashe.
