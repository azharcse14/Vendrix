package order_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"vendrix/backend/internal/httpx"
)

// pickList: GET pick list er response (test er jonno typed).
type pickList struct {
	Date     string  `json:"date"`
	Variants []pickV `json:"variants"`
	Areas    []struct {
		District string  `json:"district"`
		Area     string  `json:"area"`
		Variants []pickV `json:"variants"`
	} `json:"areas"`
	Fulfillments []struct {
		ID            int64           `json:"id"`
		OrderID       int64           `json:"order_id"`
		OrderNumber   string          `json:"order_number"`
		Seq           int             `json:"seq"`
		VendorID      int64           `json:"vendor_id"`
		RecipientName string          `json:"recipient_name"`
		District      string          `json:"district"`
		Area          string          `json:"area"`
		ConfirmedAt   json.RawMessage `json:"confirmed_at"`
	} `json:"fulfillments"`
}

type pickV struct {
	VariantID      int64           `json:"variant_id"`
	ProductName    json.RawMessage `json:"product_name"`
	VariantLabel   json.RawMessage `json:"variant_label"`
	SKU            string          `json:"sku"`
	VariableWeight bool            `json:"variable_weight"`
	Quantity       int             `json:"quantity"`
	OrderedWeightG *int64          `json:"ordered_weight_g"`
}

// pv: "variant:qty" ba ojon-variable e "variant:qty:gram", variant id kram e.
func pv(vs []pickV) string {
	var out []string
	for _, v := range vs {
		x := fmt.Sprintf("%d:%d", v.VariantID, v.Quantity)
		if v.OrderedWeightG != nil {
			x += fmt.Sprintf(":%d", *v.OrderedWeightG)
		}
		out = append(out, x)
	}
	return strings.Join(out, ",")
}

func (p pickList) areas() string {
	var out []string
	for _, a := range p.Areas {
		out = append(out, a.District+"/"+a.Area+"="+pv(a.Variants))
	}
	return strings.Join(out, " ")
}

func (p pickList) fids() []int64 {
	var out []int64
	for _, f := range p.Fulfillments {
		out = append(out, f.ID)
	}
	return out
}

