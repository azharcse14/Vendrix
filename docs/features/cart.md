# Cart ar wishlist

Design: 8.4 (v1.28–v1.31), 7.4, 7.3, 8.2 step 6 · Milestone: M5 · Code: `backend/internal/cart`

## Ki kore

Customer (login chhara-o) cart e variant rakhe; dam protibar bortoman (cart e dam save hoy na). Ekhon cart API: `GET /api/cart`, `GET /api/cart/count` (badge), `POST /api/cart/items`, `PATCH`/`DELETE /api/cart/items/{variant_id}`. Login thakle account cart, na hole `X-Cart-Key` er guest cart. OTP login e guest cart account cart e merge hoy. Wishlist (login chay): `GET /api/wishlist`, `POST`/`DELETE /api/wishlist/{product_id}`, `POST /api/wishlist/merge` (guest `wl` cookie er ID). `cart.cleanup` job protidin 30 din purono guest cart muche.

## Kivabe kaj kore

1. Router: `ShopFromHost` → `RateLimit` (120/minute) → `httpx.OptionalCustomer`. Bearer thakle account cart (key ignore); bhul/expire Bearer `401`. Bearer na thakle `X-Cart-Key` → `KeyHash` (`^[A-Z2-7]{26}$` na mille "nai").
2. `GET` (`View`): cart row na thakle khali cart, row banay na. Thakle: bondho product/variant ba active na vendor er item ek `DELETE ... RETURNING` e muche (notice `item_removed`, naam bortoman bhashay); `NormalizeQuantity` e stock komle qty komay (DB te o, notice `quantity_reduced`); stock 0 er item thake (`in_stock: false`). Line total `pricing.LineTotal`, subtotal `pricing.Calculate` shudhu in_stock line er.
3. `POST` (`Add`): transaction e cart row `UPDATE ... updated_at = now() RETURNING` (lock); na thakle notun cart: account e `ON CONFLICT`, guest e `NewKey` (response e `cart_key`; client er key diye cart kokhono na). Ekoi variant abar add e qty jog (jog er age `quantity` 51 e cap, tai boro shonkhya overflow hoye `400` na, `409`); jog `CheckQuantity` (1..`min(stock, 50)`), notun variant e `CheckRoom` (50 line).
4. `PATCH` (`SetQuantity`): cart e thaka variant er qty set; cart/item na thakle `404`. `DELETE` (`Remove`): idempotent `200`.
5. Protiti route puro cart ferot dey, `Cache-Control: no-store`. `GET /api/cart/count` (`Count`, `countLines`): owner ekoi, shudhu `SELECT count(*)` (normalize na, row banay na, notice na; bondho item o gone, porer `GET /api/cart` muche) → `{count}`, `no-store` (7.4 v1.31).
6. Merge (`Merge`, `auth.verifyOTP` er transaction theke, `X-Cart-Key` shoho): `KeyHash` na mille kichu na. Age guest cart `FOR UPDATE` (`lockGuestCart`; na pele kichu na), tarpor account cart upsert (`createCart`, row lock) — shob merge ek kram e lock kore, tai deadlock na, ar parallel merge e duita-r ekta guest cart pay na. Guest er line add er kram e: bondho bad (notice na); ekoi variant e qty jog, `NormalizeQuantity(min(jog, 50), stock)` (stock 0 e `min(jog, 50)`); notun variant e `CheckRoom` fail hole `item_dropped`. Shesh e guest cart row muche (item CASCADE). Notice verify er response er `cart_notices` e (7.3 v1.30).
7. Wishlist: router e public + `RequireCustomer` (Bearer na/bhul `401`). Protiti query `shop_id` ar `user_id` diye (Rule 16). `GET` (`wishCards`): `catalog.Card` list, shudhu active product (active variant ache, vendor active), notun age, max 200; bondho product er row thake, chalu hole abar dekhay. `POST` (`AddWish`): user row `FOR NO KEY UPDATE` (`lockWishlist`, parallel e 200 na bhange); ager product e kichu na; notun e `activeProducts` (onno shop er ID o `404`), 200 thakle `409 wishlist_full` (limit e lukano row o gona). `DELETE` (`RemoveWish`) idempotent. Merge (`MergeWish`): prothom 50 ID, shudhu oi shop er active product, request er kram e mot 200 porjonto; ochena/bhul ID chupchap bad. `POST`/`DELETE`/merge o `GET` er list ferot dey, `Cache-Control: no-store`.
8. `cart.cleanup` (`CleanupJob`, worker protidin, [jobs](jobs.md)): `updated_at` 30 din er purono guest cart (`user_id IS NULL`) ek `DELETE` e, shob shop er (platform job, `carts_updated_idx`); item CASCADE. Login cart kokhono na. Guest key er 30 din `__Host-cart` er `Max-Age` er sathe mile.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/cmd/api/router.go` → `newHandler` | Cart route: public + `OptionalCustomer`; wishlist: public + `RequireCustomer` |
| `backend/internal/cart/handler.go` → `Routes`, `OwnerFrom`, `Get`, `Count`, `Add`, `Patch`, `Delete`, `fail`, `reply` | HTTP, body (`DisallowUnknownFields`), error mapping, puro cart response |
| `backend/internal/cart/service.go` → `View`, `Add`, `SetQuantity`, `Remove`, `checkVariant`, `line`, `label` | Niyom: normalize, notice, pricing, qty/limit, transaction |
| `backend/internal/cart/service.go` → `Merge`; `backend/cmd/api/router.go` → `cartMerge`; `backend/internal/auth/service.go` → `verifyOTP` | Login e guest cart merge (verify er tx e; router `auth.CartMerge` hishebe dey, import cycle er karone) |
| `backend/internal/cart/service.go` → `CheckoutLines`; `store.go` → `checkoutRows` | Checkout quote ar order (M6) er read-only line: kena jay kina (`out_of_stock`, `vendor_unavailable`), normalize na; `OwnerFrom` `order` o use kore |
| `backend/internal/cart/service.go` → `Clear`, `Label`; `store.go` → `clearItems` | Order er tx e owner er cart khali (8.6 step 17; cart row thake); `Label` `order_items.variant_label` er snapshot o banay |
| `backend/internal/cart/service.go` → `NewKey`, `KeyHash` | Guest key banano ar format check, `sha256` |
| `backend/internal/cart/service.go` → `MaxQuantity`, `CheckQuantity`, `CheckRoom`, `NormalizeQuantity` | Quantity, line limit, normalize (pure) |
| `backend/internal/cart/store.go` → `findCart`, `countLines`, `touchCart`, `createCart`, `activeVariant`, `itemState`, `setItem`, `deleteItem`, `removeInactive`, `reduceItem`, `items`, `lockGuestCart`, `guestItems`, `quantities`, `deleteCart` | SQL, protiti query `shop_id` diye |
| `backend/internal/cart/handler.go` → `WishlistRoutes`, `GetWishlist`, `AddWishlist`, `DeleteWishlist`, `MergeWishlist`; `service.go` → `AddWish`, `RemoveWish`, `MergeWish`; `store.go` → `wishCards`, `lockWishlist`, `activeProducts`, `insertWish`, `deleteWish` | Wishlist API (router e public + `RequireCustomer`) |
| `backend/internal/cart/service.go` → `CleanupJob`; `store.go` → `deleteStaleGuestCarts`; `backend/cmd/api/main.go` → `runServe` | `cart.cleanup` job handler (main worker e jog kore) |
| `backend/internal/httpx/session.go` → `OptionalCustomer` | Bearer optional, thakle `RequireCustomer` |
| `backend/internal/cart/cart_test.go`, `cart_integration_test.go`, `wishlist_integration_test.go` | Pure niyom ar asol Postgres e API test |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Guest key Go banay, `^[A-Z2-7]{26}$`, DB te `sha256`; bhul format "nai" (8.4 item 1) | `NewKey`, `KeyHash` | `TestCartKeyFormat` |
| Notun guest cart e `cart_key`; GET row banay na; ochena key e Go er notun key; Bearer e account cart, key ignore; bhul/expire Bearer `401` (8.4 item 1, 2) | `Add`, `View`, `OwnerFrom`, `OptionalCustomer` | `TestCartKeyIntegration`, `TestOptionalCustomer` |
| `max_quantity` = `min(stock, 50)` (8.4 item 3, 4) | `MaxQuantity` | `TestMaxQuantity` |
| Add/patch e 1..`min(stock, 50)`, chupchap komay na; `409 out_of_stock` + `details`; abar add e jog, boro qty e o `409` (overflow na) (8.4 item 3, 7.4 v1.31) | `CheckQuantity`, `checkVariant`, `Add` | `TestCheckQuantity`, `TestCartQuantityIntegration` |
| Badge er count: alada line, normalize/notice na, row banay na, onno shop e 0, expire Bearer `401`, `no-store` (7.4 v1.31) | `Count`, `countLines` | `TestCartCountIntegration` |
| Ek cart e max 50 variant, `409 cart_full` (8.4 item 3) | `CheckRoom` | `TestCheckRoom`, `TestCartQuantityIntegration` |
| `GET` e stock komle qty komay + `quantity_reduced`, stock 0 er item thake, subtotal e na (8.4 item 4) | `NormalizeQuantity`, `View` | `TestNormalizeQuantity`, `TestCartQuantityIntegration` |
| Dam bortoman, subtotal `pricing` diye; body te dam field `400` (8.4 item 5, Rule 1) | `View`, `line`, `DecodeJSON` | `TestCartFreshPriceIntegration`, `TestCartRejectsPriceField` |
| Bondho product/variant, active na vendor: `GET` e muche + `item_removed` ekbar; add e `404` (8.4 item 4, Rule 13) | `removeInactive`, `activeVariant` | `TestCartInactiveRemovedIntegration` |
| Onno shop er key/session/variant chhoy na (Rule 3, 14.2 test 5) | protiti query `shop_id` | `TestCartShopIsolationIntegration` |
| Merge: qty jog `min(jog, 50, stock)`, stock 0 e `min(jog, 50)`, bondho bad, guest cart muche, 50 variant er beshi e account er item thake + guest er notun kram e, baki `item_dropped`; parallel e ekbar; onno shop/bhul format key e kichu na (8.4 item 6, v1.29) | `Merge`, `lockGuestCart` | `TestCartMergeIntegration` |
| Wishlist: login chhara `401`; query `user_id` + `shop_id` (onno user/shop er product na, IDOR na); duibar add e ek row; bondho product `GET` e lukano (row thake); 200 e notun add `409 wishlist_full`, parallel e o (8.4 item 8, 7.4, Rule 16) | `WishlistRoutes`, `AddWish`, `lockWishlist`, `wishCards` | `TestWishlistIntegration` |
| Wishlist merge: prothom 50 ID, shudhu oi shop er active product, mot 200; bhul body `400` (8.4 item 8, 10.12) | `MergeWish`, `MergeWishlist` | `TestWishlistMergeIntegration` |
| `cart.cleanup`: 30 din er purono guest cart ar item muche (shob shop), login cart ar notun guest cart thake; idempotent (8.17 v1.28, 8.4 item 7) | `CleanupJob`, `deleteStaleGuestCarts` | `TestCartCleanupIntegration` |
| Merge verify er transaction e (bhul code e na), response e `cart_notices` (khali `[]`) (8.2 step 6, 7.3 v1.30) | `verifyOTP`, `VerifyOTP` | `TestVerifyMergesCartIntegration` |
| Merge er por transaction e bhul hole puro verify rollback (guest cart, OTP, user, session thake na) (8.2 step 6) | `verifyOTP` (`db.InTx`) | `TestVerifyMergeFailRollsBackIntegration` |
| Order hole oi owner er cart khali (onno cart e hat na); checkout bhul hole cart thake (8.6 step 17) | `Clear` (`order.PlaceOrder` er tx e) | `TestCheckoutSnapshotIntegration`, `TestCheckoutValidationIntegration` |

## Data

`carts` (`session_key` = key er `sha256`; `user_id` thakle account cart), `cart_items` (`quantity > 0`), `001_init.sql`. Merge e guest `carts` row muche (`cart_items` CASCADE), account cart upsert. Write e `carts.updated_at = now()` (8.4 item 7, `cart.cleanup` er jonno). Dam/stock `product_variants` theke protibar. `wishlists` (PK `(shop_id, user_id, product_id)`, `ON CONFLICT DO NOTHING`); product bondho holeo row muche na.

## Bhul hole ki hoy

`ErrInvalidQuantity` ba bhul body → `400 validation_failed`; `OutOfStockError` → `409 out_of_stock` + `details {variant_id, max_quantity}`; `ErrCartFull` → `409 cart_full`; `ErrNotFound` → `404 not_found` (7.1); `ErrWishlistFull` → `409 wishlist_full`. Wishlist merge e bhul ID error na (bad); `product_ids` na thakle ba number na hole `400 validation_failed`. `PATCH` e cart ba oi variant cart e na thakle `404 not_found` (add na; `DELETE` e kintu `200`). Bhul format er key error na, notun cart. Ojon-variable variant e `nominal_weight_g` NULL hole add `409 out_of_stock` (`max_quantity` 0), cart e thakle `in_stock: false`.

## Bodlanor age mone rakho

- 50 er shima (`MaxLineQuantity`, `MaxLines`) 8.4 er niyom; bodlate `design-change`, website/l10n er lekha o.
- Key format bodlale website er `__Host-cart` (10.12) ar purono cart er key ochena hoye jabe.
- Dam/total er hishab shudhu `pricing` e; cart e kokhono dam save korbe na.
- Cart e notun write path holeo `updated_at = now()` koro, na hole active guest cart `cart.cleanup` e muche jabe.

## Proshno-uttor

- **Bhul key e error na keno?** Client er key ochena hole notun cart; onner cart kokhono pay na (8.4 item 1). Cookie expire ba bhul hole customer notun cart e add korte pare.
- **Ekoi variant abar add korle?** Qty jog (product page e duibar "Cart e add" = 2), jog o `min(stock, 50)` er beshi hole `409`. Set korte `PATCH`.
- **Merge e bad hoya item er notice porer `GET` e ashe na keno?** Tar jonno state (column) lagto; notice verify er response er `cart_notices` e, website login form e ek line (10.12 item 2, v1.30).
- **Wishlist merge e 50 er beshi ID hole?** Prothom 50 nei, baki chupchap bad (`wl` cookie max 50; merge fail hole login fail na, 10.12).
- **Wishlist er 200 e bondho product gona hoy keno?** Row DB te thake (chalu hole ferot), tai limit row er; GET e kom dekhale-o notun add `409` hote pare.
- **`PATCH` e item na thakle `404`, `DELETE` e `200` keno?** `PATCH` qty set kore, nai emon line chupchap banale add er niyom (`CheckRoom`, notun cart) baad pore; `DELETE` er fol (line nai) already shotti, tai idempotent.
- **`variant_label` ki?** Variant er `options` er value gulo key er kram e, bortoman bhashay, koma diye.
- **Badge `GET /api/cart` na dake keno?** `GET /api/cart` normalize kore notice ekbar dey; product page er badge dakle notice oikhane harato. Tai badge `GET /api/cart/count`, notice shudhu `/cart` page e (7.4 v1.31, 10.12 item 5).
- **Stock 0 line er `line_total` 0 na keno?** Hishab kora dam dekhay (stock ferot ele ki lagbe), kintu `in_stock: false`, tai subtotal e na (7.4 v1.31).
