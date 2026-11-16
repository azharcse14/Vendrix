# Inventory: batch, adjust, variant khoja

Design: 8.12, 11.7 item 1, 2, 6.4, 7.7 (Inventory) · Milestone: M7a · Code: `backend/internal/inventory`

## Ki kore

Admin (owner, admin, staff, vendor_owner, vendor_staff) `/inventory` e variant khoje (naam/SKU, low stock), notun maal ele batch dey (stock barano shudhu eta), nosto ba bhul gona hole stock komay (wastage/adjust, note shoho), ar movement er itihash dekhe. Vendor role shudhu nijer vendor er variant. `staff` ar `vendor_staff` kena dam (`cost_price`) dekhe na.

## Kivabe kaj kore

1. `handler.go` → `Routes`: 5 ta route, role `RequireRole` (shob inventory role); body/query check (`CreateBatch`, `Adjust`, `Variants`, `Movements`, `Batches`), error → 7.1 (`fail`).
2. `service.go` → `CreateBatch`: `db.InTx` e `lockVariant` (vendor scope, na hole `ErrNotFound`), `expiry_date` shop er ajker age hole `FieldError`, `insertBatch` (received = remaining), `addStock` (+qty), `movement` `purchase` `batch_id` shoho, `audit.Log` `inventory.batch`. `batch_expiry` off eo batch row hoy.
3. `service.go` → `Adjust`: `lockVariant` → `stock < qty` hole `OutOfStockError{stock}` → `batchExpiry` (shop er `published.features`). Off: `batch_id` dile `FieldError`, shudhu `variant.stock`. On: `batch_id` dile `lockBatch` (onno variant er hole `ErrNotFound`, kom hole `OutOfStockError{remaining}`) + `takeBatch`; na dile `Allocate` (FIFO), batch e na pawa ongsho purono stock (batch chhara movement). Tarpor `addStock` (−qty), protiti ongsher movement (`wastage`/`adjust`, note, staff), `audit.Log` `inventory.adjust` (before/after `stock`).
4. `store.go` → `Allocate`: variant lock er por batch gulo `id` kram e `FOR UPDATE` (deadlock na), tarpor 6.4 er FIFO (`expiry_date NULLS LAST, received_at`, expire batch bad) e qty porjonto komay; `rest` ferot. Pack (M7a step 5a) eo eta use korbe.
5. List: `Variants` (`listVariants`: `batch_remaining`, `next_expiry`), `Batches` (`listBatches`, FIFO kram), `Movements` (`listMovements`, batch er `cost_price` join); `hideCost` staff/vendor_staff e `cost_price` muche.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/inventory/handler.go` → `Routes`, `CreateBatch`, `Adjust`, `Variants`, `Batches`, `Movements`, `fail` | HTTP, input check, error map |
| `backend/internal/inventory/service.go` → `CreateBatch`, `Adjust`, `Variants`, `Batches`, `Movements`, `hideCost`, `vendorScope` | Niyom, transaction, audit |
| `backend/internal/inventory/store.go` → `lockVariant`, `lockBatch`, `Allocate`, `insertBatch`, `movement`, `listVariants`, `listBatches`, `listMovements` | SQL (static, Rule 12) |
| `backend/cmd/api/router.go` | `inventory.NewHandlers(d, log).Routes(mux, staff)` |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Variant khoja: `q` naam (bn/en) ba SKU, `low=1` = `stock <= low_stock_at`, `batch_remaining`, `next_expiry`, dam nai; vendor role nijer (11.7 item 1, Rule 13) | `listVariants`, `Variants` | `TestInventoryVariantsIntegration` |
| `staff`/`vendor_staff` e batch ar movement e `cost_price` nai, purchase e pathate pare (11.7 item 1) | `hideCost` | `TestInventoryVariantsIntegration`, `TestBatchCreateIntegration`, `TestInventoryAdjustIntegration` |
| Batch: received = remaining, `variant.stock + qty`, movement `purchase` `batch_id` shoho, audit, ek tx; expiry ≥ harvest, expiry shop er ajker age na (8.12, 11.7 item 2) | `CreateBatch` | `TestBatchCreateIntegration` |
| Adjust shudhu komano (`change < 0`), reason `wastage`/`adjust`, note 1–500; stock er beshi `409 out_of_stock {available}`, kichu likha na (11.7 item 2) | `Adjust` (handler + service) | `TestInventoryAdjustIntegration` |
| `batch_expiry` on: `batch_id` ba FIFO (expire batch bad), baki purono stock; off: shudhu `variant.stock` (6.4, 11.7 item 2) | `Adjust`, `Allocate` | `TestInventoryAdjustIntegration` |
| Lock: variant tarpor batch `id` kram; checkout ar wastage ekshathe stock ≥ 0, deadlock na (11.7 item 2) | `lockVariant`, `Allocate` | `TestCheckoutWastageRaceIntegration` |
| Route role (11.2) | `Routes` | `TestAdminRouteRoles` |

## Data

`inventory_batches` (`quantity_received`, `quantity_remaining`, `cost_price`, `harvest_date`, `expiry_date`, `batch_code`), `inventory_movements` (`reason` `purchase`/`wastage`/`adjust`, `batch_id`, `staff_id`, `note`), `product_variants.stock`/`low_stock_at`, `shop_settings.published.features` (`batch_expiry`), `audit_logs`; `001_init.sql`, migration lage na.

## Bhul hole ki hoy

- Bhul body/query: `400 validation_failed` + `fields` (`variant_id`, `quantity` 1–1,000,000, `cost_price`, `batch_code` max 64, `harvest_date`, `expiry_date` (`invalid` ba `expired`), `change`, `reason`, `note`, `batch_id` (off e `batch_expiry_off`), `q` max 100, `low`).
- Variant/batch nai, onno shop ba onno vendor er: `404 not_found`.
- Stock ba batch e kom: `409 out_of_stock` `details {available}`; transaction rollback, kichu likha na.

## Bodlanor age mone rakho

- Notun stock code (pack, expire job, restock) eo lock er kram variant → batch `id`; `Allocate` er FIFO 6.4 er sathe mile.
- `cost_price` notun response e dile `hideCost` lagao.
- Route bodlale `backend/cmd/api/main_test.go` er `adminRoutes`.

## Proshno-uttor

- **Stock barate adjust e `+` na keno?** Notun maal shob shomoy batch e (kena dam, expiry, recall); bhul gona thik korte notun batch (`batch_code` "count").
- **Wastage batch er stock komay, kintu order er stock?** Checkout shudhu `variant.stock` komay; batch kome packing e. Tai batch er mot `stock` er beshi thakte pare.
- **`variant_label` ki?** Variant er `options` JSON hubohu (`{"size": {"bn", "en"}}`); admin UI lekha banay.
