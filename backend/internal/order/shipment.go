package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"vendrix/backend/internal/db"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/jobs"
)

// errTrackingTaken: shipments_tracking_uniq; 400 fields.tracking_code: taken (v1.43).
var errTrackingTaken = errors.New("order: tracking code ache")

var carriers = []string{"own_rider", "steadfast", "pathao", "redx", "other"}

// trackingOK (v1.43): 1–40 printable ASCII (SMS e jay).
func trackingOK(t string) bool {
	if t == "" || len(t) > 40 {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] < 0x20 || t[i] > 0x7e {
			return false
		}
	}
	return true
}

// ShipIn: POST shipments er body (11.7 item 5).
type ShipIn struct {
	Carrier      string `json:"carrier"`
	RiderStaffID *int64 `json:"rider_staff_id"`
	TrackingCode string `json:"tracking_code"`
	CarrierFee   *int64 `json:"carrier_fee"`
}

// Shipment: POST/PATCH er response.
type Shipment struct {
	ID            int64   `json:"id"`
	FulfillmentID int64   `json:"fulfillment_id,omitempty"`
	Carrier       string  `json:"carrier,omitempty"`
	TrackingCode  *string `json:"tracking_code,omitempty"`
	Status        string  `json:"status"`
	CODAmount     *int64  `json:"cod_amount,omitempty"`
	DisplayStatus string  `json:"display_status"`
}

// AdminShip: POST /api/admin/fulfillments/{id}/shipments {carrier, rider_staff_id?, tracking_code?, carrier_fee?}.
func (h *Handlers) AdminShip(w http.ResponseWriter, r *http.Request) {
	var in ShipIn
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	in.TrackingCode = strings.TrimSpace(in.TrackingCode)
	switch {
	case !slices.Contains(carriers, in.Carrier):
		invalid(w, r, "carrier", "invalid")
		return
	case in.TrackingCode != "" && !trackingOK(in.TrackingCode):
		invalid(w, r, "tracking_code", "invalid")
		return
	case in.CarrierFee != nil && *in.CarrierFee < 0:
		invalid(w, r, "carrier_fee", "invalid")
		return
	}
	id, ok := adminID(r)
	if !ok {
		h.adminError(w, r, ErrNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	sh, err := Ship(r.Context(), h.db, st, id, in)
	if err != nil {
		h.shipError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, sh)
}

// AdminShipment: PATCH /api/admin/shipments/{id} {status?: delivered|returned|cancelled, tracking_code?}.
func (h *Handlers) AdminShipment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status       string  `json:"status"`
		TrackingCode *string `json:"tracking_code"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		invalid(w, r, "body", "invalid")
		return
	}
	if in.TrackingCode != nil {
		*in.TrackingCode = strings.TrimSpace(*in.TrackingCode)
	}
	switch {
	case in.Status == "" && in.TrackingCode == nil:
		invalid(w, r, "body", "invalid")
		return
	case in.Status != "" && in.Status != "delivered" && in.Status != "returned" && in.Status != "cancelled":
		invalid(w, r, "status", "invalid")
		return
	case in.TrackingCode != nil && !trackingOK(*in.TrackingCode):
		invalid(w, r, "tracking_code", "invalid")
		return
	}
	id, ok := adminID(r)
	if !ok {
		h.adminError(w, r, ErrNotFound)
		return
	}
	st, _ := httpx.StaffFrom(r.Context())
	sh, err := UpdateShipment(r.Context(), h.db, st, id, in.Status, in.TrackingCode)
	if err != nil {
		h.shipError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sh)
}

func (h *Handlers) shipError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errTrackingTaken) {
		invalid(w, r, "tracking_code", "taken")
		return
	}
	h.adminError(w, r, err)
}

// trackingErr: tracking er unique index → errTrackingTaken.
func trackingErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "shipments_tracking_uniq" {
		return errTrackingTaken
	}
	return err
}

func displayStatus(ctx context.Context, tx *sql.Tx, shopID, orderID int64) (string, error) {
	var d string
	err := tx.QueryRowContext(ctx, `SELECT display_status FROM orders WHERE shop_id = $1 AND id = $2`, shopID, orderID).Scan(&d)
	return d, err
}

// Ship (8.8 "packed → shipped", 11.7 item 5 v1.43): ek tx e lock order → fulfillment, shipment (COD e cod_amount =
// fulfillment total, na hole 0), packed hole TransitionFulfillment(→ shipped); already shipped (ager parcel
// returned/cancelled) hole shudhu notun shipment. Active shipment thakle 409. sms.shipped (protiti shipment e) ar
// audit ek tx e.
func Ship(ctx context.Context, d *sql.DB, st httpx.Staff, fid int64, in ShipIn) (Shipment, error) {
	by := ByStaff(st)
	sh := Shipment{FulfillmentID: fid, Carrier: in.Carrier, Status: "booked"}
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		var orderID, vendorID int64
		err := tx.QueryRowContext(ctx, `SELECT order_id, vendor_id FROM order_fulfillments WHERE shop_id = $1 AND id = $2`,
			st.ShopID, fid).Scan(&orderID, &vendorID)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && by.Actor == Vendor && vendorID != by.VendorID) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		o, err := lockOrder(ctx, tx, st.ShopID, orderID)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(o.fs, func(f fRow) bool { return f.id == fid })
		if i < 0 {
			return ErrNotFound
		}
		f := o.fs[i]
		if f.status != Packed && f.status != Shipped {
			return ErrInvalidTransition
		}
		var active bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM shipments WHERE shop_id = $1 AND fulfillment_id = $2
			AND status NOT IN ('cancelled', 'returned'))`, st.ShopID, fid).Scan(&active); err != nil {
			return err
		}
		if active {
			return ErrInvalidTransition // ek fulfillment e ek parcel (shipments_one_active)
		}
		if in.RiderStaffID != nil {
			var ok bool
			// vendor role e rider nijer vendor er staff (vendorID 0 = shop staff, shob)
			rv := int64(0)
			if by.Actor == Vendor {
				rv = by.VendorID
			}
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM staff WHERE shop_id = $1 AND id = $2 AND is_active
				AND ($3::bigint = 0 OR vendor_id = $3))`, st.ShopID, *in.RiderStaffID, rv).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return FieldError("rider_staff_id")
			}
		}
		var cod int64
		if o.method == "cod" {
			cod = f.total
		}
		var tracking *string
		if in.TrackingCode != "" {
			tracking = &in.TrackingCode
		}
		if err := tx.QueryRowContext(ctx, `INSERT INTO shipments (shop_id, order_id, fulfillment_id, vendor_id, carrier, rider_staff_id,
			tracking_code, cod_amount, carrier_fee) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
			st.ShopID, orderID, fid, vendorID, in.Carrier, in.RiderStaffID, tracking, cod, in.CarrierFee).Scan(&sh.ID); err != nil {
			return trackingErr(err)
		}
		sh.TrackingCode, sh.CODAmount = tracking, &cod
		if f.status == Packed {
			if err := transitionFulfillment(ctx, tx, st.ShopID, fid, Shipped, by, "", nil, nil); err != nil {
				return err
			}
		}
		if err := jobs.Enqueue(ctx, tx, jobs.New{ShopID: st.ShopID, Kind: "sms.shipped",
			Payload: shippedPayload{orderID, sh.ID}}); err != nil {
			return err
		}
		if sh.DisplayStatus, err = displayStatus(ctx, tx, st.ShopID, orderID); err != nil {
			return err
		}
		after, _ := json.Marshal(map[string]any{"shipment_id": sh.ID, "carrier": in.Carrier, "cod_amount": cod})
		return logAudit(ctx, tx, st, "shipment.create", "shipment", sh.ID, nil, after)
	})
	return sh, err
}

