package order_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/order"
)

// printLink: POST /print-links → url theke token (url er akar ar expires_at o dekhe).
func (f *checkoutFx) printLink(st httpx.Staff, body map[string]any) string {
	f.t.Helper()
	out := f.admJSON(st, "POST", "/api/admin/print-links", body, http.StatusOK)
	kind := body["kind"].(string)
	u, err := url.Parse(out["url"].(string))
	if err != nil || u.Scheme != "https" || u.Host != "a.com" || u.Path != "/print/"+kind || len(u.Query()) != 1 {
		f.t.Fatalf("url %v", out["url"])
	}
	exp, err := time.Parse(time.RFC3339, out["expires_at"].(string))
	if d := time.Until(exp); err != nil || d < 9*time.Minute || d > 10*time.Minute+time.Second {
		f.t.Errorf("expires_at %v (%v)", out["expires_at"], err)
	}
	return u.Query().Get("token")
}

// printGet: GET /api/print?kind=&token= (website SSR er moto X-Shop-Host shoho).
func (f *checkoutFx) printGet(host, kind, token string) *httptest.ResponseRecorder {
	f.t.Helper()
	return f.do("GET", host, "/api/print?"+url.Values{"kind": {kind}, "token": {token}}.Encode(), "", nil)
}

// printData: 200 + no-store, typed.
func (f *checkoutFx) printData(kind, token string) printOut {
	f.t.Helper()
	rec := f.printGet("a.com", kind, token)
	var p printOut
	if rec.Code != http.StatusOK || json.NewDecoder(rec.Body).Decode(&p) != nil {
		f.t.Fatalf("print %s: %d %s", kind, rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		f.t.Errorf("print Cache-Control %q", cc)
	}
	return p
}

type printOut struct {
	Kind  string `json:"kind"`
	Slips []struct {
		OrderNumber    string `json:"order_number"`
		Seq            int    `json:"seq"`
		VendorID       int64  `json:"vendor_id"`
		RecipientName  string `json:"recipient_name"`
		RecipientPhone string `json:"recipient_phone"`
		District       string `json:"district"`
		Area           string `json:"area"`
		Address        string `json:"address"`
		Note           *string
		PaymentMethod  string `json:"payment_method"`
		CODAmount      int64  `json:"cod_amount"`
		Items          []struct {
			SKU            string `json:"sku"`
			Quantity       int    `json:"quantity"`
			VariableWeight bool   `json:"variable_weight"`
			OrderedWeightG *int64 `json:"ordered_weight_g"`
			PackedWeightG  *int64 `json:"packed_weight_g"`
		} `json:"items"`
	} `json:"slips"`
	Invoices []struct {
		OrderNumber   string `json:"order_number"`
		CustomerPhone string `json:"customer_phone"`
		PaymentMethod string `json:"payment_method"`
		PaymentStatus string `json:"payment_status"`
		Subtotal      int64  `json:"subtotal"`
		Total         int64  `json:"total"`
		Items         []struct {
			UnitPrice int64 `json:"unit_price"`
			Quantity  int   `json:"quantity"`
			LineTotal int64 `json:"line_total"`
		} `json:"items"`
	} `json:"invoices"`
	PickList *pickList `json:"pick_list"`
}

// TestPrintLinkIntegration (11.6, 11.7 item 8, 10.1, v1.44): POST /print-links {kind, order_ids|date}: packing_slip/
// invoice 1–50 order, pick_list {date} (na dile shop er aj token e), kind er sathe na mila field 400, vendor invoice 403,
// {url: https://<primary domain>/print/{kind}?token=, expires_at} (10 minute), audit print.link (id/PII na).
// GET /api/print?kind=&token= public: path kind ≠ token kind, onno shop host, expire, bhul token, issuing staff
// inactive → 404; render e vendor scope (onno vendor er fulfillment bad, kichu na thakle 404); data shape; no-store.
func TestPrintLinkIntegration(t *testing.T) {
	f := newCheckout(t)
	p := f.seedPick()
	st := f.staffAs("staff", 0)
	o2 := f.id(`SELECT order_id FROM order_fulfillments WHERE id = $1`, p.f2)
	o6 := f.id(`SELECT order_id FROM order_fulfillments WHERE id = $1`, p.fx)
	f.exec(`UPDATE orders SET note = 'gate e call' WHERE id = $1`, p.o1)

	// packing slip: ids er kram e, fulfillment onujayi, removed item bad
	tok := f.printLink(st, map[string]any{"kind": "packing_slip", "order_ids": []int64{o6, p.o1}})
	if tk, ok := order.VerifyPrint(printSecret, tok, time.Now()); !ok || tk.ShopID != f.shop || tk.StaffID != st.ID || tk.VendorID != 0 ||
		tk.Kind != order.PrintPackingSlip || len(tk.IDs) != 2 || tk.IDs[0] != o6 {
		t.Errorf("token %+v %v", tk, ok)
	}
	if n := f.count(`SELECT count(*) FROM audit_logs WHERE shop_id = $1 AND staff_id = $2 AND action = 'print.link'
		AND entity_type = 'packing_slip' AND after = '{"kind":"packing_slip","count":2}'`, f.shop, st.ID); n != 1 {
		t.Errorf("audit print.link %d", n)
	}
	d := f.printData("packing_slip", tok)
	if d.Kind != "packing_slip" || len(d.Slips) != 3 || d.Invoices != nil || d.PickList != nil {
		t.Fatalf("slips: %+v", d)
	}
	n6, n1 := f.number(o6), f.number(p.o1)
	if a, b, c := d.Slips[0], d.Slips[1], d.Slips[2]; a.OrderNumber != n6 || a.Seq != 1 || a.VendorID != p.vx || b.OrderNumber != n6 ||
		b.Seq != 2 || c.OrderNumber != n1 || c.Seq != 1 || c.CODAmount != 20000 || len(c.Items) != 1 || c.Items[0].Quantity != 2 ||
		c.RecipientPhone != "+8801711000009" || c.Address != "x" || c.Area != "Dhanmondi" || c.PaymentMethod != "cod" ||
		c.Note == nil || *c.Note != "gate e call" {
		t.Errorf("slip: %+v", d.Slips)
	}
	// prepaid e cod_amount 0; cancelled fulfillment bad
	o5 := f.id(`SELECT order_id FROM order_fulfillments WHERE id = $1`, p.f5)
	f.exec(`UPDATE order_fulfillments SET status = 'cancelled' WHERE id = $1`, p.fy)
	d = f.printData("packing_slip", f.printLink(st, map[string]any{"kind": "packing_slip", "order_ids": []int64{o5, o6}}))
	if len(d.Slips) != 2 || d.Slips[0].CODAmount != 0 || !d.Slips[0].Items[0].VariableWeight || d.Slips[0].Items[0].OrderedWeightG == nil ||
		*d.Slips[0].Items[0].OrderedWeightG != 700 || d.Slips[1].Seq != 1 {
		t.Errorf("prepaid/cancelled slip: %+v", d.Slips)
	}
	f.exec(`UPDATE order_fulfillments SET status = 'open' WHERE id = $1`, p.fy)

	// invoice: puro order, customer phone, payment; cancelled order bad
	f.exec(`UPDATE orders SET status = 'cancelled', display_status = 'cancelled' WHERE id = $1`, o2)
	itok := f.printLink(f.staffAs("owner", 0), map[string]any{"kind": "invoice", "order_ids": []int64{o2, p.o1}})
	d = f.printData("invoice", itok)
	if len(d.Invoices) != 1 || d.Slips != nil {
		t.Fatalf("invoice: %+v", d)
	}
	if iv := d.Invoices[0]; iv.OrderNumber != n1 || iv.CustomerPhone != "+8801711000009" || iv.PaymentMethod != "cod" ||
		iv.PaymentStatus != "unpaid" || iv.Total != 20000 || iv.Subtotal != 20000 || len(iv.Items) != 1 ||
		iv.Items[0].UnitPrice != 10000 || iv.Items[0].LineTotal != 20000 {
		t.Errorf("invoice: %+v", iv)
	}
	wantErr(t, "shudhu cancelled order", f.printGet("a.com", "invoice",
		f.printLink(st, map[string]any{"kind": "invoice", "order_ids": []int64{o2}})), http.StatusNotFound, "not_found")

	// pick list: date token e, data GET pick list er moto; date chhara shop er aj
	ptok := f.printLink(st, map[string]any{"kind": "pick_list", "date": "2026-01-11"})
	d = f.printData("pick_list", ptok)
	if d.PickList == nil || d.PickList.Date != "2026-01-11" || len(d.PickList.Fulfillments) != 4 { // o2 cancelled
		t.Errorf("print pick list: %+v", d.PickList)
	}
	tk, _ := order.VerifyPrint(printSecret, f.printLink(st, map[string]any{"kind": "pick_list"}), time.Now())
	if today := f.str(`SELECT (now() AT TIME ZONE 'Asia/Dhaka')::date::text`); tk.Date != today {
		t.Errorf("pick_list default date %q, chai %s", tk.Date, today)
	}

	// verify: kind, shop, expire, bhul token → 404 (kichu bola na)
	exp := order.SignPrint(printSecret, order.PrintToken{ShopID: f.shop, Kind: order.PrintPackingSlip, IDs: []int64{p.o1}, StaffID: st.ID,
		Exp: time.Now().Add(-time.Second).Unix()})
	b := f.id(`SELECT id FROM shops WHERE slug = 'b'`)
	other := order.SignPrint(printSecret, order.PrintToken{ShopID: b, Kind: order.PrintPackingSlip, IDs: []int64{p.o1},
		StaffID: f.id(`SELECT id FROM staff WHERE shop_id = $1`, b), Exp: time.Now().Add(time.Minute).Unix()})
	for name, c := range map[string][3]string{
		"path kind alada":     {"a.com", "invoice", tok},
		"kind nai":            {"a.com", "", tok},
		"onno shop er host":   {"b.com", "packing_slip", tok},
		"onno shop er token":  {"a.com", "packing_slip", other},
		"expire":              {"a.com", "packing_slip", exp},
		"bhul token":          {"a.com", "packing_slip", tok + "x"},
		"token nai":           {"a.com", "packing_slip", ""},
		"pick_list kind e":    {"a.com", "pick_list", tok},
		"slip path e pick tk": {"a.com", "packing_slip", ptok},
	} {
		wantErr(t, name, f.printGet(c[0], c[1], c[2]), http.StatusNotFound, "not_found")
	}
	// issuing staff inactive hole ager link o bondho
	f.exec(`UPDATE staff SET is_active = false WHERE id = $1`, st.ID)
	wantErr(t, "staff inactive", f.printGet("a.com", "packing_slip", tok), http.StatusNotFound, "not_found")
	wantErr(t, "staff inactive pick list", f.printGet("a.com", "pick_list", ptok), http.StatusNotFound, "not_found")
	f.exec(`UPDATE staff SET is_active = true WHERE id = $1`, st.ID)
	f.printData("packing_slip", tok)

	// vendor: nijer packing_slip/pick_list, invoice 403
	vo := f.staffAs("vendor_owner", p.vx)
	wantErr(t, "vendor invoice", f.adm(vo, "POST", "/api/admin/print-links", map[string]any{"kind": "invoice", "order_ids": []int64{o6}}),
		http.StatusForbidden, "forbidden")
	vtok := f.printLink(vo, map[string]any{"kind": "packing_slip", "order_ids": []int64{o6, p.o1}})
	if tk, _ := order.VerifyPrint(printSecret, vtok, time.Now()); tk.VendorID != p.vx {
		t.Errorf("vendor token vendor_id %d", tk.VendorID)
	}
	if d = f.printData("packing_slip", vtok); len(d.Slips) != 1 || d.Slips[0].VendorID != p.vx {
		t.Errorf("vendor slip: %+v", d.Slips)
	}
	wantErr(t, "vendor er kichu nai", f.printGet("a.com", "packing_slip",
		f.printLink(vo, map[string]any{"kind": "packing_slip", "order_ids": []int64{p.o1}})), http.StatusNotFound, "not_found")
	// token e vendor scope: vendor invoice token (sign kora gele-o) render e 404
	vinv := order.SignPrint(printSecret, order.PrintToken{ShopID: f.shop, Kind: order.PrintInvoice, IDs: []int64{o6}, StaffID: vo.ID,
		VendorID: p.vx, Exp: time.Now().Add(time.Minute).Unix()})
	wantErr(t, "vendor invoice token", f.printGet("a.com", "invoice", vinv), http.StatusNotFound, "not_found")

	// body validation
	ids := func(n int) []int64 {
		out := make([]int64, n)
		for i := range out {
			out[i] = int64(i + 1)
		}
		return out
	}
	f.printLink(st, map[string]any{"kind": "invoice", "order_ids": ids(50)})
	for name, c := range map[string]struct {
		body  any
		field string
	}{
		"51 order":              {map[string]any{"kind": "invoice", "order_ids": ids(51)}, "order_ids"},
		"order nai":             {map[string]any{"kind": "packing_slip", "order_ids": []int64{}}, "order_ids"},
		"order_ids chhara":      {map[string]any{"kind": "packing_slip"}, "order_ids"},
		"duplicate":             {map[string]any{"kind": "packing_slip", "order_ids": []int64{1, 1}}, "order_ids"},
		"0 id":                  {map[string]any{"kind": "packing_slip", "order_ids": []int64{0}}, "order_ids"},
		"slip e date":           {map[string]any{"kind": "packing_slip", "order_ids": []int64{1}, "date": "2026-01-11"}, "date"},
		"pick_list e order_ids": {map[string]any{"kind": "pick_list", "order_ids": []int64{1}}, "order_ids"},
		"pick_list bhul date":   {map[string]any{"kind": "pick_list", "date": "2026-02-30"}, "date"},
		"pick_list year 0":      {map[string]any{"kind": "pick_list", "date": "0000-01-01"}, "date"},
		"ochena kind":           {map[string]any{"kind": "label", "order_ids": []int64{1}}, "kind"},
		"kind nai":              {map[string]any{"order_ids": []int64{1}}, "kind"},
		"ochena field":          {map[string]any{"kind": "pick_list", "shop_id": 2}, "body"},
		"json bhul":             {`{"kind":`, "body"},
	} {
		e := wantErr(t, name, f.adm(st, "POST", "/api/admin/print-links", c.body), http.StatusBadRequest, "validation_failed")
		if e.Fields[c.field] != "invalid" {
			t.Errorf("%s: fields %v, chai %s", name, e.Fields, c.field)
		}
	}
	if !strings.HasPrefix(tok, "ey") { // base64url(json) "{" diye shuru
		t.Errorf("token %q", tok)
	}
}
