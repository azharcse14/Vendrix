package order_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"vendrix/backend/internal/inventory"
)

// TestCheckoutWastageRaceIntegration (11.7 item 2, M7a acceptance): ekoi variant e checkout ar wastage (batch_expiry
// on, FIFO batch) ekshathe: stock 0 er niche na, deadlock/500 na, shob kaj hoy ba out_of_stock; stock = shuru −
// order − wastage, batch e shudhu wastage kome (order er batch packing e). Lock: variant → batch id kram.
func TestCheckoutWastageRaceIntegration(t *testing.T) {
	f := newCheckout(t)
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{features}', '["batch_expiry"]') WHERE shop_id = $1`, f.shop)
	v := f.variant("race", 50000, 0)
	owner := f.staffAs("owner", 0)
	for _, q := range []int{4, 6} {
		if _, err := inventory.CreateBatch(f.ctx, f.d, owner, inventory.BatchInput{VariantID: v, Quantity: q}); err != nil {
			t.Fatal(err)
		}
	}
	const n = 10
	carts := make([]string, n)
	for i := range carts {
		carts[i] = f.guestCart([2]int64{v, 1})
	}
	type res struct {
		rec *httptest.ResponseRecorder
		err error
	}
	out := parallel(2*n, func(i int) res {
		if i%2 == 0 {
			return res{rec: f.place(idemKey(), carts[i/2], orderBody(phoneN(100+i), 56000))}
		}
		_, err := inventory.Adjust(f.ctx, f.d, owner, inventory.AdjustInput{VariantID: v, Change: -1, Reason: "wastage", Note: "race"})
		return res{err: err}
	})
	placed, wasted := 0, 0
	for i, r := range out {
		var oos *inventory.OutOfStockError
		switch {
		case r.rec != nil && r.rec.Code == http.StatusCreated:
			placed++
		case r.rec != nil:
			wantErr(t, "checkout", r.rec, http.StatusConflict, "out_of_stock")
		case r.err == nil:
			wasted++
		case !errors.As(r.err, &oos):
			t.Errorf("wastage %d: %v", i, r.err)
		}
	}
	stock := f.count(`SELECT stock FROM product_variants WHERE id = $1`, v)
	batches := f.count(`SELECT sum(quantity_remaining) FROM inventory_batches WHERE variant_id = $1`, v)
	if stock != int64(n-placed-wasted) || stock != 0 || batches != int64(n-wasted) {
		t.Errorf("stock %d batch %d, order %d wastage %d", stock, batches, placed, wasted)
	}
	if m := f.count(`SELECT count(*) FROM inventory_movements WHERE variant_id = $1 AND reason = 'wastage'`, v); m != int64(wasted) {
		t.Errorf("wastage movement %d, chai %d", m, wasted)
	}
}
