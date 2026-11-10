> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 12. Security, privacy ar legal compliance

### 12.1 Legal checklist (Bangladesh)

Ami lawyer na, ar niyom bodlay. Niche-r tottho 28 October 2026 e prokashito khobor ar nirdeshona theke milano (16.3); v1.3 e BFSA, PDPA, DCOG ar VAT abar milano hoyeche; v1.5 e BFSA (29 October 2026). Launch er age ekjon lawyer ba e-CAB er sathe confirm koro.

| Bishoy | Ki jana geche | System e ki korte hobe |
|---|---|---|
| DBID | Shob ecommerce ar social media-based business er jonno DBID lagbe. Online presence thakle trade license chhara-o apply kora jay. Beshirbhag application reject hoy website e Bangla te terms spashto na thakle. DBID number website e dekhano badhyotamulok | Bangla terms page, footer e DBID number (settings `branding.dbid_number`) |
| Delivery shoma | Digital Commerce Operational Guideline 2021: same city 5 din, onno city 10 din. Full payment er 48 ghontar moddhe delivery service e hostantor ar customer ke SMS/email/phone e janano. Perishable/nittoproyojoniyo jinish aro taratari, ar shomoy ta order er shomoy bolte hobe | `delivery_deadline`, deadline alert (8.15), prepaid 36 ghontay ship na hole alert, SMS, checkout e delivery er shomoy |
| Advance payment | 48 ghontay hostantor kora na gele escrow chhara 10% er beshi advance neya jay na | Pre-order (Phase 2) e advance max 10% ba escrow |
| Supply na korte parle | Order er 48 ghontar moddhe janano, tarpor 72 ghontay full refund. Advance er por deri hole 10 diner moddhe refund | Packing e batch na pele alert (8.11), refund er deadline alert (8.15) |
| Refund | Refund je madhyome payment, shei madhyome, ar charge business bohon kore (DCOG ar Bangladesh Bank, 2021) | `refunds.method` = payment er madhyom, `provider_ref` |
| Complaint | DCOG 2021: complaint record kore 72 ghontar moddhe shomadhan | `complaints.due_at` (72 ghonta), `alert.complaint_due` (8.17), dashboard e lal |
| Escrow | Bangladesh Bank 2021: gateway delivery confirm er por merchant ke taka chhare; nijer production unit/showroom wala merchant ager settlement e, kintu 7 din e delivery | SSLCommerz e shop er settlement kivabe hobe confirm koro (8.9) |
| BFSA | Safe Food (Registration and Licensing) Regulations 2026 (gazette 16 July 2026). 1 November 2026 theke apply: 16 dhoroner purono food business er jonno registration (free), special/super food hole licence. Protiti food package e BFSA registration number. Registration chhara chalano Safe Food Act 2013 onujayi oporadh. Packaged food labeling er notun regulation (draft, January 2026) track koro | **1 November 2026 er moddhe registration** (launch ba M11 er jonno opekkha na; v1.5: BFSA er 21 October er notice e apply 1 November theke). v1.7 (10 November 2026): shima par hoye geche; registration hoyeche kina ekhon-i confirm koro. Chal, dal, tel, modhu, gur 16 ta conventional category te (registration, free) pore, naki special/superfood (licence) e, BFSA ke jiggesh kore confirm koro. Product `attributes.bfsa_number`, footer e business er BFSA number, product page e dekhano |
| "Organic" ar health dabi | Proman chhara dabi bhul bigyapon hote pare. Health ba rog-jhuki komanor dabi er jonno Safe Food (Advertisement and Claims) Regulations 2025 | Certificate upload, shudhu certificate thakle "organic" badge. Na hole "krishok theke shorasori" er moto shotto kotha. Description e health dabi lekhar age regulation dekho |
| Personal data | Personal Data Protection Act 2026 (Act No. 63 of 2026, 10 April 2026), 2025 er ordinance ar February 2026 er amendment er jaygay. Ekta source onujayi enforce korbe National Data Management Authority. Spashto onumoti (consent), data rights, ar restricted personal data ar critical infrastructure er jonno localization (February 2026 er amendment e data residency er niyom bodleche). Commencement ar "restricted" er songgya lawyer er sathe | Checkout/login e privacy policy link + consent, `privacy_requests` (export/delete), delete = anonymize, log e personal data lukano, IP 180 din por muche (8.17), server region thik korar age lawyer (13.2) |
| Gift card/wallet | Digital wallet, gift card, voucher (cash er bikolpo) Bangladesh Bank er onumoti chhara na. Coupon (shudhu discount) alada | Phase 3 er loyalty/gift card er age lawyer |
| VAT | Invoice e VAT/BIN lagbe kina. FY26 budget e online sale er commission er VAT 5% theke 15%; FY27 e e-commerce niye nirdishto bodol pawa jayni | Accountant er sathe thik koro, invoice template e jayga rakho; Phase 3 commission e VAT line (8.18) |
| Marketplace (Phase 3) | Onner maal bikri korle vendor er tottho (trade license/DBID, food hole BFSA) rakha ar dekhanor dayitto ashe. Customer er taka dhore rekhe pore vendor ke dewa payment er niyom er moddhe pore kina, sheta-o dekhte hobe. Seller ke taka 10 diner moddhe dite hoy, alada chukti na thakle | Vendor KYC (`vendor_documents`), product page e vendor er naam, launch er age lawyer ar e-CAB er sathe confirm |
| Multi-tenant (Phase 3) | Protiti shop alada business: tader DBID, BFSA, policy, payment account nijer. Platform tader hoye customer data rakhe | Shop onujayi settings ar credential (ache), shop er sathe data processing chukti, lawyer er sathe confirm |

