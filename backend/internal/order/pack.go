package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/inventory"
	"vendrix/backend/internal/jobs"
	"vendrix/backend/internal/pricing"
)

// BatchShortError (8.11 item 2, 11.7 item 13 iii): item er jonno FIFO batch e qty nai; 409 out_of_stock details.
type BatchShortError struct {
	ItemID, VariantID int64
	Available         int
}

func (e *BatchShortError) Error() string {
	return fmt.Sprintf("order: item %d er batch e %d", e.ItemID, e.Available)
}

// overweightPct (8.11 item 3 v1.37): COD pack e ordered ojon er upor eto % porjonto customer er OK chhara; kono shop
// alada chaile tokhon shop setting.
const overweightPct = 10

// WeightOver: 409 weight_over_limit er details.items er ek item.
type WeightOver struct {
	ItemID         int64 `json:"item_id"`
	OrderedWeightG int   `json:"ordered_weight_g"`
	MaxWeightG     int   `json:"max_weight_g"`
}

// WeightOverError (8.11 item 3 v1.37): COD e ojon-variable line +10% er beshi, customer_ok_note nai.
type WeightOverError struct{ Items []WeightOver }

func (e *WeightOverError) Error() string {
	return fmt.Sprintf("order: %d item ojon shima er beshi", len(e.Items))
}

// PackItem: pack body er item (shudhu ojon-variable).
type PackItem struct {
	ItemID        int64 `json:"item_id"`
	PackedWeightG int   `json:"packed_weight_g"`
}

