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
		if err := transitionFulfillment(ctx, tx, st.ShopID, fid, Packed, ByStaff(st), note, weights); err != nil {
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
		if err := reprice(ctx, tx, shopID, o, fid, weights, customerOK); err != nil {
			return err
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

// reprice (8.11 item 3, 8.5): order er shob item LineFromItem diye Original, pack er ojon diye notun line,
// pricing.Reprice (Edit = false; onno vendor freeze); COD e +10% er beshi (customer_ok_note chhara)
// WeightOverError, kichu likha na. Tarpor item (packed_weight_g, taka, commission), fulfillment ar order er taka;
// prepaid e refund reprice; COD e total bodlale sms.total_changed (commit er por jay).
func reprice(ctx context.Context, tx *sql.Tx, shopID int64, o *locked, fid int64, weights map[int64]int, customerOK bool) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, vendor_id, unit_price, quantity, variable_weight, COALESCE(ordered_weight_g, 0),
		COALESCE(packed_weight_g, 0), status = 'removed', line_total, discount_share, commission_bps
		FROM order_items WHERE shop_id = $1 AND order_id = $2 ORDER BY id`, shopID, o.id)
	if err != nil {
		return err
	}
	var ids []int64
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
			return err
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
		ids, bps, lines = append(ids, id), append(bps, b), append(lines, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(over) > 0 && !customerOK {
		return &WeightOverError{over}
	}

	rows, err = tx.QueryContext(ctx, `SELECT vendor_id, subtotal, discount, delivery_charge, total FROM order_fulfillments
		WHERE shop_id = $1 AND order_id = $2 ORDER BY id`, shopID, o.id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var g pricing.Group
		if err := rows.Scan(&g.VendorID, &g.Subtotal, &g.Discount, &g.DeliveryCharge, &g.Total); err != nil {
			rows.Close()
			return err
		}
		orig.Result.Groups = append(orig.Result.Groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	var kind, funded sql.NullString
	var value, maxD, cVendor sql.NullInt64
	t := &orig.Result.Totals
	if err := tx.QueryRowContext(ctx, `SELECT subtotal, discount, delivery_charge, total, coupon_kind, coupon_value,
		coupon_max_discount, coupon_vendor_id, coupon_funded_by FROM orders WHERE shop_id = $1 AND id = $2`, shopID, o.id).Scan(
		&t.Subtotal, &t.Discount, &t.DeliveryCharge, &t.Total, &kind, &value, &maxD, &cVendor, &funded); err != nil {
		return err
	}
	if kind.Valid { // snapshot (8.5 v1.4); min_order abar dekha hoy na
		orig.Coupon = &pricing.Coupon{Kind: kind.String, Value: value.Int64, MaxDiscount: maxD.Int64, VendorID: cVendor.Int64}
	}
	orig.Prepaid = !cod
	r, refund, err := pricing.Reprice(orig, lines)
	if err != nil {
		return err
	}

	for i, id := range ids {
		_, packed := weights[id]
		if !packed && r.Lines[i] == orig.Result.Lines[i] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_items SET packed_weight_g = COALESCE($3::int, packed_weight_g), line_total = $4,
			discount_share = $5, commission = $6 WHERE shop_id = $1 AND id = $2`, shopID, id, sql.NullInt64{Int64: int64(weights[id]), Valid: packed},
			r.Lines[i].LineTotal, r.Lines[i].DiscountShare, pricing.Commission(r.Lines[i], funded.String == "vendor", bps[i])); err != nil {
			return err
		}
	}
	for _, g := range r.Groups {
		if _, err := tx.ExecContext(ctx, `UPDATE order_fulfillments SET subtotal = $4, discount = $5, delivery_charge = $6, total = $7
			WHERE shop_id = $1 AND order_id = $2 AND vendor_id = $3`, shopID, o.id, g.VendorID, g.Subtotal, g.Discount, g.DeliveryCharge, g.Total); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET subtotal = $3, discount = $4, delivery_charge = $5, total = $6
		WHERE shop_id = $1 AND id = $2`, shopID, o.id, r.Subtotal, r.Discount, r.DeliveryCharge, r.Total); err != nil {
		return err
	}
	if refund > 0 {
		if err := insertRefund(ctx, tx, shopID, o, fid, refund, "reprice", "reprice"); err != nil {
			return err
		}
	}
	if cod && r.Total != t.Total {
		return jobs.Enqueue(ctx, tx, jobs.New{ShopID: shopID, Kind: "sms.total_changed", Payload: jobPayload{o.id}})
	}
	return nil
}
