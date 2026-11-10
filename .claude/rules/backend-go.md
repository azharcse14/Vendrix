---
paths:
  - "backend/**/*.go"
---
# Go backend (Section 1.4, 2.2, 4.2)
- Module `vendrix/backend`. Shudhu `net/http` + standard library + `github.com/jackc/pgx/v5` (`database/sql` er maddhome). Notun dependency lagle age jiggesh koro (1.5 decision log).
- Notun package e `doc.go`: package comment e ki kore ar `Feature doc: docs/features/<naam>.md` (docsync dekhe). Notun env: `internal/config` + `backend/env.example` + 13.5 + test.
- HTTP er common kaj `internal/httpx` e (`WriteError` 7.1 format, `DecodeJSON`, `IP(ctx)`); transaction `db.InTx` (40P01/40001 e ekbar retry).
- Taka `int64` poisha, float ba NUMERIC na. Dam, discount, delivery, total er hishab shudhu `pricing` package e (Rule 1, 2, 8.5). Client er pathano dam ba total kokhono bishshash na; `expected_total` shudhu tulona (8.6).
- Protiti query te `shop_id = $n`; vendor staff e `vendor_id`; customer er address, order, wishlist e `user_id` (Rule 3, 13, 16). Shop ar vendor ashe context theke (3.2), request body theke na.
- SQL shudhu `$n` parameter diye; `fmt.Sprintf` ba string jora diye kokhono na (Rule 12). `SET`/`SET LOCAL` er bodole `SELECT set_config($1, $2, true)`.
- Layer: `handler.go` (HTTP, validate, JSON; niyom na) → `service.go` (niyom; HTTP ba SQL jane na) → `store.go` (SQL).
- Order ar fulfillment er status shudhu `TransitionOrder`/`TransitionFulfillment` diye (8.8); onno kothao `UPDATE ... SET status` na.
- Ek transaction e lock er kram: order → payment → fulfillment → variant (`variant_id` sajano) → coupon → counter (8.6, 8.8). `40P01` ba `40001` hole puro transaction ekbar abar. Checkout e: idempotency lock → OTP consume (v1.7) → variant → coupon → counter.
- Payment validation e shudhu gateway er response er `tran_id` er payment bodlao; VALID na hole kichu bodlabe na (8.9, v1.7).
- Shomoy UTC (`TIMESTAMPTZ`); "ajker", cutoff, pick list shop er timezone e (Rule 5). Phone save er age `phone.Normalize` (Rule 6).
- Token ar secret plain text e na; log e phone mask, thikana na (12.2). Error JSON 7.1 er format e, `code` stable; 500 er details shudhu log e.
- DB lage emon test asol Postgres e, `TEST_DATABASE_URL` na thakle `t.Skip`.
