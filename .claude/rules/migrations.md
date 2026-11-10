---
paths:
  - "backend/migrations/**"
---
# Migration (6.1, 6.6)
- `001_init.sql` schema er asol spec (6.7). Deploy er por kono migration edit na: naam `DEPLOYED` e thakle hook atkay, CI hash mile kina dekhe. Bodol notun `00N_naam.sql` e, porer number (6.6 er table update).
- Protiti business table e `shop_id NOT NULL`; parent e `UNIQUE (shop_id, id)`, child er FK `(shop_id, parent_id)` (Rule 3). Taka `BIGINT` poisha, shomoy `TIMESTAMPTZ`, status `TEXT` + `CHECK` (ENUM na), phone `phone_bd`.
- Notun CHECK e naam dao (`<table>_<ki>_check`), ar `tests/` e reject test `'reject:<naam>'` diye.
- Notun table hard-delete-nishiddho ba history hole oi file e-i `REVOKE ... FROM shop_app` (6.1).
- Test SQL shudhu `tests/` e; top-level e `*_test.sql` na (embed e migration hishebe chole jabe).
- Migration samner dike compatible: column age jog, purono column pore-r deploy e bad (13.6).
- Runner (`internal/db`) `schema_migrations` e sha256 rakhe: applied file bodlale ba ager number er notun file dile `migrate` fail (6.1).
