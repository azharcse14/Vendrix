package httpx

import (
	"context"
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

// RateLimit (7.8 "baki public"): ek shop e ek client IP te window e limit ta request, tarpor 429
// rate_limited + Retry-After. ShopFromHost ar ClientIP er pore boshe (key shop_id:ip).
// ponytail: process er memory te fixed window; ek er beshi API process hole protitir alada gona.
func RateLimit(limit int, window time.Duration) Middleware {
	return newRateLimiter(limit, window).middleware
}

type rateKey struct {
	shopID int64
	ip     string
}

type rateLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time
	mu     sync.Mutex
	start  time.Time
	counts map[rateKey]int // shudhu ei window er; window bodlale notun map, tai purono key thake na
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, now: time.Now, counts: map[rateKey]int{}}
}

// allow: ei request cholbe kina; na hole koto second por.
func (l *rateLimiter) allow(k rateKey) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if start := now.Truncate(l.window); !start.Equal(l.start) {
		l.start, l.counts = start, map[rateKey]int{}
	}
	if l.counts[k] >= l.limit {
		return false, int(math.Ceil(l.start.Add(l.window).Sub(now).Seconds()))
	}
	l.counts[k]++
	return true, 0
}

func (l *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := ShopFrom(r.Context())
		if ok, retry := l.allow(rateKey{s.ID, ClientKey(IP(r.Context()))}); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(max(retry, 1)))
			WriteError(w, r, http.StatusTooManyRequests, ErrorBody{Code: "rate_limited"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientKey: IPv6 hole /64 (ek client er puro subnet; address ghuriye limit edano na), IPv4 hubohu. orders.ip er tulona o eta diye (8.19 item 13 c).
func ClientKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil || a.Unmap().Is4() {
		return ip
	}
	p, _ := a.Prefix(64)
	return p.String()
}

// Limit: RateLimit er ekoi gona route er middleware ar code er Take duijaygay (7.8 v1.34: login ar checkout er
// OTP send ek client IP limit).
type Limit struct{ l *rateLimiter }

func NewLimit(limit int, window time.Duration) *Limit { return &Limit{newRateLimiter(limit, window)} }

func (l *Limit) Middleware(next http.Handler) http.Handler { return l.l.middleware(next) }

// Take: ctx er shop ar client IP er slot; na pele false ar koto second por.
func (l *Limit) Take(ctx context.Context) (bool, int) {
	s, _ := ShopFrom(ctx)
	return l.l.allow(rateKey{s.ID, ClientKey(IP(ctx))})
}

// FailLimit (7.8 staff login): ek client IP te window e limit ta bhul. Take age gone (parallel e-o limit par
// hoy na), bhul na hole Refund; tai shudhu bhul gona thake.
type FailLimit struct{ l *rateLimiter }

func NewFailLimit(limit int, window time.Duration) *FailLimit {
	return &FailLimit{newRateLimiter(limit, window)}
}

// Take: request er client IP (ctx) er slot; na pele false ar koto second por.
func (f *FailLimit) Take(ctx context.Context) (bool, int) {
	return f.l.allow(rateKey{ip: ClientKey(IP(ctx))})
}

// Refund: Take er slot ferot (thik login, lock ba server er bhul).
func (f *FailLimit) Refund(ctx context.Context) {
	f.l.mu.Lock()
	defer f.l.mu.Unlock()
	if k := (rateKey{ip: ClientKey(IP(ctx))}); f.l.counts[k] > 0 {
		f.l.counts[k]--
	}
}
