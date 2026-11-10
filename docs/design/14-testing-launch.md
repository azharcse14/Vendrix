> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 14. Testing ar limited launch plan

### 14.1 Test er stor

| Stor | Ki | Tool |
|---|---|---|
| Unit | `pricing`, `phone`, state machine, discount, risk rule, contrast, money format | Go `testing`, Dart `test` |
| Integration | Asol Postgres e store ar service (transaction, constraint) | Go `testing` + Docker Postgres |
| API | Handler er request/response, error code, auth, rate limit | `httptest` |
| Component | Jaspr component, Flutter bloc | `jaspr_test`, `flutter_test` |
| End-to-end | Staging e puro kenakata | Playwright |
| Manual | Device, Facebook browser, payment | Checklist (14.3) |

### 14.2 Je test gulo obosshoi thakbe

1. **Stock race**: stock 1, ek shathe 10 ta checkout → thik 1 ta order, baki `out_of_stock`, stock 0 (minus na).
2. **Coupon limit**: `usage_limit = 1`, ek shathe 5 checkout → thik 1 ta discount.
3. **Phone limit**: ek phone e `per_phone_limit` er beshi use na.
4. **Idempotency**: ek-i key e duibar (ek shathe-o, shesh stock e) `POST /api/orders` → ek-i order, ek bar stock kome, duplicate ta `409` pay na. Ekoi key e alada body → `422`. Prothom ta commit howar por response harano retry (cart ekhon khali) → ager order `200`, `422` na (v1.4).
5. **Shop isolation**: shop A er token diye shop B er order, product, settings pawa jay na.
6. **IPN**: fake IPN (validation API `INVALID` ferot dey) → paid hoy na. Amount mile na → paid hoy na. Ek-i IPN duibar (ek shathe-o) → ek bar. Validation timeout → payment `failed` na. Auto-cancel er por IPN → refund row. Duita payment success → ekta auto refund. IPN na ashle reconcile paid kore. Purono tab er payment (local `cancelled`, `/pay` retry) pore success hole IPN ar return na ashleo reconcile dhore (v1.4). v1.7: fake success return (bhul `val_id`, ba onno payment er `val_id` + ei `tran_id`) ar failed attempt er IPN e payment bodlay na, `reconcile_after` thake, porer asol payment reconcile dhore.
7. **State machine**: shob na-allowed bodol `409`. Online unpaid order e customer ba staff confirm → `409 payment_required`. v1.4: unit test e shob `(from, to, actor)` er golden table (order e payment ar fulfillment er obostha shoho).
8. **Cancel**: stock, batch, coupon shob ferot. Paid order cancel e thik ekta refund protiti fulfillment e, order level e alada na.
9. **Ojon-variable**: prepaid e beshi ojon → customer beshi dey na; kom ojon → refund row.
10. **OTP/TOTP**: 5 bar bhul er por block (error path e attempt thake, parallel request e-o), expire er por kaj kore na. TOTP ekoi code duibar na. Temp_token bodle-o `failed_mfa` 10 e lock. Ek phone e dine 10 OTP er beshi na, 24 ghontay 10 bhul e lock. v1.7: ekoi code e duita parallel verify → ekta-i session.
11. **Guest → account**: OTP er por-i shudhu purono order jog hoy.
12. **Translation fallback**: English na thakle Bangla.
13. **Tenant foreign key**: shop A er variant, vendor, category diye shop B er cart, product, order item insert → database error.
14. **Vendor isolation**: vendor X er staff vendor Y er product, fulfillment, stock pay na (`404`), list e o dekhe na.
15. **Taka er bhag**: random line, coupon, delivery e line `discount_share` er jog = order discount, fulfillment gulor jog = order er taka (ek poisha o kom-beshi na). Database e save korar por-o mile.
16. **Summary**: single store e `display_status` v1.0 er status er moto; marketplace e mixed hole `partial`; shob fulfillment cancel hole order `cancelled`.
17. **Mode bodol**: single store er shop `marketplace` kore notun vendor jog → purono order, report, packing thik thake.
18. **Client IP**: website diye 11 ta order alada IP theke → `429` na; ekoi IP theke 11 ta → `429`. Admin ar app (Caddy diye) er request o alada IP pay. Secret chhara `X-Client-IP` ar client er boshano `X-Real-IP` mane na.
19. **Session**: customer token admin route e `401`; staff temp_token (`mfa_pending`) diye admin API `401`. `setup_pending` session diye setup endpoint chhara baki shob `403 setup_required`.
20. **Router**: shob route register e panic nai.
21. **Batch**: expire hoya batch FIFO te ashe na; batch na pele pack hoy na, alert.
22. **Edit**: item bad e fixed coupon onupate kome (৳900 + ৳100 → ৳100 er item free na); prepaid e total bare na.
23. **Turnstile**: OTP resubmit e notun token lage na; retry (ekoi key, Turnstile token bodlaleo) ager order pay, karon hash e token nai. `otp_code` thakle shob shomoy verify, tai bhul code diye Turnstile edano jay na.
24. **Cache**: cookie thaka page e `Cache-Control: private, no-store`.
25. **Database**: `tests/001_init_test.sql` shob pass (83/83), protiti reject nirdishto constraint er karone.
26. **Cancel cascade**: customer cancel, staff cancel (packed shoho), `auto_cancel_unconfirmed` ar `auto_cancel_unpaid` er por kono cancelled order e `open` ba `packed` fulfillment thake na; stock, batch, coupon ferot.
27. **Pricing input**: ojon-variable e `WeightG` 0, quantity 0 ba minus, coupon kind er bhul banan → `ErrInvalidInput`; edit e quantity barano → error.
28. **Doc sync**: `backend/internal/docsync` test pass (v1.6: doc er link er file ache, protiti doc file 60 KB er niche, 80 line er beshi code block nai, section thik file e, README ar AGENTS.md e protiti file).
29. **Upload**: 30 second er beshi shomoy newa upload er-o response ashe (write deadline).
30. **Payment lock**: ek order e IPN ar `/pay` retry ek shathe ashle deadlock nai (order → payment).
31. **COD confirm** (v1.4): `public_token` diye confirm er route nai; SMS er `confirm_token` diye-i confirm; online order e `confirm_token_hash` DB atkay; SMS job retry te ekoi link.
32. **Reprice freeze** (v1.4): marketplace e ek vendor er packing ba edit e onno vendor er line, discount er bhag, delivery ar fulfillment total hubohu ager; kichu na bodlale `Reprice` ager result-i dey; dui dhape-o prepaid e total bare na.
33. **Coupon snapshot** (v1.4): order er por coupon edit ba bondho korleo packing/edit er hishab ek-i; snapshot chhara `Reprice` error.
34. **Edit e vendor er shob item bad** (v1.5): prepaid e refund ekbar-i (`kind = 'reprice'`), fulfillment cancel e 0 taka refund row na, mot refund ≤ paid (A + B te B puro bad, B ≤ A holeo); single store e shob bad → order `cancelled`, mot refund = paid.
35. **Edit er role** (v1.5): `staff` role e kono vendor er shob item bad ba refund toiri kora edit → `403`; owner/admin pare. v1.7: `staff` role e fulfillment cancel, ba refund toiri kore emon return → `403`; refund chhara return (COD, taka newa hoyni) pare.
36. **Removed item** (v1.5): edit e bad deya item er por packing e item abar bill e ashe na; DB er quantity diye line banale `Reprice` `ErrInvalidInput` (`TestLineFromItem`).
37. **Settings link** (v1.5): banner/social e `//x`, `/\x`, tab/newline shoho link, `javascript:` → `400`.
38. **Token page** (v1.5): `/order/*`, `/confirm/*`, `/invoice/*`, `/print/*` e Pixel/GA4 ba third-party script nai.
39. **Coupon khate na** (v1.7): `min_order` er kom, ba coupon er eligible line er subtotal 0 (vendor coupon er line nai) → `409 coupon_invalid`, `used_count` ar redemption bodlay na (ROLLBACK); quote e coupon chhara total ar `coupon_invalid` (`TestCouponNotApplicable`).
40. **Checkout OTP** (v1.7): OTP thik kintu `price_changed` (ba `out_of_stock`) → ekoi code e abar submit e order hoy (consume ROLLBACK e ferot); order hoye gele ekoi code e onno key er order `400 otp_invalid`.
41. **Edit → pack** (v1.7): random order e edit, tarpor vendor onujayi pack (asol ojon kom/beshi), protiti dhap `LineFromItem` diye row theke: bhag er jog mile, frozen vendor bodlay na, removed item e taka 0, prepaid e mot refund = paid − shesh total (`TestRepriceEditThenPackRandom`).
42. **Migration checksum** (v1.7): applied migration file er ek byte bodlale `migrate` fail; shesh applied er cheye choto number er notun file e o fail; `app.env` `local` chhara `shop_app` na thakle `001` fail.