// UpdateShipment (11.7 item 5 v1.43): shudhu rasta e thaka shipment (na hole 409). delivered →
// TransitionFulfillment(shipped → delivered) (shipment o delivered, COD paid); returned/cancelled shudhu shipment;
// tracking_code bodol. Lock order → fulfillment → shipment. Audit {status, tracking_code}.
func UpdateShipment(ctx context.Context, d *sql.DB, st httpx.Staff, id int64, status string, tracking *string) (Shipment, error) {
	by := ByStaff(st)
	sh := Shipment{ID: id}
	err := db.InTx(ctx, d, func(tx *sql.Tx) error {
		var orderID, fid, vendorID int64
		err := tx.QueryRowContext(ctx, `SELECT order_id, fulfillment_id, vendor_id FROM shipments WHERE shop_id = $1 AND id = $2`,
			st.ShopID, id).Scan(&orderID, &fid, &vendorID)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && by.Actor == Vendor && vendorID != by.VendorID) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if _, err := lockOrder(ctx, tx, st.ShopID, orderID); err != nil {
			return err
		}
		var before struct {
			Status   string  `json:"status"`
			Tracking *string `json:"tracking_code"`
		}
		if err := tx.QueryRowContext(ctx, `SELECT status, tracking_code FROM shipments WHERE shop_id = $1 AND id = $2 FOR UPDATE`,
			st.ShopID, id).Scan(&before.Status, &before.Tracking); err != nil {
			return err
		}
		if before.Status != "booked" && before.Status != "picked" && before.Status != "in_transit" {
			return ErrInvalidTransition
		}
		after := before
		if tracking != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE shipments SET tracking_code = $3 WHERE shop_id = $1 AND id = $2`,
				st.ShopID, id, *tracking); err != nil {
				return trackingErr(err)
			}
			after.Tracking = tracking
		}
		switch status {
		case "delivered":
			if err := transitionFulfillment(ctx, tx, st.ShopID, fid, Delivered, by, "", nil, nil); err != nil {
				return err
			}
		case "returned", "cancelled":
			if _, err := tx.ExecContext(ctx, `UPDATE shipments SET status = $3 WHERE shop_id = $1 AND id = $2`,
				st.ShopID, id, status); err != nil {
				return err
			}
		}
		if status != "" {
			after.Status = status
		}
		sh.Status = after.Status
		if sh.DisplayStatus, err = displayStatus(ctx, tx, st.ShopID, orderID); err != nil {
			return err
		}
		b, _ := json.Marshal(before)
		a, _ := json.Marshal(after)
		return logAudit(ctx, tx, st, "shipment.update", "shipment", id, b, a)
	})
	return sh, err
}
