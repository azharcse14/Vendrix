package httpx

import (
	"net/http/httptest"
	"testing"
)

// TestAdminCookie (11.3 v1.19, 12.2): __Host-asid = HttpOnly; Secure; SameSite=Strict; Path=/, Domain na;
// Max-Age 43200 (active), 900 (setup_pending), logout e 0 (value khali).
func TestAdminCookie(t *testing.T) {
	for _, c := range []struct {
		token, want string
		maxAge      int
	}{
		{"tok", "__Host-asid=tok; Path=/; Max-Age=43200; HttpOnly; Secure; SameSite=Strict", 43200},
		{"tok", "__Host-asid=tok; Path=/; Max-Age=900; HttpOnly; Secure; SameSite=Strict", 900},
		{"", "__Host-asid=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Strict", 0},
	} {
		rec := httptest.NewRecorder()
		AdminCookie(rec, c.token, c.maxAge)
		if got := rec.Header().Values("Set-Cookie"); len(got) != 1 || got[0] != c.want {
			t.Errorf("Max-Age %d: %q, chai %q", c.maxAge, got, c.want)
		}
	}
}
