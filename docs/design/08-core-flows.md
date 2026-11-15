> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 8. Core flow ar business rule

Ei section er Go code (`pricing`, `phone`, `order` state machine) `go vet` ar `go test -race` pass kore (v1.5 Go 1.27.3 e chalano, CI te `GOTOOLCHAIN=go1.27.3`; mutation test e `order` er 40 ta mutant shob dhora pore, `pricing` er 179 tar moddhe baki 4 ta equivalent). v1.7: Go 1.27.3 e abar chalano (`-race` shoho, coverage 100%); `pricing` er notun `ErrCouponNotApplicable` branch e 6 ta hate kora mutant shob dhora pore, puro mutation test v1.7 e abar chalano hoyni. Code file e: [`backend/internal/phone`](../../backend/internal/phone/phone.go), [`pricing`](../../backend/internal/pricing/pricing.go), [`order`](../../backend/internal/order/order.go) (test shoho). v1.6: doc e code er copy nai; niyom ei section er lekha te, asol code file e. Niyom bodlate age ei lekha, tarpor code ar test.

### 8.1 Phone number

Je format ei ashuk (dash, space, `880`, `00880`, Bangla shonkhya), save er age `+8801XXXXXXXXX` e anbe. Tritiyo digit 3 theke 9 (Bangladesh er mobile operator). Digit ar separator (space, `.`, `(`, `)`, `-` ar `–` er moto dash) chhara onno okkhor thakle bhul, chupchap fele dey na. v1.3: phone theke copy kora lukano chinho (LRM, zero-width space, U+202A/U+202C) bad jay; `+` shudhu shuru te, ar tarpor `880` thakte hobe (`+1 712 345 678` ager version e BD number hoye jeto).

Code: [`backend/internal/phone/phone.go`](../../backend/internal/phone/phone.go), test [`phone_test.go`](../../backend/internal/phone/phone_test.go).

### 8.2 Customer login (OTP)

**OTP pathano** (`POST /api/auth/otp/send`):

1. Phone normalize. Turnstile, rate limit (phone e 15 minute e 3, dine 10), shop er daily SMS cap, ar oi phone 24 ghontar lock e kina check (Section 7.8). v1.19: phone er "dine" 24 ghonta rolling, shop er SMS cap shop er timezone er calendar din (Rule 5); lock alada table na, `otp_codes` theke gona (7.8). Response shob shomoy ekoi `{"sent": true}` (`429` chhara), phone er account ache kina bole na.
2. `crypto/rand` diye 6 digit code (`rand.Int(rand.Reader, big.NewInt(1000000))`, samne zero shoho 6 digit).
3. `otp_codes` e `code_hash = HMAC-SHA256(OTP_SECRET, phone + ":" + purpose + ":" + code)`, `expires_at = now + 5 minute`.
4. `sms.send` job queue te. Shudhu `APP_ENV=local` e SMS na pathiye code log e likho (staging e shudhu team er number, 13.1); production e log mode chalu thakle server start-i hobe na. v1.19: payload e code AES-GCM e (6.1): key `OTP_SECRET` theke `crypto/hkdf` (notun env na), AAD = `otp_codes.id`, payload `{otp_id, nonce, ct}`; handler decrypt kore pathay, done hole payload `{}` (8.17). Plain text hole WAL ar backup e prai ek bochor thakto. M6 er COD confirm link (`confirm_token`) eki upay nite pare.

**OTP verify** (`POST /api/auth/otp/verify`), attempt gona chhara baki ta ek transaction e:

1. Oi phone ar purpose er shesh, use na hoya, expire na hoya OTP nao. Na pele `400 otp_invalid`.
2. Attempt age gono: ei `UPDATE` verify er transaction er age, nijer commit e (autocommit), jate porer step e `ROLLBACK` holeo gona thake, ar parallel request e-o 5 er beshi na hoy. Kichu na ele `400 otp_too_many_attempts` (ba expire hole `otp_invalid`):
   ```sql
   UPDATE otp_codes SET attempts = attempts + 1
   WHERE id = $1 AND attempts < 5 AND consumed_at IS NULL AND expires_at > now()
   RETURNING code_hash;
   ```
