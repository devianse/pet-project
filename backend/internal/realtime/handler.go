// backend/internal/realtime/handler.go
package realtime

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// defaultPingInterval balances reclaiming dead connections promptly
// against not spamming keepalive traffic — well under the read/write
// timeouts elsewhere in this package, so a missed ping has time to be
// noticed before anything else times out first.
const defaultPingInterval = 30 * time.Second

type HandlerOption func(*Handler)

func WithPingInterval(d time.Duration) HandlerOption {
	return func(h *Handler) { h.pingInterval = d }
}

// Handler is the GET /api/ws upgrade endpoint: authenticate, accept the
// WS upgrade, register with the Hub, and run the connection's read/write/
// ping loops until it closes.
type Handler struct {
	hub          *Hub
	authenticate Authenticator
	pingInterval time.Duration
}

func NewHandler(hub *Hub, authenticator Authenticator, opts ...HandlerOption) *Handler {
	h := &Handler{
		hub:          hub,
		authenticate: authenticator,
		pingInterval: defaultPingInterval,
	}
	for _, opt := range opts {
		opt(h)
	}
	if h.pingInterval <= 0 {
		panic("realtime: NewHandler: pingInterval must be positive")
	}
	return h
}

// Compile-time check that *Handler satisfies http.Handler.
var _ http.Handler = (*Handler)(nil)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, err := h.authenticate.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		// Accept already wrote a response (e.g. non-WS request) — nothing
		// more to do.
		return
	}
	// Deferred immediately per coder/websocket's own recommended pattern:
	// a guard against anything unexpected between here and readPump's own
	// deferred close (e.g. a panic inside Register) leaking the raw
	// connection. CloseNow after a normal close is already a no-op, so
	// this never double-closes in the ordinary path.
	defer ws.CloseNow()

	conn := newConnection(newConnID(), identity.UserID, ws)
	if err := h.hub.Register(conn, identity.UserID); err != nil {
		_ = ws.Close(websocket.StatusPolicyViolation, err.Error())
		return
	}
	defer h.hub.Unregister(conn, identity.UserID)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go conn.writePump(ctx)
	go h.pingLoop(ctx, conn)

	// readPump blocks until the connection closes (read error, close
	// frame, or ctx cancellation) — ServeHTTP must not return before
	// then, since returning would let the caller consider the request
	// finished while the hijacked connection is still live.
	conn.readPump(ctx, h.hub, identity)
}

// pingLoop periodically pings the client; coder/websocket's Ping blocks
// until the corresponding pong arrives or ctx's deadline passes. A
// failed ping means the peer is unreachable — close the connection so
// the Hub stops trying to deliver to it and Register's per-user slot
// frees up for a real reconnect.
func (h *Handler) pingLoop(ctx context.Context, c *connection) {
	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, defaultWriteTimeout)
			err := c.ws.Ping(pingCtx)
			cancel()
			if err != nil {
				slog.Warn("realtime: ping failed, closing connection", "conn_id", c.id(), "error", err)
				c.close("ping failed")
				return
			}
		case <-c.closed:
			return
		case <-ctx.Done():
			return
		}
	}
}
