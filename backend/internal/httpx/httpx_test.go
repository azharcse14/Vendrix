package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChainOrder(t *testing.T) {
	var got []string
	mw := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = append(got, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { got = append(got, "h") }), mw("A"), mw("B"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Join(got, ",") != "A,B,h" {
		t.Errorf("kram: %v", got)
	}
}

func TestRequestID(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = RequestIDFrom(r.Context()) }))
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-Id", "client-pathano")
	h.ServeHTTP(rec, r)
	if seen == "" || seen == "client-pathano" || rec.Header().Get("X-Request-Id") != seen {
		t.Errorf("id %q, header %q", seen, rec.Header().Get("X-Request-Id"))
	}
}

const secret = "0123456789abcdef0123456789abcdef"

// TestClientIP (3.2 Niyom 5, 14.2 test 18 er unit ongsho).
func TestClientIP(t *testing.T) {
	cases := []struct {
		name, remote string
		hdr          map[string]string
		want         string
	}{
		{"Jaspr: loopback + thik secret", "127.0.0.1:5000", map[string]string{"X-Internal-Auth": secret, "X-Client-IP": "103.4.5.6", "X-Real-IP": "9.9.9.9"}, "103.4.5.6"},
		{"bhul secret: X-Client-IP mane na", "127.0.0.1:5000", map[string]string{"X-Internal-Auth": "nope", "X-Client-IP": "103.4.5.6"}, "127.0.0.1"},
		{"Caddy: loopback + X-Real-IP", "127.0.0.1:5000", map[string]string{"X-Real-IP": "103.4.5.7"}, "103.4.5.7"},
		{"Caddy: IPv6 loopback", "[::1]:5000", map[string]string{"X-Real-IP": "2001:db8::1"}, "2001:db8::1"},
		{"bairer connection header boshale-o mane na", "198.51.100.9:443", map[string]string{"X-Internal-Auth": secret, "X-Client-IP": "1.1.1.1", "X-Real-IP": "2.2.2.2"}, "198.51.100.9"},
		{"header e IP na", "127.0.0.1:5000", map[string]string{"X-Real-IP": "evil, 1.2.3.4"}, "127.0.0.1"},
		{"IPv4-mapped IPv6 theke IPv4", "[::ffff:192.0.2.1]:80", nil, "192.0.2.1"},
		{"thik secret, bhul X-Client-IP: X-Real-IP", "127.0.0.1:1", map[string]string{"X-Internal-Auth": secret, "X-Client-IP": "x", "X-Real-IP": "203.0.113.5"}, "203.0.113.5"},
	}
	for _, c := range cases {
		var got string
		h := ClientIP(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = IP(r.Context()) }))
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = c.remote
		for k, v := range c.hdr {
			r.Header.Set(k, v)
		}
		h.ServeHTTP(httptest.NewRecorder(), r)
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	// secret khali (config bhul) hole X-Client-IP kokhono mane na
	var got string
	h := ClientIP("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = IP(r.Context()) }))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("X-Client-IP", "1.1.1.1")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got != "127.0.0.1" {
		t.Errorf("khali secret: %q", got)
	}
}

// TestLogRouteNotPath (12.2, 13.8): log e route pattern, path er token na.
func TestLogRouteNotPath(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/orders/{token}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := Chain(mux, RequestID, Log(log, mux))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/orders/SECRETTOKEN26CHARSXXXXXXXX?x=1", nil))
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if line["route"] != "GET /api/orders/{token}" || line["status"] != float64(http.StatusTeapot) || line["request_id"] == "" {
		t.Errorf("log: %v", line)
	}
	if strings.Contains(buf.String(), "SECRETTOKEN") {
		t.Errorf("log e token: %s", buf.String())
	}
	buf.Reset()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nai", nil))
	if !strings.Contains(buf.String(), `"route":"-"`) || !strings.Contains(buf.String(), `"status":404`) {
		t.Errorf("404 er log: %s", buf.String())
	}
}

// TestRecover (12.2 Server 4): panic e 500 internal, client e details na, log e ache.
func TestRecover(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), RequestID, Recover(log))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	var body map[string]ErrorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusInternalServerError || body["error"].Code != "internal" || strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("recover: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Error("log e panic nai")
	}
	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Errorf("ErrAbortHandler abar panic hoyni: %v", v)
		}
	}()
	Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestCORS(t *testing.T) {
	h := CORS([]string{"https://admin.x.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	r := httptest.NewRequest(http.MethodOptions, "/api/admin/products", nil)
	r.Header.Set("Origin", "https://admin.x.com")
	r.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "https://admin.x.com" ||
		rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("preflight: %d %v", rec.Code, rec.Header())
	}
	r = httptest.NewRequest(http.MethodGet, "/api/products", nil)
	r.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" || rec.Header().Get("Vary") != "Origin" {
		t.Errorf("onno origin: %v", rec.Header())
	}
}

func TestDecodeJSON(t *testing.T) {
	type in struct {
		Name string `json:"name"`
	}
	cases := []struct {
		body string
		ok   bool
	}{
		{`{"name":"a"}`, true},
		{`{"name":"a"}  `, true},
		{`{"name":"a","x":1}`, false},
		{`{"name":"a"}{"name":"b"}`, false},
		{`{`, false},
		{`{"name":"` + strings.Repeat("a", MaxBody) + `"}`, false},
	}
	for _, c := range cases {
		var v in
		err := DecodeJSON(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(c.body)), &v)
		if (err == nil) != c.ok {
			t.Errorf("%.40q: err = %v", c.body, err)
		}
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest("GET", "/", nil), http.StatusConflict, ErrorBody{Code: "out_of_stock", Message: "x", Details: map[string]any{"variant_id": 12}})
	want := `{"error":{"code":"out_of_stock","message":"x","details":{"variant_id":12}}}`
	if rec.Code != http.StatusConflict || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
		strings.TrimSpace(rec.Body.String()) != want {
		t.Errorf("%d %s", rec.Code, rec.Body.String())
	}
}