3. Hash tulona `hmac.Equal` diye (constant time). Mille consume, v1.7: conditional, `UPDATE otp_codes SET consumed_at = now() WHERE id = $1 AND consumed_at IS NULL RETURNING id`; kichu na ele already use hoyeche, `400 otp_invalid` (ekoi code e duita parallel verify duita session pay na, 8.3 er TOTP er moto). Purpose `checkout` e consume order er transaction e (8.6 step 9). Na mille, oi phone er 24 ghontar bhul 10 chhuyle 24 ghonta lock ar alert (7.8; v1.19: alert `alert.security` job, 8.17).
4. `users` e `(shop_id, phone)` diye upsert. Delete kora account er phone `NULL`, tai shei manush notun account pay, purono data ashe na. v1.19: notun user er `locale` = request er `Accept-Language`.
5. Oi phone er guest order account e jog: `UPDATE orders SET user_id = $1 WHERE shop_id = $2 AND customer_phone = $3 AND user_id IS NULL`. Eta **shudhu OTP verify er por**, kokhono age na.
6. `X-Cart-Key` thakle guest cart merge (8.4). v1.28: ei transaction e-i (8.4 item 6), tai verify fail hole merge na.
7. Session: `token = rand.Text()`, database e `sha256(token)`, `subject_type = 'customer'`, `user_id`, `expires_at = now + 30 din`. `last_used_at` ghontay ekbar-er beshi update na.

Website e cookie: `__Host-sid=<token>; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=2592000`. Token kokhono JavaScript e (localStorage) rakhbe na.

### 8.3 Staff login

