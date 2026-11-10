# Audit log

Design: 12 (Operation 3), 6.1, 6.7 · Milestone: M1 · Code: `backend/internal/audit`

## Ki kore

Protiti admin kaj (category, product, variant bodol; pore order, settings) `audit_logs` e ek row rakhe: ke, kon shop e, ki, ager ar porer obostha, IP. Owner pore dekhte pare ke ki bodleche.

## Kivabe kaj kore

1. Admin handler kaj `db.InTx` e kore; row bodlanor age `to_jsonb(row)` (before), pore (after).
2. Ek-i transaction e `audit.Log(ctx, tx, audit.Entry{...})`; IP `httpx.IP(ctx)`.
3. Log fail korle (jemon onno shop er staff) kaj o rollback.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/audit/audit.go` → `Log`, `Entry` | `audit_logs` e INSERT |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Admin kaj er ek transaction e row; staff ar shop mile (composite FK) (12 Operation 3, Rule 3) | `Log` | `TestAuditIntegration` |

## Data

`audit_logs` (`shop_id`, `staff_id`, `action`, `entity_type`, `entity_id`, `before`, `after` JSONB, `ip`, `created_at`); `shop_app` er UPDATE/DELETE nai (6.1); `001_init.sql`.

## Bhul hole ki hoy

- Onno shop er staff: foreign key error, puro kaj rollback (`500`).

## Bodlanor age mone rakho

- `action` er naam `<entity>.<kaj>` (`category.create`); admin UI (M7) ei naam diye filter kore.
- Secret (password hash, credential) `before`/`after` e jabe na; dorkar hole column bad diye JSON banao.

## Proshno-uttor

- **Alada transaction e na keno?** Kaj hoye log na hole (ba ulto) audit bishshash kora jay na.
