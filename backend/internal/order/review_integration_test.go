package order_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/order"
)

// withPeer: test e connection er IP (X-Test-Peer) bodlay, ClientIP er age.
func withPeer(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := r.Header.Get("X-Test-Peer"); p != "" {
			r.RemoteAddr = p
		}
		h.ServeHTTP(w, r)
	})
}

// TestCheckoutOTPIPLimitIntegration (7.8, 8.19 item 13 a): checkout er OTP send login er ekoi client IP limit e
// (ghontay 10, ek gona): 1 login + 9 checkout, tarpor checkout ar login duitai 429 + Retry-After.
func TestCheckoutOTPIPLimitIntegration(t *testing.T) {
	f := newCheckout(t)
	f.srv = httpx.ClientIP("")(f.srv)
	f.otpMode("risky")
	big := f.variant("big", 250000, 50)
	login := func(ph string) *httptest.ResponseRecorder {
		return f.post("/api/auth/otp/send", "", "", "", map[string]string{"phone": ph, "purpose": "login", "turnstile_token": "tok"})
	}
	if rec := login(phoneN(100)); rec.Code != http.StatusOK {
		t.Fatalf("login send: %d %s", rec.Code, rec.Body.String())
	}
	for i := range 9 {
		wantErr(t, fmt.Sprintf("checkout send %d", i+1), f.place(idemKey(), f.guestCart([2]int64{big, 1}), orderBody(phoneN(101+i), 250000)),
			http.StatusConflict, "otp_required")
	}
	wantRetry(t, "11 tomo (checkout)", f.place(idemKey(), f.guestCart([2]int64{big, 1}), orderBody(phoneN(120), 250000)))
	wantRetry(t, "12 tomo (login)", login(phoneN(121)))
	if n := f.count(`SELECT count(*) FROM otp_codes`); n != 10 {
		t.Errorf("OTP %d, chai 10", n)
	}
}

// TestOrderIPv6Integration (8.19 item 3, 9, 13 c): ek IPv6 /64 er alada address ek IP: 3 tomo order e many_orders,
// 11 tomo 429; onno /64 chole.
func TestOrderIPv6Integration(t *testing.T) {
	f := newCheckout(t)
	f.srv = withPeer(httpx.ClientIP("")(f.srv))
	rice := f.variant("rice", 50000, 100)
	from := func(peer, ph string) *httptest.ResponseRecorder {
		return f.post("/api/orders", idemKey(), f.guestCart([2]int64{rice, 1}), "", orderBody(ph, 56000), "X-Test-Peer", peer)
	}
	for i := range 10 {
		p := decodePlaced(t, from(fmt.Sprintf("[2001:db8:1:1::%x]:443", i+1), phoneN(50+i)), http.StatusCreated)
		if fl := f.str(`SELECT risk_flags::text FROM orders WHERE public_token = $1`, p.PublicToken); (i >= 2) != strings.Contains(fl, "many_orders") {
			t.Errorf("order %d: flags %s", i+1, fl)
		}
	}
	wantRetry(t, "ek /64 er 11 tomo", from("[2001:db8:1:1::ff]:443", phoneN(70)))
	decodePlaced(t, from("[2001:db8:1:2::1]:443", phoneN(71)), http.StatusCreated)
}

// TestCustomerActorIntegration (8.19 item 13 d): customer cancel/confirm er history actor_id shudhu order er
// nijer user hole; onno account er login e NULL.
func TestCustomerActorIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 50000, 100)
	actor := func(token, to string) string {
		t.Helper()
		return f.str(`SELECT coalesce(h.actor_id::text, 'NULL') FROM order_status_history h JOIN orders o ON o.id = h.order_id
			WHERE o.public_token = $1 AND h.to_status = $2 AND h.fulfillment_id IS NULL`, token, to)
	}
	// guest er order, onno account login kore cancel
	g := f.guestOrder(rice, phoneN(1), "")
	if rec := f.do("POST", "a.com", "/api/orders/"+g.PublicToken+"/cancel", f.tok, map[string]any{}); rec.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	if a := actor(g.PublicToken, "cancelled"); a != "NULL" {
		t.Errorf("guest order cancel e actor_id %s, NULL chai", a)
	}
	// confirm: guest order, onno account
	f.setCheckout("cod_confirmation", `"all"`)
	g = f.guestOrder(rice, phoneN(2), "")
	if rec := f.do("POST", "a.com", "/api/confirm/"+order.ConfirmToken(confirmSecret, g.PublicToken), f.tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body.String())
	}
	if a := actor(g.PublicToken, "confirmed"); a != "NULL" {
		t.Errorf("guest order confirm e actor_id %s, NULL chai", a)
	}
	// nijer order: actor_id = user
	f.userCart([2]int64{rice, 1})
	own := decodePlaced(t, f.post("/api/orders", idemKey(), "", f.tok, orderBody(userPhone, 56000)), http.StatusCreated)
	if rec := f.do("POST", "a.com", "/api/orders/"+own.PublicToken+"/cancel", f.tok, map[string]any{}); rec.Code != http.StatusOK {
		t.Fatalf("own cancel: %d %s", rec.Code, rec.Body.String())
	}
	if a := actor(own.PublicToken, "cancelled"); a != fmt.Sprint(f.user) {
		t.Errorf("nijer order cancel e actor_id %s, %d chai", a, f.user)
	}
}

// TestOrderPlacedSkipsCancelledIntegration (8.19 item 13 g): job chalanor age order cancel hole SMS na, job shesh.
func TestOrderPlacedSkipsCancelledIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 50000, 10)
	sms := &smsLog{}
	h := order.JobHandlers(f.d, sms, slog.New(slog.DiscardHandler), confirmSecret)
	g := f.guestOrder(rice, phoneN(1), "")
	if rec := f.do("POST", "a.com", "/api/orders/"+g.PublicToken+"/cancel", "", map[string]any{}); rec.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", rec.Code, rec.Body.String())
	}
	if err := f.runJob(h, "sms.order_placed", f.id(`SELECT id FROM orders WHERE public_token = $1`, g.PublicToken)); err != nil {
		t.Fatalf("job: %v", err)
	}
	if got := sms.take(); len(got) != 0 {
		t.Errorf("cancelled order e SMS: %q", got)
	}
}
