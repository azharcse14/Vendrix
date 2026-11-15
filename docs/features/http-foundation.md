# HTTP er bhitti: client IP, log, error

Design: 3.2, 7.1, 11.3, 12.2, 13.8 · Milestone: M0, M1, M4 · Code: `backend/internal/httpx`

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

Handler `WriteJSON`, `WriteError` (7.1) ar `DecodeJSON` (1 MB, ochena field e error) use kore. Customer route (`/api/me`, logout) e `ShopFromHost` er pore `RequireCustomer`: `Authorization: Bearer` token `LookupCustomer` (`auth.LookupSession`) diye oi shop er session e khoje, `CustomerFrom(ctx)` e `UserID`, `SessionID`. Cart route e `OptionalCustomer`: `Authorization` header na thakle guest (Customer nai), thakle `RequireCustomer` hubohu (bhul/expire `401`, guest e fall back na; 8.4 item 2). Admin route e `AdminSession`: cookie `__Host-asid` (`AdminCookieName`) `LookupStaff` (`auth.LookupStaffSession`) diye staff session e khoje (shop session theke, `X-Shop-Host` na; Bearer dekhe na), `WithStaff` e `Staff` (`SessionID`, `VendorID`, `Setup`); tarpor `RequireStaff`/`RequireRole` (`setup_pending` e `403 setup_required`).

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/httpx/httpx.go` → `Chain`, `RequestID`, `Recover`, `WriteJSON`, `WriteError`, `DecodeJSON` | Chain, id, panic, JSON |
| `backend/internal/httpx/clientip.go` → `ClientIP`, `IP`, `resolveIP`, `parseIP` | Client IP (3.2) |
| `backend/internal/httpx/log.go` → `Log`, `statusWriter` | Request log |
| `backend/internal/httpx/cors.go` → `CORS` | Admin er CORS |
| `backend/internal/httpx/page.go` → `JSONErrors`, `captureWriter`, `Page`, `NewPaged` | 404/405 JSON, paging |
| `backend/internal/httpx/message.go` → `messages`, `lang`, `message`, `Locale` | Error message er bhasha (`invalid_phone`, `turnstile_failed`, `phone_blocked`, `otp_invalid`, `otp_too_many_attempts`, `session_expired`, `invalid_credentials`, `mfa_invalid`, `mfa_too_many_attempts`, `setup_required`, `password_change_required`, `out_of_stock`, `cart_full`, `wishlist_full` shoho); `Locale` public lekha er bhasha (catalog) |
| `backend/internal/httpx/ratelimit.go` → `RateLimit`, `rateLimiter.allow`, `clientKey` | Public rate limit |
| `backend/internal/httpx/ratelimit.go` → `FailLimit`, `NewFailLimit`, `Take`, `Refund` | Staff login er client IP limit: shudhu bhul gone (Take age, bhul na hole Refund; 7.8) |
| `backend/internal/httpx/cookie.go` → `AdminCookie`, `AdminCookieName` | Admin session cookie `__Host-asid` (`HttpOnly; Secure; SameSite=Strict; Path=/`, Max-Age; 11.3) |
| `backend/internal/httpx/staff.go` → `Staff`, `WithStaff`, `StaffFrom`, `RequireStaff`, `RequireRole` | Admin er staff ar role, setup gate |
| `backend/internal/httpx/session.go` → `Customer`, `LookupCustomer`, `RequireCustomer`, `OptionalCustomer`, `CustomerFrom` | Customer route er Bearer session (lookup `auth` e); cart e optional |
| `backend/internal/httpx/session.go` → `LookupStaff`, `AdminSession` | Admin route er `__Host-asid` session (lookup `auth` e) |
| `backend/internal/httpx/httpx_test.go`, `page_test.go`, `message_test.go`, `staff_test.go`, `ratelimit_test.go` | Test |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Client er boshano `X-Client-IP`/`X-Real-IP` mane na (3.2 Niyom 5) | `resolveIP` | `TestClientIP` |
| Log e path er token, query, IP na (12.2, 13.8) | `Log` (route pattern) | `TestLogRouteNotPath` |
| Panic e client e stack na (12.2 Server 4) | `Recover` | `TestRecover` |
| CORS shudhu admin origin, credentials shoho (11.3) | `CORS` | `TestCORS` |
| Ochena field, duita JSON, 1 MB er beshi → error (7.1) | `DecodeJSON` | `TestDecodeJSON` |
| Error format (7.1) | `WriteError` | `TestWriteError` |
| `message` `Accept-Language` onujayi (`bn` default, ochena bhasha/code e-o faka na), caller er message thakle sheta (7.1, v1.10) | `WriteError`, `message`, `lang` | `TestErrorMessageLocale` |
| Middleware er kram | `Chain` | `TestChainOrder` |
| Ochena path `404 not_found`, bhul method `405 method_not_allowed` JSON e, `Allow` header shoho (7.1) | `JSONErrors` | `TestNotFoundJSON`, `TestMethodNotAllowedJSON`, `TestRouterBuild` |
| Admin route e staff na thakle `401 unauthenticated` (fail-closed, v1.10), `setup_pending` e `403 setup_required` (8.3 step 4), role na mille `403 forbidden` (11.2); vendor staff er `VendorID` context e (Rule 13) | `RequireStaff`, `RequireRole` | `TestRequireStaff`, `TestRequireRole` |
| Admin session: customer token (Bearer ba cookie) ar `mfa_pending` temp_token admin e `401`; `setup_pending` e shudhu setup route; staff cookie/token customer route e `401` (3.2 Niyom 3, Rule 16, 14.2 test 19) | `AdminSession`, `auth.LookupStaffSession`, `RequireCustomer` | `TestSessionRouteKind` |
| `__Host-asid` flag: `HttpOnly; Secure; SameSite=Strict; Path=/`, Domain na; Max-Age 43200/900, logout e 0 (11.3) | `AdminCookie` | `TestAdminCookie` |
| Customer route: Bearer na, bhul, onno shop er, staff er ba revoke kora token `401 unauthenticated`, expire `401 session_expired` (3.2, Rule 16) | `RequireCustomer`, `auth.LookupSession` | `TestCustomerSessionIntegration`, `TestSessionShopIsolationIntegration` |
| Cart route: Bearer na thakle guest; thakle `RequireCustomer` er path, bhul/expire `401` (8.4 item 2) | `OptionalCustomer` | `TestOptionalCustomer`, `TestCartKeyIntegration` |
| Public: ek shop e ek client IP te minute e 120, tarpor `429 rate_limited` + `Retry-After`; key `shop_id:ip`, IPv6 e /64 (address ghuriye edano na) (7.8) | `RateLimit`, `clientKey` | `TestRateLimit` |
| Paging `page`/`limit` (default 24, limit max 100, page max 10000: OFFSET overflow na), bhul e `400 validation_failed`; response `{"items","page","limit","total"}`, items `[]` (7.1) | `Page`, `NewPaged` | `TestPage` |
| Client er pathano request id mane na | `RequestID` | `TestRequestID` |

## Data

Nai. `IP(ctx)` `sessions.ip`, `audit_logs.ip`, Turnstile er `remoteip` ar rate limit e; `orders.ip` M6 e.

## Bhul hole ki hoy

- `JASPR_INTERNAL_SECRET` website ar API te alada hole website er shob request `127.0.0.1` hoye rate limit e atke (14.2 test 18).
- `page` ba `limit` bhul: `400 validation_failed`, `fields` e `page`/`limit`.
- Trailing slash ba path clean er redirect ServeMux er motoi thake (JSON na).

## Bodlanor age mone rakho

- `statusWriter` er `Unwrap` muchbe na: upload er deadline `http.ResponseController` diye (2.2).
- Log e notun field dile personal data (phone, thikana, IP, token) na (12.2).
- Public rate limit process er memory te (fixed window, window bodlale map notun); ek er beshi process hole protitir alada gona. Alada limit (7.8): OTP send route e `RateLimit(10, time.Hour)`, staff login `FailLimit` (shudhu bhul); order er M6 e.
- Staff context `AdminSession` boshay (test e `WithStaff`); notun admin route e `AdminSession` → `RequireRole` (ba `RequireStaff`), shudhu setup route e `RequireStaff` chhara. Admin handler `StaffFrom` er `ShopID` (ar `VendorID`) diye query filter kore, request body theke na (3.2).
- Notun error code: `message.go` er `messages` e bn ar en duitai (test dhore); `WriteError` e `r` lage, `Message` faka rakho.

## Proshno-uttor

- **Log e keno route, path na?** Path e `public_token` ba `confirm_token` thake; log e gele token faash hoy.
- **Recover keno Log er bhetore?** Tahole panic er 500 o log e ashe.
