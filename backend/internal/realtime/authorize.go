// backend/internal/realtime/authorize.go
package realtime

import "context"

// TopicAuthorizer decides whether identity may subscribe to topic —
// checked on every subscribe request (Hub.Subscribe), not just once at
// connect time, so a connection's allowed topics can be as fine-grained
// as a consumer needs.
type TopicAuthorizer interface {
	Authorize(ctx context.Context, identity Identity, topic string) bool
}

type TopicAuthorizerFunc func(ctx context.Context, identity Identity, topic string) bool

func (f TopicAuthorizerFunc) Authorize(ctx context.Context, identity Identity, topic string) bool {
	return f(ctx, identity, topic)
}

// AllowAuthenticated permits any topic to any authenticated identity. It
// is the shell's default policy because no real topic exists yet to gate
// (see docs/superpowers/specs/2026-08-15-websockets-shell-design.md's
// follow-ups) — the first real consumer (Ops panel) is expected to wire
// a real policy (e.g. checking internal/access per topic prefix) in
// cmd/api/main.go instead of this default, not to extend this default
// with hardcoded topic names.
var AllowAuthenticated TopicAuthorizerFunc = func(_ context.Context, _ Identity, _ string) bool {
	return true
}

// Compile-time check that TopicAuthorizerFunc satisfies TopicAuthorizer.
var _ TopicAuthorizer = TopicAuthorizerFunc(nil)
