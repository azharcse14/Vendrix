package httpx

import "net/http"

// AdminCookie (11.3 v1.19): admin session __Host-asid; HttpOnly (JS token dekhe na), Secure, SameSite=Strict,
// Path=/ ar Domain na (__Host- prefix er shorto). maxAge second (active 43200, setup_pending 900, logout 0).
func AdminCookie(w http.ResponseWriter, token string, maxAge int) {
	if maxAge <= 0 {
		maxAge = -1 // net/http e -1 = "Max-Age=0"
	}
	http.SetCookie(w, &http.Cookie{Name: "__Host-asid", Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
}
