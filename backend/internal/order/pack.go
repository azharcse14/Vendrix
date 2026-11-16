package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/inventory"
)

// BatchShortError (8.11 item 2, 11.7 item 13 iii): item er jonno FIFO batch e qty nai; 409 out_of_stock details.
type BatchShortError struct {
	ItemID, VariantID int64
	Available         int
}

func (e *BatchShortError) Error() string {
	return fmt.Sprintf("order: item %d er batch e %d", e.ItemID, e.Available)
}

// PackItem: pack body er item (shudhu ojon-variable).
type PackItem struct {
	ItemID        int64 `json:"item_id"`
	PackedWeightG int   `json:"packed_weight_g"`
}

// AdminPack: POST /api/admin/fulfillments/{id}/pack {items: [{item_id, packed_weight_g}]} (7.7, 11.7 item 13 ii).
func (h *Handlers) AdminPack(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []PackItem `json:"items"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	weights := make(map[int64]int, len(in.Items))
	for _, it := range in.Items {
		if _, dup := weights[it.ItemID]; dup || it.PackedWeightG < 1 || it.PackedWeightG > 1_000_000 {
			invalid(w, r, "items", "invalid")
			return
		}
		weights[it.ItemID] = it.PackedWeightG
	}
	id, ok := adminID(r)
	if !ok {
		h.adminError(w, r, ErrNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	display, err := Pack(r.Context(), h.db, st, id, weights)
	var short *BatchShortError
	var fe FieldError
	switch {
	case errors.As(err, &short):
		// tx rollback hoye geche; alert M12 e email/webhook, ekhon log (11.7 item 13 iii)
		h.log.WarnContext(r.Context(), "alert.batch_short", "shop_id", st.ShopID, "fulfillment_id", id,
			"item_id", short.ItemID, "variant_id", short.VariantID, "available", short.Available)
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "out_of_stock",
			Details: map[string]any{"item_id": short.ItemID, "variant_id": short.VariantID, "available": short.Available}})
	case errors.As(err, &fe):
		invalid(w, r, string(fe), "invalid")
	case err != nil:
		h.adminError(w, r, err)
	default:
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": string(Packed), "display_status": display})
	}
}

// Pack (8.11, 11.7 item 13 x): TransitionFulfillment er open → packed path + audit ek tx e; display_status ferot.
func Pack(ctx context.Context, d *sql.DB, st httpx.Staff, fid int64, weights map[int64]int) (string, error) {
	var display string
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		if err := transitionFulfillment(ctx, tx, st.ShopID, fid, Packed, ByStaff(st), "", weights); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `SELECT o.display_status FROM orders o JOIN order_fulfillments f
			ON f.shop_id = o.shop_id AND f.order_id = o.id WHERE f.shop_id = $1 AND f.id = $2`, st.ShopID, fid).Scan(&display); err != nil {
			return err
		}
		return logAudit(ctx, tx, st, "fulfillment.pack", "fulfillment", fid, statusJSON(string(Open)), statusJSON(string(Packed)))
	})
	return display, err
}

type packLine struct {
	id, variantID int64
	qty           int
	weighted      bool
}

// pack (8.8 "open → packed", 8.11 item 2): order ar fulfillment lock er por. Body te thik active ojon-variable
// item gulo (partial pack na); batch_expiry on hole variant (id kram) lock, tarpor protiti item FIFO Allocate
// (variant, item id kram e: batch lock o id kram e), order_item_batches. Batch kom hole BatchShortError, caller
// rollback kore. Ojon save ar Reprice M7a step 5b e.
func pack(ctx context.Context, tx *sql.Tx, shopID, fid int64, weights map[int64]int) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, variant_id, quantity, variable_weight FROM order_items
		WHERE shop_id = $1 AND fulfillment_id = $2 AND status = 'active' ORDER BY variant_id, id`, shopID, fid)
	if err != nil {
		return err
	}
	var lines []packLine
	for rows.Next() {
		var l packLine
		if err := rows.Scan(&l.id, &l.variantID, &l.qty, &l.weighted); err != nil {
			rows.Close()
			return err
		}
		lines = append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	n := 0
	for _, l := range lines {
		if _, ok := weights[l.id]; ok != l.weighted {
			return FieldError("items")
		}
		if l.weighted {
			n++
		}
	}
	if n != len(weights) { // onno fulfillment er, removed ba ochena item
		return FieldError("items")
	}

	on, err := inventory.BatchExpiry(ctx, tx, shopID)
	if err != nil || !on {
		return err
	}
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM product_variants WHERE shop_id = $1 AND id IN (SELECT variant_id
		FROM order_items WHERE shop_id = $1 AND fulfillment_id = $2 AND status = 'active') ORDER BY id FOR UPDATE`, shopID, fid); err != nil {
		return err
	}
	for _, l := range lines {
		parts, rest, err := inventory.Allocate(ctx, tx, shopID, l.variantID, l.qty)
		if err != nil {
			return err
		}
		if rest > 0 {
			return &BatchShortError{ItemID: l.id, VariantID: l.variantID, Available: l.qty - rest}
		}
		for _, p := range parts {
			if _, err := tx.ExecContext(ctx, `INSERT INTO order_item_batches (shop_id, order_item_id, batch_id, variant_id, quantity)
				VALUES ($1, $2, $3, $4, $5)`, shopID, l.id, p.BatchID, l.variantID, p.Qty); err != nil {
				return err
			}
		}
	}
	return nil
}
