package inventory_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/inventory"
	"vendrix/backend/internal/shop"
)

type fx struct {
	t                  *testing.T
	d                  *sql.DB
	ctx                context.Context
	srv                http.Handler
	shop, shopB, house int64
}

// newFx: shop a (house vendor) ar b; inventory route staff context test e request e (RequireRole handler e).
func newFx(t *testing.T) *fx {
	t.Helper()
	f := &fx{t: t, d: dbtest.New(t), ctx: context.Background()}
	for _, s := range []string{"a", "b"} {
		id, _, err := shop.Create(f.ctx, f.d, shop.Params{Slug: s, Prefix: "S" + strings.ToUpper(s), Domain: s + ".com",
			NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		if s == "a" {
			f.shop = id
		} else {
			f.shopB = id
		}
	}
	f.house = f.id(`SELECT id FROM vendors WHERE shop_id = $1 AND is_house`, f.shop)
	mux := http.NewServeMux()
	inventory.NewHandlers(f.d, slog.New(slog.DiscardHandler)).Routes(mux, func(h http.Handler) http.Handler { return h })
	f.srv = httpx.JSONErrors(mux)
	return f
}

func (f *fx) id(query string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.d.QueryRowContext(f.ctx, query, args...).Scan(&id); err != nil {
		f.t.Fatalf("%s: %v", query, err)
	}
	return id
}

func (f *fx) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.d.ExecContext(f.ctx, query, args...); err != nil {
		f.t.Fatalf("%s: %v", query, err)
	}
}

func (f *fx) batchExpiry(on bool) {
	f.t.Helper()
	v := `[]`
	if on {
		v = `["batch_expiry"]`
	}
	f.exec(`UPDATE shop_settings SET published = jsonb_set(published, '{features}', $2::jsonb) WHERE shop_id = $1`, f.shop, v)
}

func (f *fx) vendor(slug string) int64 {
	return f.id(`INSERT INTO vendors (shop_id, slug, name) VALUES ($1, $2, '{"bn":"ভ"}') RETURNING id`, f.shop, slug)
}

// variant: shop er vendor er notun product + variant (stock sorasori, batch chhara purono stock).
func (f *fx) variant(shopID, vendor int64, sku, nameBn, nameEn string, stock int) int64 {
	f.t.Helper()
	p := f.id(`INSERT INTO products (shop_id, vendor_id, slug, name) VALUES ($1, $2, lower($3), jsonb_build_object('bn', $4::text, 'en', $5::text))
		RETURNING id`, shopID, vendor, sku, nameBn, nameEn)
	return f.id(`INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock, options)
		VALUES ($1, $2, $3, $4, 10000, $5, '{"size":{"bn":"১ কেজি","en":"1 kg"}}') RETURNING id`, shopID, p, vendor, sku, stock)
}

func (f *fx) staffAs(role string, vendor int64) httpx.Staff {
	f.t.Helper()
	st := httpx.Staff{ShopID: f.shop, Role: role}
	if role == "owner" {
		st.ID = f.id(`SELECT id FROM staff WHERE shop_id = $1 AND role = 'owner'`, f.shop)
		return st
	}
	if vendor != 0 {
		st.VendorID = &vendor
	}
	st.ID = f.id(`INSERT INTO staff (shop_id, vendor_id, email, name, password_hash, role)
		VALUES ($1, NULLIF($2, 0), $3 || '-' || $2 || '@a.com', 'S', 'x', $3) RETURNING id`, f.shop, vendor, role)
	return st
}

func (f *fx) req(st httpx.Staff, method, path string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	var b bytes.Buffer
	if s, ok := body.(string); ok {
		b.WriteString(s)
	} else if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			f.t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &b)
	r = r.WithContext(httpx.WithStaff(r.Context(), st))
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, r)
	return rec
}

