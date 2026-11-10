> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 1. Project overview ar decision

### 1.1 Lokkho

Bangladesh er customer der jonno ekta online dokan, jeta prothome organic food (chal, dal, tel, modhu, gur, mousumi fol) bikri korbe. Platform ta emon bhabe banano hobe jate dokaner naam, logo, rong, font, lekha, bhasha, feature, shob admin panel theke bodlano jay, ar ekoi code diye onno dhoroner business o chalano jay.

### 1.2 Launch strategy

MVP holo ekta **limited launch**: invite kora 50 theke 200 jon customer, ekta shohor ba area, 20 theke 100 ta product. MVP **website-first**: customer ra Jaspr diye banano website use korbe. Flutter customer app Phase 2 te ashbe. Admin panel MVP thekei Flutter web e.

| Phase | Ki | Ke use korbe |
|---|---|---|
| 0. Prostuti | Legal kagoj, design, project setup | Shudhu team |
| 1. MVP (limited launch) | Website, admin, COD + SSLCommerz, wishlist, review, coupon, guest checkout | Invite kora customer, ekta area |
| 2. Public launch | Flutter app, courier automation, bKash/Nagad, PWA, search upgrade, marketing tools | Shobai |
| 3. Growth | AI shopping assistant, recommendation, subscription basket, flash sale, marketplace (multi-vendor), multi-tenant platform | Business boro hole |

### 1.3 Mul decision

| Bishoy | Decision | Karon |
|---|---|---|
| Backend | Go, shudhu `net/http` + standard library, ekta matro dependency `pgx` (Postgres driver) | Shohoj, stable, kom dependency |
| Database | PostgreSQL | Transaction, JSONB, full-text/trigram search |
| Customer website | Jaspr, server mode (SSR) + Tailwind | Google SEO, Facebook link preview, Dart code share |
| Admin panel | Flutter web | Dart, druto UI banano |
| Customer app | Flutter (Phase 2) | Ekoi shared Dart code |
| Flutter state, routing, DI | `bloc`, `go_router`, `get_it` | Team er pochhondo |
| Login | Phone + OTP. Guest checkout (login chhara order) allowed | Bangladesh e phone-i identity, checkout e kom badha |
| Payment | MVP: Cash on Delivery + SSLCommerz. Phase 2: bKash/Nagad direct | Shuru te shohoj |
| Courier | MVP: manual booking. Phase 2: Steadfast, Pathao, RedX API | Shuru te kom kaj |
| Taka | Shob jaygay integer **poisha** (`BIGINT`) | Float e hishab bhul hoy |
| Multi-tenant ar marketplace | Protiti table e `shop_id` (tenant), catalog ar order e `vendor_id` (seller), pack-ship er unit `order_fulfillments`. MVP te ekta shop, ekta house vendor | Ekoi code e single store, marketplace, multi-tenant (1.6) |
| Design | Theme token (CSS variable), admin theke edit | Ek code, onek brand |
| Bhasha | Bangla default, English optional, Bangla fallback | Bangladesh er customer |

### 1.4 Je niyom gulo kokhono bhangbe na

Ei niyom gulo shob code e, shob shomoy mante hobe. AI agent ke eta bishesh kore mone koriye dao.

