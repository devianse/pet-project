// backend/internal/auth/middleware.go
package auth

import (
	"context"
	"net/http"
)

type contextKey string

const claimsContextKey contextKey = "auth_claims"

// Require wraps a handler so it 401s without a valid session cookie,
// otherwise injects the parsed claims into the request context so
// downstream handlers can read the caller's user id/role without
// re-parsing the cookie themselves.
func Require(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			claims, err := ParseToken(secret, cookie.Value)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*Claims)
	return claims, ok
}
