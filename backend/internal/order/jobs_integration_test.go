package order_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"vendrix/backend/internal/jobs"
	"vendrix/backend/internal/notify"
	"vendrix/backend/internal/order"
)

// smsLog: fake notify.SMS; pathano phone|lekha jome.
type smsLog struct {
	mu   sync.Mutex
	sent []string
}

func (s *smsLog) Send(_ context.Context, _ int64, phone, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, phone+"|"+text)
	return nil
}

func (s *smsLog) take() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.sent
	s.sent = nil
	return out
}

// runJob: jobs table er ei order er ei kind er job (ekta-i thakte hobe) handler diye chalay.
func (f *checkoutFx) runJob(h map[string]jobs.Handler, kind string, oid int64) error {
	f.t.Helper()
	var j jobs.Job
	if err := f.d.QueryRowContext(f.ctx, `SELECT id, shop_id, kind, payload FROM jobs WHERE kind = $1 AND (payload->>'order_id')::bigint = $2`,
		kind, oid).Scan(&j.ID, &j.ShopID, &j.Kind, &j.Payload); err != nil {
		f.t.Fatalf("%s job (order %d): %v", kind, oid, err)
	}
	return h[kind](f.ctx, j)
}

// jobCount: ei order er ei kind er job, payload shudhu {order_id} (8.10 item 1: token/phone payload e na).
func (f *checkoutFx) jobCount(kind string, oid int64) int64 {
	f.t.Helper()
	return f.count(`SELECT count(*) FROM jobs WHERE kind = $1 AND shop_id = $3 AND payload = jsonb_build_object('order_id', $2::bigint)`,
		kind, oid, f.shop)
}

