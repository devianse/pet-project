// backend/internal/realtime/hub.go
package realtime

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	defaultMaxConnsPerUser = 8
	defaultShutdownTimeout = 5 * time.Second
)

// subscriber is everything the Hub needs from a connection. Unexported
// and satisfied by connection.go's *connection in production and a fake
// in hub_test.go — the Hub's routing/authz/lifecycle logic is fully
// testable without a real socket.
type subscriber interface {
	id() string
	userID() int64
	// send enqueues env without blocking. false means the connection's
	// outbound buffer is full — the Hub treats that as "drop this
	// connection" rather than let one slow reader stall Broadcast for
	// everyone else.
	send(env Envelope) bool
	close(reason string)
}

// Option configures a Hub. NewHub's zero-option call uses production
// defaults (defaultMaxConnsPerUser, defaultShutdownTimeout) — tests and
// callers with different needs override individual knobs here rather
// than reaching into Hub's private fields.
//
// Options don't validate individually and don't return error — every
// call site in this codebase passes a hardcoded literal (cmd/api/main.go),
// never external input, so there's no runtime condition for a caller to
// handle. NewHub validates the fully-assembled config once, after all
// options are applied, and panics on an invalid value: a Must*-style
// fail-fast at startup, not a recoverable error path.
type Option func(*Hub)

func WithMaxConnsPerUser(n int) Option {
	return func(h *Hub) { h.maxConnsPerUser = n }
}

func WithShutdownTimeout(d time.Duration) Option {
	return func(h *Hub) { h.shutdownTimeout = d }
}

var ErrTooManyConnections = errors.New("realtime: too many connections for this user")

// Hub is the in-process connection registry and topic router. One
// instance per backend process — no cross-instance fan-out, matching the
// single-VPS/no-orchestration standing decision.
type Hub struct {
	mu              sync.Mutex
	authorizer      TopicAuthorizer
	maxConnsPerUser int
	shutdownTimeout time.Duration

	conns       map[string]subscriber      // connID -> subscriber
	byUser      map[int64]int              // userID -> open connection count
	subscribers map[string]map[string]bool // topic -> set of connIDs
}

func NewHub(authorizer TopicAuthorizer, opts ...Option) *Hub {
	h := &Hub{
		authorizer:      authorizer,
		maxConnsPerUser: defaultMaxConnsPerUser,
		shutdownTimeout: defaultShutdownTimeout,
		conns:           make(map[string]subscriber),
		byUser:          make(map[int64]int),
		subscribers:     make(map[string]map[string]bool),
	}
	for _, opt := range opts {
		opt(h)
	}
	if h.maxConnsPerUser < 1 {
		panic("realtime: NewHub: maxConnsPerUser must be at least 1")
	}
	if h.shutdownTimeout <= 0 {
		panic("realtime: NewHub: shutdownTimeout must be positive")
	}
	return h
}

// Register adds c to the registry, rejecting it with ErrTooManyConnections
// if c.userID() already has maxConnsPerUser open connections — a defensive
// cap so a reconnect-loop bug (frontend or otherwise) can't grow the
// process's connection count without bound.
func (h *Hub) Register(c subscriber) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.byUser[c.userID()] >= h.maxConnsPerUser {
		return ErrTooManyConnections
	}
	h.conns[c.id()] = c
	h.byUser[c.userID()]++
	return nil
}

// Unregister removes c and every subscription it held. It does not close
// c — by the time a caller unregisters, it already knows the connection
// is gone (this is called from the same code path that observed the
// socket close), so closing again here would be redundant.
//
// A no-op if c was never registered (or already unregistered) — guards
// against a spurious or duplicate call corrupting byUser's count.
func (h *Hub) Unregister(c subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.conns[c.id()]; !ok {
		return
	}
	delete(h.conns, c.id())
	h.byUser[c.userID()]--
	if h.byUser[c.userID()] <= 0 {
		delete(h.byUser, c.userID())
	}
	for topic, ids := range h.subscribers {
		delete(ids, c.id())
		if len(ids) == 0 {
			delete(h.subscribers, topic)
		}
	}
}

// Subscribe adds c to topic's subscriber set if authorizer permits
// identity on topic — checked on every call, not cached, so revoking
// access mid-connection takes effect on the next subscribe attempt (the
// same "re-check, don't cache" posture internal/access already uses for
// REST's admin-bypass check).
func (h *Hub) Subscribe(ctx context.Context, c subscriber, identity Identity, topic string) bool {
	if !h.authorizer.Authorize(ctx, identity, topic) {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subscribers[topic] == nil {
		h.subscribers[topic] = make(map[string]bool)
	}
	h.subscribers[topic][c.id()] = true
	return true
}

func (h *Hub) Unsubscribe(c subscriber, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ids, ok := h.subscribers[topic]
	if !ok {
		return
	}
	delete(ids, c.id())
	if len(ids) == 0 {
		delete(h.subscribers, topic)
	}
}

// SubscriberCount reports how many connections are currently subscribed
// to topic. Used by producers (e.g. internal/ops's HealthTicker) to skip
// generating a message nobody's listening for — never used for authz or
// routing, just a cheap "is anyone watching" check.
func (h *Hub) SubscriberCount(topic string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subscribers[topic])
}

// Broadcast delivers env to every connection currently subscribed to
// env.Topic. Copies the target list under the lock, then delivers
// outside it — send() itself is non-blocking (bounded channel), so this
// is a defensive separation, not a correctness requirement, and it keeps
// the lock held for a bounded, tiny critical section regardless of
// subscriber count.
func (h *Hub) Broadcast(env Envelope) {
	h.mu.Lock()
	ids := h.subscribers[env.Topic]
	targets := make([]subscriber, 0, len(ids))
	for id := range ids {
		if c, ok := h.conns[id]; ok {
			targets = append(targets, c)
		}
	}
	h.mu.Unlock()

	for _, c := range targets {
		if !c.send(env) {
			c.close("outbound buffer full")
		}
	}
}

// Shutdown closes every registered connection with a shutdown reason,
// bounded by h.shutdownTimeout (or ctx's own deadline, whichever is
// sooner) so one unresponsive connection can't stall process exit. Wired
// into cmd/api's graceful shutdown alongside srv.Shutdown (Task 6).
func (h *Hub) Shutdown(ctx context.Context) {
	h.mu.Lock()
	targets := make([]subscriber, 0, len(h.conns))
	for _, c := range h.conns {
		targets = append(targets, c)
	}
	h.mu.Unlock()

	done := make(chan struct{})
	go func() {
		for _, c := range targets {
			c.close("server shutting down")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(h.shutdownTimeout):
	case <-ctx.Done():
	}
}
