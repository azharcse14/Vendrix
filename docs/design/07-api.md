> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 7. API design

### 7.1 Convention

| Bishoy | Niyom |
|---|---|
| Base URL | `https://api.<domain>` , shob path `/api/...` diye shuru |
| Format | JSON, UTF-8. Field naam `snake_case` |
| Taka | Integer poisha (`"price": 18000` = ৳180) |
| Shomoy | ISO 8601 UTC (`"2026-10-27T08:30:00Z"`) |
| Shop | Public request e `X-Shop-Host: <domain>` header (Jaspr server customer er `Host` pathay, app pathay shop er primary domain). Staff request e session theke. Bistarito 3.2 |
| Vendor | Public list e optional `?vendor=<slug>` (marketplace). Admin e vendor staff er vendor session theke; shop staff `?vendor_id=` diye filter korte pare |
| Login | Customer: `Authorization: Bearer <token>`. Website e token HttpOnly cookie te, Jaspr server API te Bearer hishebe pathay; app (Phase 2) header e. Staff (admin web): api domain er HttpOnly cookie `__Host-asid` (11.3), Dart code token dekhe na |
| Guest cart | `X-Cart-Key: <random>` header (website e cookie theke) |
| Bhasha | `Accept-Language: bn` ba `en`. Public API resolve kora lekha ferot dey (fallback `bn`). Admin API puro `{"bn","en"}` object dey |
| Order create | `Idempotency-Key` header (max 64 okkhor) bad dile `400` |
| Paging | `?page=1&limit=24` (max 100). Response: `{"items": [...], "page": 1, "limit": 24, "total": 130}` |
| Body size | Default 1 MB, chobi upload 10 MB (`http.MaxBytesReader`, oi route e read deadline 2 minute; 2.2) |
| Unknown field | Nijer client er JSON e ochena field thakle `400` (`DisallowUnknownFields`). Webhook e na |
| Client IP | `X-Client-IP` + `X-Internal-Auth` shudhu Jaspr theke; baki shob request e Caddy er `X-Real-IP` (3.2 Niyom 5) |

Error er format shob jaygay ekoi:

```json
{
  "error": {
    "code": "out_of_stock",
    "message": "এই পণ্যটি এখন স্টকে নেই",
    "fields": { "phone": "invalid_phone" },
    "details": { "variant_id": 12 }
  }
}
```

`message` `Accept-Language` onujayi. Client `code` diye logic korbe, `message` shudhu dekhabe. v1.10: `httpx` e code → `{"bn","en"}` (Bangla okkhore ar English) map; ochena code ba bhasha e `bn`.

| HTTP | Kokhon | Code er udahoron |
|---|---|---|
| 400 | Input bhul | `validation_failed`, `invalid_phone`, `missing_idempotency_key`, `turnstile_failed`, `otp_invalid`, `otp_too_many_attempts` |
| 401 | Login nai ba token bhul | `unauthenticated`, `session_expired` |
| 403 | Onumoti nai | `forbidden`, `phone_blocked` |
| 404 | Pawa jayni | `not_found`, `shop_not_found` (v1.10: `closed` shop o) |
| 423 | Shop bondho | `shop_suspended` |
| 409 | Obostha mile na | `out_of_stock`, `coupon_invalid`, `coupon_limit_reached`, `otp_required`, `invalid_transition`, `area_not_served`, `below_min_order`, `vendor_unavailable`, `price_changed`, `payment_required`, v1.10: `slug_taken`, `category_not_empty`, v1.11: `sku_taken` |
| 422 | Ekoi `Idempotency-Key` e alada body | `idempotency_key_reused` |
| 429 | Beshi request | `rate_limited` (`Retry-After` header shoho) |
| 500 | Server er bhul | `internal` (details log e, client e na) |

### 7.2 Public ar system