1. Email + password. Password hash: `crypto/pbkdf2`, SHA-256, 600,000 iteration, 16 byte random salt. Format `pbkdf2-sha256$600000$<base64 salt>$<base64 hash>`. Tulona `subtle.ConstantTimeCompare`. Ochena email e-o ekta dummy hash er sathe tulona (timing dekhe email chena na jay), ar ek shathe 4 tar beshi PBKDF2 na (semaphore, CPU DoS). `locked_until` er age hole `429`. Bhul hole `failed_logins + 1`; 5 er por protiti bhul e `locked_until = now + 1 minute` (soft lock), 10 ar 50 e owner alert; thik hole 0. IP onujayi limit 7.8. v1.24: PBKDF2 er AGE autocommit e atomic reservation `UPDATE staff SET failed_logins = failed_logins + 1, locked_until = CASE WHEN failed_logins + 1 >= 5 THEN GREATEST(coalesce(locked_until, now()), now() + 1 minute) ELSE locked_until END WHERE id AND shop_id AND (locked_until IS NULL OR locked_until <= now()) RETURNING failed_logins` (8.2 er OTP attempt er moto), tai parallel burst e-o lock er age 5 tar beshi asol PBKDF2 na; row na ele lock. Lock e response bhul password er motoi `401 invalid_credentials` (dummy hash er PBKDF2 kore, `Retry-After` nai, audit `reason: locked`), `429` na: lock dekhe email chena na jay (user decision 15 November 2026, 16.1). Thik hole `failed_logins = 0` ar password er lock khole (`failed_mfa` er 30 minute lock na). `GREATEST`: choto lock lambha lock ke choto kore na.
2. Password thik ar TOTP set thakle `sessions` e `stage = 'mfa_pending'` row (temp_token, 5 minute). TOTP (6 digit, 30 second, RFC 6238, `crypto/hmac` + SHA-1, ±1 step) chaibe; ek temp_token e 5 bar (`mfa_attempts`). Temp_token jure gona `staff.failed_mfa`: TOTP ba recovery code e 10 bar bhul hole 30 minute `locked_until` ar owner alert, thik hole 0. Na hole password jana keu protibar notun login e notun temp_token niye ghontay prai 10% chance e code milay. v1.24: TOTP/recovery check er AGE (temp_token er `mfa_attempts` er por) staff e autocommit atomic reservation `failed_mfa = failed_mfa + 1` (10 e `GREATEST(..., now() + 30 minute)`), `WHERE` lock nai `RETURNING`; row na ele `429` (lock er majhe thik code o na; temp_token er malik password jane, tai ekhane `429` e enumeration nai). Tai parallel temp_token diye-o lock er age 10 tar beshi guess na. Thik hole `failed_mfa = 0` ar lock khole; bhul e abar gona na (reservation-i gona), shudhu audit ar 10 e alert. Notun thik password login e oi staff er purono `mfa_pending` session revoke (ek shathe ek temp_token). Time-step `staff.totp_last_step` er beshi hote hobe, tai ekoi code duibar cholbe na; v1.5: eta ekta conditional `UPDATE staff SET totp_last_step = $2 WHERE id = $1 AND (totp_last_step IS NULL OR totp_last_step < $2) RETURNING id`, kichu na ele code already use hoyeche (duita parallel login ekoi code e duita session pay na). Owner, admin ar staff, shobar jonno TOTP badhyotamulok (staff o customer er phone ar thikana dekhe); Phase 3 e vendor role o.
3. TOTP thik hole oi row `stage = 'active'`, `expires_at = now + 12 ghonta`, cookie `__Host-asid` (7.1). Phone harale 10 tar ekta recovery code (`staff_recovery_codes`, protiti ekbar).
4. Prothom login (`must_change_password` ba TOTP nai): password (ar TOTP thakle TOTP) er por `stage = 'setup_pending'` session (15 minute, cookie `__Host-asid`). Ei session shudhu `/auth/password`, `/auth/totp/setup`, `/auth/totp/enable` ar `/auth/logout` e chole, baki shob admin API `403 setup_required`. Age password bodlano, tarpor TOTP setup; enable hole session `active` (12 ghonta) ar 10 ta recovery code dekhay. Password bodlale, deactivate ba role bodlale shob session revoke (v1.24: password bodlale ekhon er session chhara shob; setup flow e ekhon er session-i TOTP setup e lage). v1.25: TOTP live ar `must_change_password` true hole TOTP er por `setup_pending`; `/auth/password` e password bodlale oi row-i `active` (12 ghonta), token notun (`__Host-asid` 43200), notun TOTP setup lage na. Ei obostha (`setup_pending`, `must_change_password`, live TOTP) shudhu TOTP par kora session e hoy: TOTP chhara `setup_pending` e enable er age password bodlate hoy, ar password bodlano onno shob session revoke kore. Live TOTP thakle `/auth/totp/setup` ar `/enable` `403 forbidden` (TOTP bodlano shudhu `reset-2fa`); active session theke setup bondho-i (security-reviewer M2). v1.24: `/auth/password` e purono password o step 1 er reservation diye (bhul e `failed_logins` gona, lock e `429`, audit `staff.password_change_failed`). Shob login (thik/bhul) audit log e; v1.24: ochena email er audit e email na, shudhu domain (`email_domain`), karon keu email er ghore password likhe phelle plain password audit e thakto.
5. v1.5: TOTP secret `crypto/aes` GCM e `TOTP_KEY` (13.5) diye encrypt, AAD = `shop_id:staff_id:key_version`, tai ek staff er ciphertext onno staff er row e copy korle decrypt hoy na; key er version `staff.totp_key_version` e (rotation e notun version, purono row pore re-encrypt). Recovery code er hash `HMAC-SHA256(RECOVERY_SECRET, staff_id:code)`; `RECOVERY_SECRET` rotate korle shob staff ke notun code dite hobe.
6. v1.19 (M4): owner alert `alert.security` job (8.17). Recovery code 10 ta, protiti `crypto/rand` 10 okkhor base32 (50 bit), `XXXXX-XXXXX` e dekhano, tulona e `-`, space ar case bad; enable e notun 10 ta (purono muche). Notun password kom-poksha 12, beshi-te 128 okkhor (PBKDF2 er CPU), purono-r sathe ekoi na. `/auth/totp/setup` → `{otpauth_url, secret}` (base32, `encoding/base32`). v1.25: setup er secret live na, `staff.totp_pending_enc`, `totp_pending_nonce`, `totp_pending_key_version` e (ager pending replace; AAD step 5 er moto). `/auth/totp/enable` e pending er code thik hole ek transaction e pending → live (`totp_secret_enc`, `totp_nonce`, `totp_key_version`), pending NULL, `totp_last_step` = oi step, notun recovery code; bhul code e pending thake. Login ar `/auth/totp` shudhu live secret dekhe, tai setup kore enable na korle staff atke na (porer login abar `setup_pending`). `reset-2fa` pending o muche (user decision 15 November 2026, 16.1). `closed` shop e o login chole (3.2 Niyom 2). v1.26: `/auth/totp`, `/recovery` ar `/auth/totp/enable` e session row er `token_hash` notun (purono token ar chole na). `otpauth_url` er issuer ar label er prefix shop er slug (`otpauth://totp/<slug>:<email>?issuer=<slug>`). Audit action: `staff.login` (`stage`), `staff.login_failed` (`reason`: `password`, `locked`, `inactive`, `unknown_email` + `email_domain`), `staff.login_mfa` (`method`), `staff.login_mfa_failed`, `staff.password_changed`, `staff.password_change_failed`, `staff.totp_setup`, `staff.totp_enabled`, `staff.reset_2fa` (`staff_id` NULL, actor `cli`, entity oi staff). Owner TOTP ar recovery code duitai harale `shop-api staff reset-2fa --shop --email` CLI: TOTP ar recovery code muche, shob session revoke, audit; porer login e `setup_pending` (step 4). v1.26: `reset-2fa` `failed_mfa` 0 kore, kintu `locked_until` ar `must_change_password` e hat dey na.

