package httpx

import (
	"math"
	"net/http"
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
		if ok, retry := l.allow(rateKey{s.ID, IP(r.Context())}); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(max(retry, 1)))
			WriteError(w, r, http.StatusTooManyRequests, ErrorBody{Code: "rate_limited"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
