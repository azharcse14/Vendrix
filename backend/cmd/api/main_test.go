package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vendrix/backend/internal/config"
)

type fakeDB struct{ err error }

func (f fakeDB) PingContext(context.Context) error { return f.err }

func quiet() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func testConfig() config.Config {
	return config.Config{Env: "local", Addr: "127.0.0.1:0", AdminOrigins: []string{"http://localhost:3001"},
		JasprInternalSecret: "0123456789abcdef0123456789abcdef"}
}

// TestRouterBuild (M0, 12.2 Server 7): puro router panic chhara bane; /healthz database er obostha dekhay.
func TestRouterBuild(t *testing.T) {
	h, err := newHandler(testConfig(), quiet(), fakeDB{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) || rec.Header().Get("X-Request-Id") == "" {
		t.Errorf("healthz: %d %s", rec.Code, rec.Body.String())
	}
	down, err := newHandler(testConfig(), quiet(), fakeDB{err: errors.New("down")})
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	down.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "db_unavailable") {
		t.Errorf("database bondho: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := newHandler(config.Config{AdminOrigins: []string{"not a url"}}, quiet(), fakeDB{}); err == nil {
		t.Error("bhul ADMIN_ORIGINS e error ashe ni")
	}
}

// TestCrossOrigin (12.2 Server 3): onno site er browser theke POST 403; admin origin CrossOriginProtection paar hoy.
func TestCrossOrigin(t *testing.T) {
	h, err := newHandler(testConfig(), quiet(), fakeDB{})
	if err != nil {
		t.Fatal(err)
	}
	post := func(origin string) int {
		r := httptest.NewRequest(http.MethodPost, "/healthz", nil)
		r.Header.Set("Origin", origin)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	if c := post("https://evil.example"); c != http.StatusForbidden {
		t.Errorf("onno site: %d", c)
	}
	if c := post("http://localhost:3001"); c != http.StatusMethodNotAllowed { // CrossOriginProtection paar, tarpor mux er 405
		t.Errorf("admin origin: %d", c)
	}
}

// TestServeShutdown (2.2): ctx bondho hole server thame, timeout 4.2 er moto.
func TestServeShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv := newServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	done := make(chan error, 1)
	go func() { done <- serve(ctx, srv, ln, quiet()) }()
	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown hoyni")
	}
	if srv.ReadHeaderTimeout != 5*time.Second || srv.ReadTimeout != 15*time.Second || srv.WriteTimeout != 30*time.Second || srv.IdleTimeout != 60*time.Second {
		t.Error("timeout 4.2 er moto na")
	}
}

func TestRun(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	noEnv := func(string) (string, bool) { return "", false }
	if err := run(ctx, []string{"version"}, noEnv, &out); err != nil || out.Len() == 0 {
		t.Errorf("version: %v", err)
	}
	if err := run(ctx, []string{"nai"}, noEnv, &out); err == nil {
		t.Error("ochena command e error ashe ni")
	}
	for _, cmd := range []string{"serve", "migrate"} {
		if err := run(ctx, []string{cmd}, noEnv, &out); err == nil || !strings.Contains(err.Error(), "APP_ENV") {
			t.Errorf("%s env chhara: %v", cmd, err)
		}
	}
	env := map[string]string{"APP_ENV": "local", "DATABASE_URL": "postgres://x@127.0.0.1:1/db?connect_timeout=1",
		"JASPR_INTERNAL_SECRET": strings.Repeat("s", 32)}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	if err := run(ctx, nil, lookup, &out); err == nil {
		t.Error("database chhara serve chole geche")
	}
}