### 8.4 Cart

| Niyom | Bistarito |
|---|---|
| Dam | Cart e dam save hoy na. Protibar `GET /api/cart` e variant er bortoman dam |
| Quantity | Kom-poksha 1, beshi hole stock ar 50 er moddhe je ta kom |
| Bondho product | `is_active = false` hole cart theke bad, response e notice |
| Guest cart | Website cookie `__Host-cart=<random>`, API te `X-Cart-Key`. Database e `sha256` |
| Merge | Login e guest cart er item account cart e jog. Ek-i variant hole quantity jog, stock e cap. Tarpor guest cart muche fela |
| Purono cart | 30 din kono bodol na hole guest cart muche fela (daily job) |
| Vendor | Marketplace e cart vendor onujayi group kore dekhay (alada parcel; `per_vendor` hole alada delivery charge). Single store e group nai |

v1.28 (M5, user 15 November 2026):

1. **Guest key**: Go banay. Key chhara (ba `^[A-Z2-7]{26}$` na mile emon key) guest `POST /api/cart/items` e notun `rand.Text()` (128 bit), DB te `sha256`, response e `cart_key`; `GET` kokhono row banay na (bot er GET e DB bhore na). Bhul format er key "nai" (notun cart, onner cart na); key er cart onno shop e (`X-Shop-Host`) ochena. Client nijer key diye cart banate pare na. Website er cookie 10.12.
2. **Login ar key**: Bearer thakle shudhu account cart, key ignore. Bearer bhul ba expire hole `401` (Rule 16 er session check hubohu), guest e fall back na: `httpx.OptionalCustomer` (Bearer na thakle guest, thakle `RequireCustomer` er path). Merge shudhu verify e.
3. **Quantity ar limit**: add/patch e `quantity` 1..`min(stock, 50)`; 0 ba minus `400 validation_failed`; beshi hole (50 er beshi o) `409 out_of_stock` + `details {variant_id, max_quantity}`, chupchap komay na; stock 0 e add `409 out_of_stock`. Ek cart e max 50 ta alada variant, 51 tomo add e `409 cart_full`. Guest cart toiri te alada limit na (public 120/minute, 7.8; row choto, 30 din e muche).
4. **Normalize (`GET`)**: qty > stock (stock > 0) hole qty = `min(stock, 50)` (DB te o) + notice `quantity_reduced`; stock 0 er item thake (`in_stock: false`, subtotal e na; checkout M6 e abar dekhe). Item e `max_quantity = min(stock, 50)` (exact stock 50 er niche dekha jay, catalog e shudhu `in_stock`). Product ba variant `is_active = false` (marketplace e vendor `active` na, Rule 13) er item `DELETE ... RETURNING` (ek query) + notice `{code: "item_removed", name}` (naam bortoman bhashay); notice ekbar-i, product abar chalu holeo cart e ferot na. POST/PATCH e bondho ba ochena variant `404 not_found` (public catalog e o nai).
5. **Dam**: line total `pricing.LineTotal`, subtotal `pricing.Calculate(lines, nil, Zone{}, PerOrder).Subtotal`, shudhu `in_stock` line (Rule 1). Ojon-variable e `WeightG = nominal_weight_g × qty` (website e "approx"); `nominal_weight_g` NULL er ojon-variable variant add e `409 out_of_stock`. Dam bodlale cart e notice na (`seen_price` column na): cart shob shomoy bortoman dam (`compare_at_price` thakle kata dam); order er dam M6 er `expected_total` → `409 price_changed` (8.6) dhore.
6. **Merge** (8.2 step 6): verify er transaction e dui cart `SELECT ... FOR UPDATE` (account cart na thakle toiri); ekoi variant e qty jog, `min(jog, 50, stock)`; stock 0 hole item thake `min(jog, 50)` (out of stock dekhay, stock ferot ele kena jay); bondho product bad; tarpor guest cart (row) muche. Verify fail hole merge na. Website e `__Host-cart` muche (10.12). v1.29: guest + account milie 50 er beshi alada variant hole account cart er item thake, guest er notun item 50 porjonto, baki bad; cart response e notice `item_dropped`. v1.30 (user 15 November 2026): notice verify er response er `cart_notices` e (7.3), porer `GET /api/cart` e na (tar jonno state lagto, migration na); protiti bad item e `{code: "item_dropped", name}`. Bondho guest item chupchap bad (notice na).
7. **Purono cart**: item bodlale `carts.updated_at = now()`; `cart.cleanup` (8.17) shudhu guest cart, login cart muche na. `DELETE` e item na thakleo `200` (idempotent).
8. **Wishlist** (7.4): `GET` e bondho product lukano (row thake, chalu hole ferot); merge e shudhu oi shop er active product, ochena/bhul ID bad, max 50 ID, wishlist e mot 200. v1.29: wishlist e 200 thakle notun `POST /api/wishlist/{product_id}` → `409 wishlist_full` (ager product e `200`).
9. v1.31 (M5 review, user 15 November 2026): badge er jonno `GET /api/cart/count` (normalize ar notice na); normalize ar notice shudhu `GET /api/cart` e, website e shudhu `/cart` page (7.4, 10.12 item 5). Qty jog/set, `variant_label`, stock 0 er `line_total` ar wishlist er `404`/limit er choto niyom 7.4 e. Vendor `active` cart ar wishlist dekhe, catalog list dekhe na: MV er age milano (16.1).

