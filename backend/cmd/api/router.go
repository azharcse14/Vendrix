package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"vendrix/backend/internal/catalog"
	"vendrix/backend/internal/config"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/shop"
)

// pinger: *sql.DB (test e fake).
type pinger interface {
	PingContext(ctx context.Context) error
}

// newHandler: shob route ekhane (4.2). Notun route: method shoho pattern ("GET /api/products/{slug}");
// public route e public middleware (ShopFromHost → RateLimit 120/minute, 3.2, 7.8); admin route
// RequireStaff → RequireRole (staff session M4 e, tar age fail-closed 401, v1.10). health = healthz er ping
// (test e fake). ServeMux pattern conflict e panic
// kore, tai TestRouterBuild protibar puro router banay (12.2 Server 7).
//
// Middleware er kram (bairer theke): request id → client IP → log → recover → CORS → CrossOriginProtection
// → JSONErrors (route na mille 404/405 JSON, 7.1).
// Log recover er bairer, tai panic er 500 o log e ashe.
func newHandler(cfg config.Config, log *slog.Logger, d *sql.DB, health pinger) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(health))

	shopMW, limit := httpx.ShopFromHost(log, shop.NewResolver(d).Resolve), httpx.RateLimit(120, time.Minute)
	public := func(h http.Handler) http.Handler { return shopMW(limit(h)) }
	sh := shop.NewHandlers(d, log)
	mux.Handle("GET /api/settings", public(http.HandlerFunc(sh.Settings)))
	mux.Handle("GET /api/settings/version", public(http.HandlerFunc(sh.SettingsVersion)))
	mux.Handle("GET /api/translations", public(http.HandlerFunc(sh.Translations)))
	mux.Handle("GET /api/policies/{kind}", public(http.HandlerFunc(sh.Policy)))
	catalog.NewHandlers(d, log).Routes(mux, public)

	cop := http.NewCrossOriginProtection()
	for _, o := range cfg.AdminOrigins {
		if err := cop.AddTrustedOrigin(o); err != nil {
			return nil, fmt.Errorf("ADMIN_ORIGINS %q: %w", o, err)
		}
	}
	return httpx.Chain(httpx.JSONErrors(mux),
		httpx.RequestID,
		httpx.ClientIP(cfg.JasprInternalSecret),
		httpx.Log(log, mux),
		httpx.Recover(log),
		httpx.CORS(cfg.AdminOrigins),
		cop.Handler,
	), nil
}

// healthz: API ar database (13.8 er uptime monitor, protiti minute). Database 2 second e na pele 503.
func healthz(p pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := p.PingContext(ctx); err != nil {
			httpx.WriteError(w, r, http.StatusServiceUnavailable, httpx.ErrorBody{Code: "db_unavailable"})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
