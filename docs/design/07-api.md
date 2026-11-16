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
| Guest cart | `X-Cart-Key: <random>` header (website e cookie theke). v1.28: key Go banay (`rand.Text()`, `^[A-Z2-7]{26}$`), notun cart e response e `cart_key`; bhul format er key "nai" dhore (8.4) |
| Bhasha | `Accept-Language: bn` ba `en`. Public API resolve kora lekha ferot dey (fallback `bn`). Admin API puro `{"bn","en"}` object dey |
| Order create | `Idempotency-Key` header (max 64 okkhor) bad dile `400`. v1.32: format `^[A-Za-z0-9_-]{16,64}$`, na mille `400 missing_idempotency_key` (8.19 item 12) |
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
| 400 | Input bhul | `validation_failed`, `invalid_phone`, `missing_idempotency_key`, `turnstile_failed`, `otp_invalid`, `otp_too_many_attempts`, v1.26: `mfa_invalid` (TOTP ba recovery code bhul), `mfa_too_many_attempts` (ek temp_token e 5 bhul), v1.32: `cart_empty` (checkout e cart khali, 8.6 step 7) |
| 401 | Login nai ba token bhul | `unauthenticated`, `session_expired`, v1.26: `invalid_credentials` (staff login: ochena shop/email, inactive, bhul password, account lock; 8.3) |
| 403 | Onumoti nai | `forbidden`, `phone_blocked`, v1.26: `setup_required` (`setup_pending` session e admin API), `password_change_required` (password bodlanor age TOTP setup) |
| 405 | Path ache, method na | v1.26: `method_not_allowed` (`Allow` header shoho) |
| 404 | Pawa jayni | `not_found`, `shop_not_found` (v1.10: `closed` shop o) |
| 423 | Shop bondho | `shop_suspended` |
| 409 | Obostha mile na | `out_of_stock`, `coupon_invalid`, `coupon_limit_reached`, `otp_required`, `invalid_transition`, `area_not_served`, `below_min_order`, `vendor_unavailable`, `price_changed`, `payment_required`, v1.10: `slug_taken`, `category_not_empty`, v1.11: `sku_taken`, v1.28: `cart_full` (cart e 50 ta alada variant), v1.29: `wishlist_full` (wishlist e 200), `out_of_stock` e cart er `details {variant_id, max_quantity}` (8.4) |
| 422 | Ekoi `Idempotency-Key` e alada body | `idempotency_key_reused` |
| 429 | Beshi request | `rate_limited` (`Retry-After` header shoho) |
| 500 | Server er bhul | `internal` (details log e, client e na) |
| 503 | Database pawa jay na | v1.26: `db_unavailable` (`/healthz` chhara) |

### 7.2 Public ar system

| Method | Path | Kaj |
|---|---|---|
| GET | `/healthz` | Server ar database thik ache kina |
| GET | `/api/settings` | Published settings er public onsho. v1.10: whitelist (9.2): `branding`, `theme`, `locales`, `features`, `homepage`, `seo`, `integrations`, `checkout.payment_methods`, `checkout.return_window_days`, `marketplace.show_vendor_name`; baki jay na |
| GET | `/api/settings/version` | Shudhu version number (Jaspr cache check) |
| GET | `/api/settings/preview?token=` | Draft settings (signed preview token, 30 min) |
| GET | `/api/policies/{kind}` | v1.13: shop er policy lekha (`terms`, `return`, `refund`, `delivery`, `privacy`), published settings er `policies` block theke `{"bn","en"}` resolve kore (ochena kind `404`) |
| GET | `/api/translations?locale=bn` | Admin override gulo (default file er upore) |
| GET | `/api/categories` | Category tree |
| GET | `/api/products` | List: `category`, `q`, `min_price`, `max_price`, `in_stock`, `vendor` (marketplace), `sort` (`newest`, `price_asc`, `price_desc`, `popular`), `page`, `limit`. v1.10: `q` thakle ar `sort` na dile relevance (6.4); search o 7.1 er moto paged; `popular` = `review_count DESC, rating_sum DESC` |
| GET | `/api/products/{slug}` | Product, variant, chobi, rating, attributes |
| GET | `/api/products/{slug}/reviews` | Review list (lukano na) |
| GET | `/api/vendors/{slug}` | Vendor er profile ar product (shudhu marketplace mode e; single e `404`) |
| GET | `/api/delivery-zones` | Checkout er district/area list. v1.32: `[{district, areas: [{area, name}], district_wide: bool}]`, shudhu `is_active`; `district_wide` = district e `area NULL` zone ache (form e "Onno area" + free text, 8.19 item 2). v1.35: area e `days`, district e `district_wide_days` (`district_wide` na hole `null`): delivery er din (8.15 er niyom server e, client jane na), checkout e order er age dekhano (10.6) |
| POST | `/api/delivery/quote` | `{district, area, subtotal}` → charge, deadline, same-day possible kina. v1.32: M6 e na (checkout quote-i jothesto); Phase 2 te product page er delivery estimate |
| POST | `/api/complaints` | Complaint form (Turnstile + rate limit) |