func (f *checkoutFx) setCheckout(key, value string) {
	f.t.Helper()
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, ARRAY['checkout', $2], $3::jsonb) WHERE shop_id = $1`,
		f.shop, key, value)
}

// TestOrderPlacedSMSIntegration (14.2 test 31, 8.6 step 18, 8.10 item 1, 8.16, 8.19 item 7): order er tx e
// sms.order_placed ar alert.new_order (payload shudhu order_id); confirm lagle sms.order_confirm lekha +
// /confirm/<ConfirmToken> link, na hole sms.order_placed + /order/<public_token>; primary domain; customer er
// locale; retry e ekoi lekha (ekoi link); idempotent retry e notun job na; alert e phone na.
func TestOrderPlacedSMSIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 50000, 10)
	sms := &smsLog{}
	var logs bytes.Buffer
	h := order.JobHandlers(f.d, sms, slog.New(slog.NewTextHandler(&logs, nil)), confirmSecret)

	// confirm lage (cod_confirmation all), bn
	f.setCheckout("cod_confirmation", `"all"`)
	ck := f.guestCart([2]int64{rice, 1})
	phone := phoneN(1)
	key := idemKey()
	body := orderBody(phone, f.quoteTotal(ck, "", "", "Dhaka", "Dhanmondi", phone))
	p := decodePlaced(t, f.place(key, ck, body), http.StatusCreated)
	oid := f.id(`SELECT id FROM orders WHERE public_token = $1`, p.PublicToken)
	for _, k := range []string{"sms.order_placed", "alert.new_order"} {
		if n := f.jobCount(k, oid); n != 1 {
			t.Fatalf("%s job %d, 1 chai (payload shudhu order_id)", k, n)
		}
	}
	// idempotent retry: ager order, notun job na
	decodePlaced(t, f.place(key, ck, body), http.StatusOK)
	if n := f.count(`SELECT count(*) FROM jobs WHERE kind IN ('sms.order_placed', 'alert.new_order')`); n != 2 {
		t.Errorf("retry er por job %d, 2 chai", n)
	}
	link := "https://a.com/confirm/" + order.ConfirmToken(confirmSecret, p.PublicToken)
	want := phone + "|" + notify.OrderText("sms.order_confirm", "bn", nil, "দোকান", p.OrderNumber, link)
	for i := range 2 { // retry e ekoi link
		if err := f.runJob(h, "sms.order_placed", oid); err != nil {
			t.Fatalf("sms.order_placed (%d): %v", i, err)
		}
		if got := sms.take(); len(got) != 1 || got[0] != want || !strings.Contains(got[0], link) {
			t.Errorf("confirm SMS (%d): %q, chai %q", i, got, want)
		}
	}
	if strings.Contains(want, p.PublicToken) {
		t.Error("confirm SMS e public_token")
	}

	// alert.new_order: order number, phone na
	if err := f.runJob(h, "alert.new_order", oid); err != nil {
		t.Fatal(err)
	}
	if s := logs.String(); !strings.Contains(s, p.OrderNumber) || strings.Contains(s, phone) || strings.Contains(s, p.PublicToken) {
		t.Errorf("alert.new_order log: %s", s)
	}

	// confirm lage na, en: /order/<public_token>
	f.setCheckout("cod_confirmation", `"none"`)
	ck = f.guestCart([2]int64{rice, 1})
	phone2 := phoneN(2)
	rec := f.post("/api/orders", idemKey(), ck, "", orderBody(phone2, 56000), "Accept-Language", "en")
	p2 := decodePlaced(t, rec, http.StatusCreated)
	oid2 := f.id(`SELECT id FROM orders WHERE public_token = $1`, p2.PublicToken)
	if err := f.runJob(h, "sms.order_placed", oid2); err != nil {
		t.Fatal(err)
	}
	want = phone2 + "|" + notify.OrderText("sms.order_placed", "en", nil, "দোকান", p2.OrderNumber, "https://a.com/order/"+p2.PublicToken)
	if got := sms.take(); len(got) != 1 || got[0] != want {
		t.Errorf("order SMS (en): %q, chai %q", got, want)
	}
	if n := f.jobCount("order.auto_cancel_unconfirmed", oid2); n != 0 {
		t.Error("confirmed order e auto_cancel job")
	}

	// onno shop er job e order pay na (Rule 3)
	other := f.id(`SELECT id FROM shops WHERE slug = 'b'`)
	if err := h["sms.order_placed"](f.ctx, jobs.Job{ShopID: other, Kind: "sms.order_placed",
		Payload: []byte(`{"order_id":` + strconv.FormatInt(oid2, 10) + `}`)}); err == nil || len(sms.take()) != 0 {
		t.Error("onno shop er order er SMS gelo")
	}
}

// TestAutoCancelUnconfirmedIntegration (14.2 test 26, 8.6 step 18, 8.10 item 4, 8.19 item 12 viii): COD pending
// e job RunAt = placed_at + setting er ghonta, dedupe_key; shomoy hole worker cancel kore (system), stock ar
// coupon ferot, sms.cancelled; duibar chalale kichu bodlay na; confirmed hole kichu na; shomoy na hole chole na.
func TestAutoCancelUnconfirmedIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 20000, 10)
	cid := f.id(`INSERT INTO coupons (shop_id, code, kind, value) VALUES ($1, 'TAKA', 'fixed', 5000) RETURNING id`, f.shop)
	f.setCheckout("cod_confirmation", `"all"`)
	f.setCheckout("auto_cancel_unconfirmed_hours", `2`)
	sms := &smsLog{}
	log := slog.New(slog.DiscardHandler)
	h := order.JobHandlers(f.d, sms, log, confirmSecret)

	place := func(i int, coupon string) (placed, int64) {
		ck := f.guestCart([2]int64{rice, 2})
		ph := phoneN(i)
		b := orderBody(ph, f.quoteTotal(ck, "", coupon, "Dhaka", "Dhanmondi", ph))
		b["coupon_code"] = coupon
		p := decodePlaced(t, f.place(idemKey(), ck, b), http.StatusCreated)
		if p.Status != "pending" {
			t.Fatalf("status %s, pending chai", p.Status)
		}
		return p, f.id(`SELECT id FROM orders WHERE public_token = $1`, p.PublicToken)
	}
	p1, o1 := place(1, "TAKA")
	// pore setting bodlaleo ager order er time bodlay na
	f.setCheckout("auto_cancel_unconfirmed_hours", `48`)
	p2, o2 := place(2, "")
	for oid, hours := range map[int64]int{o1: 2, o2: 48} {
		if n := f.count(`SELECT count(*) FROM jobs j JOIN orders o ON o.id = $1 WHERE j.kind = 'order.auto_cancel_unconfirmed'
			AND j.dedupe_key = 'order.auto_cancel_unconfirmed:' || o.id AND j.run_at = o.placed_at + make_interval(hours => $2)
			AND j.payload = jsonb_build_object('order_id', o.id)`, oid, hours); n != 1 {
			t.Errorf("order %d er auto_cancel job (RunAt placed_at + %d ghonta) %d", oid, hours, n)
		}
	}
	if n := f.count(`SELECT stock FROM product_variants WHERE id = $1`, rice); n != 6 {
		t.Fatalf("stock %d", n)
	}
	// o2 customer confirm kore (SMS er link)
	if err := order.ConfirmByToken(f.ctx, f.d, f.shop, order.ConfirmToken(confirmSecret, p2.PublicToken), customer); err != nil {
		t.Fatal(err)
	}

	// worker: o1 er shomoy holo, o2 er hoyni
	f.exec(`UPDATE jobs SET run_at = now() - interval '1 second' WHERE dedupe_key = $1`, "order.auto_cancel_unconfirmed:"+strconv.FormatInt(o1, 10))
	w := jobs.NewWorker(f.d, log, h)
	for {
		n, err := w.RunOnce(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	if n := f.count(`SELECT count(*) FROM jobs WHERE kind = 'order.auto_cancel_unconfirmed' AND done_at IS NOT NULL`); n != 1 {
		t.Errorf("done auto_cancel job %d, 1 chai (o2 er shomoy hoyni)", n)
	}
	if n := f.count(`SELECT count(*) FROM jobs WHERE failed_at IS NOT NULL OR last_error IS NOT NULL`); n != 0 {
		t.Errorf("fail kora job %d: %s", n, f.str(`SELECT string_agg(kind || ': ' || last_error, '; ') FROM jobs WHERE last_error IS NOT NULL`))
	}
	check := func(when string) {
		t.Helper()
		if s := f.orderStatus(o1); s != "cancelled/cancelled" {
			t.Errorf("%s: o1 %s", when, s)
		}
		if s := f.fStatuses(o1); s != "cancelled" {
			t.Errorf("%s: o1 fulfillment %s", when, s)
		}
		if n := f.count(`SELECT stock FROM product_variants WHERE id = $1`, rice); n != 8 {
			t.Errorf("%s: stock %d, 8 chai (o1 er 2 ferot, o2 er 2 na)", when, n)
		}
		if n := f.count(`SELECT COALESCE(sum(change), 0) FROM inventory_movements WHERE order_id = $1 AND reason = 'cancel'`, o1); n != 2 {
			t.Errorf("%s: cancel movement %d", when, n)
		}
		if n := f.count(`SELECT used_count FROM coupons WHERE id = $1`, cid); n != 0 {
			t.Errorf("%s: used_count %d", when, n)
		}
		if n := f.count(`SELECT count(*) FROM coupon_redemptions WHERE order_id = $1`, o1); n != 0 {
			t.Errorf("%s: redemption ache", when)
		}
		if n := f.count(`SELECT count(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'cancelled' AND actor_type = 'system'`, o1); n != 2 {
			t.Errorf("%s: system cancel history %d, 2 chai (order + fulfillment)", when, n)
		}
		if n := f.jobCount("sms.cancelled", o1); n != 1 {
			t.Errorf("%s: sms.cancelled job %d", when, n)
		}
		if s := f.orderStatus(o2); s != "confirmed/confirmed" {
			t.Errorf("%s: o2 %s", when, s)
		}
	}
	check("prothom bar")
	// duibar chalale-o thik (8.17 idempotent), confirmed o2 te kichu na
	for _, oid := range []int64{o1, o2} {
		if err := f.runJob(h, "order.auto_cancel_unconfirmed", oid); err != nil {
			t.Errorf("abar chalano (order %d): %v", oid, err)
		}
	}
	check("duibar")

	// sms.cancelled: number, link na; customer cancel e o enqueue
	sms.take()
	if err := f.runJob(h, "sms.cancelled", o1); err != nil {
		t.Fatal(err)
	}
	want := phoneN(1) + "|" + notify.OrderText("sms.cancelled", "bn", nil, "দোকান", p1.OrderNumber, "")
	if got := sms.take(); len(got) != 1 || got[0] != want {
		t.Errorf("sms.cancelled: %q, chai %q", got, want)
	}
	f.exec(`UPDATE product_variants SET stock = 10 WHERE id = $1`, rice)
	p3, o3 := place(3, "")
	if rec := f.post("/api/orders/"+p3.PublicToken+"/cancel", "", "", "", map[string]any{}); rec.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	if n := f.jobCount("sms.cancelled", o3); n != 1 {
		t.Errorf("customer cancel e sms.cancelled %d", n)
	}
}
