package order

import (
	"context"
	"database/sql"
	"errors"
)

// By: ke bodlay (history er actor_type/actor_id); Vendor e VendorID.
type By struct {
	Actor    Actor
	ID       int64
	VendorID int64
}

var (
	ErrNotFound          = errors.New("order: nai")                 // 404 not_found
	ErrInvalidTransition = errors.New("order: ei bodol hoy na")     // 409 invalid_transition
	ErrPaymentRequired   = errors.New("order: online, taka ase ni") // 409 payment_required
	ErrNotImplemented    = errors.New("order: M7 e")                // packed/shipped/delivered/returned
)

func TransitionOrder(ctx context.Context, tx *sql.Tx, shopID, orderID int64, to Status, by By, note string) error {
	return errors.ErrUnsupported
}

func TransitionFulfillment(ctx context.Context, tx *sql.Tx, shopID, fulfillmentID int64, to FStatus, by By, note string) error {
	return errors.ErrUnsupported
}