### 12.2 Security checklist

**Server**

1. `http.Server` e `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` (4.2).
2. `http.MaxBytesReader` diye body limit. JSON e `DisallowUnknownFields`.
3. `http.CrossOriginProtection` (CSRF). CORS shudhu nijer domain gulo.
4. Panic recover middleware, client e stack trace na.
5. Client IP: Jaspr er `X-Client-IP` (shudhu loopback + `X-Internal-Auth`), baki shob Caddy er `X-Real-IP` (Cloudflare range Caddy er `trusted_proxies` e); Caddy client er `X-Client-IP` ar `X-Internal-Auth` muche dey (3.2, 13.3).
6. Bahirer API call e `http.Client{Timeout: 10 * time.Second}`.
7. Router build test: ServeMux pattern conflict e panic kore, M0 tei dhoro.
8. Upload: 10 MB, dimension check, re-encode (EXIF mucche), SVG ar WebP na, read ar write deadline duitai 2 minute (2.2).

**Login ar session**

1. Token random (`rand.Text()`), database e shudhu `sha256`.
2. OTP er hash `HMAC-SHA256(secret, phone:purpose:code)`, 5 minute, 5 attempt (atomic `UPDATE`, error path eo commit), phone e dine 10 OTP, 24 ghontay 10 bhul e lock (7.8).
3. Staff password `pbkdf2` 600k (dummy hash, semaphore, soft lock), owner/admin/staff e TOTP badhyotamulok, temp_token e 5 attempt ar `failed_mfa` (temp_token jure 10), `totp_last_step` (replay), recovery code, prothom login e `setup_pending` (8.3).
4. Cookie `__Host-` prefix, `HttpOnly; Secure`; customer `SameSite=Lax`, admin `SameSite=Strict`. Token localStorage e kokhono na.
5. Logout e session revoke. Staff er password, role bodlale ba deactivate korle tar shob session revoke.
6. Session er `subject_type` route er sathe (Rule 16): customer token admin route e `401`.

**Data**

1. Protiti query te `shop_id`, ar vendor staff er query te `vendor_id` (test kore dekho ek shop ba vendor er token diye onno shop ba vendor er data pawa jay na). Database er composite foreign key cross-shop reference atkay (6.4).
2. SQL shudhu parameter diye.
3. Public link e `public_token`, serial ID na (IDOR thekate). COD confirm alada `confirm_token`, shudhu SMS e (8.10).
4. Credential (`shop_credentials`) AES-GCM (AAD = shop_id:provider:key_version), key env e (`CREDENTIALS_KEY`), API theke kokhono ferot na.
5. Log e phone er majhkhan lukano (`+88017****5678`), thikana log e na.
6. Database user er shudhu dorkari permission: migration owner role, app `shop_app` (DDL nai, order/payment e DELETE nai, audit e UPDATE nai; `001` er shesh block, 6.1).

**Payment**

1. Paid shudhu IPN + Order Validation API er por (8.9). Amount ar currency mila-o. v1.7: VALID na hole ba response er `tran_id` form er sathe na mille kono payment bodlay na (fake return e payment `failed` ba reconcile bondho hoy na).
2. Return URL kichu paid kore na.
3. Dam, discount, total shudhu server e.
4. IPN path e Cloudflare WAF skip rule; Bot Fight Mode skip hoy na, tai off rakho ba Super Bot Fight Mode (8.9); reconcile job.
5. Ekoi order e duita successful payment hole auto refund (8.9).

**Website**

1. Bot: Turnstile (checkout, OTP send, complaint) + honeypot + rate limit + daily SMS cap.
2. Admin er HTML/lekha (banner, description) render er age escape. Markdown allow korle shudhu nirdishto tag. JSON-LD e `<` escape. Banner ar social link e shudhu `/` (kintu `//`, `\` ba control character na, URL parse kore; 9.2) ba `https://`.
3. Security header: `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin` (token ar print page e `no-referrer`), `Content-Security-Policy` (nijer domain + Turnstile + payment + chobi er domain, Pixel/GA4 settings onujayi kintu token page e na (10.1); v1.5: inline script e nonce na, `sha256` hash ba external file, karon public HTML cache hoy (10.3) ar cache kora page e shobai ekoi nonce pay), `frame-ancestors 'none'` (preview chhara).
4. CSRF: BFF er POST e Origin check (10.9).
5. Session, cart ba `preview` cookie thaka page e `Cache-Control: private, no-store` (10.3).

**Operation**

1. Secret shudhu env/secret manager e, Git e kokhono na (`.env` `.gitignore` e).
2. Backup encrypted.
3. Shob admin kaj audit log e.
4. Dependency (pgx, Dart package) mashe ekbar update check; CI te `govulncheck`. v1.7: Go toolchain er notun 1.27.x patch o: `go.mod` er `toolchain`, `.claude/settings.json` ar `scripts/` er `GOTOOLCHAIN` ek PR e (2.1).
