# Audit log

Design: 12 (Operation 3), 6.1, 6.7, 7.7, 11.7 item 11 · Milestone: M1, M7a · Code: `backend/internal/audit`

## Ki kore

Protiti admin kaj (category, product, variant bodol, product chobi ([media.md](media.md)); order transition, note, fulfillment transition ([order-status.md](order-status.md)); pack, order edit, shipment ([fulfillment.md](fulfillment.md)); print link ([print.md](print.md)); staff create/update/reset ([auth.md](auth.md)); batch, stock adjust ([inventory.md](inventory.md)); zone, coupon ([delivery-zone.md](delivery-zone.md), [coupon.md](coupon.md)); customer block ([admin-api.md](admin-api.md), phone chhara); pore settings) `audit_logs` e ek row rakhe: ke, kon shop e, ki, ager ar porer obostha, IP. Owner/admin `GET /api/admin/audit-logs` e dekhe ke ki bodleche. Admin screen (`/audit`, filter, before/after JSON): [admin-app.md](admin-app.md).

## Kivabe kaj kore

1. Admin handler kaj `db.InTx` e kore; row bodlanor age `to_jsonb(row)` (before), pore (after).
2. Ek-i transaction e `audit.Log(ctx, tx, audit.Entry{...})`; IP `httpx.IP(ctx)`.
3. Log fail korle (jemon onno shop er staff) kaj o rollback.
4. Login er audit (`staff.login`, `staff.login_failed`, `staff.login_mfa*`, `staff.password_changed`, `staff.totp_*`, `staff.reset_2fa`) `audit.Log` na, `auth` er `insertAudit` e ([auth.md](auth.md)): bhul login er row transaction chhara-o (`*sql.DB`), ochena staff e `staff_id` NULL.
5. List (`handler.go` → `List`, owner/admin, v1.46): paged, notun age (`created_at DESC, id DESC`, `audit_shop_idx`); filter `action` hubohu (max 64), `staff_id` (> 0; onno shop er hole khali), `from`/`to` `YYYY-MM-DD` shop er timezone e (`to` shoho; `httpx.ValidDate`, year 0 `400`); row `{id, staff_id, staff_name (LEFT JOIN, CLI row e null), action, entity_type, entity_id, before, after, ip, created_at}`; `no-store`.

## Code er map

Ei kram e poro:

| File → function | Ki kore |
|---|---|
| `backend/internal/audit/audit.go` → `Log`, `Entry` | `audit_logs` e INSERT |
| `backend/internal/audit/handler.go` → `NewHandlers`, `AdminRoutes`, `List` | `GET /api/admin/audit-logs` |

## Niyom ar tar test

| Niyom (design section) | Code e kothay | Test |
|---|---|---|
| Admin kaj er ek transaction e row; staff ar shop mile (composite FK) (12 Operation 3, Rule 3) | `Log` | `TestAuditIntegration` |
| List owner/admin, filter, paged, onno shop er row na, row e puro phone na (7.7, 11.2, 11.7 item 11, v1.46) | `List` | `TestAuditListIntegration`, `TestAdminRouteRoles` |

## Data

`audit_logs` (`shop_id`, `staff_id`, `action`, `entity_type`, `entity_id`, `before`, `after` JSONB, `ip`, `created_at`); `shop_app` er UPDATE/DELETE nai (6.1); `001_init.sql`.

## Bhul hole ki hoy

- Onno shop er staff: foreign key error, puro kaj rollback (`500`).
- List e bhul `from`/`to`/`staff_id`/`action` `400 fields.<naam>: invalid`; `staff`/vendor role `403`.

## Bodlanor age mone rakho

- `action` er naam `<entity>.<kaj>` (`category.create`); admin UI (M7) ei naam diye filter kore.
- Secret (password hash, credential) `before`/`after` e jabe na; dorkar hole column bad diye JSON banao.

## Proshno-uttor

- **Alada transaction e na keno?** Kaj hoye log na hole (ba ulto) audit bishshash kora jay na.