func (f *fx) json(st httpx.Staff, method, path string, body any, status int) map[string]any {
	f.t.Helper()
	rec := f.req(st, method, path, body)
	var out map[string]any
	dec := json.NewDecoder(rec.Body)
	dec.UseNumber() // Rule 2: float na
	if rec.Code != status || dec.Decode(&out) != nil {
		f.t.Fatalf("%s %s %v: chai %d, pelam %d %s", method, path, body, status, rec.Code, rec.Body.String())
	}
	return out
}

func wantErr(t *testing.T, name string, rec *httptest.ResponseRecorder, status int, code string) map[string]any {
	t.Helper()
	var e struct{ Error map[string]any }
	dec := json.NewDecoder(rec.Body)
	dec.UseNumber()
	_ = dec.Decode(&e)
	if rec.Code != status || e.Error["code"] != code {
		t.Errorf("%s: chai %d %s, pelam %d %v", name, status, code, rec.Code, e.Error)
	}
	return e.Error
}

func s(n int64) string        { return strconv.FormatInt(n, 10) }
func num(n int64) json.Number { return json.Number(s(n)) }
func items(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, it := range m["items"].([]any) {
		out = append(out, it.(map[string]any))
	}
	return out
}

func (f *fx) stock(v int64) int64 {
	return f.id(`SELECT stock FROM product_variants WHERE id = $1`, v)
}

func (f *fx) remaining(b int64) int64 {
	return f.id(`SELECT quantity_remaining FROM inventory_batches WHERE id = $1`, b)
}

// batch: POST /batches diye (owner), id ferot.
func (f *fx) batch(v int64, qty int, expiry string) int64 {
	f.t.Helper()
	body := map[string]any{"variant_id": v, "quantity": qty, "cost_price": 5000}
	if expiry != "" {
		body["expiry_date"] = expiry
	}
	m := f.json(f.staffAs("owner", 0), "POST", "/api/admin/batches", body, http.StatusCreated)
	n, _ := m["id"].(json.Number).Int64()
	return n
}

