package customer_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/customer"
	"vendrix/backend/internal/dbtest"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/order"
	"vendrix/backend/internal/shop"
)

const (
	rahim = "+8801711000001"
	karim = "+8801811000002"
)

type fx struct {
	t      *testing.T
	d      *sql.DB
	a, b   int64
	rahim  int64 // shop a er user
	rahimB int64 // shop b er ekoi phone er user
	srv    http.Handler
}

func (f *fx) id(q string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.d.QueryRow(q, args...).Scan(&id); err != nil {
		f.t.Fatalf("%s: %v", q, err)
	}
	return id
}

// order: user (0 = guest) er order; status cancelled hole display o.
func (f *fx) order(shopID, user int64, phone, status string, total int64) int64 {
	return f.id(`INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash, user_id,
		customer_name, customer_phone, recipient_name, recipient_phone, ship_district, ship_area, ship_address,
		subtotal, total, payment_method, status, display_status)
		VALUES ($1, 'N-' || gen_random_uuid(), gen_random_uuid()::text, gen_random_uuid()::text, '\x00', NULLIF($2, 0),
		'C', $3, 'C', $3, 'Dhaka', 'Banani', 'Road 1', $5, $5, 'cod', $4, $4) RETURNING id`, shopID, user, phone, status, total)
}

func newFx(t *testing.T) *fx {
	f := &fx{t: t, d: dbtest.New(t)}
	for _, s := range []string{"a", "b"} {
		id, _, err := shop.Create(context.Background(), f.d, shop.Params{Slug: s, Prefix: "S" + strings.ToUpper(s), Domain: s + ".com",
			NameBn: "দোকান", OwnerEmail: "o@" + s + ".com", OwnerName: "O", Preset: "organic"})
		if err != nil {
			t.Fatal(err)
		}
		if s == "a" {
			f.a = id
		} else {
			f.b = id
		}
	}
	f.rahim = f.id(`INSERT INTO users (shop_id, phone, name, created_at) VALUES ($1, $2, 'Rahim Uddin', now() - interval '2 day') RETURNING id`, f.a, rahim)
	f.id(`INSERT INTO users (shop_id, phone, name, email, created_at) VALUES ($1, $2, 'Karim', 'k@x.com', now() - interval '1 day') RETURNING id`, f.a, karim)
	f.id(`INSERT INTO users (shop_id, deleted_at) VALUES ($1, now()) RETURNING id`, f.a)
	f.rahimB = f.id(`INSERT INTO users (shop_id, phone, name) VALUES ($1, $2, 'B Rahim') RETURNING id`, f.b, rahim)
	f.order(f.a, f.rahim, rahim, "pending", 10000)
	f.order(f.a, f.rahim, rahim, "cancelled", 5000)
	f.order(f.a, 0, rahim, "pending", 7000) // guest: customer er count e na
	f.order(f.b, f.rahimB, rahim, "pending", 99900)
	f.id(`INSERT INTO delivery_zones (shop_id, name, district, charge, same_city) VALUES ($1, '{"bn":"ঢাকা"}', 'Dhaka', 6000, true) RETURNING id`, f.a)

	mux := http.NewServeMux()
	customer.NewHandlers(f.d, slog.New(slog.DiscardHandler)).AdminRoutes(mux, func(h http.Handler) http.Handler { return h })
	f.srv = httpx.JSONErrors(mux)
	return f
}

func (f *fx) staff(shopID int64, role string) httpx.Staff {
	st := httpx.Staff{ShopID: shopID, Role: role}
	if role == "owner" {
		st.ID = f.id(`SELECT id FROM staff WHERE shop_id = $1 AND role = 'owner'`, shopID)
		return st
	}
	st.ID = f.id(`INSERT INTO staff (shop_id, email, name, password_hash, role) VALUES ($1, $2 || '@x.com', 'S', 'x', $2) RETURNING id`, shopID, role)
	return st
}

func (f *fx) call(st httpx.Staff, method, path string, body any) (int, map[string]any) {
	f.t.Helper()
	var b bytes.Buffer
	if s, ok := body.(string); ok {
		b.WriteString(s)
	} else if body != nil {
		json.NewEncoder(&b).Encode(body)
	}
	r := httptest.NewRequest(method, path, &b)
	r = r.WithContext(httpx.WithStaff(r.Context(), st))
	w := httptest.NewRecorder()
	f.srv.ServeHTTP(w, r)
	var out map[string]any
	if w.Code != http.StatusNoContent {
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			f.t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
		}
	} else if w.Body.Len() != 0 {
		f.t.Errorf("204 e body: %s", w.Body)
	}
	return w.Code, out
}