// AdminPack: POST /api/admin/fulfillments/{id}/pack {items: [{item_id, packed_weight_g}], customer_ok_note?}
// (7.7, 11.7 item 13 ii, 8.11 item 3).
func (h *Handlers) AdminPack(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Items []PackItem `json:"items"`
		Note  string     `json:"customer_ok_note"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	note := strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(note) > 500 {
		invalid(w, r, "customer_ok_note", "invalid")
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
	display, err := Pack(r.Context(), h.db, st, id, weights, note)
	var short *BatchShortError
	var over *WeightOverError
	var fe FieldError
	switch {
	case errors.As(err, &short):
		// tx rollback hoye geche; alert M12 e email/webhook, ekhon log (11.7 item 13 iii)
		h.log.WarnContext(r.Context(), "alert.batch_short", "shop_id", st.ShopID, "fulfillment_id", id,
			"item_id", short.ItemID, "variant_id", short.VariantID, "available", short.Available)
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "out_of_stock",
			Details: map[string]any{"item_id": short.ItemID, "variant_id": short.VariantID, "available": short.Available}})
	case errors.As(err, &over):
		httpx.WriteError(w, r, http.StatusConflict, httpx.ErrorBody{Code: "weight_over_limit", Details: map[string]any{"items": over.Items}})
	case errors.As(err, &fe):
		invalid(w, r, string(fe), "invalid")
	case err != nil:
		h.adminError(w, r, err)
	default:
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": string(Packed), "display_status": display})
	}
}

// Pack (8.11, 11.7 item 13 x): TransitionFulfillment er open → packed path + audit ek tx e; display_status ferot.
// note = customer_ok_note (history te).
func Pack(ctx context.Context, d *sql.DB, st httpx.Staff, fid int64, weights map[int64]int, note string) (string, error) {
	var display string
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		if err := transitionFulfillment(ctx, tx, st.ShopID, fid, Packed, ByStaff(st), note, weights, nil); err != nil {
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
// rollback kore. Ojon thakle age reprice (ojon save, taka, +10% check).
func pack(ctx context.Context, tx *sql.Tx, shopID int64, o *locked, fid int64, weights map[int64]int, customerOK bool) error {
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
	if n > 0 {
		changed, _, err := reprice(ctx, tx, shopID, o, fid, weights, nil, customerOK)
		if err != nil {
			return err
		}
		if changed { // commit er por jay
			if err := jobs.Enqueue(ctx, tx, jobs.New{ShopID: shopID, Kind: "sms.total_changed", Payload: jobPayload{o.id}}); err != nil {
				return err
			}
		}
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

// reprice (8.11 item 3, 4; 8.5): order er shob item LineFromItem diye Original, pack er ojon (weights) ba edit er
// notun qty (qty; 0 = removed, ojon-variable e EditWeightG) diye notun line, pricing.Reprice (Edit = qty != nil; onno
// vendor freeze); COD e +10% er beshi (customer_ok_note chhara) WeightOverError, kichu likha na. Tarpor item, fulfillment
// ar order er taka; prepaid e refund reprice (fid 0 = NULL, edit). Ojon 0 = packed_weight_g NULL (undo). COD e total
// bodlale true (SMS caller er: undo/edit e na); refund ferot (edit er role check).
func reprice(ctx context.Context, tx *sql.Tx, shopID int64, o *locked, fid int64, weights, qty map[int64]int, customerOK bool) (bool, int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, vendor_id, unit_price, quantity, variable_weight, COALESCE(ordered_weight_g, 0),
		COALESCE(packed_weight_g, 0), status = 'removed', line_total, discount_share, commission_bps
		FROM order_items WHERE shop_id = $1 AND order_id = $2 ORDER BY id`, shopID, o.id)
	if err != nil {
		return false, 0, err
	}
	var ids []int64
	var items []pricing.Item
	var touched []bool
	var bps []int
	var orig pricing.Original
	var lines []pricing.Line
	var over []WeightOver
	cod := o.method == "cod"
	for rows.Next() {
		var it pricing.Item
		var id int64
		var b int
		if err := rows.Scan(&id, &it.VendorID, &it.UnitPrice, &it.Quantity, &it.VariableWeight, &it.OrderedWeightG,
			&it.PackedWeightG, &it.Removed, &it.LineTotal, &it.DiscountShare, &b); err != nil {
			rows.Close()
			return false, 0, err
		}
		l, lr := pricing.LineFromItem(it)
		orig.Lines, orig.Result.Lines = append(orig.Lines, l), append(orig.Result.Lines, lr)
		if g, ok := weights[id]; ok {
			if cod && g*100 > it.OrderedWeightG*(100+overweightPct) {
				over = append(over, WeightOver{id, it.OrderedWeightG, it.OrderedWeightG * (100 + overweightPct) / 100})
			}
			it.PackedWeightG = g
			l, _ = pricing.LineFromItem(it)
		}
		q, edit := qty[id]
		if edit {
			if q == 0 {
				it.Removed = true // DB te ager quantity thake (CHECK > 0)
			} else {
				if it.VariableWeight {
					it.OrderedWeightG = pricing.EditWeightG(it.OrderedWeightG, it.Quantity, q)
				}
				it.Quantity = q
			}
			l, _ = pricing.LineFromItem(it)
		}
		_, packed := weights[id]
		ids, items, touched = append(ids, id), append(items, it), append(touched, packed || edit)
		bps, lines = append(bps, b), append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, 0, err
	}
	if len(over) > 0 && !customerOK {
		return false, 0, &WeightOverError{over}
	}

	rows, err = tx.QueryContext(ctx, `SELECT vendor_id, subtotal, discount, delivery_charge, total FROM order_fulfillments
		WHERE shop_id = $1 AND order_id = $2 ORDER BY id`, shopID, o.id)
	if err != nil {
		return false, 0, err
	}
	for rows.Next() {
		var g pricing.Group
		if err := rows.Scan(&g.VendorID, &g.Subtotal, &g.Discount, &g.DeliveryCharge, &g.Total); err != nil {
			rows.Close()
			return false, 0, err
		}
		orig.Result.Groups = append(orig.Result.Groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, 0, err
	}
	var kind, funded sql.NullString
	var value, maxD, cVendor sql.NullInt64
	t := &orig.Result.Totals
	if err := tx.QueryRowContext(ctx, `SELECT subtotal, discount, delivery_charge, total, coupon_kind, coupon_value,
		coupon_max_discount, coupon_vendor_id, coupon_funded_by FROM orders WHERE shop_id = $1 AND id = $2`, shopID, o.id).Scan(
		&t.Subtotal, &t.Discount, &t.DeliveryCharge, &t.Total, &kind, &value, &maxD, &cVendor, &funded); err != nil {
		return false, 0, err
	}
	if kind.Valid { // snapshot (8.5 v1.4); min_order abar dekha hoy na
		orig.Coupon = &pricing.Coupon{Kind: kind.String, Value: value.Int64, MaxDiscount: maxD.Int64, VendorID: cVendor.Int64}
	}
	orig.Prepaid, orig.Edit = !cod, qty != nil
	r, refund, err := pricing.Reprice(orig, lines)
	if err != nil {
		return false, 0, err
	}

	for i, id := range ids {
		if !touched[i] && r.Lines[i] == orig.Result.Lines[i] {
			continue
		}
		it := items[i]
		if _, err := tx.ExecContext(ctx, `UPDATE order_items SET packed_weight_g = NULLIF($3::int, 0), line_total = $4, discount_share = $5,
			commission = $6, quantity = $7, ordered_weight_g = NULLIF($8::int, 0), status = CASE WHEN $9 THEN 'removed' ELSE status END
			WHERE shop_id = $1 AND id = $2`, shopID, id, it.PackedWeightG, r.Lines[i].LineTotal, r.Lines[i].DiscountShare,
			pricing.Commission(r.Lines[i], funded.String == "vendor", bps[i]), it.Quantity, it.OrderedWeightG, it.Removed); err != nil {
			return false, 0, err
		}
	}
	for _, g := range r.Groups {
		if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillments SET subtotal = $4, discount = $5, delivery_charge = $6, total = $7
			WHERE shop_id = $1 AND order_id = $2 AND vendor_id = $3`, shopID, o.id, g.VendorID, g.Subtotal, g.Discount, g.DeliveryCharge, g.Total); err != nil {
			return false, 0, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET subtotal = $3, discount = $4, delivery_charge = $5, total = $6
		WHERE shop_id = $1 AND id = $2`, shopID, o.id, r.Subtotal, r.Discount, r.DeliveryCharge, r.Total); err != nil {
		return false, 0, err
	}
	if refund > 0 {
		if err := insertRefund(ctx, tx, shopID, o, fid, refund, "reprice", "reprice"); err != nil {
			return false, 0, err
		}
	}
	return cod && r.Total != t.Total, refund, nil
}

