// backend/internal/auth/cookie.go
package auth

import "net/http"

const cookieName = "auth_token"

// setAuthCookie writes the session cookie. Secure is conditional on the
// caller's environment — see the design spec: a Secure cookie is
// silently dropped by browsers over plain http://localhost, which is
// how local dev runs.
func setAuthCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(TokenTTL.Seconds()),
	})
}

func clearAuthCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
