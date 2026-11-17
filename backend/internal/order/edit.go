package order

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"slices"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
)

// EditItem: PATCH items body er item; quantity 0 = bad.
type EditItem struct {
	ItemID   int64 `json:"item_id"`
	Quantity int   `json:"quantity"`
}

// AdminEdit: PATCH /api/admin/orders/{id}/items {items: [{item_id, quantity}], note} (7.7, 11.7 item 7, 8.11 item 4).
func (h *Handlers) AdminEdit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []EditItem `json:"items"`
		Note  string     `json:"note"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	qty := make(map[int64]int, len(in.Items))
	for _, it := range in.Items {
		if _, dup := qty[it.ItemID]; dup || it.Quantity < 0 {
			invalid(w, r, "items", "invalid")
			return
		}
		qty[it.ItemID] = it.Quantity
	}
	if len(qty) == 0 {
		invalid(w, r, "items", "invalid")
		return
	}
	note, ok := text(in.Note, 1, 500)
	if !ok {
		invalid(w, r, "note", "invalid")
		return
	}
	id, ok := adminID(r)
	if !ok {
		h.adminError(w, r, ErrNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	status, display, err := Edit(r.Context(), h.db, st, id, qty, note)
	if err != nil {
		h.adminError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": status, "display_status": display})
}

type editLine struct {
	ItemID   int64 `json:"item_id"`
	Quantity int   `json:"quantity"`
}

// Edit (8.11 item 4 v1.5, 11.7 item 7): ek tx e lock order → fulfillment → variant (id kram; fulfillment open tai batch
// nai), stock ferot (movement cancel, note edit), reprice (Edit = true; prepaid refund ekta, fulfillment_id NULL), history
// te note, je vendor er shob item bad tar fulfillment TransitionFulfillment(→ cancelled) (total 0, tai cancel refund na;
// shob gele order o), audit. staff role e vendor er shob bad ba refund hole ErrForbidden (rollback). (status,
// display_status) ferot.
func Edit(ctx context.Context, d *sql.DB, st httpx.Staff, orderID int64, qty map[int64]int, note string) (string, string, error) {
	by := ByStaff(st)
	if by.Actor != Staff {
		return "", "", ErrForbidden
	}
	var status, display string
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		o, err := lockOrder(ctx, tx, st.ShopID, orderID)
		if err != nil {
			return err
		}
		if o.status != Pending && o.status != Confirmed {
			return ErrInvalidTransition
		}
		if o.method != "cod" && o.payStatus != "paid" && o.payStatus != "partially_refunded" {
			return ErrPaymentRequired // gateway e ager taka cholte pare (confirm er moto)
		}
		fstatus := map[int64]FStatus{}
		for _, f := range o.fs {
			fstatus[f.id] = f.status
		}
		rows, err := tx.QueryContext(ctx, `SELECT id, fulfillment_id, variant_id, quantity FROM order_items
			WHERE shop_id = $1 AND order_id = $2 AND status = 'active' ORDER BY variant_id, id`, st.ShopID, o.id)
		if err != nil {
			return err
		}
		type line struct {
			id, fid, variant int64
			qty              int
		}
		var all []line
		for rows.Next() {
			var l line
			if err := rows.Scan(&l.id, &l.fid, &l.variant, &l.qty); err != nil {
				rows.Close()
				return err
			}
			all = append(all, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		left := map[int64]int{} // fulfillment → edit er por active item
		var before, after []editLine
		var variants []int64
		n := 0
		for _, l := range all {
			q, ok := qty[l.id]
			if !ok {
				left[l.fid]++
				continue
			}
			n++
			if q >= l.qty {
				return FieldError("items") // shudhu kom (barano notun order)
			}
			if fstatus[l.fid] != Open {
				return ErrInvalidTransition // packed hole age undo
			}
			if q > 0 {
				left[l.fid]++
			}
			before, after = append(before, editLine{l.id, l.qty}), append(after, editLine{l.id, q})
			variants = append(variants, l.variant)
		}
		if n != len(qty) { // onno order er, removed ba ochena item
			return FieldError("items")
		}
		var gone []int64
		for _, f := range o.fs {
			if f.status == Open && left[f.id] == 0 && slices.ContainsFunc(all, func(l line) bool { return l.fid == f.id }) {
				gone = append(gone, f.id)
			}
		}
		staffRole := st.Role == "staff"
		if staffRole && len(gone) > 0 {
			return ErrForbidden // fulfillment cancel shudhu owner/admin (11.2)
		}

		if _, err := tx.ExecContext(ctx, `SELECT 1 FROM product_variants WHERE shop_id = $1 AND id = ANY($2) ORDER BY id FOR UPDATE`,
			st.ShopID, variants); err != nil {
			return err
		}
		for _, l := range all { // variant_id kram e
			q, ok := qty[l.id]
			if !ok {
				continue
			}
			if _, err := tx.ExecContext(ctx, `UPDATE product_variants SET stock = stock + $3 WHERE shop_id = $1 AND id = $2`,
				st.ShopID, l.variant, l.qty-q); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO inventory_movements (shop_id, variant_id, change, reason, order_id, staff_id, note)
				VALUES ($1, $2, $3, 'cancel', $4, $5, 'edit')`, st.ShopID, l.variant, l.qty-q, o.id, st.ID); err != nil {
				return err
			}
		}
		var oldTotal, newTotal int64
		if err := tx.QueryRowContext(ctx, `SELECT total FROM orders WHERE shop_id = $1 AND id = $2`, st.ShopID, o.id).Scan(&oldTotal); err != nil {
			return err
		}
		_, refund, err := reprice(ctx, tx, st.ShopID, o, 0, nil, qty, false)
		if err != nil {
			return err
		}
		if staffRole && refund > 0 {
			return ErrForbidden // refund shudhu owner/admin (11.2)
		}
		if err := history(ctx, tx, st.ShopID, o.id, 0, string(o.status), string(o.status), by, note); err != nil {
			return err
		}
		for _, fid := range gone {
			if err := transitionFulfillment(ctx, tx, st.ShopID, fid, FCancelled, by, note, nil); err != nil {
				return err
			}
		}
		if err := tx.QueryRowContext(ctx, `SELECT status, display_status, total FROM orders WHERE shop_id = $1 AND id = $2`,
			st.ShopID, o.id).Scan(&status, &display, &newTotal); err != nil {
			return err
		}
		slices.SortFunc(before, func(a, b editLine) int { return cmp.Compare(a.ItemID, b.ItemID) })
		slices.SortFunc(after, func(a, b editLine) int { return cmp.Compare(a.ItemID, b.ItemID) })
		b, _ := json.Marshal(map[string]any{"items": before, "total": oldTotal})
		a, _ := json.Marshal(map[string]any{"items": after, "total": newTotal})
		return logAudit(ctx, tx, st, "order.edit", "order", o.id, b, a)
	})
	return status, display, err
}
