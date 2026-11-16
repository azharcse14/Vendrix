package inventory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"vendrix/backend/internal/audit"
	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
)

// ErrNotFound: variant/batch nai, onno shop ba onno vendor er (Rule 13); 404.
var ErrNotFound = errors.New("inventory: nai")

// OutOfStockError: komanor moto stock (ba batch e) nai; 409 out_of_stock details {available} (11.7 item 2).
type OutOfStockError struct{ Available int }

func (e *OutOfStockError) Error() string { return fmt.Sprintf("inventory: stock %d", e.Available) }

// FieldError: input DB er obosthar sathe mile na (jemon expiry shop er ajker age); 400 validation_failed fields.
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return "inventory: " + e.Field + " " + e.Reason }

// vendorScope (Rule 13): vendor role e tar vendor, shop staff e nil (shob vendor).
func vendorScope(st httpx.Staff) any {
	if st.VendorID == nil {
		return nil
	}
	return *st.VendorID
}

// hideCost (11.7 item 1): staff ar vendor_staff kena dam dekhe na.
func hideCost(st httpx.Staff) bool { return st.Role == "staff" || st.Role == "vendor_staff" }

// BatchInput: POST /batches (handler format check kore; tarikh YYYY-MM-DD).
type BatchInput struct {
	VariantID               int64
	Quantity                int
	CostPrice               *int64
	BatchCode               *string
	HarvestDate, ExpiryDate *string
}

// CreateBatch (8.12, 11.7 item 2): stock barano shudhu eta: batch (received = remaining), variant.stock + qty,
// movement purchase batch_id shoho, audit, ek tx e. batch_expiry off eo batch row hoy (kena dam, recall).
func CreateBatch(ctx context.Context, d *sql.DB, st httpx.Staff, in BatchInput) (Batch, error) {
	var b Batch
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		if _, err := lockVariant(ctx, tx, st.ShopID, in.VariantID, vendorScope(st)); err != nil {
			return err
		}
		if in.ExpiryDate != nil {
			today, err := shopToday(ctx, tx, st.ShopID)
			if err != nil {
				return err
			}
			if *in.ExpiryDate < today { // YYYY-MM-DD, string tulona = tarikh tulona
				return &FieldError{"expiry_date", "expired"}
			}
		}
		var err error
		if b, err = insertBatch(ctx, tx, st.ShopID, in); err != nil {
			return err
		}
		if err := addStock(ctx, tx, st.ShopID, in.VariantID, in.Quantity); err != nil {
			return err
		}
		if err := movement(ctx, tx, st.ShopID, in.VariantID, b.ID, in.Quantity, "purchase", st.ID, ""); err != nil {
			return err
		}
		after, _ := json.Marshal(b)
		return logAudit(ctx, tx, st, "inventory.batch", "batch", b.ID, nil, after)
	})
	if hideCost(st) {
		b.CostPrice = nil
	}
	return b, err
}

// AdjustInput: POST /inventory/adjust (handler check kore: Change < 0, Reason wastage/adjust, Note 1–500).
type AdjustInput struct {
	VariantID int64
	Change    int
	Reason    string
	Note      string
	BatchID   *int64
}

// Adjust (8.12, 11.7 item 2): shudhu komano. Lock variant tarpor batch (id kram). variant.stock er beshi hole
// OutOfStockError. batch_expiry on: BatchID dile oi batch, na dile FIFO (Allocate), batch e na pawa ongsho purono
// stock (batch chhara movement); off: shudhu variant.stock (BatchID FieldError). Notun stock ferot.
func Adjust(ctx context.Context, d *sql.DB, st httpx.Staff, in AdjustInput) (int, error) {
	var left int
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		stock, err := lockVariant(ctx, tx, st.ShopID, in.VariantID, vendorScope(st))
		if err != nil {
			return err
		}
		qty := -in.Change
		if stock < qty {
			return &OutOfStockError{stock}
		}
		on, err := batchExpiry(ctx, tx, st.ShopID)
		if err != nil {
			return err
		}
		var parts []Part
		rest := qty
		switch {
		case !on && in.BatchID != nil:
			return &FieldError{"batch_id", "batch_expiry_off"}
		case in.BatchID != nil:
			rem, err := lockBatch(ctx, tx, st.ShopID, in.VariantID, *in.BatchID)
			if err != nil {
				return err
			}
			if rem < qty {
				return &OutOfStockError{rem}
			}
			if err := takeBatch(ctx, tx, st.ShopID, *in.BatchID, qty); err != nil {
				return err
			}
			parts, rest = []Part{{*in.BatchID, qty}}, 0
		case on:
			if parts, rest, err = Allocate(ctx, tx, st.ShopID, in.VariantID, qty); err != nil {
				return err
			}
		}
		if err := addStock(ctx, tx, st.ShopID, in.VariantID, in.Change); err != nil {
			return err
		}
		if rest > 0 {
			parts = append(parts, Part{0, rest})
		}
		for _, p := range parts {
			if err := movement(ctx, tx, st.ShopID, in.VariantID, p.BatchID, -p.Qty, in.Reason, st.ID, in.Note); err != nil {
				return err
			}
		}
		left = stock - qty
		before, _ := json.Marshal(map[string]int{"stock": stock})
		after, _ := json.Marshal(map[string]any{"stock": left, "change": in.Change, "reason": in.Reason})
		return logAudit(ctx, tx, st, "inventory.adjust", "variant", in.VariantID, before, after)
	})
	return left, err
}

func logAudit(ctx context.Context, tx *sql.Tx, st httpx.Staff, action, entity string, id int64, before, after json.RawMessage) error {
	return audit.Log(ctx, tx, audit.Entry{ShopID: st.ShopID, StaffID: st.ID, Action: action, EntityType: entity,
		EntityID: id, Before: before, After: after, IP: httpx.IP(ctx)})
}

// Variants, Batches, Movements: list (vendor scope; staff/vendor_staff e cost_price na).
func Variants(ctx context.Context, d *sql.DB, st httpx.Staff, q string, low bool, page, limit int) ([]VariantRow, int64, error) {
	return listVariants(ctx, d, st.ShopID, vendorScope(st), q, low, page, limit)
}

func Batches(ctx context.Context, d *sql.DB, st httpx.Staff, variantID int64, page, limit int) ([]Batch, int64, error) {
	out, total, err := listBatches(ctx, d, st.ShopID, variantID, vendorScope(st), page, limit)
	if hideCost(st) {
		for i := range out {
			out[i].CostPrice = nil
		}
	}
	return out, total, err
}

func Movements(ctx context.Context, d *sql.DB, st httpx.Staff, variantID int64, page, limit int) ([]Movement, int64, error) {
	out, total, err := listMovements(ctx, d, st.ShopID, vendorScope(st), variantID, page, limit)
	if hideCost(st) {
		for i := range out {
			out[i].CostPrice = nil
		}
	}
	return out, total, err
}
