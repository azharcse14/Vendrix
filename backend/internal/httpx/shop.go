package httpx

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// Shop: public request er shop (3.2), ShopFromHost context e rakhe.
type Shop struct {
	ID       int64
	Slug     string
	Mode     string // single, marketplace
	Status   string // active, suspended, closed
	Timezone string
}

// ResolveShop: domain (lowercase, port chhara) → shop; na pele ok false.
type ResolveShop func(ctx context.Context, domain string) (s Shop, ok bool, err error)

// ShopFromHost (3.2): public route e X-Shop-Host theke shop. Nai, ochena ba closed → 404 shop_not_found;
// suspended → 423 shop_suspended (v1.10).
func ShopFromHost(log *slog.Logger, resolve ResolveShop) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var s Shop
			var ok bool
			var err error
			if host := hostname(r.Header.Get("X-Shop-Host")); host != "" {
				s, ok, err = resolve(r.Context(), host)
			}
			switch {
			case err != nil:
				log.ErrorContext(r.Context(), "shop resolve", "request_id", RequestIDFrom(r.Context()), "err", err)
				WriteError(w, r, http.StatusInternalServerError, ErrorBody{Code: "internal"})
			case !ok || s.Status == "closed":
				WriteError(w, r, http.StatusNotFound, ErrorBody{Code: "shop_not_found"})
			case s.Status == "suspended":
				WriteError(w, r, http.StatusLocked, ErrorBody{Code: "shop_suspended"})
			default:
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), shopKey, s)))
			}
		})
	}
}

// ShopFrom: ShopFromHost er shop; middleware na thakle ok false.
func ShopFrom(ctx context.Context) (Shop, bool) {
	s, ok := ctx.Value(shopKey).(Shop)
	return s, ok
}

// hostname: "SobujBazar.com:443" → "sobujbazar.com" (shop_domains lowercase, 6.7).
func hostname(h string) string {
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
}
