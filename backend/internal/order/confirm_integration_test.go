package order_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vendrix/backend/internal/order"
)

// confirmReq: /api/confirm er GET/POST, host er shop e.
func (f *checkoutFx) confirmReq(method, host, token string) *httptest.ResponseRecorder {
	f.t.Helper()
	r := httptest.NewRequest(method, "/api/confirm/"+token, nil)
	r.Header.Set("X-Shop-Host", host)
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r)
	return rec
}

// TestConfirmTokenIntegration (14.2 test 31, 8.10, Rule 11): COD confirm shudhu SMS er confirm_token diye;
// public_token, ochena token ba onno shop e 404; GET e mask kora shar-songkhep; POST e confirmed (lowercase
// link o); duibar 409 invalid_transition; confirm na lagle ba online e token_hash nai.
func TestConfirmTokenIntegration(t *testing.T) {
	f := newCheckout(t)
	rice := f.variant("rice", 50000, 10)
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,cod_confirmation}', '"all"') WHERE shop_id = $1`, f.shop)
	ck := f.guestCart([2]int64{rice, 1})
	phone := phoneN(1)
	p := decodePlaced(t, f.place(idemKey(), ck, orderBody(phone, f.quoteTotal(ck, "", "", "Dhaka", "Dhanmondi", phone))), http.StatusCreated)
	if p.Status != "pending" {
		t.Fatalf("cod_confirmation all e status %s", p.Status)
	}
	tok := order.ConfirmToken(confirmSecret, p.PublicToken)
	sum := sha256.Sum256([]byte(tok))
	var hash []byte
	if err := f.d.QueryRowContext(f.ctx, `SELECT confirm_token_hash FROM orders WHERE public_token = $1`, p.PublicToken).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if len(tok) != 26 || !bytes.Equal(hash, sum[:]) {
		t.Fatalf("confirm_token_hash sha256(ConfirmToken) na (token %q)", tok)
	}

	// public_token, ochena token, onno shop: 404 (GET ar POST)
	for _, c := range []struct{ name, host, token string }{
		{"public_token", "a.com", p.PublicToken},
		{"ochena", "a.com", rand.Text()},
		{"onno shop", "b.com", tok},
	} {
		for _, m := range []string{"GET", "POST"} {
			wantErr(t, m+" "+c.name, f.confirmReq(m, c.host, c.token), http.StatusNotFound, "not_found")
		}
	}
	if s := f.str(`SELECT status FROM orders WHERE public_token = $1`, p.PublicToken); s != "pending" {
		t.Fatalf("404 er por status %s", s)
	}

	// GET: shar-songkhep, phone mask, thikana shudhu district + area, no-store
	rec := f.confirmReq("GET", "a.com", tok)
	var v struct {
		OrderNumber   string `json:"order_number"`
		DisplayStatus string `json:"display_status"`
		PhoneMasked   string `json:"phone_masked"`
		District      string `json:"district"`
		Area          string `json:"area"`
		Total         int64  `json:"total"`
		Items         []struct {
			Quantity  int   `json:"quantity"`
			LineTotal int64 `json:"line_total"`
		} `json:"items"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &v) != nil {
		t.Fatalf("GET confirm: %d %s", rec.Code, rec.Body.String())
	}
	if v.OrderNumber != p.OrderNumber || v.DisplayStatus != "pending" || v.PhoneMasked != phone[:6]+"****"+phone[len(phone)-4:] ||
		v.District != "Dhaka" || v.Area != "Dhanmondi" || v.Total != 56000 || len(v.Items) != 1 || v.Items[0].Quantity != 1 ||
		v.Items[0].LineTotal != 50000 {
		t.Errorf("GET confirm: %s", rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range []string{phone, areaLine, p.PublicToken, tok} {
		if strings.Contains(body, secret) {
			t.Errorf("GET confirm e %q dekha jay: %s", secret, body)
		}
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control %q", cc)
	}

	// POST: lowercase link o chole → confirmed, history customer
	rec = f.confirmReq("POST", "a.com", strings.ToLower(tok))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"confirmed"`) {
		t.Fatalf("POST confirm: %d %s", rec.Code, rec.Body.String())
	}
	if s := f.str(`SELECT status || '/' || display_status FROM orders WHERE public_token = $1`, p.PublicToken); s != "confirmed/confirmed" {
		t.Errorf("confirm er por %s", s)
	}
	if n := f.count(`SELECT count(*) FROM order_status_history h JOIN orders o ON o.id = h.order_id WHERE o.public_token = $1
		AND h.to_status = 'confirmed' AND h.actor_type = 'customer'`, p.PublicToken); n != 1 {
		t.Errorf("confirm er history row %d", n)
	}
	// duibar: 409 invalid_transition
	wantErr(t, "abar confirm", f.confirmReq("POST", "a.com", tok), http.StatusConflict, "invalid_transition")

	// cod_confirmation none: confirmed, token_hash nai
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{checkout,cod_confirmation}', '"none"') WHERE shop_id = $1`, f.shop)
	ck = f.guestCart([2]int64{rice, 1})
	p2 := decodePlaced(t, f.place(idemKey(), ck, orderBody(phoneN(2), 56000)), http.StatusCreated)
	if n := f.count(`SELECT count(*) FROM orders WHERE public_token = $1 AND status = 'confirmed' AND confirm_token_hash IS NULL`,
		p2.PublicToken); n != 1 {
		t.Error("confirm na lagle-o confirm_token_hash ache")
	}
	wantErr(t, "confirm na laga order er token", f.confirmReq("POST", "a.com", order.ConfirmToken(confirmSecret, p2.PublicToken)),
		http.StatusNotFound, "not_found")

	// online order e token_hash DB atkay (orders_confirm_token_cod_check)
	online, _ := f.seedOrder("sslcommerz", "pending", "pending", seedF{vendor: f.vnd, status: "open", items: [][2]int64{{rice, 1}}})
	if _, err := f.d.ExecContext(f.ctx, `UPDATE orders SET confirm_token_hash = $2 WHERE id = $1`, online, sum[:]); err == nil ||
		!strings.Contains(err.Error(), "orders_confirm_token_cod_check") {
		t.Errorf("online order e confirm_token_hash: %v", err)
	}
}