### 7.3 Customer login ar account

v1.19: M4 e `otp/*`, `logout`, `GET`/`PATCH /api/me`; `/api/me/addresses` ar `orders` M6 e, `privacy-requests` M11 e. v1.32 (user 15 November 2026): M6 e shudhu `/api/me/orders`; `/api/me/addresses` M11 e (account privacy er sathe, 8.19 item 11).

| Method | Path | Kaj |
|---|---|---|
| POST | `/api/auth/otp/send` | `{phone, purpose: "login", turnstile_token}` OTP pathano. v1.19: response shob shomoy `{"sent": true}` (`429` chhara) |
| POST | `/api/auth/otp/verify` | `{phone, code}` → `{token, user}`. Ek-i shathe guest cart merge (`X-Cart-Key`), oi phone er (`customer_phone`) guest order account e jog. v1.30: response e `cart_notices` (shob shomoy array, khali hole `[]`): merge e 50 variant er jayga na thakay bad guest item `[{code: "item_dropped", name}]` (naam request er bhashay, 8.4 item 6) |
| POST | `/api/auth/logout` | Session revoke, `204` |
| GET, PATCH | `/api/me` | Profile `{phone, name, email, locale}` (v1.26: `id` nai). PATCH e shudhu deya field bodlay: `name` trim, max 100 okkhor (rune); `email` trim, `net/mail` e ekta thikana (naam ba `<>` chhara), max 254; `""` dile `NULL`; `locale` `bn` ba `en`. Bhul hole `400 validation_failed` + `fields` (`too_long`, `invalid`) |
| GET, POST | `/api/me/addresses` | Address list, notun (v1.32: M11) |
| PATCH, DELETE | `/api/me/addresses/{id}` | Address bodlano, muche fela |
| GET | `/api/me/orders` | Nijer order list (Rule 16: `user_id` filter). v1.32: paged (7.1), card `{order_number, public_token, display_status, total, placed_at}` |
| POST | `/api/me/privacy-requests` | `{kind: "export" \| "delete"}` |

### 7.4 Cart ar wishlist

| Method | Path | Kaj |
|---|---|---|
| GET | `/api/cart` | Cart (login ba `X-Cart-Key`). Dam ar stock server theke fresh |
| GET | `/api/cart/count` | v1.31: badge er jonno shudhu `{count}` (normalize na) |
| POST | `/api/cart/items` | `{variant_id, quantity}` |
| PATCH | `/api/cart/items/{variant_id}` | `{quantity}` |
| DELETE | `/api/cart/items/{variant_id}` | Bad dewa |
| GET | `/api/wishlist` | Login kora customer er wishlist |
| POST, DELETE | `/api/wishlist/{product_id}` | Add/remove (`ON CONFLICT DO NOTHING`) |
| POST | `/api/wishlist/merge` | `{product_ids}` guest cookie theke account e |