1. **Business niyom shudhu Go backend e.** Dam, discount, delivery charge, stock, coupon, total, egulo shudhu backend hishab korbe. Website, app ba admin kokhono final total hishab kore pathabe na. Client er pathano dam kokhono bishshash korbe na.
2. **Taka = `BIGINT` poisha.** ৳150.50 = `15050`. Kokhono `float` ba `NUMERIC` e taka rakhbe na. Percent discount er rounding: niche (floor).
3. **`shop_id` shob jaygay.** Protiti business table e, child table eo, `shop_id` thakbe. Parent table e `UNIQUE (shop_id, id)`, child er foreign key `(shop_id, parent_id)`, jate database nijei ek shop er row ke onno shop er row point korte na dey. Order er child (history, refund, payment, batch) o `(shop_id, order_id, ...)` diye oi order er-i. Protiti query `shop_id` diye filter korbe. Ek shop er data kokhono onno shop e dekhabe na.
4. **UI te hardcode nai.** Kono component e hex color ba user-facing lekha shorasori likhbe na. Rong ashbe theme token theke, lekha ashbe translation key theke.
5. **Shomoy UTC te save, shop er timezone e dekhano** (`shops.timezone`, default Asia/Dhaka). Database e `TIMESTAMPTZ`. "Ajker", cutoff, pick list er din o shop er timezone e.
6. **Phone number E.164 format e:** `+8801XXXXXXXXX`. Input e je format-i ashuk, save er age normalize.
7. **Payment status client theke na.** Payment "paid" hobe shudhu gateway er server-to-server validation er por. Online payment er order paid na hole `confirmed` hote pare na (state machine ar DB CHECK duitai atkay).
8. **Order e snapshot.** Order er moddhe product er naam, dam, kena dam, customer er naam, phone, thikana copy kore rakho. Pore product ba address bodlaleo purono order bodlabe na. v1.4: coupon er shorto (`orders.coupon_*`) ar vendor er `commission_bps` o copy; packing ba edit er hishab egulo diyei hoy, `coupons`/`vendors` table theke na (8.5). v1.5: item ojon-variable kina (`order_items.variable_weight`) o snapshot, `products` theke na; `order_items` theke hishab er line shudhu `pricing.LineFromItem` diye (8.5).
9. **Order create idempotent.** Ek-i `Idempotency-Key` e duita order hobe na, duita request ek shathe ashleo (advisory lock, 8.6). Ekoi key e alada body hole `422`.
10. **Shared Dart package pure Dart.** `packages/shared` e Flutter import, `dart:io`, `dart:html` nishiddho.
11. **Public URL e serial ID na.** Order dekhano link e random `public_token`, order number ba database ID na. v1.4: COD confirm er link e `public_token` na, alada `confirm_token` (8.10), karon `public_token` order-dewa browser o pay.
12. **SQL shudhu parameter diye.** String jora diye kokhono SQL banabe na.
13. **Vendor dhore code likho, "ekta-i vendor" dhore na.** Product, variant, order item, fulfillment, shipment e `vendor_id` ache. Single store e shop er ekta house vendor (`vendors.is_house`). Code kokhono "shop = vendor" dhore likhbe na. Vendor staff er protiti query `shop_id` ar `vendor_id` duita diyei filter.
14. **Pack ar ship hoy fulfillment e, order e na.** Order holo customer er kena ar payment er unit. Fulfillment holo ek vendor er pack-ship er unit (`order_fulfillments`). Single store e protiti order e ekta fulfillment. Packing, shipment, delivery er code shob fulfillment dhore.
15. **Shop er kichu hardcode ba env e na.** Domain, order prefix, timezone, credential, theme, bhasha: shob `shops`, `shop_domains`, `shop_settings`, `shop_credentials` theke. Env e shudhu platform er jinish (database, secret key, admin origin, R2).
16. **Session er dhoron route er sathe mile.** Customer er token shudhu customer route e, staff er token shudhu admin route e (`sessions.user_id` / `sessions.staff_id`). Customer er address, order, wishlist er protiti query te `user_id` filter (IDOR atkay).

### 1.5 Decision log (alochonar shar-songkhep)

| Kram | Ki thik holo |
|---|---|
| 1 | Stack: Go backend, Flutter (Dart). Postgres database. |
| 2 | MVP te review, coupon, wishlist thakbe. |
| 3 | Plan e shob feature thakbe, kintu MVP limited launch. |
| 4 | Customer website lagbe. Flutter web SEO ar Facebook preview er jonno bhalo na. Go `html/template` + htmx o bhabcha hoyechilo. |
| 5 | Jaspr beche neya holo: SSR, Flutter er moto component, Dart code share. Performance tulona: Go + htmx server e halka, kintu ei scale e duitai jothesto; Dart reuse er jonno Jaspr. |
| 6 | Website-first MVP. Flutter customer app Phase 2. |
| 7 | Shuru organic food, kintu multi-business ready. Theme, naam, rong admin theke bodlabe. Localization (Bangla/English) thakbe. |
| 8 | Mobile-first responsive design. |
| 9 | Guest checkout (login chhara order). Fake COD order thekate risk-based OTP ar order confirmation. |
| 10 | Legal ar operation er gap jog holo: BFSA registration, data protection, batch/expiry, ojon onujayi dam, COD hishab, cost price. |
| 11 | Go te shudhu `net/http` + standard library (+ pgx driver). Flutter e `bloc`, `go_router`, `get_it`. |
| 12 | Shuru single store, kintu design marketplace ar multi-tenant ready: `vendors`, `order_fulfillments`, tenant-safe foreign key, pricing e vendor onujayi bhag `001` thekei. Vendor er taka-poysa ar platform er table pore migration e (1.6). |
| 13 | v1.2 review er fix: Jaspr theke client IP forward ar BFF endpoint (10.9), payment dekhe confirm, shob fulfillment cancel e order cancel, refund shudhu fulfillment e, IPN reconcile ar duplicate payment, OTP/TOTP attempt, order er child e tenant-safe FK, `phone_bd`, account anonymize, DB role, edit e coupon onupate, legal update (PDPA 2026, BFSA 2026, DCOG 2021). |
| 14 | v1.3 review er fix: order cancel er niche-r cascade (`CancelCascade`), client IP Caddy te (`trusted_proxies`, `X-Real-IP`), TOTP `failed_mfa` ar `setup_pending`, OTP er dine cap, Turnstile skip hole OTP shob shomoy verify, idempotency hash e token na, `audit_logs.ip` grant, Go 1.27 toolchain, job sweeper, payment lock er kram, pricing input validation, Bangla NFC search, Bot Fight Mode, complaint 72 ghonta, phone input, docsync test. Legal abar milano (BFSA, PDPA, DCOG, VAT). |
| 15 | v1.4 review er fix: idempotency hash e server er cart na ar `expected_total` badhyotamulok (8.6), COD confirm er alada `confirm_token` (8.10), `Reprice` e je vendor er line bodlay nai tar taka freeze (8.5), coupon er shorto ar `commission_bps` order e snapshot (Rule 8), reconcile e local `cancelled` payment o (8.9), state machine er golden-table ar pricing er boundary test (14.2). |
| 16 | v1.5 review er fix: edit e vendor er shob item bad hole refund ekbar-i ar `staff` role e edit diye cancel/refund na (8.8, 8.11, 11.2); `order_items.variable_weight` snapshot, `pricing.LineFromItem`/`EditWeightG`, removed item e taka 0, `Reprice` e Original er jog-biyog check ar fixed line e `WeightG` 0 (8.5); coupon chhara discount na ar coupon snapshot er check (6.7); TOTP key (`TOTP_KEY`, `totp_key_version`, AAD) ar `RECOVERY_SECRET` (8.3, 13.5); systemd restart, API shudhu loopback e, backup bucket lock (13); token page e third-party script na, CSP hash, banner link parse, `intl` currency (9, 10, 12); BFSA (1 November 2026) ar Jaspr `^0.23.5` abar milano; mutation test er fak bondho, DB test e constraint er naam (14). |
| 17 | v1.6: design section onujayi `docs/design/` e (Claude Code er Read er shima), code er copy bad (file-i asol), docsync er kaj bodlalo; migration runner: `tests/` subfolder, `embed.go`, `pg_advisory_xact_lock`, `set_config`; Flutter card e `surface` token; Claude Code setup (`.claude/`, `Makefile`, `scripts/`); review er 6 ta open decision 16.1 e |
| 18 | v1.7 review er fix: payment validation e response er `tran_id`, VALID na hole kichu bodlay na (8.9); `CONFIRM_SECRET` rotation er dabi thik (8.10); checkout OTP order er transaction e consume (8.6); risk e `expected_total` (8.6); `pricing.ErrCouponNotApplicable` (8.5); `staff` role e refund wala fulfillment transition na (11.2); migration checksum ar `DEPLOYED` (6.1, 13.6); `001` er role check fail-closed; review er 2 ta open decision 16.1 e |
| 19 | v1.8: Vendrix repo; Go module `vendrix/backend`; feature doc (`docs/features/`) ar tracking (`docs/STATUS.md`) (4.7); Claude Code er puro kram: hook, skill, agent, rule; Dart er jora: path dependency, website alada (4.6); M0 er shuru code, CI ar infra file |

