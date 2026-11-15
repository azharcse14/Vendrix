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
| SMS provider | Bangladesh er kono SMS gateway (masking/sender naam approval shoho) | `notify` package e interface, provider bodlano shohoj. v1.19: M4 e adapter na (local e log, onno env e `ErrNoProvider`); provider M12 er age (8.16) |
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
| IP-bhittik rate limit ar CGNAT (v1.6 review) | 7.8 er fixed limit, naki shop setting | Limited launch e 7.8 cholbe; public launch er age setting e nao, phone-bhittik limit ar Turnstile e beshi bhorsha (mobile network e onek customer ek IP share kore). M1 review: ochena `X-Shop-Host` er request rate limit er age 404 pay (key e shop lage), tai protiti request e ekta `shop_domains` lookup; shop resolve er age IP-only limit (koto?) ba negative cache lagbe kina eki shathe thik koro |
| Packed fulfillment undo (v1.6 review) | `packed → open` (batch ferot, ager taka), naki cancel + notun order | Thik hoyni; M7 er age. Ekhon bhul ojon ba item pack hole ferot jawar path nai |
| Purono `jobs` row (v1.6 review) | `data.retention` e purono done/failed job muche fela, naki rakha | Thik hoyni; ekhon kono cleanup nai (protiti SMS ekta job) |
| `pricing.Allocate` er input (v1.6 review) | Exported thakbe (minus weight ba total e `bits.Div64` panic), naki guard ba unexported | Thik hoyni; pricing er bhetore input valid, tai ekhon bug na |
| `order_notes` e DELETE (v1.6 review) | Itihash hishebe `shop_app` theke REVOKE, naki rakha | Thik hoyni |
| OTP SMS er code `jobs.payload` e (v1.7 review) | (a) payload e AES-GCM (notun key ba `OTP_SECRET` theke HKDF, AAD = `otp_codes.id`), (b) OTP SMS job chhara request er bhetore pathano (ekta retry) | Thik holo (v1.19, user 15 November 2026): (a). Key `OTP_SECRET` theke `crypto/hkdf`, AAD = `otp_codes.id`, payload `{otp_id, nonce, ct}`, done hole `{}` (8.2, 6.1) |
| `jobs` er purono row (M0 review) | (a) `jobs.cleanup` protidin: `done_at` 30 din er purono row muche, (b) done hole payload khali, row rakho, (c) duitai | Thik holo (v1.19, user 15 November 2026): (c). Done `sms.send` er payload shathe shathe `{}`; `jobs.cleanup` protidin done 30 din ar failed 90 din er purono row muche (8.17) |
| OTP SMS cap er DoS (v1.26, M4 review) | (a) ekhon er moto: cap shesh hole oi din OTP login bondho, monitor ar alert, (b) cap er kache Turnstile er upor extra (jemon phone prefix ba ASN limit), (c) cap shudhu khoroch er alert, login bondho na | Thik hoyni; M12 (launch) er age. Ekhon (a) er prothom bhag: cap approximate (parallel e 1–2 par, 7.8) |
| Turnstile er test secret local chhara (v1.20) | (a) config e atkano (start hoy na), (b) shudhu log e warning, (c) kichu na | Thik holo (v1.20, user 15 November 2026): (a). `APP_ENV` local chhara `TURNSTILE_SECRET` Cloudflare er test secret (`1x`/`2x`/`3x0000000000000000000000000000000AA`) hole `config.Load` error (13.5) |
| Cart merge e 50 er beshi variant ar wishlist 200 (v1.29, M5) | (a) account cart er item thake, guest er notun item 50 porjonto, baki bad + notice; wishlist 200 e notun add `409`, (b) limit chhariye jete dewa | Thik holo (v1.29, user 15 November 2026): (a). Merge e notice `item_dropped` (8.4 item 6; v1.30: verify response er `cart_notices` e, 7.3); `409 wishlist_full` (7.1, 8.4 item 8) |
| Vendor staff (`vendor_owner`) category list porte parbe kina (M1 review; 11.2 "category na") | (a) shudhu manage na, `GET /admin/categories` porte pare (product e `category_id` dite lage), (b) kichui na | Thik hoyni; M7 (admin UI) er age. Ekhon `GET` o owner/admin |
| COD e packing e beshi ojon (v1.7 review) | (a) shima nai (ekhon: notun total SMS e), (b) shima (jemon +10%) er beshi hole staff call ar customer er OK, (c) free delivery shima abar dekha (beshi ojon e shima chhuyle delivery 0) | Thik hoyni; M7 er age. Customer refuse korle `phone_refused_before` (8.7) tar gaye lage |
| Dart package er jora (v1.8) | (a) ek pub workspace, (b) `packages/shared` path dependency ar website alada resolution | Thik holo (v1.8): (b); admin ar app chaile ek workspace (4.6) |
| Production domain (v1.8) | Platform er domain (doc e `tomarshop.com` placeholder) | Thik hoyni; M12 er age (`docs/STATUS.md`) |
| Client JS er `?v=<git sha>` (M3 review, 10.3, 10.10 item 10) | (a) Jaspr 0.23.5 nijei `<script src="main.client.dart.js">` boshay, hook nai: deploy e Cloudflare er HTML + JS cache purge (10.3 er onno upay); (b) Jaspr er client script bondho kore `Document` e nijer `<script src="...?v=">` (Jaspr internal e hat, update e bhangte pare); (c) ekhon kichu na: HTML ar JS duitai `max-age=60`, tai purono/notun mile na emon shomoy beshi hole 60 second | Thik hoyni; M12 (deploy) er age. Recommendation (a): deploy script e purge, code bodol na |
| M3 Lighthouse LCP (M3 step 15, 10.5, 9.6) | Simulate (Lantern) e category/product LCP 2.7s (> 2.5s), devtools throttle e 1.7s; kon ta acceptance? (a) devtools/asol throttle er number accept, simulate er number Report e, font kom korar kaj M12 er age (Lighthouse CI er sathe); (b) ekhon-i font kom (jemon shudhu bengali subset 400/500 body, heading 500, ba system font fallback), 9.6 bodle; (c) acceptance e simulate-i lagbe, font subset/size bodlate design change | Thik holo (v1.18, user 15 November 2026): (a). Acceptance devtools (asol) throttle er number (LCP 1.3/1.7/1.6s, Performance 100/99/99); simulate (2.3/2.7/2.7s) M3 Report e. Font kom korar kaj M12 er age, Lighthouse CI er sathe (10.5, 15) |
| Turnstile test secret e `hostname` (v1.21, M4 step 4) | Cloudflare er test secret (`1x...AA`) siteverify e shob shomoy `hostname: "example.com"` dey, tai 10.11 er `hostname` = `X-Shop-Host` check local e (`localhost`/nijer domain) fail kore, local login (M4 step 12-13) hoy na. (a) Turnstile secret test secret hole (config shudhu local e dey, v1.20) hostname check bad; (b) `APP_ENV=local` e hostname check bad; (c) local shop er domain `example.com` (`/etc/hosts`); (d) local e Cloudflare er asol widget (test key na) | Thik holo (v1.22, user 15 November 2026): (a). `TURNSTILE_SECRET` Cloudflare er test secret (`1x`/`2x`/`3x...AA`, config er list) hole siteverify er `hostname` check bad; asol secret e local eo check. Test secret local chhara config e atke (v1.20), tai production e skip hoy na (10.11) |
| Staff account lock e response (v1.24, M4 step 15 review) | (a) `429` + `Retry-After` (ekhon), (b) bhul password er moto `401 invalid_credentials` (dummy PBKDF2, `Retry-After` na) | Thik holo (v1.24, user 15 November 2026): (b). `429` ar thik/bhul password e alada response dekhe email ar password duitai chena jeto (8.3 step 1, 7.8) |
| TOTP setup er secret enable er age (v1.25, M4 step 15 review) | (a) setup e-i live column e (ager; enable na korle porer login TOTP chay, staff atke), (b) pending column, enable e thik code e live | Thik holo (v1.25, user 15 November 2026): (b). `staff.totp_pending_*`; login shudhu live dekhe; TOTP live ar `must_change_password` e password bodlale session `active` (8.3 step 4, 6) |
| Local e `__Host-` cookie (v1.23, M4 step 13) | `__Host-sid`/`__Host-asid` er `__Host-` prefix e Chromium https scheme chay (`http://*.localhost` secure context holeo cookie fele dey, `InvalidPrefix`), tai `make web` (http) e browser e login hoy na; curl/test e thik. (a) local e https: `infra/caddy` e `tls internal` er local Caddyfile ba `make web-https` (doc + script), cookie naam bodlay na; (b) `APP_ENV=local` e prefix chhara naam (`sid`/`asid`), Go ar BFF e config; (c) kichu na, local browser proman shudhu proxy diye (M4 step 13 er moto) | Thik holo (v1.27, user 15 November 2026): (a). `make https` (Caddy `tls internal`, `infra/caddy/Caddyfile.local`), cookie naam shob env e ekoi (13.1) |

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
