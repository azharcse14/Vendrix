package httpx

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIP (3.2 Niyom 5): Caddy theke asha request e X-Real-IP (Caddy client er boshano header muche nijer
// ta boshay), Jaspr theke X-Client-IP + X-Internal-Auth; duijon-i loopback theke ashe. Onno shob khetre
// connection er IP. Header er value valid IP na hole mane na. Fol IP(ctx) e: rate limit, orders.ip,
// sessions.ip, many_orders.
func ClientIP(internalSecret string) Middleware {
	secret := []byte(internalSecret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := resolveIP(r, secret)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey, ip)))
		})
	}
}

// IP: ClientIP middleware er thik kora IP ("" jodi middleware na thake).
func IP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey).(string)
	return ip
}

func resolveIP(r *http.Request, secret []byte) string {
	peer, ok := parseIP(r.RemoteAddr)
	if !ok {
		return ""
	}
	if peer.IsLoopback() {
		if len(secret) > 0 && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Auth")), secret) == 1 {
			if ip, ok := parseIP(r.Header.Get("X-Client-IP")); ok {
				return ip.String()
			}
		}
		if ip, ok := parseIP(r.Header.Get("X-Real-IP")); ok {
			return ip.String()
		}
	}
	return peer.String()
}

// parseIP: "1.2.3.4", "1.2.3.4:5678", "[::1]:80" ba "::1"; IPv4-mapped IPv6 ke IPv4, zone bad.
func parseIP(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap().WithZone(""), true
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, false
	}
	return a.Unmap().WithZone(""), true
}