v1.28 (M5): Cart route e Bearer thakle account cart (bhul/expire `401`), na hole `X-Cart-Key` er guest cart (8.4). `GET`, `POST`, `PATCH`, `DELETE` shob puro cart ferot dey: `{items: [{variant_id, product_slug, name, variant_label, image, unit_price, compare_at_price, variable_weight, quantity, max_quantity, in_stock, line_total}], count, subtotal, notices}` (notun guest cart e `cart_key` o); `count` = alada line er shonkhya (qty jog na); `Cache-Control: no-store`. Body te dam/total er field nai (`DisallowUnknownFields` e `400`). `DELETE` e item na thakleo `200`. Wishlist er shob route login chay (`401`), protiti query `user_id` diye (Rule 16); `GET /api/wishlist` = product card er list (catalog er card er moto), bondho product lukano, max 200; merge e max 50 ID, mot 200 (8.4 item 8). v1.29: 200 thakle notun product `409 wishlist_full`.

v1.31 (M5 review, user 15 November 2026): `GET /api/cart/count` → `{count}` (alada line): owner `GET /api/cart` er moto (Bearer, bhul/expire `401`; na hole `X-Cart-Key`), `shop_id` diye, kintu normalize na: row banay na, `DELETE`/`UPDATE` na, notice na; tai bondho item o gona, porer `GET /api/cart` muche. `Cache-Control: no-store`. Notice (`item_removed`, `quantity_reduced`) shudhu `GET /api/cart` e; website e shudhu `/cart` page eta dake (10.12 item 5), jate onno page er badge notice gile na fele.

v1.31 spashto niyom (M5 er code, review e lekha):

- `POST /api/cart/items` ekoi variant e qty **jog** kore (jog o 1..`min(stock, 50)`); `PATCH` qty **set** kore; cart e na thaka variant e `PATCH` `404 not_found` (`DELETE` `200`). Jog er age `quantity` 51 e cap, tai boro shonkhya overflow hoye `400` na, `409 out_of_stock`.
- `variant_label` = variant er `options` er value, key er alphabet kram e, request er bhashay, `", "` diye; option na thakle `""`.
- Ojon-variable variant e `nominal_weight_g` NULL hole item `in_stock: false`, `line_total` 0 (dam hishab hoy na).
- Stock 0 line er `line_total` hishab kora dam (`pricing.LineTotal`), kintu `in_stock: false`, tai subtotal e na.
- `409 out_of_stock` er `details.max_quantity` (`min(stock, 50)`) public catalog er `in_stock` er cheye beshi bole (stock 50 er niche exact): mene neya (12.2 Data 7).
- Wishlist: `POST`, `DELETE` ar merge o `GET` er puro card list ferot dey. "Active" product = `is_active`, kom-poksha ekta active variant, vendor `active`. Add e onno shop er, bondho ba ochena product `404 not_found`; age theke thaka product e `200` (bondho holeo). 200 er limit e `GET` e lukano (bondho) row o gona. Merge e prothom 50 ID nay, baki chupchap bad; `product_ids` na thakle ba `null` `400 validation_failed`, khali list `200`.

### 7.5 Checkout ar order

