package realtime

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeConn is a subscriber test double — no real socket, just enough to
// observe what the Hub does: envelopes it was sent and whether/why it
// was told to close.
type fakeConn struct {
	connID     string
	uid        int64
	mu         sync.Mutex
	received   []Envelope
	full       bool // when true, send() always reports the buffer as full
	closed     bool
	closeReason string
}

func (f *fakeConn) id() string    { return f.connID }
func (f *fakeConn) userID() int64 { return f.uid }

func (f *fakeConn) send(env Envelope) bool {
	if f.full {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.received = append(f.received, env)
	return true
}

func (f *fakeConn) close(reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	f.closeReason = reason
}

func TestHub_RegisterEnforcesMaxConnsPerUser(t *testing.T) {
	h := NewHub(AllowAuthenticated, WithMaxConnsPerUser(2))
	c1 := &fakeConn{connID: "c1", uid: 42}
	c2 := &fakeConn{connID: "c2", uid: 42}
	c3 := &fakeConn{connID: "c3", uid: 42}

	if err := h.Register(c1); err != nil {
		t.Fatalf("Register c1: %v", err)
	}
	if err := h.Register(c2); err != nil {
		t.Fatalf("Register c2: %v", err)
	}
	if err := h.Register(c3); err != ErrTooManyConnections {
		t.Fatalf("Register c3: expected ErrTooManyConnections, got %v", err)
	}
}

func TestHub_UnregisterFreesUpUserSlot(t *testing.T) {
	h := NewHub(AllowAuthenticated, WithMaxConnsPerUser(1))
	c1 := &fakeConn{connID: "c1", uid: 42}
	c2 := &fakeConn{connID: "c2", uid: 42}

	if err := h.Register(c1); err != nil {
		t.Fatalf("Register c1: %v", err)
	}
	h.Unregister(c1)
	if err := h.Register(c2); err != nil {
		t.Fatalf("Register c2 after unregister: %v", err)
	}
}

func TestHub_SubscribeRespectsAuthorizer(t *testing.T) {
	deny := TopicAuthorizerFunc(func(_ context.Context, id Identity, topic string) bool {
		return id.Role == "admin"
	})
	h := NewHub(deny)
	c := &fakeConn{connID: "c1", uid: 1}
	_ = h.Register(c)

	if h.Subscribe(context.Background(), c, Identity{Role: "user"}, "ops.health") {
		t.Fatal("expected non-admin subscribe to be rejected")
	}
	if !h.Subscribe(context.Background(), c, Identity{Role: "admin"}, "ops.health") {
		t.Fatal("expected admin subscribe to be accepted")
	}
}

func TestHub_BroadcastDeliversOnlyToSubscribers(t *testing.T) {
	h := NewHub(AllowAuthenticated)
	subscribed := &fakeConn{connID: "c1", uid: 1}
	notSubscribed := &fakeConn{connID: "c2", uid: 2}
	_ = h.Register(subscribed)
	_ = h.Register(notSubscribed)
	h.Subscribe(context.Background(), subscribed, Identity{UserID: 1}, "ops.health")

	env := Envelope{Topic: "ops.health", Type: MessageTypeUpdate}
	h.Broadcast(env)

	if len(subscribed.received) != 1 || subscribed.received[0].Topic != "ops.health" {
		t.Fatalf("expected subscribed conn to receive the envelope, got %+v", subscribed.received)
	}
	if len(notSubscribed.received) != 0 {
		t.Fatalf("expected non-subscribed conn to receive nothing, got %+v", notSubscribed.received)
	}
}

func TestHub_BroadcastClosesFullBufferSubscriber(t *testing.T) {
	h := NewHub(AllowAuthenticated)
	c := &fakeConn{connID: "c1", uid: 1, full: true}
	_ = h.Register(c)
	h.Subscribe(context.Background(), c, Identity{UserID: 1}, "ops.health")

	h.Broadcast(Envelope{Topic: "ops.health", Type: MessageTypeUpdate})

	if !c.closed {
		t.Fatal("expected a full-buffer subscriber to be closed by Broadcast")
	}
}

func TestHub_UnregisterCleansUpSubscriptions(t *testing.T) {
	h := NewHub(AllowAuthenticated)
	c := &fakeConn{connID: "c1", uid: 1}
	_ = h.Register(c)
	h.Subscribe(context.Background(), c, Identity{UserID: 1}, "ops.health")
	h.Unregister(c)

	// A fresh connection subscribing to the same topic should not somehow
	// observe stale state from c — broadcasting shouldn't panic or try to
	// deliver to the unregistered connection.
	fresh := &fakeConn{connID: "c2", uid: 1}
	_ = h.Register(fresh)
	h.Broadcast(Envelope{Topic: "ops.health", Type: MessageTypeUpdate})
	if len(fresh.received) != 0 {
		t.Fatalf("fresh unsubscribed conn should not receive broadcast, got %+v", fresh.received)
	}
	if c.closed {
		t.Fatal("Unregister itself should not close the connection — the caller already knows it's gone")
	}
}

func TestNewHub_PanicsOnInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{"zero maxConnsPerUser", []Option{WithMaxConnsPerUser(0)}},
		{"negative maxConnsPerUser", []Option{WithMaxConnsPerUser(-1)}},
		{"zero shutdownTimeout", []Option{WithShutdownTimeout(0)}},
		{"negative shutdownTimeout", []Option{WithShutdownTimeout(-time.Second)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected NewHub to panic on invalid config")
				}
			}()
			NewHub(AllowAuthenticated, tt.opts...)
		})
	}
}

func TestHub_ShutdownClosesAllConnectionsWithinTimeout(t *testing.T) {
	h := NewHub(AllowAuthenticated, WithShutdownTimeout(200*time.Millisecond))
	c1 := &fakeConn{connID: "c1", uid: 1}
	c2 := &fakeConn{connID: "c2", uid: 2}
	_ = h.Register(c1)
	_ = h.Register(c2)

	start := time.Now()
	h.Shutdown(context.Background())
	elapsed := time.Since(start)

	if !c1.closed || !c2.closed {
		t.Fatalf("expected both connections closed, got c1=%v c2=%v", c1.closed, c2.closed)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("Shutdown took %v, expected well under the 200ms timeout for prompt closes", elapsed)
	}
}