// TestInventoryVariantsIntegration (11.7 item 1): GET /inventory/variants: q = naam (bn/en) ba SKU, low=1 =
// stock <= low_stock_at, batch_remaining, next_expiry; staff/vendor_staff 200, vendor role e shudhu nijer vendor;
// GET /variants/{id}/batches e staff/vendor_staff er cost_price nai, onno vendor 404.
func TestInventoryVariantsIntegration(t *testing.T) {
	f := newFx(t)
	rice := f.variant(f.shop, f.house, "RICE-1", "লাল চাল", "Red rice", 0)
	honey := f.variant(f.shop, f.house, "HONEY-1", "মধু", "Honey", 20)
	vx := f.vendor("vx")
	tea := f.variant(f.shop, vx, "TEA-1", "চা", "Tea", 1)
	f.variant(f.shopB, f.id(`SELECT id FROM vendors WHERE shop_id = $1 AND is_house`, f.shopB), "RICE-B", "চাল", "Rice B", 0)
	f.batch(rice, 2, "2099-01-01")
	f.batch(rice, 1, "2098-12-01")
	owner, staff := f.staffAs("owner", 0), f.staffAs("staff", 0)
	vs := f.staffAs("vendor_staff", vx)

	ids := func(st httpx.Staff, q string) []int64 {
		t.Helper()
		var out []int64
		for _, it := range items(f.json(st, "GET", "/api/admin/inventory/variants"+q, nil, http.StatusOK)) {
			n, _ := it["variant_id"].(json.Number).Int64()
			out = append(out, n)
		}
		return out
	}
	for _, c := range []struct {
		st   httpx.Staff
		q    string
		want []int64
	}{
		{owner, "", []int64{rice, honey, tea}},
		{staff, "?q=rice", []int64{rice}},
		{staff, "?q=honey-1", []int64{honey}},
		{staff, "?q=" + "%E0%A6%9A%E0%A6%BE%E0%A6%B2", []int64{rice}}, // "চাল"
		{owner, "?low=1", []int64{rice, tea}},
		{owner, "?low=1&q=tea", []int64{tea}},
		{owner, "?limit=1&page=2", []int64{honey}},
		{vs, "", []int64{tea}},
		{vs, "?q=rice", nil},
		{f.staffAs("vendor_owner", vx), "?low=1", []int64{tea}},
	} {
		if got := ids(c.st, c.q); !slices.Equal(got, c.want) {
			t.Errorf("%s %q: %v, chai %v", c.st.Role, c.q, got, c.want)
		}
	}
	for _, q := range []string{"?low=2", "?limit=101", "?q=" + strings.Repeat("x", 101)} {
		wantErr(t, q, f.req(owner, "GET", "/api/admin/inventory/variants"+q, nil), http.StatusBadRequest, "validation_failed")
	}

	m := f.json(staff, "GET", "/api/admin/inventory/variants?q=RICE-1", nil, http.StatusOK)
	row := items(m)[0]
	if row["stock"] != num(3) || row["low_stock_at"] != num(5) || row["batch_remaining"] != num(3) ||
		row["next_expiry"] != "2098-12-01" || row["sku"] != "RICE-1" || m["total"] != num(1) {
		t.Errorf("rice row: %v", row)
	}
	if name, _ := row["product_name"].(map[string]any); name["bn"] != "লাল চাল" || name["en"] != "Red rice" {
		t.Errorf("product_name: %v", row)
	}
	if _, ok := row["variant_label"].(map[string]any); !ok {
		t.Errorf("variant_label: %v", row)
	}
	if _, ok := row["price"]; ok {
		t.Errorf("dam thakbe na: %v", row)
	}

	// batch list: FIFO kram; cost_price owner e ache, staff/vendor_staff e nai; onno vendor/shop 404
	bl := items(f.json(owner, "GET", "/api/admin/variants/"+s(rice)+"/batches", nil, http.StatusOK))
	if len(bl) != 2 || bl[0]["expiry_date"] != "2098-12-01" || bl[0]["cost_price"] != num(5000) || bl[0]["quantity_remaining"] != num(1) {
		t.Errorf("owner batches: %v", bl)
	}
	for _, it := range items(f.json(staff, "GET", "/api/admin/variants/"+s(rice)+"/batches", nil, http.StatusOK)) {
		if _, ok := it["cost_price"]; ok {
			t.Errorf("staff e cost_price: %v", it)
		}
	}
	wantErr(t, "vendor_staff onno vendor er batch", f.req(vs, "GET", "/api/admin/variants/"+s(rice)+"/batches", nil), http.StatusNotFound, "not_found")
	riceB := f.id(`SELECT id FROM product_variants WHERE sku = 'RICE-B'`)
	wantErr(t, "onno shop er batch", f.req(owner, "GET", "/api/admin/variants/"+s(riceB)+"/batches", nil), http.StatusNotFound, "not_found")
	f.json(vs, "GET", "/api/admin/variants/"+s(tea)+"/batches", nil, http.StatusOK)
}