### 1.6 Tin ta mode: single store, marketplace, multi-tenant

Ekoi code tin bhabe chole. Parthokko shudhu data te (koyta shop, protiti shop e koyta vendor), code e `if single` er moto branch na.

| Mode | Shop (tenant) | Vendor (seller) | Udahoron | Kokhon |
|---|---|---|---|---|
| Single store | 1 | 1 (house vendor) | Sobuj Bazar nijer maal bikri kore | MVP |
| Marketplace | 1 | Onek | Sobuj Bazar e onek krishok ar dokan nijer maal bikri kore, ek cart e | Phase 3 |
| Multi-tenant | Onek | Protiti shop e 1 (ba marketplace hole onek) | Ek platform e onek alada dokan, protitir nijer domain, theme, customer, payment | Phase 3 |

| Shobdo | Mane | Table |
|---|---|---|
| Platform | Puro deployment er malik. Shop khole, bondho kore | Pore: `platform_admins` |
| Shop (tenant) | Ekta website/brand: domain, theme, bhasha, customer, payment gateway, category tree | `shops` |
| Vendor (seller) | Shop er bhetore je maal bikri ar pack kore. Single store e shop nijei (`is_house`) | `vendors` |
| Order | Customer er ek bar er kena ar payment | `orders` |
| Fulfillment | Ek order e ek vendor er ongsho: pack, ship, deliver | `order_fulfillments` |

**Ekhon (`001`) ki, pore ki.** Niyom: je jinish na thakle MVP er code pore abar likhte hoy, sheta `001` e. Je jinish shudhu notun table ba feature jog kore, sheta pore migration e (6.6).

| Ekhon `001` e (MVP code ei bhabe likhbe) | Pore migration e (Phase 3) |
|---|---|
| `vendors`, protiti shop e ekta house vendor | Vendor KYC kagoj, payout account (`vendor_documents`, `vendor_payout_accounts`) |
| `vendor_id`: products, variants, suppliers, order_items, fulfillments, shipments, reviews, coupons, staff | Vendor er taka er hishab ar payout (`vendor_ledger`, `vendor_payouts`, 8.18) |
| `order_fulfillments` ar duita state machine (order ar fulfillment, 8.8) | Vendor product approval, vendor rating |
| `order_items.discount_share`, `commission`; pricing e vendor onujayi bhag (8.5) | Platform admin, shop signup, billing (`platform_admins`, `shop_plans`) |
| Tenant-safe foreign key `(shop_id, id)` | Custom domain verify ar auto HTTPS (13.3) |
| `shops.mode`, `order_prefix`, `timezone`; domain diye shop chena (3.2) | Postgres Row Level Security (extra suraksha) |

Single store e eigulo prai binamullo: vendor ekta, fulfillment ekta, commission 0. Admin ar website e vendor ba fulfillment chokhe pore na (3.7).