| Method | Path | Kaj |
|---|---|---|
| POST | `/api/checkout/quote` | Cart + `{coupon_code, district, area, payment_method, phone}` → subtotal, discount, delivery charge, total, `otp_required`, shomossha gulo. Kichu save hoy na. v1.32: response `{subtotal, discount, delivery_charge, total, otp_required, problems}`; coupon na khatle coupon chhara total ar `problems` e `coupon_invalid`/`coupon_limit_reached` (read-only, `used_count` bodlay na, 8.19 item 5). v1.34: guest er `otp_required` history chhara (shudhu `otp_mode` ar `cod_high_amount`; `risky` er baki flag order e `409 otp_required`), coupon er `per_phone_limit` quote e na (order e dhore); client IP te minute e 30 (8.19 item 13 b) |
| POST | `/api/orders` | Order create (Section 8.6). Response `201 {order_number, public_token, status, payment_url?}`, `409 otp_required` ba `409 price_changed`. v1.32: `otp_required` er `details {phone_masked}`, `price_changed` er `details {total, subtotal, discount, delivery_charge}`, `out_of_stock` er `details {variant_id}`; rate limit `429` (7.8); ekoi key er ager order `200` + ager response (8.19) |
| GET | `/api/orders/{token}` | SMS link theke order dekha (phone, thikana mask kora). v1.32: order number, `display_status`, tarikh, item (naam, variant, qty, line total), taka, payment method, `delivery_deadline`, customer/recipient naam, phone `+88017****5678`, thikana shudhu district + area, `can_cancel` (8.19 item 6) |
| GET, POST | `/api/confirm/{confirm_token}` | COD confirm (8.10): GET e order er shar-songkhep (phone/thikana mask), POST e confirm. Token shudhu SMS e; v1.4 theke `public_token` diye confirm hoy na |
| POST | `/api/orders/{token}/cancel` | Customer cancel (order `pending`, ba `confirmed` ar shob fulfillment `open`). v1.32: body `{reason?}` (max 200) → `orders.cancel_reason` |
| POST | `/api/orders/{token}/pay` | Fail hoya online payment abar cheshta → `payment_url` |
| GET | `/api/orders/{token}/invoice` | Invoice page er data |
| POST | `/api/orders/track` | `{number, phone}` (phone URL ba log e jay na). Login chhara tracking. Shudhu status, tarikh, courier tracking. Rate limited. v1.32: phone `customer_phone` ba `recipient_phone` mille `{order_number, display_status, placed_at, history: [{status, at}], tracking: null}` (courier M7), token na; na mille `404 not_found` (8.19 item 6) |
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

v1.10: staff login M4 e; tar age admin route thake kintu fail-closed (staff context na thakle shob shomoy `401 unauthenticated`). Product ar category er `slug` admin dey (`^[a-z0-9]+(-[a-z0-9]+)*$`, max 80; PATCH e bodlano jay, redirect na; duplicate `409 slug_taken`). `DELETE /categories/{id}` shudhu khali hole (product ba child thakle `409 category_not_empty`; lukate `is_active = false`). Product create e `vendor_id` optional: shop staff na dile house vendor, vendor staff er session er vendor, onno vendor `404` (Rule 13). Variant create/PATCH e stock na (shuru 0; stock 8.12 er batch/adjust e, M7). v1.11: `sku` ek vendor e unique; duplicate `409 sku_taken`. v1.19: M4 e Login bhag ar `shop-api staff reset-2fa` CLI (8.3); `/staff` M7 e.