// TestBatchCreateIntegration (8.12, 11.7 item 2): POST /batches: received = remaining, variant.stock + qty, movement
// purchase batch_id shoho, audit, ek tx; expiry/harvest/quantity/cost_price check (bhul e kichu likha na); staff
// cost_price pathate pare kintu response e nai; vendor role nijer variant, onno vendor/shop 404; batch_expiry off
// eo batch row hoy.
func TestBatchCreateIntegration(t *testing.T) {
	f := newFx(t)
	rice := f.variant(f.shop, f.house, "RICE-1", "চাল", "Rice", 0)
	vx := f.vendor("vx")
	tea := f.variant(f.shop, vx, "TEA-1", "চা", "Tea", 0)
	other := f.variant(f.shopB, f.id(`SELECT id FROM vendors WHERE shop_id = $1 AND is_house`, f.shopB), "B-1", "খ", "B", 0)
	owner, staff := f.staffAs("owner", 0), f.staffAs("staff", 0)

	m := f.json(owner, "POST", "/api/admin/batches", map[string]any{"variant_id": rice, "quantity": 10, "cost_price": 5000,
		"batch_code": "B1", "harvest_date": "2026-11-01", "expiry_date": "2099-12-31"}, http.StatusCreated)
	bid, _ := m["id"].(json.Number).Int64()
	if m["quantity_received"] != num(10) || m["quantity_remaining"] != num(10) || m["cost_price"] != num(5000) ||
		m["batch_code"] != "B1" || m["expiry_date"] != "2099-12-31" || m["harvest_date"] != "2026-11-01" || m["variant_id"] != num(rice) {
		t.Errorf("batch response: %v", m)
	}
	if f.stock(rice) != 10 {
		t.Errorf("stock %d", f.stock(rice))
	}
	if n := f.id(`SELECT count(*) FROM inventory_movements WHERE variant_id = $1 AND batch_id = $2 AND change = 10
		AND reason = 'purchase' AND staff_id = $3`, rice, bid, owner.ID); n != 1 {
		t.Errorf("purchase movement %d", n)
	}
	if n := f.id(`SELECT count(*) FROM audit_logs WHERE action = 'inventory.batch' AND entity_id = $1`, bid); n != 1 {
		t.Errorf("audit %d", n)
	}

	// staff: cost_price pathay, DB te thake, response e na
	m = f.json(staff, "POST", "/api/admin/batches", map[string]any{"variant_id": rice, "quantity": 5, "cost_price": 4000}, http.StatusCreated)
	if _, ok := m["cost_price"]; ok {
		t.Errorf("staff response e cost_price: %v", m)
	}
	sid, _ := m["id"].(json.Number).Int64()
	if c := f.id(`SELECT cost_price FROM inventory_batches WHERE id = $1`, sid); c != 4000 || f.stock(rice) != 15 {
		t.Errorf("staff batch cost %d stock %d", c, f.stock(rice))
	}

	for _, c := range []struct {
		name  string
		body  any
		field string
	}{
		{"quantity 0", map[string]any{"variant_id": rice, "quantity": 0}, "quantity"},
		{"quantity -1", map[string]any{"variant_id": rice, "quantity": -1}, "quantity"},
		{"quantity boro", map[string]any{"variant_id": rice, "quantity": 1000001}, "quantity"},
		{"cost_price -1", map[string]any{"variant_id": rice, "quantity": 1, "cost_price": -1}, "cost_price"},
		{"expiry < harvest", map[string]any{"variant_id": rice, "quantity": 1, "harvest_date": "2099-01-02", "expiry_date": "2099-01-01"}, "expiry_date"},
		{"expiry past", map[string]any{"variant_id": rice, "quantity": 1, "expiry_date": "2020-01-01"}, "expiry_date"},
		{"bhul tarikh", map[string]any{"variant_id": rice, "quantity": 1, "expiry_date": "2027-13-01"}, "expiry_date"},
		{"harvest bhul", map[string]any{"variant_id": rice, "quantity": 1, "harvest_date": "01-01-2026"}, "harvest_date"},
		{"batch_code lomba", map[string]any{"variant_id": rice, "quantity": 1, "batch_code": strings.Repeat("x", 65)}, "batch_code"},
		{"variant nai", map[string]any{"quantity": 1}, "variant_id"},
		{"ochena field", map[string]any{"variant_id": rice, "quantity": 1, "stock": 5}, "body"},
	} {
		e := wantErr(t, c.name, f.req(owner, "POST", "/api/admin/batches", c.body), http.StatusBadRequest, "validation_failed")
		if fl, _ := e["fields"].(map[string]any); fl[c.field] == nil {
			t.Errorf("%s: fields %v", c.name, e)
		}
	}
	if f.stock(rice) != 15 || f.id(`SELECT count(*) FROM inventory_batches`) != 2 {
		t.Errorf("bhul input e likha hoyeche: stock %d", f.stock(rice))
	}

	// vendor scope (Rule 13), onno shop
	vs := f.staffAs("vendor_staff", vx)
	wantErr(t, "vendor onno vendor", f.req(vs, "POST", "/api/admin/batches", map[string]any{"variant_id": rice, "quantity": 1}), http.StatusNotFound, "not_found")
	wantErr(t, "onno shop", f.req(owner, "POST", "/api/admin/batches", map[string]any{"variant_id": other, "quantity": 1}), http.StatusNotFound, "not_found")
	m = f.json(vs, "POST", "/api/admin/batches", map[string]any{"variant_id": tea, "quantity": 3, "cost_price": 100}, http.StatusCreated)
	if _, ok := m["cost_price"]; ok || f.stock(tea) != 3 {
		t.Errorf("vendor_staff batch: %v stock %d", m, f.stock(tea))
	}
	if m = f.json(f.staffAs("vendor_owner", vx), "POST", "/api/admin/batches", map[string]any{"variant_id": tea, "quantity": 1, "cost_price": 100},
		http.StatusCreated); m["cost_price"] != num(100) {
		t.Errorf("vendor_owner cost_price dekhe: %v", m)
	}
	if f.stock(rice) != 15 {
		t.Errorf("rice stock %d", f.stock(rice))
	}
}