**8.5–8.10** (dam er hishab, checkout, risk ar OTP, state machine, SSLCommerz, COD confirmation) ar 8.19 (M6 er decision): [08-checkout.md](08-checkout.md). v1.32: 08 file 60 KB er kache, tai bhag; section number ekoi.

### 8.11 Packing, batch ar ojon

Packing hoy **fulfillment** dhore. Single store e order ar fulfillment ek, tai staff er kache kono parthokko nai.

1. **Pick list**: ek diner (shop er timezone e) shob `open` fulfillment (order `confirmed`) er item, variant onujayi jog, area onujayi bhag. Vendor staff shudhu nijer vendor er pick list dekhe.
2. **FIFO batch** (`batch_expiry` module on thakle): protiti item er jonno expiry age shesh hobe emon batch theke `quantity_remaining` komao (6.4 er query, expire hoya batch query tei bad, `FOR UPDATE`), `order_item_batches` e likho (`variant_id` shoho). Jothesto batch na pele fulfillment pack hoy na, alert; prepaid hole customer ke 48 ghontay janao ar 72 ghontay refund (12.1).
3. **Ojon-variable item**: staff puro line er asol mot ojon (`packed_weight_g`) dey (shudhu ojon-variable item e; v1.5: fixed item e ojon dile `400 validation_failed`, DB `order_items_weight_check` o atkay). `pricing.Reprice` (8.5) diye notun line total, discount er bhag, fulfillment ar order er total. Reprice coupon er shorto abar check kore na (min_order na), delivery charge ager ta-i rakhe, percent coupon notun subtotal e, fixed coupon ager discount (notun subtotal er beshi na). Fole ojon komle customer er total kokhono bare na. v1.4: marketplace e onno vendor er fulfillment er taka bodlay na (8.5).
   - Prepaid order e customer kokhono beshi dibe na: Reprice protiti line ager line total e cap kore, ar parthokko ferot dey (`refunds`, `pending`).
   - COD order e notun total save, ar SMS e final amount.
