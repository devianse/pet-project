// Package datenight implements the Date Night feature — see
// docs/superpowers/specs/2026-08-12-date-night-design.md.
package datenight

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/devianse/pet-project/backend/internal/auth"
)

// Pair is the fixed set of exactly two usernames this feature is scoped
// to, independent of how many total accounts exist in the system —
// see the design spec's "The pair" section.
type Pair struct {
	usernames [2]string
}

// LoadPair parses a "user1,user2" value (as read from the
// DATE_NIGHT_USERNAMES env var) into a Pair. It errors unless the value
// contains exactly two distinct, non-empty usernames.
//
// Surrounding whitespace is trimmed but case is NOT folded: matching is
// exact, the same way auth's Store.FindByUsername looks users up. A
// mis-cased env value therefore reads as a 403, not a login failure.
func LoadPair(raw string) (Pair, error) {
	parts := strings.Split(raw, ",")
	if len(parts) != 2 {
		return Pair{}, fmt.Errorf("expected exactly 2 comma-separated usernames, got %d", len(parts))
	}
	a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if a == "" || b == "" {
		return Pair{}, fmt.Errorf("usernames must not be empty")
	}
	if a == b {
		return Pair{}, fmt.Errorf("usernames must be distinct, got %q twice", a)
	}
	return Pair{usernames: [2]string{a, b}}, nil
}

// Contains reports whether username is one of the pair.
func (p Pair) Contains(username string) bool {
	return username == p.usernames[0] || username == p.usernames[1]
}

// Other returns the pair member that isn't username, and false if
// username isn't in the pair at all.
func (p Pair) Other(username string) (string, bool) {
	switch username {
	case p.usernames[0]:
		return p.usernames[1], true
	case p.usernames[1]:
		return p.usernames[0], true
	default:
		return "", false
	}
}

// RequirePair wraps a handler so only requests from a pair member (per
// DATE_NIGHT_USERNAMES) succeed — everyone else gets 403, even a valid
// logged-in user elsewhere in the app. Must sit behind auth.Require so
// claims are already in the request context (see the design spec's
// "belt-and-suspenders" note — this doesn't depend on the not-yet-built
// per-user page-visibility system).
func RequirePair(pair Pair) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok || !pair.Contains(claims.Username) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