// unpack (8.8 "packed → open", 11.7 item 3): pack er ulto, order ar fulfillment lock er por. Ojon-variable item
// packed_weight_g NULL kore reprice (ordered ojon e ager taka; Edit = false), tarpor variant tarpor batch (id kram)
// lock, order_item_batches er qty batch e ferot ar row muche. variant.stock bodlay na (checkout e newa); batch
// already expire hole oi ongsho batch e na, inventory.expire er moto variant.stock − LEAST(qty, stock), movement expired.
func unpack(ctx context.Context, tx *sql.Tx, shopID int64, o *locked, fid int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM order_items WHERE shop_id = $1 AND fulfillment_id = $2
		AND status = 'active' AND packed_weight_g IS NOT NULL`, shopID, fid)
	if err != nil {
		return err
	}
	weights := map[int64]int{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		weights[id] = 0
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(weights) > 0 {
		if _, _, err := reprice(ctx, tx, shopID, o, fid, weights, nil, false); err != nil {
			return err
		}
	}

	rows, err = tx.QueryContext(ctx, `SELECT id, stock FROM product_variants WHERE shop_id = $1 AND id IN
		(SELECT variant_id FROM (SELECT ob.batch_id, ob.variant_id, ob.quantity FROM order_item_batches ob JOIN order_items i
		ON i.shop_id = ob.shop_id AND i.id = ob.order_item_id WHERE i.shop_id = $1 AND i.fulfillment_id = $2) x) ORDER BY id FOR UPDATE`, shopID, fid)
	if err != nil {
		return err
	}
	stock := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			rows.Close()
			return err
		}
		stock[id] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM inventory_batches WHERE shop_id = $1 AND id IN
		(SELECT batch_id FROM (SELECT ob.batch_id, ob.variant_id, ob.quantity FROM order_item_batches ob JOIN order_items i
		ON i.shop_id = ob.shop_id AND i.id = ob.order_item_id WHERE i.shop_id = $1 AND i.fulfillment_id = $2) x) ORDER BY id FOR UPDATE`, shopID, fid); err != nil {
		return err
	}
	type part struct {
		batch, variant int64
		qty            int
		expired        bool
	}
	rows, err = tx.QueryContext(ctx, `SELECT x.batch_id, x.variant_id, sum(x.quantity),
		COALESCE(b.expiry_date < (SELECT (now() AT TIME ZONE timezone)::date FROM shops WHERE id = $1), false)
		FROM (SELECT ob.batch_id, ob.variant_id, ob.quantity FROM order_item_batches ob JOIN order_items i
		ON i.shop_id = ob.shop_id AND i.id = ob.order_item_id WHERE i.shop_id = $1 AND i.fulfillment_id = $2) x JOIN inventory_batches b ON b.shop_id = $1 AND b.id = x.batch_id
		GROUP BY x.batch_id, x.variant_id, b.expiry_date ORDER BY x.batch_id`, shopID, fid)
	if err != nil {
		return err
	}
	var parts []part
	for rows.Next() {
		var p part
		if err := rows.Scan(&p.batch, &p.variant, &p.qty, &p.expired); err != nil {
			rows.Close()
			return err
		}
		parts = append(parts, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range parts {
		if !p.expired {
			if _, err := tx.ExecContext(ctx, `UPDATE inventory_batches SET quantity_remaining = quantity_remaining + $3
				WHERE shop_id = $1 AND id = $2`, shopID, p.batch, p.qty); err != nil {
				return err
			}
			continue
		}
		take := min(p.qty, stock[p.variant]) // purono stock bhul hole 0 er niche na (8.12)
		if take == 0 {
			continue
		}
		stock[p.variant] -= take
		if _, err := tx.ExecContext(ctx, `UPDATE product_variants SET stock = stock - $3 WHERE shop_id = $1 AND id = $2`,
			shopID, p.variant, take); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO inventory_movements (shop_id, variant_id, batch_id, change, reason, order_id)
			VALUES ($1, $2, $3, $4, 'expired', $5)`, shopID, p.variant, p.batch, -take, o.id); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM order_item_batches WHERE shop_id = $1 AND order_item_id IN
		(SELECT id FROM order_items WHERE shop_id = $1 AND fulfillment_id = $2)`, shopID, fid)
	return err
}