4. **Order edit** (pack er age): item bad ba quantity kom hole stock ferot, `pricing.Reprice` (`Edit = true`) diye notun hishab, prepaid hole partial refund, history te note. Quantity barano ba notun item jog edit e na (notun order); `Reprice` eta error diye atkay. Ek vendor er shob item bad hole oi fulfillment `cancelled`, shob vendor er hole order o (8.8).
   - v1.5 er kram, ek transaction e: (a) `LineFromItem` diye Original ar notun line (bad deya item `status = 'removed'`, taka 0, quantity ager ta; ojon-variable e `EditWeightG`); (b) `Reprice`; (c) item, fulfillment ar order er notun taka save; (d) prepaid e `refund > 0` hole ekta-i refund row (`kind = 'reprice'`, `fulfillment_id` NULL); (e) je vendor er shob line bad tar fulfillment `TransitionFulfillment(→ cancelled)`: tar total ekhon 0, tai cancel er refund row hoy na; shob vendor gele order o cancel (`AllCancelled`). Fole mot refund ekbar-i, paid er beshi na. Ager version e kram bola chhilo na: cancel age hole duibar refund (B ≤ A hole guard o dhorto na), pore hole 0 taka refund e CHECK fail.
   - v1.5: je edit e kono vendor er shob item bad hoy, ba prepaid e refund toiri hoy, sheta shudhu `owner`/`admin` (11.2); `staff` role e `403 forbidden`, karon staff cancel ba refund korte pare na. v1.7: fulfillment transition eo ekoi niyom (cancel, ba refund wala return; 8.8).
5. Packing slip ar invoice website er printable page theke (A4 ar 80mm thermal er jonno alada print CSS). Marketplace e packing slip fulfillment onujayi (`SB-100245-2`).
6. **Courier parcel ferot**: shipment `returned`. Staff abar pathale notun shipment (ek fulfillment e ek shomoy ekta-i active), na hole fulfillment `returned` (8.8).

### 8.12 Inventory

| Ghotona | Ki hobe |
|---|---|
| Notun maal ashlo | `inventory_batches` (received = remaining), `variant.stock + qty`, movement `purchase` |
| Nosto/kom | Batch ar `variant.stock` theke komano, movement `wastage` ba `adjust` (note lagbe) |
| Order | `variant.stock` checkout e kome, batch kome packing e |
| Cancel/return | `variant.stock + qty`; pack hoye thakle `order_item_batches` theke batch e ferot (perishable return hole `wastage`) |
| Expire (protidin job, shop er timezone) | Expire hoya batch er `quantity_remaining` 0, variant.stock theke oi qty komano (0 er niche na), movement `expired`. Kom porle open order er jonno alert |
| Mil (protidin job) | `batch_expiry` on thakle `sum(batch remaining) = stock + open fulfillment er qty` mile kina; na mille alert |
| Protidin (job) | Low stock (`stock <= low_stock_at`) ar expiry (setting er din er moddhe, default 7) alert |
| Vendor | Stock variant e, variant ek vendor er. Vendor staff shudhu nijer variant er batch, adjust, wastage; alert oi vendor ke-o jay |

### 8.13 Coupon

| Niyom | Bistarito |
|---|---|
| Code | Save ar khoja duitai `upper()` |
| Shomoy | `starts_at`/`expires_at` (shop er timezone e admin dekhe, UTC te save) |
| Limit | Mot `usage_limit`, ar ek phone e `per_phone_limit` (guest er-o phone ache) |
| Cancel | Delivery er age cancel hole redemption muche `used_count = GREATEST(used_count - 1, 0)` (v1.7) |
| Edit | Fixed coupon er discount eligible subtotal er onupate kome (8.5) |
| Snapshot | v1.4: order e shorto copy (`orders.coupon_*`); pore coupon edit ba bondho holeo purono order er packing/edit er hishab bodlay na (8.5) |
| Hishab | Shudhu `pricing` package (8.5) |
| Vendor | `vendor_id` NULL = shob vendor er line e; thakle shudhu oi vendor er line e, `min_order` o oi line gulor upor |
| M6 (v1.32) | Code `trim` + `upper`, max 32; `409 coupon_invalid` ba `coupon_limit_reached` er karon, quote e read-only: 8.19 item 5 ([08-checkout.md](08-checkout.md)) |
| Taka ke dey | `funded_by`: `shop` (shop/platform bohon kore; vendor er commission discount er ager dam e) ba `vendor` (vendor bohon kore). Single store e parthokko nai |

### 8.14 Review

