package httpx

import "net/http"

// CORS: shudhu ADMIN_ORIGINS (admin panel, 11.3), credentials shoho (cookie `__Host-asid`). Onno origin e
// kono CORS header na, tai browser block kore. Website er browser API te ashe na (BFF, 10.9). CSRF er
// pahara http.CrossOriginProtection (12.2); CORS shudhu admin er browser ke response porte dey.
func CORS(origins []string) Middleware {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			o := r.Header.Get("Origin")
			if o != "" {
				w.Header().Add("Vary", "Origin")
			}
			if o == "" || !allowed[o] {
				next.ServeHTTP(w, r)
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", o)
			h.Set("Access-Control-Allow-Credentials", "true")
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
