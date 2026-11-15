package shipping_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/shipping"
	"vendrix/backend/internal/shop"
)

// zoneFixture: duita shop (a.com, b.com) ar zone; a te Dhaka er area + district-wide, Chattogram e shudhu area,
// Sylhet er district-wide bondho; b te onno zone (Rule 3 dhorar jonno).
func zoneFixture(t *testing.T) (*sql.DB, map[string]int64) {
	t.Helper()
	d := dbtest.New(t)
	ids := map[string]int64{}
	for _, s := range []string{"a", "b"} {
		id, _, err := shop.Create(context.Background(), d, shop.Params{Slug: s, Prefix: "S" + string(s[0]-32),
			Domain: s + ".com", NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		ids[s] = id
	}
	for _, z := range []struct {
		shop, district string
		area           any // nil = puro district
		bn, en         string
		charge, min    int64
		free           any
		sameCity       bool
		active         bool
	}{
		{"a", "Dhaka", "Dhanmondi", "ধানমন্ডি", "Dhanmondi", 6000, 30000, int64(200000), true, true},
		{"a", "Dhaka", "Mirpur", "মিরপুর", "Mirpur", 7000, 0, nil, true, false},
		{"a", "Dhaka", "Banani", "বনানী", "Banani", 6500, 0, nil, true, true},
		{"a", "Dhaka", nil, "ঢাকা (অন্য এলাকা)", "Dhaka (other)", 10000, 0, nil, true, true},
		{"a", "Chattogram", "Agrabad", "আগ্রাবাদ", "", 12000, 0, nil, false, true},
		{"a", "Sylhet", nil, "সিলেট", "Sylhet", 13000, 0, nil, false, false},
		{"b", "Khulna", nil, "খুলনা", "Khulna", 8000, 0, nil, false, true},
		{"b", "Dhaka", "Gulshan", "গুলশান", "Gulshan", 5000, 0, nil, true, true},
	} {
		name := map[string]string{"bn": z.bn}
		if z.en != "" {
			name["en"] = z.en
		}
		nj, _ := json.Marshal(name)
		var id int64
		if err := d.QueryRow(`INSERT INTO delivery_zones (shop_id, name, district, area, charge, min_order,
			free_delivery_over, same_city, is_active) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
			ids[z.shop], nj, z.district, z.area, z.charge, z.min, z.free, z.sameCity, z.active).Scan(&id); err != nil {
			t.Fatal(err)
		}
		a, _ := z.area.(string)
		ids[z.shop+"/"+z.district+"/"+a] = id
	}
	return d, ids
}

// TestZoneLookupIntegration (8.6 step 5, 8.19 item 2): age (district, area), na pele (district, area NULL);
// bondho zone bad; onno shop er zone na; "Onno area" er free text district-wide zone e.
func TestZoneLookupIntegration(t *testing.T) {
	d, ids := zoneFixture(t)
	ctx := context.Background()

	z, err := shipping.Lookup(ctx, d, ids["a"], "Dhaka", "Dhanmondi")
	if err != nil {
		t.Fatal(err)
	}
	want := shipping.Zone{ID: ids["a/Dhaka/Dhanmondi"], Charge: 6000, MinOrder: 30000, FreeDeliveryOver: 200000, SameCity: true}
	if z != want {
		t.Errorf("Dhanmondi: %+v, chai %+v", z, want)
	}

	for _, c := range []struct {
		name           string
		shop           string
		district, area string
		zone           string // khali = area_not_served
	}{
		{"area mile", "a", "Dhaka", "Banani", "a/Dhaka/Banani"},
		{"Onno area free text → district-wide", "a", "Dhaka", "Uttara sector 7", "a/Dhaka/"},
		{"area khali → district-wide", "a", "Dhaka", "", "a/Dhaka/"},
		{"area bondho → district-wide", "a", "Dhaka", "Mirpur", "a/Dhaka/"},
		{"string hubohu (case alada = free text)", "a", "Dhaka", "banani", "a/Dhaka/"},
		{"district-wide nai", "a", "Chattogram", "Halishahar", ""},
		{"shudhu area zone", "a", "Chattogram", "Agrabad", "a/Chattogram/Agrabad"},
		{"district-wide bondho", "a", "Sylhet", "Zindabazar", ""},
		{"onno shop er district", "a", "Khulna", "", ""},
		{"onno shop er area na, nijer district-wide", "a", "Dhaka", "Gulshan", "a/Dhaka/"},
		{"b te a er area na", "b", "Dhaka", "Dhanmondi", ""},
		{"b er nijer", "b", "Khulna", "Sonadanga", "b/Khulna/"},
		{"ochena district", "a", "Barishal", "", ""},
	} {
		z, err := shipping.Lookup(ctx, d, ids[c.shop], c.district, c.area)
		if c.zone == "" {
			if !errors.Is(err, shipping.ErrAreaNotServed) {
				t.Errorf("%s: %+v %v, chai ErrAreaNotServed", c.name, z, err)
			}
			continue
		}
		if err != nil || z.ID != ids[c.zone] {
			t.Errorf("%s: %+v %v, chai zone %d", c.name, z, err, ids[c.zone])
		}
	}
	// free_delivery_over NULL = 0 (pricing.Zone er mane)
	if z, _ := shipping.Lookup(ctx, d, ids["a"], "Dhaka", "x"); z.FreeDeliveryOver != 0 || z.Charge != 10000 || !z.SameCity {
		t.Errorf("district-wide: %+v", z)
	}
	// transaction er bhitore o chole (checkout er tx)
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if z, err := shipping.Lookup(ctx, tx, ids["a"], "Dhaka", "Banani"); err != nil || z.Charge != 6500 {
		t.Errorf("tx: %+v %v", z, err)
	}
}

// TestDeliveryZonesIntegration (7.2 v1.32): GET /api/delivery-zones →
// [{district, areas: [{area, name}], district_wide}], shudhu active, oi shop er, naam Accept-Language e.
func TestDeliveryZonesIntegration(t *testing.T) {
	d, _ := zoneFixture(t)
	log := slog.New(slog.DiscardHandler)
	public := httpx.ShopFromHost(log, shop.NewResolver(d).Resolve)
	mux := http.NewServeMux()
	shipping.NewHandlers(d, log).Routes(mux, public)
	srv := httpx.JSONErrors(mux)

	get := func(host, lang string) (*httptest.ResponseRecorder, string) {
		r := httptest.NewRequest(http.MethodGet, "/api/delivery-zones", nil)
		r.Header.Set("X-Shop-Host", host)
		if lang != "" {
			r.Header.Set("Accept-Language", lang)
		}
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		var v any
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatalf("%s: %v", w.Body, err)
		}
		b, _ := json.Marshal(v) // key sajano, tulona shohoj
		return w, string(b)
	}

	// Sylhet nai (shudhu bondho zone); Mirpur nai (bondho); district ar area naam er kram e; en na thakle bn
	w, got := get("a.com", "en")
	want := `[{"areas":[{"area":"Agrabad","name":"আগ্রাবাদ"}],"district":"Chattogram","district_wide":false},` +
		`{"areas":[{"area":"Banani","name":"Banani"},{"area":"Dhanmondi","name":"Dhanmondi"}],"district":"Dhaka","district_wide":true}]`
	if w.Code != http.StatusOK || got != want {
		t.Errorf("a en: %d %s\nchai %s", w.Code, got, want)
	}
	if _, got := get("a.com", ""); got != `[{"areas":[{"area":"Agrabad","name":"আগ্রাবাদ"}],"district":"Chattogram","district_wide":false},`+
		`{"areas":[{"area":"Banani","name":"বনানী"},{"area":"Dhanmondi","name":"ধানমন্ডি"}],"district":"Dhaka","district_wide":true}]` {
		t.Errorf("a bn: %s", got)
	}
	// shudhu district-wide zone: areas khali array (null na)
	if _, got := get("b.com", "en"); got != `[{"areas":[{"area":"Gulshan","name":"Gulshan"}],"district":"Dhaka","district_wide":false},`+
		`{"areas":[],"district":"Khulna","district_wide":true}]` {
		t.Errorf("b: %s", got)
	}
	if w, _ := get("nai.com", ""); w.Code != http.StatusNotFound {
		t.Errorf("ochena shop: %d", w.Code)
	}
}