1. Review dite parbe jodi product ta oi order e thake ar tar fulfillment `delivered`.
2. Duibhabe: login kora customer nijer order theke, ba guest SMS er link theke (`order_token`). Duitai "verified purchase".
3. Ek order e ek product er ekta review (`UNIQUE (order_id, product_id)`). 30 din porjonto edit.
4. Ek-i transaction e `products.rating_sum += rating`, `review_count += 1`. Hide korle biyog, unhide korle abar jog, edit e rating er parthokko. Gor = `rating_sum / review_count`, ek doshomik e dekhano. Review e `vendor_id` (order item theke) o save hoy, pore vendor rating er jonno.
5. Account delete hole `reviewer_name = 'Deleted user'`, review thake.

### 8.15 Delivery deadline

1. Order er shomoy `delivery_deadline` (same city 5 din, onno 10 din). Perishable (fol, mach, shobji) er jonno checkout e aro kom shomoy promise ar dekhano (12.1). v1.32 (M6): M6 e shudhu 5/10 din; perishable er kom shomoy Phase 2 (16.1).
2. Ghontay ekbar job: deadline er 24 ghontar moddhe kintu deliver hoyni emon fulfillment er alert (shop admin ke; marketplace e oi vendor ke-o). Dashboard e lal kore dekhano.
3. Prepaid order 36 ghontar moddhe ship na hole alert (advance payment er 48 ghontar moddhe delivery service e dewar niyom er jonno).
4. Supply na korte parle (stock, expiry): order er 48 ghontar moddhe customer ke janano, ar prepaid hole 72 ghontay refund er alert (12.1).

### 8.16 SMS

| Event | Template key | Kokhon |
|---|---|---|
| OTP | `sms.otp` | Login/checkout OTP |
| Order hoyeche | `sms.order_placed` | Order er por (v1.32: confirm na lagle, `/order/{public_token}` link) |
| Order confirm korun | `sms.order_confirm` | v1.32: order er por, COD confirmation lagle (`sms.order_placed` er bodole, confirm link shoho) |
| Ship hoyeche | `sms.shipped` | Tracking shoho |
| Deliver hoyeche | `sms.delivered` | Review link shoho |
| Final dam bodlalo | `sms.amount_updated` | Ojon-variable COD order |
| Cancel | `sms.cancelled` | v1.32: order number, karon chhara (8.19 item 7) |
| Payment hoyeche | `sms.paid` | Online payment validate hole |
| Refund | `sms.refund` | Refund `done` hole, poriman shoho |

Template gulo translation key (`sms.*`), admin bodlate pare. Customer er `locale` onujayi bhasha. Mone rakho: Bangla SMS Unicode (UCS-2), ek SMS e 70 unit, multipart e protiti 67 (English e 160/153). Juktakkhor ar kar-fola alada unit, tai "okkhor" er cheye beshi gona hoy; template er length code unit e test koro. Template choto rakho, na hole ek SMS e duita-tinta SMS er khoroch hobe.

v1.19 (M4): `sms.otp` er default lekha Go te (`notify` e bn/en), shop er `translations` (`sms.otp`) override kore; `{code}` placeholder, bn ≤ 70 unit. M4 e provider adapter na: local e log, staging/production e `ErrNoProvider` (job fail ar alert); provider thik hole adapter ar `shop_credentials` pora, M12 er age (16.1).

v1.32 (M6, user 15 November 2026): `sms.order_placed` job confirm lagle notun key `sms.order_confirm` ("confirm korun", `/confirm/` link), na hole `sms.order_placed` (`/order/` link); `sms.cancelled` karon chhara; bn ≤ 134 unit (2 part), en ≤ 160; link e primary domain (8.19 item 7, [08-checkout.md](08-checkout.md)).

### 8.17 Background job