| Method | Path | Kaj |
|---|---|---|
| GET | `/healthz` | Server ar database thik ache kina |
| GET | `/api/settings` | Published settings er public onsho. v1.10: whitelist (9.2): `branding`, `theme`, `locales`, `features`, `homepage`, `seo`, `integrations`, `checkout.payment_methods`, `checkout.return_window_days`, `marketplace.show_vendor_name`; baki jay na |
| GET | `/api/settings/version` | Shudhu version number (Jaspr cache check) |
| GET | `/api/settings/preview?token=` | Draft settings (signed preview token, 30 min) |
| GET | `/api/translations?locale=bn` | Admin override gulo (default file er upore) |
| GET | `/api/categories` | Category tree |
| GET | `/api/products` | List: `category`, `q`, `min_price`, `max_price`, `in_stock`, `vendor` (marketplace), `sort` (`newest`, `price_asc`, `price_desc`, `popular`), `page`, `limit`. v1.10: `q` thakle ar `sort` na dile relevance (6.4); search o 7.1 er moto paged; `popular` = `review_count DESC, rating_sum DESC` |
| GET | `/api/products/{slug}` | Product, variant, chobi, rating, attributes |
| GET | `/api/products/{slug}/reviews` | Review list (lukano na) |
| GET | `/api/vendors/{slug}` | Vendor er profile ar product (shudhu marketplace mode e; single e `404`) |
| GET | `/api/delivery-zones` | Checkout er district/area list |
| POST | `/api/delivery/quote` | `{district, area, subtotal}` → charge, deadline, same-day possible kina |
| POST | `/api/complaints` | Complaint form (Turnstile + rate limit) |

### 7.3 Customer login ar account

| Method | Path | Kaj |
|---|---|---|
| POST | `/api/auth/otp/send` | `{phone, purpose: "login", turnstile_token}` OTP pathano |
| POST | `/api/auth/otp/verify` | `{phone, code}` → `{token, user}`. Ek-i shathe guest cart merge (`X-Cart-Key`), oi phone er (`customer_phone`) guest order account e jog |
| POST | `/api/auth/logout` | Session revoke |
| GET, PATCH | `/api/me` | Profile (`name`, `email`, `locale`) |
| GET, POST | `/api/me/addresses` | Address list, notun |
| PATCH, DELETE | `/api/me/addresses/{id}` | Address bodlano, muche fela |
| GET | `/api/me/orders` | Nijer order list |
| POST | `/api/me/privacy-requests` | `{kind: "export" \| "delete"}` |

### 7.4 Cart ar wishlist

| Method | Path | Kaj |
|---|---|---|
| GET | `/api/cart` | Cart (login ba `X-Cart-Key`). Dam ar stock server theke fresh |
| POST | `/api/cart/items` | `{variant_id, quantity}` |
| PATCH | `/api/cart/items/{variant_id}` | `{quantity}` |
| DELETE | `/api/cart/items/{variant_id}` | Bad dewa |
| GET | `/api/wishlist` | Login kora customer er wishlist |
| POST, DELETE | `/api/wishlist/{product_id}` | Add/remove (`ON CONFLICT DO NOTHING`) |
| POST | `/api/wishlist/merge` | `{product_ids}` guest cookie theke account e |

### 7.5 Checkout ar order

| Method | Path | Kaj |
|---|---|---|
| POST | `/api/checkout/quote` | Cart + `{coupon_code, district, area, payment_method, phone}` → subtotal, discount, delivery charge, total, `otp_required`, shomossha gulo. Kichu save hoy na |
| POST | `/api/orders` | Order create (Section 8.6). Response `201 {order_number, public_token, status, payment_url?}`, `409 otp_required` ba `409 price_changed` |
| GET | `/api/orders/{token}` | SMS link theke order dekha (phone, thikana mask kora) |
| GET, POST | `/api/confirm/{confirm_token}` | COD confirm (8.10): GET e order er shar-songkhep (phone/thikana mask), POST e confirm. Token shudhu SMS e; v1.4 theke `public_token` diye confirm hoy na |
| POST | `/api/orders/{token}/cancel` | Customer cancel (order `pending`, ba `confirmed` ar shob fulfillment `open`) |
| POST | `/api/orders/{token}/pay` | Fail hoya online payment abar cheshta → `payment_url` |
| GET | `/api/orders/{token}/invoice` | Invoice page er data |
| POST | `/api/orders/track` | `{number, phone}` (phone URL ba log e jay na). Login chhara tracking. Shudhu status, tarikh, courier tracking. Rate limited |
| POST | `/api/reviews` | `{order_token \| login, product_id, rating, comment, reviewer_name}` |

`POST /api/orders` er body:

```json
{
  "customer": { "name": "Rahim", "phone": "01712345678", "email": null },
  "recipient": { "name": "Karim", "phone": "01812345678" },
  "address": { "district": "Dhaka", "area": "Mirpur", "line": "Road 10, House 5" },
  "note": "Bikel e dile bhalo",
  "coupon_code": "EID20",
  "payment_method": "cod",
  "expected_total": 42000,
  "consent": true,
  "otp_code": null,
  "turnstile_token": "....",
  "website": ""
}
```

