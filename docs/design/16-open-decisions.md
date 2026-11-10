> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 16. Open decision, assumption ar reference

### 16.1 Ekhono thik hoyni

| Decision | Option | Ei design e |
|---|---|---|
| Multi-tenant model | (a) protiti business er alada deploy, (b) ek platform e onek dokan (Shopify er moto) | `shop_id` + tenant-safe foreign key, tai duitai shombhob. MVP te ek shop |
| Marketplace delivery charge | `per_order` (ekta charge, vendor der moddhe bhag) ba `per_vendor` (protiti parcel e) | Setting, default `per_order` |
| Marketplace e ke ship kore | Platform (shob vendor er maal ek jaygay ene), naki protiti vendor nijei | Fulfillment vendor onujayi, tai duitai shombhob |
| Staff login | Shop onujayi alada login, naki ek account e onek shop (`accounts` + `memberships`) | MVP te shop onujayi; pore migration |
| Customer account | Shop onujayi alada, naki platform jure ekta | Shop onujayi: multi-tenant e protiti dokan er nijer customer |
| Server region | Bangladesh er bhetore, naki kache-r region | Data law confirm er por (12.1, 13.2) |
| SMS provider | Bangladesh er kono SMS gateway (masking/sender naam approval shoho) | `notify` package e interface, provider bodlano shohoj |
| Delivery | Nijer rider, courier, naki duitai | Duitai (`shipments.carrier`) |
| Launch area | Kon shohor/area | `delivery_zones` e config |
| Organic certificate | Ache kina | Na thakle "organic" badge na |
| Return window | Koto din | `checkout.return_window_days` (default 3) |
| COD e doorstep e partial accept | Item level return, naki puro parcel | Ekhon order edit + return diye manual; Phase 2 te courier er partial delivery |
| Return e delivery charge ferot | Kar dosh (shop ba customer) onujayi | Policy page e likho, refund er poriman staff thik kore |
| Escrow/settlement | SSLCommerz e delivery confirm lage kina | SSLCommerz er sathe confirm (8.9, 12.1) |
| Admin token | localStorage, memory, naki cookie | Thik holo: cookie `__Host-asid` (11.3) |
| Marketplace `per_order` e vendor bad | Tar delivery bhag drop, naki baki vendor der moddhe abar bhag | Ekhon drop (8.5); Phase 3 er age thik koro |
| Freeze e discount haray (v1.5) | Bodla vendor er discount tar notun eligible er beshi na, naki frozen vendor der bhag o abar bhag | Ekhon customer er khoti (8.5 Jana shima); Phase 3 er age thik koro |
| Doc e code | Embedded code + docsync test, naki shudhu file link | Thik holo (v1.6): file link. Design `docs/design/` e section onujayi, code file-i asol; docsync ekhon link, size, code block ar section map dekhe |
| SSLCommerz validation er HTTP call ar DB lock (v1.6 review) | (a) order → payment lock dhore Validation API call (8.9 er ekhon er lekha), (b) age call (`val_id` idempotent), tarpor lock niye status abar dekhe apply | Thik hoyni; M10 er age 8.9 e spashto likho. (a) te gateway slow hole order row ar DB connection atke thake |
| IP-bhittik rate limit ar CGNAT (v1.6 review) | 7.8 er fixed limit, naki shop setting | Limited launch e 7.8 cholbe; public launch er age setting e nao, phone-bhittik limit ar Turnstile e beshi bhorsha (mobile network e onek customer ek IP share kore) |
| Packed fulfillment undo (v1.6 review) | `packed → open` (batch ferot, ager taka), naki cancel + notun order | Thik hoyni; M7 er age. Ekhon bhul ojon ba item pack hole ferot jawar path nai |
| Purono `jobs` row (v1.6 review) | `data.retention` e purono done/failed job muche fela, naki rakha | Thik hoyni; ekhon kono cleanup nai (protiti SMS ekta job) |
| `pricing.Allocate` er input (v1.6 review) | Exported thakbe (minus weight ba total e `bits.Div64` panic), naki guard ba unexported | Thik hoyni; pricing er bhetore input valid, tai ekhon bug na |
| `order_notes` e DELETE (v1.6 review) | Itihash hishebe `shop_app` theke REVOKE, naki rakha | Thik hoyni |
| OTP SMS er code `jobs.payload` e (v1.7 review) | (a) payload e AES-GCM (notun key ba `OTP_SECRET` theke HKDF, AAD = `otp_codes.id`), (b) OTP SMS job chhara request er bhetore pathano (ekta retry) | Thik hoyni; M4 er age. Ekhon plain text: 6.1 er niyom bhange, WAL ar backup e (prai ek bochor) thake. Jai hok, done SMS job er payload khali koro (upore `jobs` row er sathe) |
| `jobs` er purono row (M0 review) | (a) `jobs.cleanup` protidin: `done_at` 30 din er purono row muche, (b) done hole payload khali, row rakho, (c) duitai | Thik hoyni; M4 er age (OTP payload er sathe). `jobs.sweep` ekai dine 288 row banay, done row muche na |
| COD e packing e beshi ojon (v1.7 review) | (a) shima nai (ekhon: notun total SMS e), (b) shima (jemon +10%) er beshi hole staff call ar customer er OK, (c) free delivery shima abar dekha (beshi ojon e shima chhuyle delivery 0) | Thik hoyni; M7 er age. Customer refuse korle `phone_refused_before` (8.7) tar gaye lage |
| Dart package er jora (v1.8) | (a) ek pub workspace, (b) `packages/shared` path dependency ar website alada resolution | Thik holo (v1.8): (b); admin ar app chaile ek workspace (4.6) |
| Production domain (v1.8) | Platform er domain (doc e `tomarshop.com` placeholder) | Thik hoyni; M12 er age (`docs/STATUS.md`) |

