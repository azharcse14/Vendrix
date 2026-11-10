// Package audit: admin kaj er log (12 Operation 3), kaj er ek-i transaction e. `shop_app` audit e UPDATE
// korte pare na (6.1), tai row bodlay na.
//
// Feature doc: docs/features/audit.md
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
)

// Entry: ek admin kaj. Before/After to_jsonb(row) (nai hole nil → NULL); IP = httpx.IP(ctx).
type Entry struct {
	ShopID, StaffID    int64
	Action, EntityType string // "category.create", "category"
	EntityID           int64
	Before, After      json.RawMessage
	IP                 string
}

func raw(m json.RawMessage) any {
	if m == nil {
		return nil
	}
	return string(m)
}

// Log: audit_logs e row; staff onno shop er hole (shop_id, staff_id) FK error, tai kaj o rollback.
func Log(ctx context.Context, tx *sql.Tx, e Entry) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_logs (shop_id, staff_id, action, entity_type, entity_id, before, after, ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.ShopID, e.StaffID, e.Action, e.EntityType, e.EntityID, raw(e.Before), raw(e.After), e.IP)
	return err
}