// checkout: COD order (cart khali); block thakle ErrPhoneBlocked, na hole porer dhap (ErrCartEmpty).
func (f *fx) checkout(phone string) error {
	sv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"success": true, "hostname": "a.com"})
	}))
	defer sv.Close()
	g := order.Guard{Turnstile: auth.Turnstile{URL: sv.URL, Secret: "s"}, OTPSecret: strings.Repeat("o", 32),
		ConfirmSecret: strings.Repeat("c", 32), OTPIP: httpx.NewLimit(10, 3600e9)}
	p := order.Person{Name: "C", Phone: phone}
	_, err := order.PlaceOrder(context.Background(), f.d, g, f.a, cart.Owner{KeyHash: []byte("nai")}, order.PlaceInput{
		Customer: p, Recipient: p, District: "Dhaka", Area: "Banani", Line: "Road 1", PaymentMethod: "cod",
		TurnstileToken: "tok", Host: "a.com", Timezone: "Asia/Dhaka", Key: "k-" + strings.Repeat("x", 20) + phone[4:], Hash: []byte{1}, Locale: "bn"})
	return err
}

func errOf(out map[string]any) (string, map[string]any) {
	e, _ := out["error"].(map[string]any)
	code, _ := e["code"].(string)
	fields, _ := e["fields"].(map[string]any)
	return code, fields
}