func (f *checkoutFx) pickList(st httpx.Staff, query string) pickList {
	f.t.Helper()
	rec := f.adm(st, "GET", "/api/admin/packing/pick-list"+query, nil)
	var p pickList
	if rec.Code != http.StatusOK || json.NewDecoder(rec.Body).Decode(&p) != nil {
		f.t.Fatalf("pick list %q: %d %s", query, rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		f.t.Errorf("pick list Cache-Control %q", cc)
	}
	return p
}

// seedPick: pick list er seed. Variant: rice (fixed), fish (ojon-variable), vendor X/Y (marketplace).
// o1 Dhaka 10 January 23:30 e confirmed (Dhanmondi, rice 2 + removed rice); o2 11 January 00:00 (Gulshan, rice 1 +
// fish 1500 g); o5 9 January (ager diner baki, fish 700 g); o6 marketplace 11 January (X rx 1, Y ry 2); o3 packed,
// o4 pending, o7 confirmed_at NULL, o8 12 January: egulo 11 January er list e nai.
type pickSeed struct {
	rice, fish, rx, ry, vx, vy int64
	f1, f2, f5, fx, fy         int64
	o1                         int64
}

func (f *checkoutFx) seedPick() pickSeed {
	f.t.Helper()
	var p pickSeed
	p.rice = f.variant("rice", 10000, 50)
	p.fish = f.weighted("fish", 80000, 1000, 50)
	p.vx, p.rx = f.vendorVariant("vx", 50)
	p.vy, p.ry = f.vendorVariant("vy", 50)
	open := func(v int64, items ...[2]int64) seedF { return seedF{vendor: v, status: "open", items: items} }
	at := func(oid int64, ts string) {
		f.exec(`UPDATE orders SET confirmed_at = $2, placed_at = $2::timestamptz - interval '1 hour' WHERE id = $1`, oid, ts)
	}
	weigh := func(oid, variant int64, g int) {
		f.exec(`UPDATE order_items SET variable_weight = true, ordered_weight_g = $3 WHERE order_id = $1 AND variant_id = $2`, oid, variant, g)
	}
	var fs []int64
	p.o1, fs = f.seedOrder("cod", "unpaid", "confirmed", open(f.vnd, [2]int64{p.rice, 2}))
	p.f1 = fs[0]
	at(p.o1, "2026-01-10T17:30:00Z") // Dhaka 10 January 23:30
	f.exec(`INSERT INTO order_items (shop_id, order_id, fulfillment_id, vendor_id, variant_id, product_name, sku, unit_price,
		quantity, line_total, status) VALUES ($1, $2, $3, $4, $5, '{"bn":"প"}', 's', 10000, 4, 0, 'removed')`, f.shop, p.o1, p.f1, f.vnd, p.rice)
	o2, fs := f.seedOrder("cod", "unpaid", "confirmed", open(f.vnd, [2]int64{p.rice, 1}, [2]int64{p.fish, 1}))
	p.f2 = fs[0]
	at(o2, "2026-01-10T18:00:00Z") // Dhaka 11 January 00:00
	weigh(o2, p.fish, 1500)
	f.exec(`UPDATE orders SET ship_area = 'Gulshan' WHERE id = $1`, o2)
	o5, fs := f.seedOrder("sslcommerz", "paid", "confirmed", open(f.vnd, [2]int64{p.fish, 1}))
	p.f5 = fs[0]
	at(o5, "2026-01-09T04:00:00Z")
	weigh(o5, p.fish, 700)
	o6, fs := f.seedOrder("cod", "unpaid", "confirmed", open(p.vx, [2]int64{p.rx, 1}), open(p.vy, [2]int64{p.ry, 2}))
	p.fx, p.fy = fs[0], fs[1]
	at(o6, "2026-01-11T05:00:00Z")
	o3, _ := f.seedOrder("cod", "unpaid", "confirmed", seedF{vendor: f.vnd, status: "packed", items: [][2]int64{{p.rice, 5}}})
	at(o3, "2026-01-10T19:00:00Z")
	f.seedOrder("cod", "unpaid", "pending", open(f.vnd, [2]int64{p.rice, 6}))
	f.seedOrder("cod", "unpaid", "confirmed", open(f.vnd, [2]int64{p.rice, 7})) // confirmed_at NULL
	o8, _ := f.seedOrder("cod", "unpaid", "confirmed", open(f.vnd, [2]int64{p.rice, 8}))
	at(o8, "2026-01-11T18:00:00Z") // Dhaka 12 January 00:00
	return p
}

// TestPickListIntegration (8.11 item 1, 11.7 item 4, v1.44): GET /packing/pick-list?date= shop er timezone e (Asia/Dhaka,
// UTC theke alada): oi diner shesh porjonto confirmed shob open fulfillment (ager diner baki o), variant onujayi mot
// qty ar ojon-variable e mot ordered gram, area onujayi bhag, fulfillment list (confirmed_at kram); packed, pending,
// confirmed_at NULL, removed item ar porer din bad; date chhara shop er aj; vendor role e nijer vendor; bhul date 400.
func TestPickListIntegration(t *testing.T) {
	f := newCheckout(t)
	p := f.seedPick()
	st := f.staffAs("staff", 0)

	l := f.pickList(st, "?date=2026-01-11")
	wantV := fmt.Sprintf("%d:3,%d:2:2200,%d:1,%d:2", p.rice, p.fish, p.rx, p.ry)
	if l.Date != "2026-01-11" || pv(l.Variants) != wantV {
		t.Errorf("11 January variant: %s %s, chai %s", l.Date, pv(l.Variants), wantV)
	}
	wantA := fmt.Sprintf("Dhaka/Dhanmondi=%d:2,%d:1:700,%d:1,%d:2 Dhaka/Gulshan=%d:1,%d:1:1500", p.rice, p.fish, p.rx, p.ry, p.rice, p.fish)
	if l.areas() != wantA {
		t.Errorf("area: %s, chai %s", l.areas(), wantA)
	}
	if got, want := l.fids(), []int64{p.f5, p.f1, p.f2, p.fx, p.fy}; !slices.Equal(got, want) {
		t.Errorf("fulfillment: %v, chai %v", got, want)
	}
	if fl := l.Fulfillments[1]; fl.OrderID != p.o1 || fl.OrderNumber != f.number(p.o1) || fl.Seq != 1 || fl.VendorID != f.vnd ||
		fl.RecipientName != "R" || fl.District != "Dhaka" || fl.Area != "Dhanmondi" || string(fl.ConfirmedAt) != `"2026-01-10T17:30:00Z"` {
		t.Errorf("fulfillment row: %+v", fl)
	}
	if v := l.Variants[0]; string(v.ProductName) != `{"bn": "প"}` || v.SKU != "s" || v.VariableWeight || v.OrderedWeightG != nil {
		t.Errorf("rice row: %+v %s", v, v.ProductName)
	}
	if v := l.Variants[1]; !v.VariableWeight || v.OrderedWeightG == nil {
		t.Errorf("fish row: %+v", v)
	}

	// 10 January: o1 (23:30 Dhaka) ar o5; o2 (11 January 00:00 Dhaka, UTC te 10 January) na
	l = f.pickList(st, "?date=2026-01-10")
	if want := fmt.Sprintf("%d:2,%d:1:700", p.rice, p.fish); pv(l.Variants) != want || !slices.Equal(l.fids(), []int64{p.f5, p.f1}) {
		t.Errorf("10 January: %s %v, chai %s", pv(l.Variants), l.fids(), want)
	}
	if l = f.pickList(st, "?date=2026-01-08"); len(l.Variants) != 0 || len(l.Areas) != 0 || len(l.Fulfillments) != 0 ||
		l.Variants == nil || l.Areas == nil || l.Fulfillments == nil {
		t.Errorf("8 January khali list chai ([]): %+v", l)
	}

	// date chhara: shop er aj (Dhaka), 12 January er o8 o
	l = f.pickList(st, "")
	if today := f.str(`SELECT (now() AT TIME ZONE 'Asia/Dhaka')::date::text`); l.Date != today {
		t.Errorf("default date %s, chai %s", l.Date, today)
	}
	if len(l.Fulfillments) != 6 {
		t.Errorf("aj porjonto: %v", l.fids())
	}

	for _, role := range []string{"vendor_owner", "vendor_staff"} {
		l = f.pickList(f.staffAs(role, p.vx), "?date=2026-01-11")
		if pv(l.Variants) != fmt.Sprintf("%d:1", p.rx) || !slices.Equal(l.fids(), []int64{p.fx}) || len(l.Areas) != 1 {
			t.Errorf("%s: %s %v", role, pv(l.Variants), l.fids())
		}
	}
	for _, d := range []string{"2026-13-01", "11-01-2026", "2026-01-11T00:00:00Z", "x"} {
		e := wantErr(t, "date "+d, f.adm(st, "GET", "/api/admin/packing/pick-list?date="+d, nil), http.StatusBadRequest, "validation_failed")
		if e.Fields["date"] != "invalid" {
			t.Errorf("date %s: %v", d, e.Fields)
		}
	}
}