| Bhag | Endpoint |
|---|---|
| Login | `POST /auth/login {shop, email, password}` (`shop` = slug) → `{totp_required, temp_token}`, ba `{setup_required}` ar `setup_pending` session cookie (8.3); `POST /auth/totp {temp_token, code}` → session cookie; `POST /auth/recovery {temp_token, code}`; `POST /auth/password {old, new}` → `204` (v1.25: TOTP live ar `must_change_password` hole session `active`, notun cookie; v1.26: bhul old `400 validation_failed` + `fields.old: wrong`, `fields.new`: `too_short` (12 er kom), `too_long` (128 er beshi), `same_as_old`); `POST /auth/totp/setup` → `{otpauth_url, secret}` (v1.19; v1.25: pending secret, live na); `POST /auth/totp/enable {code}` → `{recovery_codes}`, pending → live, session `active` (v1.25: live TOTP thakle setup/enable `403`; v1.26: `active` session e setup/enable `403 forbidden`, password bodlanor age `403 password_change_required`, bhul code `400 mfa_invalid`); `/auth/totp`, `/recovery` bhul code `400 mfa_invalid`, temp_token e 5 bhul `400 mfa_too_many_attempts`; `POST /auth/logout` → `204`; `GET /me` |
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
| `POST /api/auth/otp/send` | Turnstile; ek phone e 15 minute e 3 bar ar dine 10 bar (shob purpose mile), ek client IP te ghontay 10 bar (v1.34: checkout er OTP send (8.6 step 8) o ei gonay, ek limiter, 8.19 item 13 a); shop e dine `checkout.sms_daily_cap` (default 500) er beshi OTP SMS na (v1.7: cap shudhu OTP gone; order, confirm, ship er SMS e cap nai, tai OTP er dhel order er SMS atkay na). v1.26: cap approximate: lock phone er, shop er na, tai parallel e alada phone er send 1–2 ta par hote pare (khoroch er shima, nirapotta na). Cap shesh hole oi diner baki OTP login bondho (bot onek phone diye Turnstile par korle DoS); launch er age (M12) SMS gona monitor ar alert, 16.1 |
| `POST /api/auth/otp/verify` | Ek OTP te 5 bar bhul (database e atomic `UPDATE`, error path eo commit), tarpor notun OTP. Ek phone e 24 ghontay 10 bar bhul hole 24 ghonta oi phone er OTP bondho ar alert (cap chhara dine prai 1,440 guess, mashe prai 4% chance). v1.19: lock er alada table na: shesh 24 ghontay oi phone er `otp_codes` er `sum(attempts)` theke consumed row gulo bad (consumed row er shesh attempt thik) = bhul; 10 ba beshi hole send ar verify `429 rate_limited` + `Retry-After` (lock ache bole na), rolling window (`otp_phone_idx`, migration na). Alert ekbar (`dedupe_key` `alert.otp_lock:<shop>:<phone>:<din>`; v1.26: `<phone>` er jaygay `HMAC-SHA256(OTP_SECRET, "otp_lock:" + phone)` er prothom 16 hex, karon `jobs` row 30 din thake, plain phone na) |
| `POST /api/orders` | Ek phone e ghontay 5 ta, ek IP te ghontay 10 ta. v1.32: shudhu hoya order gona, DB theke (`orders` e shesh 60 minute er `customer_phone` ar `ip`; 8.6 step 4 er por, transaction er age), `429 rate_limited` + `Retry-After`; OTP/`price_changed` er retry gone na (8.19 item 9). v1.34: IPv6 e `/64` ek IP (8.19 item 13 c) |
| `POST /api/checkout/quote` | v1.34: ek client IP te minute e 30 bar (8.19 item 13 b) |
| `POST /api/orders/track` | Ek client IP te minute e 10 bar |
| `POST /api/complaints` | Ek client IP te ghontay 5 ta |
| `POST /api/admin/auth/login` | Ek client IP te 15 minute e 10 bar bhul (memory; v1.26: key shudhu IP, shop na: admin domain e request er shop nai, shop body te, tai ek IP theke onek shop e cheshta o ek gona). Account e 5 bar bhul er por protiti bhul e (v1.26: 5 tomo bhul thekei) 1 minute `locked_until` (soft lock: keu owner er email diye barbar bhul dileo owner shudhu 1 minute opekkha kore), 10 ar 50 e owner alert. TOTP: ek temp_token e 5 bar, temp_token jure `staff.failed_mfa` 10 e 30 minute lock ar alert. v1.24: duita lock-i check er age atomic reservation (8.3); account lock e login `401 invalid_credentials` (`429` na, `Retry-After` na; email chena na jay), IP limit e `429`; `failed_mfa` lock e `/auth/totp`, `/auth/recovery` `429` |
| `POST /api/admin/auth/totp`, `/recovery` | v1.24: ek client IP te minute e 120 (baki public er moto) |
| Baki public | Ek IP te minute e 120. v1.28: cart ar wishlist o; guest cart toiri te alada limit na (cart e max 50 variant, 8.4) |

IP mane client IP (3.2 Niyom 5): website theke ashle Jaspr er `X-Client-IP` (shudhu loopback + secret), admin ar app er request e Caddy er `X-Real-IP` (Cloudflare range Caddy-i dekhe), na hole connection er IP. Na hole keu nijer header boshiye rate limit edate parbe, ba puro website, admin ar app ek IP hoye atke jabe.

Rate limit er key te shop o thake (`shop_id:phone`, `shop_id:ip`), jate multi-tenant e ek shop er bhir onno shop er customer ke atke na rakhe.

### 7.9 Platform API (Phase 3, multi-tenant)

`/api/platform/...`, alada login (`platform_admins`, TOTP badhyotamulok), shudhu platform admin er domain theke. Shop create (3.8), suspend, domain jog/verify, plan. Shop er data (order, customer) platform API theke dekha jay na; support er jonno lagle owner er onumoti niye audit log shoho.