| Job | Kokhon chole | Ki kore |
|---|---|---|
| `sms.send` | Event e | SMS gateway e pathay, fail hole 1, 5, 15, 60 minute por abar (OTP hole shudhu 1 minute por ekbar, OTP 5 minute e expire). v1.19: done hole payload `{}` (phone ar lekha PII) |
| `alert.new_order` | Order e | Admin ke email/SMS |
| `alert.security` | Event e (OTP phone lock, staff login 10/50 bhul, `failed_mfa` lock) | v1.19: `dedupe_key` shoho (v1.24: staff e `alert.staff_login_failed:<shop>:<staff>:<10 ba 50>:<din>`, `alert.staff_mfa_lock:<shop>:<staff>:<din>`; v1.26: OTP lock e `alert.otp_lock:<shop>:<phone tag>:<din>`, tag `HMAC-SHA256(OTP_SECRET, "otp_lock:" + phone)` er 16 hex, plain phone na; din shop er timezone e), payload e phone mask ar staff id; handler ekhon `slog.Warn`, M12 e email/webhook (`ALERT_EMAIL`, `ALERT_WEBHOOK_URL`) |
| `order.auto_cancel_unconfirmed` | Order + N ghonta | Ekhono `pending` COD hole cancel |
| `order.auto_cancel_unpaid` | Order + 30 minute | Age reconcile (8.9), tarpor-o unpaid online hole cancel |
| `alert.delivery_deadline` | Ghontay | 8.15 |
| `alert.inventory` | Protidin shokal 8 ta (shop er timezone) | Low stock, expiry |
| `payment.reconcile` | Protiti 10 minute | `reconcile_after` er shomoy hoya payment (local `cancelled` o) er status gateway theke (8.9, v1.4) |
| `inventory.expire`, `inventory.reconcile` | Protidin (shop er timezone) | 8.12 |
| `data.retention` | Protidin | `orders.ip`, `sessions.ip`, `audit_logs.ip` 180 din por `NULL` (`shop_app` er `audit_logs` e shudhu `ip` column e UPDATE, `001`) |
| `cart.cleanup` | Protidin | 30 diner purono guest cart. v1.28: `session_key` thaka cart, `updated_at` 30 din er purono (`carts_updated_idx`); login cart na; din er `dedupe_key` (`jobs.cleanup` er moto). v1.31: shob shop ek `DELETE` e, `shop_id` filter chhara (Rule 3 er chhar: platform job, kichu dekhay na, shudhu muche; `carts_updated_idx` shop chhara); din UTC (Rule 5 er chhar: 30 din e shop timezone er ek din er parthokko nai) |
| `session.cleanup` | Protidin | Expire hoya session ar OTP |
| `jobs.cleanup` | Protidin | v1.19: `done_at` 30 din ar `failed_at` 90 din er purono `jobs` row muche (`dedupe_key` shoho); failed beshi din thake jate alert er por dekha jay. `shop_app` er `jobs` e DELETE ache (`001`), migration na |
| `jobs.sweep` | Protiti 5 minute | Shesh attempt e crash kora job (`attempts = max_attempts`, lock 5 minute er purono) `failed_at` ar admin alert (6.4) |
| `alert.complaint_due` | Ghontay | `complaints.due_at` (72 ghonta) er 12 ghontar moddhe ba par hoye geche emon open complaint (12.1) |
| `backup.verify` | Mashe | Restore test reminder (Section 13) |

Worker niyom: 6.4 er query diye kaj nay (`failed_at IS NULL`, `attempts < max_attempts`), handler idempotent, `max_attempts` chhuyle `failed_at` ar admin alert (crash hole `jobs.sweep`), ek kaj 5 minute er beshi locked thakle abar neya jay. Recurring job scheduler `dedupe_key` (din ba ghonta shoho) diye insert kore, tai duibar hoy na. Multi-tenant e ek shop er onek job jate onno shop ke atke na rakhe, Phase 3 e shop onujayi round-robin.

### 8.18 Vendor er taka (marketplace, Phase 3)

Table gulo `010_marketplace_money.sql` e ashbe (6.6). Kintu hishab er bhitti (`line_total`, `discount_share`, `commission`, fulfillment er taka) ekhon-i `001` e save hoy, tai pore purono order er hishab o thik bhabe kora jabe.

| Ghotona | `vendor_ledger` entry (vendor er dik theke) |
|---|---|
| Fulfillment `delivered` ar return window (`checkout.return_window_days`) shesh | `sale`: + line total er jog (vendor-funded coupon hole `discount_share` bad); `commission`: − commission er jog |
| Vendor nijei deliver korle (`per_vendor`) | `delivery`: + fulfillment er `delivery_charge` |
| Window er por return ba refund | `refund`: − |
| Payout | `payout`: − pathano taka |

Payout = ledger balance, saptahe ekbar, shudhu balance 0 er beshi hole. Ledger row kokhono update ba delete na; bhul hole ulto entry (`adjustment`). Commission `pricing.Commission` (8.5) diye.

Digital Commerce guideline e marketplace ke seller er taka 10 diner moddhe dite hoy, alada chukti na thakle; return window er hold vendor chukti te likho (12.1). Commission er VAT (FY26 theke 15%) ledger e alada line, accountant er sathe thik koro.