### 16.2 Assumption

1. MVP te ekta shop, ekta house vendor, ekta warehouse.
2. Shob dam taka (BDT), bidesh e bikri nai.
3. Customer der beshirbhag mobile e, Facebook theke.
4. Staff 1–10 jon.
5. Ek order e ekta delivery thikana.

### 16.3 Reference

| Bishoy | Link |
|---|---|
| Jaspr | https://docs.jaspr.site, https://pub.dev/packages/jaspr |
| SSLCommerz developer (v4, mandatory field) | https://developer.sslcommerz.com/doc/v4 |
| pgx | https://github.com/jackc/pgx |
| Go `net/http` routing | https://pkg.go.dev/net/http |
| WCAG contrast | https://www.w3.org/TR/WCAG21/#contrast-minimum |
| Core Web Vitals | https://web.dev/articles/vitals |
| DBID khobor (March 2026) | https://today.thefinancialexpress.com.bd/print/digital-business-applications-pile-up-1774629838 |
| Digital Commerce Guideline 2021 (delivery) | https://old.thefinancialexpress.com.bd/trade/govt-orders-e-commerce-firms-to-release-products-in-48-hours-1625580967 |
| Digital Commerce Guideline 2021 (advance, refund, supply) | https://mahbub-law.com/ministry-of-commerce-issues-detailed-guidelines-on-e-commerce-business |
| Bangladesh Bank refund nirdeshona (2021) | https://mahbub-law.com/bangladesh-bank-issues-guidelines-on-settling-payments-in-e-commerce-transactions |
| Bangladesh Bank escrow circular (2021) | https://www.tbsnews.net/node/268687 |
| BFSA registration (October 2026) | https://www.jagonews24.com/en/biz-econ/news/97292 |
| BFSA: 1 November 2026 theke apply | https://bdnews24.com/business/heozz2x8du |
| Personal Data Protection Ordinance 2025 | https://www.thedailystar.net/tech-startup/news/bangladeshs-personal-data-protection-ordinance-2025-key-takeaways-4015401 |
| Personal Data Protection Act 2026 (overview) | https://securiti.ai/bangladesh-personal-data-protection-act-overview/ |
| VAT FY26 (online sale commission) | https://thefinancialexpress.com.bd/economy/bangladesh/budget-proposes-tax-cuts-to-boost-telecom-startups |
| Go CrossOriginProtection CVE | https://pkg.go.dev/vuln/GO-2025-3955 |
| Jaspr 0.23 (agent skills) | https://docs.jaspr.site/releases/v/0.23.0 |
| Jaspr release list | https://github.com/schultek/jaspr/releases |
| Cloudflare Bot Fight Mode (skip hoy na) | https://developers.cloudflare.com/bots/get-started/bot-fight-mode/ |
| Cloudflare security features interoperability | https://developers.cloudflare.com/waf/feature-interoperability/ |
| DCOG 2021 overview (complaint 72 ghonta, marketplace 10 din) | https://www.thedailystar.net/law-our-rights/news/overview-the-digital-commerce-operation-guidelines-2021-2128871 |
| BFSA: 16 category te registration free (October 2026) | https://www.daily-sun.com/business/904670 |
| BFSA notice: 1 November 2026 theke (v1.5 e milano) | https://www.dhakatribune.com/bangladesh/422593/food-businesses-must-register-obtain-licences |
| Jaspr version list (v1.5: 0.23.5, 0.24.0-beta.0) | https://pub.dev/packages/jaspr/versions |
| Go release history | https://go.dev/doc/devel/release |
| Courier API | Steadfast, Pathao, RedX er merchant panel er official API doc (account lage) |
