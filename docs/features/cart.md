# Cart ar wishlist

Design: 8.4 (v1.28), 7.4 · Milestone: M5 · Code: `backend/internal/cart`

## Ki kore

Customer (login chhara-o) cart e variant rakhe; dam protibar bortoman (cart e dam save hoy na). Ekhon shudhu pure niyom: guest cart er key, quantity er shima, cart e variant er limit, `GET` er normalize. API, merge, wishlist ar `cart.cleanup` M5 er porer step e.

## Kivabe kaj kore

1. Guest key: notun cart e `NewKey` (`rand.Text`, 26 okkhor base32) key ar tar `sha256` dey; DB te shudhu hash. Client er pathano key `KeyHash` e: `^[A-Z2-7]{26}$` na mille "nai" (notun cart).
2. Add/patch: `CheckQuantity(qty, stock)`: 1 er kom `ErrInvalidQuantity`, `MaxQuantity(stock)` = `min(stock, 50)` er beshi (stock 0 o) `ErrOutOfStock`. Notun variant e `CheckRoom(lines, inCart)`: 50 line hole `ErrCartFull`.
3. `GET`: `NormalizeQuantity(qty, stock)`: stock > 0 ar qty beshi hole `min(stock, 50)` ar `true` (notice `quantity_reduced`); stock 0 e qty thake.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/cart/service.go` → `NewKey`, `KeyHash` | Guest key banano ar format check, `sha256` |
| `backend/internal/cart/service.go` → `MaxQuantity`, `CheckQuantity`, `CheckRoom`, `NormalizeQuantity` | Quantity, line limit, normalize |
| `backend/internal/cart/cart_test.go` | Pure niyom er test |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Guest key Go banay, `^[A-Z2-7]{26}$`, DB te `sha256`; bhul format "nai" (8.4 item 1) | `NewKey`, `KeyHash` | `TestCartKeyFormat` |
| `max_quantity` = `min(stock, 50)` (8.4 item 3, 4) | `MaxQuantity` | `TestMaxQuantity` |
| Add/patch e 1..`min(stock, 50)`, chupchap komay na (8.4 item 3) | `CheckQuantity` | `TestCheckQuantity` |
| Ek cart e max 50 variant (8.4 item 3) | `CheckRoom` | `TestCheckRoom` |
| `GET` e stock komle qty komay, stock 0 er item thake (8.4 item 4) | `NormalizeQuantity` | `TestNormalizeQuantity` |

## Data

`carts` (`session_key` = key er `sha256`), `cart_items` (`quantity > 0`), `001_init.sql`. Ekhon er code DB chhoy na.

## Bhul hole ki hoy

`ErrInvalidQuantity` → `400 validation_failed`; `ErrOutOfStock` → `409 out_of_stock` + `details {variant_id, max_quantity}`; `ErrCartFull` → `409 cart_full` (7.1). Handler er mapping M5 step 2 e. Bhul format er key error na, notun cart.

## Bodlanor age mone rakho

- 50 er shima (`MaxLineQuantity`, `MaxLines`) 8.4 er niyom; bodlate `design-change`, website/l10n er lekha o.
- Key format bodlale website er `__Host-cart` (10.12) ar purono cart er key ochena hoye jabe.

## Proshno-uttor

- **Bhul key e error na keno?** Client er key ochena hole notun cart; onner cart kokhono pay na (8.4 item 1). Cookie expire ba bhul hole customer notun cart e add korte pare.