// TestAdminCustomerIntegration (7.7, 11.7 item 12, v1.45, 8.19 item 6): list (order count/total, cancelled
// total e na, guest na, deleted na, q phone/naam), detail (shesh order), block → checkout 403 phone_blocked,
// abar block 200 (write na), unblock DELETE body {phone} 204 (na thakleo), audit e phone na; owner/admin
// shudhu; onno shop er customer/block 404 ba dekha jay na.
func TestAdminCustomerIntegration(t *testing.T) {
	f := newFx(t)
	owner, admin := f.staff(f.a, "owner"), f.staff(f.a, "admin")
	ownerB := f.staff(f.b, "owner")

	code, list := f.call(owner, "GET", "/api/admin/customers", nil)
	if code != http.StatusOK || list["total"] != 2.0 {
		t.Fatalf("list: %d %v", code, list)
	}
	items := list["items"].([]any)
	k, r := items[0].(map[string]any), items[1].(map[string]any) // notun age
	if k["phone"] != karim || k["order_count"] != 0.0 || k["order_total"] != 0.0 || k["email"] != "k@x.com" || k["blocked"] != false {
		t.Errorf("karim: %v", k)
	}
	if r["phone"] != rahim || r["name"] != "Rahim Uddin" || r["order_count"] != 2.0 || r["order_total"] != 10000.0 || r["shop_id"] != nil {
		t.Errorf("rahim: %v", r)
	}
	for q, want := range map[string]float64{"01711-000001": 1, "rahim": 1, "UDDIN": 1, "kar": 1, "01911000009": 0, "%": 0, "x_": 0} {
		code, l := f.call(owner, "GET", "/api/admin/customers?q="+url.QueryEscape(q), nil)
		if code != http.StatusOK || l["total"] != want {
			t.Errorf("q %s: %d %v", q, code, l)
		}
	}
	if code, out := f.call(owner, "GET", "/api/admin/customers?q="+strings.Repeat("a", 65), nil); code != http.StatusBadRequest {
		t.Errorf("q lomba: %d %v", code, out)
	}

	code, det := f.call(admin, "GET", "/api/admin/customers/"+strconv.FormatInt(f.rahim, 10), nil)
	if code != http.StatusOK || det["phone"] != rahim || det["blocked"] != false || len(det["orders"].([]any)) != 2 {
		t.Fatalf("detail: %d %v", code, det)
	}
	o := det["orders"].([]any)[0].(map[string]any)
	if o["order_number"] == nil || o["display_status"] == nil || o["total"] == nil || o["placed_at"] == nil || o["customer_phone"] != nil {
		t.Errorf("detail order: %v", o)
	}
	if code, _ := f.call(owner, "GET", "/api/admin/customers/"+strconv.FormatInt(f.rahimB, 10), nil); code != http.StatusNotFound {
		t.Errorf("onno shop er customer: %d", code)
	}
	if code, _ := f.call(owner, "GET", "/api/admin/customers/x", nil); code != http.StatusNotFound {
		t.Errorf("bhul id: %d", code)
	}

	// block → checkout COD 403 (8.19 item 6); onno shop e prabhab na
	if err := f.checkout(rahim); !errors.Is(err, order.ErrCartEmpty) {
		t.Fatalf("block er age: %v", err)
	}
	code, bl := f.call(owner, "POST", "/api/admin/blocked-phones", map[string]any{"phone": "01711-000001", "reason": "  fake order  "})
	if code != http.StatusCreated || bl["phone"] != rahim || bl["reason"] != "fake order" || bl["staff_id"] != float64(owner.ID) {
		t.Fatalf("block: %d %v", code, bl)
	}
	if err := f.checkout(rahim); !errors.Is(err, order.ErrPhoneBlocked) {
		t.Errorf("block er por: %v", err)
	}
	if code, out := f.call(owner, "POST", "/api/admin/blocked-phones", map[string]any{"phone": rahim, "reason": "abar"}); code != http.StatusOK || out["reason"] != "fake order" {
		t.Errorf("abar block: %d %v", code, out)
	}
	if _, det := f.call(owner, "GET", "/api/admin/customers/"+strconv.FormatInt(f.rahim, 10), nil); det["blocked"] != true {
		t.Errorf("detail blocked: %v", det)
	}
	if _, l := f.call(ownerB, "GET", "/api/admin/blocked-phones", nil); l["total"] != 0.0 {
		t.Errorf("b er block list: %v", l)
	}
	if _, l := f.call(ownerB, "GET", "/api/admin/customers?q="+rahim[1:], nil); l["items"].([]any)[0].(map[string]any)["blocked"] != false {
		t.Errorf("b te blocked: %v", l)
	}
	// guest phone (user nai) o block
	if code, _ := f.call(owner, "POST", "/api/admin/blocked-phones", map[string]any{"phone": "01911000009"}); code != http.StatusCreated {
		t.Errorf("guest block: %d", code)
	}
	code, l := f.call(admin, "GET", "/api/admin/blocked-phones", nil)
	if code != http.StatusOK || l["total"] != 2.0 || l["items"].([]any)[0].(map[string]any)["phone"] != "+8801911000009" {
		t.Errorf("block list: %d %v", code, l)
	}
	for _, b := range []any{map[string]any{"phone": "123"}, map[string]any{}, map[string]any{"phone": karim, "reason": strings.Repeat("ক", 201)},
		map[string]any{"phone": karim, "x": 1}} {
		if code, out := f.call(owner, "POST", "/api/admin/blocked-phones", b); code != http.StatusBadRequest {
			t.Errorf("bhul block %v: %d %v", b, code, out)
		}
	}

	// unblock: b er owner a er block muchte pare na
	if code, _ := f.call(ownerB, "DELETE", "/api/admin/blocked-phones", map[string]any{"phone": rahim}); code != http.StatusNoContent {
		t.Errorf("b unblock: %d", code)
	}
	if err := f.checkout(rahim); !errors.Is(err, order.ErrPhoneBlocked) {
		t.Errorf("b er unblock e a er block gelo: %v", err)
	}
	if code, _ := f.call(owner, "DELETE", "/api/admin/blocked-phones", map[string]any{"phone": "01711000001"}); code != http.StatusNoContent {
		t.Errorf("unblock: %d", code)
	}
	if err := f.checkout(rahim); !errors.Is(err, order.ErrCartEmpty) {
		t.Errorf("unblock er por: %v", err)
	}
	if code, _ := f.call(owner, "DELETE", "/api/admin/blocked-phones", map[string]any{"phone": rahim}); code != http.StatusNoContent {
		t.Errorf("abar unblock: %d", code)
	}
	if code, _ := f.call(owner, "DELETE", "/api/admin/blocked-phones", map[string]any{"phone": "x"}); code != http.StatusBadRequest {
		t.Errorf("bhul phone unblock: %d", code)
	}

	// role: staff ar vendor 403 (11.2)
	st := f.staff(f.a, "staff")
	for _, rt := range [][2]string{{"GET", "/api/admin/customers"}, {"GET", "/api/admin/customers/" + strconv.FormatInt(f.rahim, 10)},
		{"GET", "/api/admin/blocked-phones"}, {"POST", "/api/admin/blocked-phones"}, {"DELETE", "/api/admin/blocked-phones"}} {
		if code, _ := f.call(st, rt[0], rt[1], map[string]any{"phone": karim}); code != http.StatusForbidden {
			t.Errorf("staff %v: %d", rt, code)
		}
	}
	var blocked int
	f.d.QueryRow(`SELECT count(*) FROM blocked_phones WHERE shop_id = $1 AND phone = $2`, f.a, karim).Scan(&blocked)
	if blocked != 0 {
		t.Error("staff block korte pereche")
	}

	// audit: rahim block + guest block + rahim unblock = 3; phone kothao nai; user thakle entity_id
	rows, err := f.d.Query(`SELECT action, coalesce(entity_id, 0), coalesce(before::text, ''), coalesce(after::text, '') FROM audit_logs
		WHERE shop_id = $1 AND entity_type = 'customer' ORDER BY id`, f.a)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var action, before, after string
		var eid int64
		rows.Scan(&action, &eid, &before, &after)
		if strings.Contains(before+after, "171100000") || strings.Contains(before+after, "1911000009") {
			t.Errorf("audit e phone: %s %s", before, after)
		}
		// security review M1: guest phone er unblock o chena jay (masked, 12.2)
		var m struct {
			Masked string `json:"phone_masked"`
		}
		json.Unmarshal([]byte(before+after), &m)
		got = append(got, action+":"+strconv.FormatBool(eid == f.rahim)+":"+m.Masked)
	}
	if s := strings.Join(got, ","); s != "customer.block:true:***001,customer.block:false:***009,customer.unblock:true:***001" {
		t.Errorf("audit: %s", s)
	}
}
