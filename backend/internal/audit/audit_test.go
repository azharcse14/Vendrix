package audit

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/shop"
)

// TestAuditIntegration (12 Operation 3): kaj er transaction e row; onno shop er staff → FK error (23503).
func TestAuditIntegration(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	var shops, staff [2]int64
	for i, s := range []string{"a", "b"} {
		id, _, err := shop.Create(ctx, d, shop.Params{Slug: s, Prefix: "AU" + string(rune('A'+i)), Domain: s + ".com",
			NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		shops[i] = id
		if err := d.QueryRow(`SELECT id FROM staff WHERE shop_id = $1`, id).Scan(&staff[i]); err != nil {
			t.Fatal(err)
		}
	}
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		return Log(ctx, tx, Entry{ShopID: shops[0], StaffID: staff[0], Action: "category.update", EntityType: "category",
			EntityID: 7, Before: []byte(`{"slug":"a"}`), After: []byte(`{"slug":"b"}`), IP: "203.0.113.1"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.QueryRow(`SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND staff_id = $2 AND action = 'category.update'
		AND entity_id = 7 AND before->>'slug' = 'a' AND after->>'slug' = 'b' AND ip = '203.0.113.1'`, shops[0], staff[0]).Scan(&n); err != nil || n != 1 {
		t.Errorf("row: %d %v", n, err)
	}
	err = db.InTx(ctx, d, func(tx *sql.Tx) error {
		return Log(ctx, tx, Entry{ShopID: shops[0], StaffID: staff[1], Action: "x", EntityType: "x"})
	})
	var pg interface{ SQLState() string }
	if !errors.As(err, &pg) || pg.SQLState() != "23503" {
		t.Errorf("onno shop er staff: %v", err)
	}
}