// TestInventoryAdjustIntegration (8.12, 11.7 item 2, 6.4): POST /inventory/adjust shudhu komano (change < 0), reason
// wastage/adjust, note 1–500; stock er beshi 409 out_of_stock {available}, kichu likha na; batch_expiry off e shudhu
// variant.stock (batch_id 400); on e batch_id ba FIFO (expiry age, NULLS LAST, expire batch bad, baki purono stock);
// vendor scope; audit; GET /inventory/movements e staff er cost_price nai.
func TestInventoryAdjustIntegration(t *testing.T) {
	f := newFx(t)
	owner, staff := f.staffAs("owner", 0), f.staffAs("staff", 0)
	adj := func(st httpx.Staff, body map[string]any) *httptest.ResponseRecorder {
		return f.req(st, "POST", "/api/admin/inventory/adjust", body)
	}

	// batch_expiry off
	rice := f.variant(f.shop, f.house, "RICE-1", "চাল", "Rice", 0)
	b0 := f.batch(rice, 10, "")
	m := f.json(staff, "POST", "/api/admin/inventory/adjust", map[string]any{"variant_id": rice, "change": -3, "reason": "wastage",
		"note": " pocha "}, http.StatusOK)
	if m["stock"] != num(7) || f.stock(rice) != 7 || f.remaining(b0) != 10 {
		t.Errorf("off adjust: %v stock %d batch %d", m, f.stock(rice), f.remaining(b0))
	}
	if n := f.id(`SELECT count(*) FROM inventory_movements WHERE variant_id = $1 AND batch_id IS NULL AND change = -3
		AND reason = 'wastage' AND note = 'pocha' AND staff_id = $2`, rice, staff.ID); n != 1 {
		t.Errorf("wastage movement %d", n)
	}
	if n := f.id(`SELECT count(*) FROM audit_logs WHERE action = 'inventory.adjust' AND entity_id = $1`, rice); n != 1 {
		t.Errorf("audit %d", n)
	}
	for _, c := range []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"change 0", map[string]any{"variant_id": rice, "change": 0, "reason": "wastage", "note": "x"}, "change"},
		{"change +", map[string]any{"variant_id": rice, "change": 2, "reason": "wastage", "note": "x"}, "change"},
		{"reason purchase", map[string]any{"variant_id": rice, "change": -1, "reason": "purchase", "note": "x"}, "reason"},
		{"note khali", map[string]any{"variant_id": rice, "change": -1, "reason": "adjust", "note": "  "}, "note"},
		{"note nai", map[string]any{"variant_id": rice, "change": -1, "reason": "adjust"}, "note"},
		{"note lomba", map[string]any{"variant_id": rice, "change": -1, "reason": "adjust", "note": strings.Repeat("ন", 501)}, "note"},
		{"off e batch_id", map[string]any{"variant_id": rice, "change": -1, "reason": "adjust", "note": "x", "batch_id": b0}, "batch_id"},
	} {
		e := wantErr(t, c.name, adj(owner, c.body), http.StatusBadRequest, "validation_failed")
		if fl, _ := e["fields"].(map[string]any); fl[c.field] == nil {
			t.Errorf("%s: fields %v", c.name, e)
		}
	}
	e := wantErr(t, "stock er beshi", adj(owner, map[string]any{"variant_id": rice, "change": -8, "reason": "adjust", "note": "x"}),
		http.StatusConflict, "out_of_stock")
	if d, _ := e["details"].(map[string]any); d["available"] != num(7) {
		t.Errorf("details: %v", e)
	}
	if f.stock(rice) != 7 || f.id(`SELECT count(*) FROM inventory_movements WHERE variant_id = $1`, rice) != 2 {
		t.Errorf("409 e likha hoyeche")
	}

	// batch_expiry on: FIFO (expiry age, NULLS LAST), batch_id, baki purono stock (batch chhara)
	f.batchExpiry(true)
	ghee := f.variant(f.shop, f.house, "GHEE-1", "ঘি", "Ghee", 0)
	a := f.batch(ghee, 5, "2099-02-01")
	b := f.batch(ghee, 5, "2099-01-01")
	c := f.batch(ghee, 5, "")
	f.exec(`UPDATE product_variants SET stock = stock + 2 WHERE id = $1`, ghee) // batch chhara purono stock
	f.json(owner, "POST", "/api/admin/inventory/adjust", map[string]any{"variant_id": ghee, "change": -7, "reason": "wastage", "note": "x"}, http.StatusOK)
	if got := []int64{f.remaining(a), f.remaining(b), f.remaining(c), f.stock(ghee)}; !slices.Equal(got, []int64{3, 0, 5, 10}) {
		t.Errorf("FIFO: a, b, c, stock = %v", got)
	}
	if n := f.id(`SELECT count(*) FROM inventory_movements WHERE variant_id = $1 AND reason = 'wastage'
		AND ((batch_id = $2 AND change = -5) OR (batch_id = $3 AND change = -2))`, ghee, b, a); n != 2 {
		t.Errorf("FIFO movement %d", n)
	}
	f.json(owner, "POST", "/api/admin/inventory/adjust", map[string]any{"variant_id": ghee, "change": -2, "reason": "adjust", "note": "x", "batch_id": c}, http.StatusOK)
	if f.remaining(c) != 3 || f.remaining(a) != 3 || f.stock(ghee) != 8 {
		t.Errorf("batch_id: c %d a %d stock %d", f.remaining(c), f.remaining(a), f.stock(ghee))
	}
	e = wantErr(t, "batch e kom", adj(owner, map[string]any{"variant_id": ghee, "change": -4, "reason": "adjust", "note": "x", "batch_id": c}),
		http.StatusConflict, "out_of_stock")
	if d, _ := e["details"].(map[string]any); d["available"] != num(3) {
		t.Errorf("batch details: %v", e)
	}
	wantErr(t, "onno variant er batch", adj(owner, map[string]any{"variant_id": ghee, "change": -1, "reason": "adjust", "note": "x", "batch_id": b0}),
		http.StatusNotFound, "not_found")
	f.json(owner, "POST", "/api/admin/inventory/adjust", map[string]any{"variant_id": ghee, "change": -8, "reason": "wastage", "note": "x"}, http.StatusOK)
	if got := []int64{f.remaining(a), f.remaining(c), f.stock(ghee)}; !slices.Equal(got, []int64{0, 0, 0}) {
		t.Errorf("FIFO + purono stock: a, c, stock = %v", got)
	}
	if n := f.id(`SELECT count(*) FROM inventory_movements WHERE variant_id = $1 AND batch_id IS NULL AND change = -2`, ghee); n != 1 {
		t.Errorf("batch chhara ongsho %d", n)
	}

	// expire batch (job er age) FIFO te na
	oil := f.variant(f.shop, f.house, "OIL-1", "তেল", "Oil", 6)
	old := f.id(`INSERT INTO inventory_batches (shop_id, variant_id, quantity_received, quantity_remaining, expiry_date)
		VALUES ($1, $2, 3, 3, (now() AT TIME ZONE 'Asia/Dhaka')::date - 1) RETURNING id`, f.shop, oil)
	fresh := f.id(`INSERT INTO inventory_batches (shop_id, variant_id, quantity_received, quantity_remaining) VALUES ($1, $2, 3, 3) RETURNING id`, f.shop, oil)
	f.json(owner, "POST", "/api/admin/inventory/adjust", map[string]any{"variant_id": oil, "change": -2, "reason": "wastage", "note": "x"}, http.StatusOK)
	if f.remaining(old) != 3 || f.remaining(fresh) != 1 {
		t.Errorf("expire batch FIFO te: old %d fresh %d", f.remaining(old), f.remaining(fresh))
	}

	// vendor scope
	vx := f.vendor("vx")
	tea := f.variant(f.shop, vx, "TEA-1", "চা", "Tea", 4)
	vs := f.staffAs("vendor_staff", vx)
	wantErr(t, "vendor onno vendor", adj(vs, map[string]any{"variant_id": oil, "change": -1, "reason": "adjust", "note": "x"}), http.StatusNotFound, "not_found")
	f.json(vs, "POST", "/api/admin/inventory/adjust", map[string]any{"variant_id": tea, "change": -1, "reason": "wastage", "note": "x"}, http.StatusOK)
	if f.stock(tea) != 3 || f.stock(oil) != 4 {
		t.Errorf("vendor adjust: tea %d oil %d", f.stock(tea), f.stock(oil))
	}

	// movements: cost_price (batch er) owner e, staff/vendor_staff e na; vendor e onno vendor er row na
	mv := items(f.json(owner, "GET", "/api/admin/inventory/movements?variant_id="+s(ghee), nil, http.StatusOK))
	if len(mv) != 9 || mv[0]["reason"] != "wastage" { // 3 purchase + 2 FIFO + 1 batch_id + 3 FIFO (a, c, purono)
		t.Errorf("owner movements: %d %v", len(mv), mv)
	}
	hasCost := false
	for _, it := range mv {
		if it["reason"] == "purchase" && it["cost_price"] == num(5000) {
			hasCost = true
		}
	}
	if !hasCost {
		t.Errorf("owner e cost_price nai: %v", mv)
	}
	for _, st := range []httpx.Staff{staff, vs} {
		for _, it := range items(f.json(st, "GET", "/api/admin/inventory/movements", nil, http.StatusOK)) {
			if _, ok := it["cost_price"]; ok {
				t.Errorf("%s e cost_price: %v", st.Role, it)
			}
			if st.Role == "vendor_staff" && it["variant_id"] != num(tea) {
				t.Errorf("vendor e onno vendor er movement: %v", it)
			}
		}
	}
	if got := items(f.json(vs, "GET", "/api/admin/inventory/movements?variant_id="+s(ghee), nil, http.StatusOK)); len(got) != 0 {
		t.Errorf("vendor onno variant: %v", got)
	}
	wantErr(t, "variant_id bhul", f.req(owner, "GET", "/api/admin/inventory/movements?variant_id=x", nil), http.StatusBadRequest, "validation_failed")
}
