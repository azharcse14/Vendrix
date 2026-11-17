package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"vendrix/backend/internal/audit"
	"vendrix/backend/internal/auth"
	"vendrix/backend/internal/cart"
	"vendrix/backend/internal/catalog"
	"vendrix/backend/internal/config"
	"vendrix/backend/internal/coupon"
	customeradmin "vendrix/backend/internal/customer"
	"vendrix/backend/internal/httpx"
	"vendrix/backend/internal/inventory"
	"vendrix/backend/internal/order"
	"vendrix/backend/internal/shipping"
	"vendrix/backend/internal/shop"
	"vendrix/backend/internal/storage"
)

// siteverifyURL: Turnstile (test e fake server).
var siteverifyURL = auth.SiteverifyURL

// lookupStaffSession: __Host-asid → staff (test e fake, TestAdminRouteRoles DB chhara role dey).
var lookupStaffSession = auth.LookupStaffSession

// pinger: *sql.DB (test e fake).
type pinger interface {
	PingContext(ctx context.Context) error
}

// newHandler: shob route ekhane (4.2). Notun route: method shoho pattern ("GET /api/products/{slug}");
// public route e public middleware (ShopFromHost → RateLimit 120/minute, 3.2, 7.8); admin route
// AdminSession → RequireRole (RequireStaff shoho; cookie na thakle 401, setup_pending e 403). health = healthz er ping
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

	// OTP send e public limit er upore client IP te ghontay 10 (7.8); checkout er OTP send o ei gonay (Guard.OTPIP)
	ts := auth.Turnstile{URL: siteverifyURL, Secret: cfg.TurnstileSecret}
	ah := auth.NewHandlers(d, log, cfg.OTPSecret, ts, cartMerge)
	otpIP := httpx.NewLimit(10, time.Hour)
	mux.Handle("POST /api/auth/otp/send", public(otpIP.Middleware(http.HandlerFunc(ah.SendOTP))))
	mux.Handle("POST /api/auth/otp/verify", public(http.HandlerFunc(ah.VerifyOTP)))
	// customer route: oi shop er customer session (3.2, Rule 16)
	customer := httpx.RequireCustomer(log, auth.LookupSession(d))
	mux.Handle("POST /api/auth/logout", public(customer(http.HandlerFunc(ah.Logout))))
	mux.Handle("GET /api/me", public(customer(http.HandlerFunc(ah.Me))))
	mux.Handle("PATCH /api/me", public(customer(http.HandlerFunc(ah.UpdateMe))))

	// staff login (7.7, 8.3): shop context chhara; shop body er slug ba temp_token er session theke
	adm := auth.NewAdminHandlers(d, log, cfg.TOTPKey, cfg.RecoverySecret)
	mux.HandleFunc("POST /api/admin/auth/login", adm.Login)
	// totp/recovery: client IP te minute e 120, duita route mile (7.8 v1.24; shop nai, key 0:ip)
	mfaIP := httpx.RateLimit(120, time.Minute)
	mux.Handle("POST /api/admin/auth/totp", mfaIP(http.HandlerFunc(adm.TOTP)))
	mux.Handle("POST /api/admin/auth/recovery", mfaIP(http.HandlerFunc(adm.Recovery)))
	// admin session (3.2 Niyom 3, 11.3): __Host-asid → staff; setup route setup_pending eo, baki RequireStaff (active)
	staff := httpx.AdminSession(log, lookupStaffSession(d))
	mux.Handle("POST /api/admin/auth/password", staff(http.HandlerFunc(adm.Password)))
	mux.Handle("POST /api/admin/auth/totp/setup", staff(http.HandlerFunc(adm.TOTPSetup)))
	mux.Handle("POST /api/admin/auth/totp/enable", staff(http.HandlerFunc(adm.TOTPEnable)))
	mux.Handle("POST /api/admin/auth/logout", staff(http.HandlerFunc(adm.Logout)))
	mux.Handle("GET /api/admin/me", staff(httpx.RequireStaff(http.HandlerFunc(adm.Me))))
	auth.NewStaffAdmin(d, log).AdminRoutes(mux, staff)
	audit.NewHandlers(d, log).AdminRoutes(mux, staff)

	catalog.NewHandlers(d, log, newStore(cfg)).Routes(mux, public, staff)
	zh := shipping.NewHandlers(d, log)
	zh.Routes(mux, public)
	zh.AdminRoutes(mux, staff)
	coupon.NewHandlers(d, log).AdminRoutes(mux, staff)
	customeradmin.NewHandlers(d, log).AdminRoutes(mux, staff)
	// cart (7.4, 8.4 item 2): Bearer thakle account cart (bhul/expire 401), na hole X-Cart-Key er guest cart
	optCustomer := httpx.OptionalCustomer(log, auth.LookupSession(d))
	ch := cart.NewHandlers(d, log)
	ch.Routes(mux, func(h http.Handler) http.Handler { return public(optCustomer(h)) })
	ch.WishlistRoutes(mux, func(h http.Handler) http.Handler { return public(customer(h)) })
	// checkout (7.5): cart er moto owner
	oh := order.NewHandlers(d, log, order.Guard{Turnstile: ts, OTPSecret: cfg.OTPSecret, ConfirmSecret: cfg.ConfirmSecret, PrintSecret: cfg.PrintSecret, OTPIP: otpIP})
	oh.Routes(mux, func(h http.Handler) http.Handler { return public(optCustomer(h)) })
	oh.MeRoutes(mux, func(h http.Handler) http.Handler { return public(customer(h)) })
	oh.AdminRoutes(mux, staff)
	inventory.NewHandlers(d, log).Routes(mux, staff)

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

// newStore: chobi er store (11.7 item 9); config.Load STORAGE=local shudhu local e dey.
func newStore(cfg config.Config) storage.Store {
	if cfg.Storage == "local" {
		return storage.Local{Dir: cfg.UploadDir}
	}
	return storage.NewR2(cfg.R2AccountID, cfg.R2Bucket, cfg.R2AccessKeyID, cfg.R2SecretAccessKey)
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

// cartMerge: auth.CartMerge (auth cart import kore na, import cycle).
func cartMerge(ctx context.Context, tx *sql.Tx, shopID, userID int64, key, locale string) (any, error) {
	return cart.Merge(ctx, tx, shopID, userID, key, locale)
}
