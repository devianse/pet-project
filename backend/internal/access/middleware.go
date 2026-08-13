// backend/internal/access/middleware.go
package access

import (
	"net/http"

	"github.com/devianse/pet-project/backend/internal/auth"
)

// RequireFeature 403s a request whose caller hasn't been granted key
// (admin bypasses via HasFeature). Must run inside auth.Require in the
// middleware chain — it reads claims auth.Require already injected into
// the request context, the same way auth.ClaimsFromContext is documented
// to be used.
func RequireFeature(store *Store, key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID, err := claims.UserID()
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			has, err := HasFeature(r.Context(), store, userID, claims.Role, key)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if !has {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
