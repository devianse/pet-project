package auth

import (
	"context"
	"net/http"
)

type contextKey string

const claimsContextKey contextKey = "auth_claims"

// ClaimsFromRequest reads the session cookie off r and parses it into
// Claims. Shared by Require and Me/UpdateMe so every path that decides
// whether a cookie is valid can't silently drift apart. Exported so
// package realtime (the WebSocket upgrade handshake) can reuse the same
// cookie-validation logic REST already relies on, without duplicating it.
func ClaimsFromRequest(r *http.Request, secret []byte) (*Claims, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, err
	}
	return ParseToken(secret, cookie.Value)
}

// Require wraps a handler so it 401s without a valid session cookie,
// otherwise injects the parsed claims into the request context so
// downstream handlers can read the caller's user id/role without
// re-parsing the cookie themselves.
func Require(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := ClaimsFromRequest(r, secret)
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

// Liveness is the narrow read RequireActive needs: whether a user id is
// still allowed to keep using an already-issued session. *Store
// satisfies it; tests pass a fake.
type Liveness interface {
	IsActive(ctx context.Context, id int64) (bool, error)
}

// RequireActive rejects a request whose session is cryptographically
// valid but belongs to a user who has since been deactivated (or
// deleted). Require only verifies the signature, and the session cookie
// is a single long-lived token with no refresh or revocation flow — so
// without this, deactivation would only block *login* and an existing
// cookie would keep working for the rest of TokenTTL (14 days).
//
// Chain it after Require, which puts the parsed claims in the context.
// This mirrors how access.RequireRole re-reads the role from the
// database instead of trusting the JWT claim: a claim is a snapshot, the
// database is the truth.
//
// Fails closed: any error from the lookup (including a missing user row)
// is a 401, not a pass-through.
func RequireActive(store Liveness) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID, err := claims.UserID()
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			active, err := store.IsActive(r.Context(), userID)
			if err != nil || !active {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