`website` holo lukano honeypot field. Manush eta dekhe na, tai khali thake. Khali na thakle bot dhore `400` dibe, order hobe na. `expected_total` holo quote e dekhano total, v1.4 theke badhyotamulok (na thakle `400 validation_failed`, 8.6 step 1); server er hishab na mille `409 price_changed` (notun quote shoho), kintu dam hishabe kokhono use hoy na (Rule 1). `consent` chhara order hoy na (12.1). Email na dile SSLCommerz e shop er `branding.email` jay (8.9).

### 7.6 Payment ar webhook

| Method | Path | Kaj |
|---|---|---|
| POST | `/api/payments/sslcommerz/ipn` | SSLCommerz er IPN (form-encoded). Validation API diye check (Section 8.9) |
| GET, POST | `/api/payments/sslcommerz/return/{result}` | `success` e `val_id` diye IPN er ekoi validation function (server-to-server, idempotent), tarpor `303` website er `/order/{token}/done` e. `fail`/`cancel` shudhu redirect. Browser er pathano status kokhono bishshash na |
| POST | `/api/webhooks/{carrier}/{shop}` | Phase 2: courier status (signature/secret check). Shop URL theke (3.2). `DisallowUnknownFields` na |

### 7.7 Admin API (`/api/admin`, staff login lagbe)

v1.10: staff login M4 e; tar age admin route thake kintu fail-closed (staff context na thakle shob shomoy `401 unauthenticated`). Product ar category er `slug` admin dey (`^[a-z0-9]+(-[a-z0-9]+)*$`, max 80; PATCH e bodlano jay, redirect na; duplicate `409 slug_taken`). `DELETE /categories/{id}` shudhu khali hole (product ba child thakle `409 category_not_empty`; lukate `is_active = false`). Product create e `vendor_id` optional: shop staff na dile house vendor, vendor staff er session er vendor, onno vendor `404` (Rule 13). Variant create/PATCH e stock na (shuru 0; stock 8.12 er batch/adjust e, M7). v1.11: `sku` ek vendor e unique; duplicate `409 sku_taken`.

| Bhag | Endpoint |
|---|---|
| Login | `POST /auth/login {shop, email, password}` (`shop` = slug) → `{totp_required, temp_token}`, ba `{setup_required}` ar `setup_pending` session cookie (8.3); `POST /auth/totp {temp_token, code}` → session cookie; `POST /auth/recovery {temp_token, code}`; `POST /auth/password {old, new}`; `POST /auth/totp/setup` → `{otpauth_url}`; `POST /auth/totp/enable {code}` → `{recovery_codes}`, session `active`; `POST /auth/logout`; `GET /me` |
| Dashboard | `GET /dashboard` (ajker order, bikri, pending, low stock, expiry kache, deadline kache) |
| Product | `GET, POST /products`; `GET, PATCH /products/{id}`; `POST /products/{id}/variants`; `PATCH /variants/{id}`; `POST /products/{id}/images` (multipart); `PATCH /products/{id}/images/order`; `DELETE /images/{id}` |
| Category | `GET, POST /categories`; `PATCH, DELETE /categories/{id}` |
| Vendor | `GET, POST /vendors`; `GET, PATCH /vendors/{id}` (shop er owner/admin; single store e shudhu house vendor er profile) |
| Inventory | `GET /variants/{id}/batches`; `POST /batches`; `POST /inventory/adjust {variant_id, change, reason, note}`; `GET /inventory/movements?variant_id=`; `GET /inventory/alerts` |
| Order | `GET /orders?status=&q=&from=&to=&vendor_id=&page=` (`status` = `display_status`); `GET /orders/{id}` (fulfillment shoho); `POST /orders/{id}/transition {to, note}` (order: confirm, cancel); `PATCH /orders/{id}/items` (bad ba quantity kom; 8.11; v1.5: kono vendor er shob item bad ba refund hole owner/admin); `POST /orders/{id}/notes {kind, note}` |
| Fulfillment | `GET /fulfillments?status=&vendor_id=&date=`; `POST /fulfillments/{id}/pack {items: [{item_id, packed_weight_g}]}`; `POST /fulfillments/{id}/transition {to, note}` (ship, deliver, return, cancel; v1.7: `staff` role e cancel na, ar je return e refund hoy sheta na, 11.2) |
| Packing | `GET /packing/pick-list?date=`; `POST /print-links {kind, order_ids}` → website er `/print/{kind}?token=` (10 minute) |
| Delivery | `POST /fulfillments/{id}/shipments`; `PATCH /shipments/{id}`; `GET, POST /delivery-zones`; `PATCH /delivery-zones/{id}` |
| Taka | `POST /orders/{id}/refunds`; `PATCH /refunds/{id}`; `GET /cod-settlements/unsettled`; `GET, POST /cod-settlements`; `GET /payments?unsettled=true`; `PATCH /payments/{id}/settlement` |
| Coupon | `GET, POST /coupons`; `PATCH /coupons/{id}` |
| Review | `GET /reviews`; `PATCH /reviews/{id} {is_hidden}` |
| Customer | `GET /customers?q=`; `GET /customers/{id}`; `GET, POST /blocked-phones`; `DELETE /blocked-phones/{phone}` |
| Settings | `GET /settings` (draft + published + version); `PUT /settings/draft`; `POST /settings/preview-token`; `POST /settings/publish`; `GET /settings/history`; `POST /settings/rollback {version}` |
| Credential | `PUT /credentials/{provider}` (shudhu owner, write-only, kokhono ferot dey na); `GET /credentials` (shudhu kon gulo set ache). M8 er age CLI (3.8) |
| Translation | `GET /translations?locale=`; `PUT /translations/{locale}/{key}`; `DELETE /translations/{locale}/{key}` (default e ferot) |
| Upload | `POST /uploads` (logo, banner, certificate; JPEG/PNG/PDF, SVG na; 2.2) → `{storage_key, url}` |
| Staff | `GET, POST /staff`; `PATCH /staff/{id}` (deactivate ba role bodlale shob session revoke); `POST /staff/{id}/reset-2fa` (owner; vendor_owner nijer vendor er staff). Owner nijer TOTP harale recovery code, na hole CLI |
| Log ar support | `GET /audit-logs`; `GET, PATCH /complaints`; `GET, PATCH /privacy-requests` |
| Report | `GET /reports/sales?from=&to=&vendor_id=`; `GET /exports/orders.csv` |

