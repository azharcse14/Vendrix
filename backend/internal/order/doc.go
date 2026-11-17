// Package order: order ar fulfillment er status kon theke kon e jete pare (8.8), checkout quote ar order create (7.5, 8.6),
// checkout er risk flag, OTP ar COD confirm er niyom (8.7, 8.10, 8.19), order page, tracking, nijer order list ar order er
// job (SMS, alert, auto-cancel; 8.17). Status ar risk er function pure; DB e status bodol (lock, side effect, cascade)
// transition.go e; HTTP handler.go, kram service.go, SQL store.go, confirm.go, view.go, jobs.go e.
//
// Pack, undo, edit, shipment, deliver, return pack.go, edit.go, shipment.go e (status bodol transition.go diye);
// pick list ar print link picklist.go, print.go e; admin route ar dashboard admin.go, dashboard.go e.
//
// Feature doc: docs/features/order-status.md, docs/features/checkout.md, docs/features/fulfillment.md,
// docs/features/print.md, docs/features/admin-api.md
package order
