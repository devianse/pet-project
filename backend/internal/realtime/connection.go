// backend/internal/realtime/connection.go
package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	// defaultWriteTimeout bounds a single outbound write — a slow client
	// stalls its own delivery, not the Hub's Broadcast loop (see hub.go's
	// Broadcast, which never blocks on a connection's send()).
	defaultWriteTimeout = 5 * time.Second
	// outboundBufferSize is how many pending envelopes a connection will
	// queue before Hub.Broadcast treats it as stuck and closes it.
	outboundBufferSize = 16
	// maxControlMessageBytes caps inbound reads — control messages
	// (subscribe/unsubscribe) are tiny; this guards against a
	// misbehaving or malicious client sending oversized frames.
	maxControlMessageBytes = 4096
)

// connection wraps a coder/websocket connection as a Hub subscriber.
// Writes are serialized through outbound so callers never need to
// coordinate concurrent writes themselves; send() is non-blocking so
// Hub.Broadcast never stalls on one connection.
type connection struct {
	connID   string
	uid      int64
	ws       *websocket.Conn
	outbound chan Envelope

	closeOnce sync.Once
	closed    chan struct{}
}

func newConnection(connID string, uid int64, ws *websocket.Conn) *connection {
	return &connection{
		connID:   connID,
		uid:      uid,
		ws:       ws,
		outbound: make(chan Envelope, outboundBufferSize),
		closed:   make(chan struct{}),
	}
}

// Compile-time check that *connection satisfies hub.go's subscriber
// interface — a signature drift here fails go build, not just whichever
// test happens to register a *connection with a Hub.
var _ subscriber = (*connection)(nil)

func (c *connection) id() string    { return c.connID }
func (c *connection) userID() int64 { return c.uid }

// send enqueues env without blocking. Returns false if the outbound
// buffer is already full or the connection is closing — the Hub
// interprets false as "drop this connection" (see hub.go's Broadcast).
func (c *connection) send(env Envelope) bool {
	select {
	case <-c.closed:
		return false
	default:
	}
	select {
	case c.outbound <- env:
		return true
	default:
		return false
	}
}

func (c *connection) close(reason string) {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.ws.Close(websocket.StatusNormalClosure, reason)
	})
}

// writePump serializes every outbound envelope through this single
// goroutine. Each write gets its own bounded timeout so one slow send
// can't hang the loop indefinitely.
func (c *connection) writePump(ctx context.Context) {
	for {
		select {
		case env := <-c.outbound:
			data, err := json.Marshal(env)
			if err != nil {
				slog.Warn("realtime: failed to marshal outbound envelope", "conn_id", c.connID, "error", err)
				continue // a malformed envelope shouldn't kill the connection
			}
			writeCtx, cancel := context.WithTimeout(ctx, defaultWriteTimeout)
			err = c.ws.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				c.close("write failed")
				return
			}
		case <-c.closed:
			return
		case <-ctx.Done():
			return
		}
	}
}

// readPump handles only control messages (subscribe/unsubscribe) — this
// shell is push-only otherwise. Every successful Read also implicitly
// proves the connection is alive; combined with Handler's ping loop
// (Task 6), a connection that stops responding gets reclaimed rather
// than lingering forever.
func (c *connection) readPump(ctx context.Context, hub *Hub, identity Identity) {
	defer c.close("read loop ended")
	c.ws.SetReadLimit(maxControlMessageBytes)
	for {
		_, data, err := c.ws.Read(ctx)
		if err != nil {
			return
		}
		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue // malformed control message — ignore, don't drop the connection over it
		}
		if err := env.Validate(); err != nil {
			continue
		}
		switch env.Type {
		case MessageTypeSubscribe:
			hub.Subscribe(ctx, c, identity, env.Topic)
		case MessageTypeUnsubscribe:
			hub.Unsubscribe(c, env.Topic)
		}
	}
}

// newConnID generates a random per-connection identifier. Not a secret —
// crypto/rand is used anyway for a uniform, dependency-free source of
// randomness rather than adding math/rand's separate seeding concerns
// for something this cheap to get right.
func newConnID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
