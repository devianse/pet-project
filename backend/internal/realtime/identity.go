// backend/internal/realtime/identity.go
package realtime

import "net/http"

// Identity is what the Hub and TopicAuthorizer know about the caller
// behind a connection — deliberately just enough for topic authorization
// decisions, not a full user profile.
type Identity struct {
	UserID int64
	Role   string
}

// Authenticator proves identity from an upgrade request. This is the seam
// that decouples the WS handshake from *how* identity is proven: today's
// implementation (Task 6) wraps auth.ClaimsFromRequest against the
// existing session cookie; an eventual OAuth-library rewrite only needs a
// new Authenticator implementation, not a Hub/protocol/connection change.
type Authenticator interface {
	Authenticate(r *http.Request) (Identity, error)
}

// AuthenticatorFunc adapts a plain function to Authenticator, the same
// pattern net/http.HandlerFunc uses.
type AuthenticatorFunc func(r *http.Request) (Identity, error)

func (f AuthenticatorFunc) Authenticate(r *http.Request) (Identity, error) {
	return f(r)
}

// Compile-time check that AuthenticatorFunc satisfies Authenticator — a
// signature drift here fails go build, not just whichever test happens
// to exercise it.
var _ Authenticator = AuthenticatorFunc(nil)
