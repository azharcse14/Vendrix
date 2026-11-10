package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRateLimit (7.8): ek shop e ek IP te minute e 120; 121 tom 429 rate_limited + Retry-After; alada shop
// ba IP alada gona; porer minute e abar shuru.
func TestRateLimit(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 10, 0, time.UTC)
	rl := newRateLimiter(120, time.Minute)
	rl.now = func() time.Time { return now }
	h := rl.middleware(http.HandlerFunc(ok204))
	do := func(shopID int64, ip string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/products", nil)
		ctx := context.WithValue(r.Context(), clientIPKey, ip)
		ctx = context.WithValue(ctx, shopKey, Shop{ID: shopID})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r.WithContext(ctx))
		return rec
	}
	for i := range 120 {
		if rec := do(1, "203.0.113.1"); rec.Code != 204 {
			t.Fatalf("%d tom: %d", i+1, rec.Code)
		}
	}
	rec := do(1, "203.0.113.1")
	if rec.Code != 429 || errCode(t, rec) != "rate_limited" || rec.Header().Get("Retry-After") != "50" {
		t.Errorf("121 tom: %d %s Retry-After=%q", rec.Code, rec.Body.String(), rec.Header().Get("Retry-After"))
	}
	if do(2, "203.0.113.1").Code != 204 || do(1, "203.0.113.2").Code != 204 {
		t.Error("alada shop ba IP atke geche")
	}
	now = now.Add(50 * time.Second)
	if rec := do(1, "203.0.113.1"); rec.Code != 204 {
		t.Errorf("porer minute: %d", rec.Code)
	}
}
