---
paths:
  - "backend/internal/{pricing,order,phone}/**"
---
# pricing, order, phone: code-i spec (8.1, 8.5, 8.8)
- Ei tin package design er asol spec. Niyom bodlate age `docs/design/08-core-flows.md` er lekha (notun bodole "v1.<n>:", `docs/design/README.md` e version line), tarpor code ar test (`design-change` skill).
- Pure function thakbe: database, HTTP, `time.Now`, random na.
- Feature doc: `docs/features/pricing.md`, `order-status.md`, `phone.md`; niyom bodlale "Niyom ar tar test" o.
- Test kokhono durbol korbe na: case muche, golden table bodle ba random case komiye pass koranor jonno na. Test bodlate hole karon ar section bolo.
- `order_items` theke line shudhu `pricing.LineFromItem`; `Reprice`/`Calculate` er `ErrInvalidInput` chupchap gile fela na: caller ROLLBACK ar log kore (8.5). v1.7: `Calculate` er `ErrCouponNotApplicable` → `409 coupon_invalid`; eligible subtotal ba min_order caller nije hishab korbe na.
- Bodoler por `go test -race ./internal/...` er shob golden ar random test pass.