Vendor staff (`vendor_owner`, `vendor_staff`) ekoi endpoint use kore, kintu backend session er `vendor_id` diye scope kore: onno vendor er product, fulfillment, stock `404`. Vendor shudhu nijer fulfillment er delivery er jonno dorkari customer tottho pay (recipient er naam, phone, thikana), payment ar onno vendor er item na.

### 7.8 Rate limit

| Endpoint | Shima |
|---|---|
| `POST /api/auth/otp/send` | Turnstile; ek phone e 15 minute e 3 bar ar dine 10 bar (shob purpose mile), ek client IP te ghontay 10 bar; shop e dine `checkout.sms_daily_cap` (default 500) er beshi OTP SMS na (v1.7: cap shudhu OTP gone; order, confirm, ship er SMS e cap nai, tai OTP er dhel order er SMS atkay na) |
| `POST /api/auth/otp/verify` | Ek OTP te 5 bar bhul (database e atomic `UPDATE`, error path eo commit), tarpor notun OTP. Ek phone e 24 ghontay 10 bar bhul hole 24 ghonta oi phone er OTP bondho ar alert (cap chhara dine prai 1,440 guess, mashe prai 4% chance) |
| `POST /api/orders` | Ek phone e ghontay 5 ta, ek IP te ghontay 10 ta |
| `POST /api/orders/track` | Ek client IP te minute e 10 bar |
| `POST /api/complaints` | Ek client IP te ghontay 5 ta |
| `POST /api/admin/auth/login` | Ek client IP te 15 minute e 10 bar bhul (memory). Account e 5 bar bhul er por protiti bhul e 1 minute `locked_until` (soft lock: keu owner er email diye barbar bhul dileo owner shudhu 1 minute opekkha kore), 10 ar 50 e owner alert. TOTP: ek temp_token e 5 bar, temp_token jure `staff.failed_mfa` 10 e 30 minute lock ar alert |
| Baki public | Ek IP te minute e 120 |

IP mane client IP (3.2 Niyom 5): website theke ashle Jaspr er `X-Client-IP` (shudhu loopback + secret), admin ar app er request e Caddy er `X-Real-IP` (Cloudflare range Caddy-i dekhe), na hole connection er IP. Na hole keu nijer header boshiye rate limit edate parbe, ba puro website, admin ar app ek IP hoye atke jabe.

Rate limit er key te shop o thake (`shop_id:phone`, `shop_id:ip`), jate multi-tenant e ek shop er bhir onno shop er customer ke atke na rakhe.

### 7.9 Platform API (Phase 3, multi-tenant)

`/api/platform/...`, alada login (`platform_admins`, TOTP badhyotamulok), shudhu platform admin er domain theke. Shop create (3.8), suspend, domain jog/verify, plan. Shop er data (order, customer) platform API theke dekha jay na; support er jonno lagle owner er onumoti niye audit log shoho.