### 14.3 Launch er age manual checklist

1. Sosta Android, Facebook in-app browser, iPhone Safari, desktop e puro kenakata: browse → cart → guest checkout → COD → confirm link → admin e pack → ship → deliver → review link.
2. SSLCommerz sandbox: success, fail, cancel, payment page e giye browser bondho, IPN dheri, IPN block (reconcile), duita tab e duibar payment, Facebook in-app browser e redirect ar ferot. Cloudflare er Bot Fight Mode off (ba Super Bot Fight Mode e IPN skip) rekhe IPN ashe kina.
3. Coupon: valid, expire, limit shesh, min order.
4. Stock shesh hole cart ar checkout e ki dekhay.
5. Ojon-variable product pack kore final dam ar SMS.
6. Bangla ar English, duita bhashay shob page, 360px e.
7. Admin theme bodle preview → publish → 1 minute e website e.
8. Settings rollback.
9. Backup theke restore.
10. Policy page (Bangla), DBID, BFSA registration number dekha jay (1 November 2026 theke badhyotamulok).
11. Facebook Sharing Debugger e product link er preview.
12. Lighthouse mobile lokkho (10.5) pass.
13. `www` → apex redirect, cookie thaka page Cloudflare e cache hoy na, asol IP diye rate limit (website, admin ar app tinta-tei).

### 14.4 Limited launch

1. **Ke**: bondhu, family, tarpor tader recommend kora manush, mot 50–200 jon. Invite link pathao. `seo.noindex = true` ar sitemap submit na, tai link chhara keu khuje pay na; public launch e `noindex` off.
2. **Kothay**: ekta shohor ba kichu area (delivery zone e shudhu oigulo `is_active`).
3. **Ki**: 20–100 ta product, beshi bikri hobe emon diye shuru.
4. **Feedback**: ekta WhatsApp/Messenger group, ar order er por choto survey.
5. **Protidin dekho**: order, conversion (visit → order), cart e rekhe chole jawa, COD refuse rate, delivery success rate, deadline miss, return, error/crash, page speed.
6. **Phase 2 te jabar shorto** (udahoron, nije thik koro): tana 3–4 shoptaho kono critical bug nai, delivery success 90% er beshi, payment/hishab e kono gormil nai, deadline miss prai shunno.
