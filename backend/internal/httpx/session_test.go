package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOptionalCustomer (8.4 v1.28 item 2): Authorization na thakle guest (Customer nai); thakle
// RequireCustomer er path: bhul 401 unauthenticated, expire 401 session_expired, guest e fall back na.
func TestOptionalCustomer(t *testing.T) {
	lookup := func(_ context.Context, _ int64, token string) (Customer, error) {
		switch token {
		case "good":
			return Customer{UserID: 7}, nil
		case "old":
			return Customer{}, ErrSessionExpired
		}
		return Customer{}, ErrNoSession
	}
	var got *Customer
	h := OptionalCustomer(slog.New(slog.DiscardHandler), lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := CustomerFrom(r.Context())
		got = nil
		if ok {
			got = &c
		}
	}))
	for _, tc := range []struct {
		auth   string
		status int
		user   int64 // 0 = guest
	}{
		{"", 200, 0},
		{"Bearer good", 200, 7},
		{"Bearer old", 401, 0},
		{"Bearer bhul", 401, 0},
		{"Bearer ", 401, 0},
		{"Basic abc", 401, 0},
	} {
		got = nil
		r := httptest.NewRequest("GET", "/", nil)
		r = r.WithContext(context.WithValue(r.Context(), shopKey, Shop{ID: 1}))
		if tc.auth != "" {
			r.Header.Set("Authorization", tc.auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != tc.status {
			t.Errorf("%q: status %d, chai %d", tc.auth, rec.Code, tc.status)
		}
		if tc.status == 200 && ((got == nil) != (tc.user == 0) || (got != nil && got.UserID != tc.user)) {
			t.Errorf("%q: customer %+v, chai user %d", tc.auth, got, tc.user)
		}
	}
}
