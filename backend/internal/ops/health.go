// backend/internal/ops/health.go
package ops

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/devianse/pet-project/backend/internal/realtime"
)

const (
	healthTopic     = "ops.health"
	defaultInterval = 15 * time.Second
	// pingTimeout bounds each individual PingContext call — Run's own
	// ctx lives for the whole process, so without this a hung DB
	// connection could block a single tick indefinitely and silently
	// stall every broadcast after it (see golang-design-patterns'
	// "every external call needs a timeout").
	pingTimeout = 5 * time.Second
)

// pinger is the one thing HealthTicker needs from *sql.DB — same
// "define the interface at the consumer" pattern used across the
// backend (e.g. internal/access's userLister). *sql.DB satisfies this
// structurally.
type pinger interface {
	PingContext(ctx context.Context) error
}

// hub is the one thing HealthTicker needs from *realtime.Hub: a way to
// tell whether anyone's listening before doing the work of a ping, and a
// way to publish the result. *realtime.Hub satisfies this structurally.
type hub interface {
	SubscriberCount(topic string) int
	Broadcast(env realtime.Envelope)
}

type healthPayload struct {
	Status  string `json:"status"`
	DB      string `json:"db"`
	Version string `json:"version"`
}

// HealthTicker periodically checks DB connectivity and broadcasts the
// result on ops.health — but only while at least one client is
// subscribed, so an idle admin panel doesn't keep a DB ping running
// forever in the background (see planning/decisions.md's "Ops panel
// live-update" entry).
type HealthTicker struct {
	db       pinger
	hub      hub
	gitSHA   string
	interval time.Duration
}

// Option configures a HealthTicker. Mirrors realtime.Hub's own
// functional-options pattern for the thing it's driving.
type Option func(*HealthTicker)

// WithInterval overrides the default 15s tick interval — mainly useful
// for tests, which need a much shorter cadence than production.
func WithInterval(d time.Duration) Option {
	return func(t *HealthTicker) { t.interval = d }
}

func NewHealthTicker(db pinger, hub hub, gitSHA string, opts ...Option) *HealthTicker {
	t := &HealthTicker{db: db, hub: hub, gitSHA: gitSHA, interval: defaultInterval}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Run blocks, ticking on t.interval, until ctx is cancelled — wired
// alongside the existing graceful-shutdown context in cmd/api/main.go
// (go ticker.Run(ctx)), so it exits cleanly on server shutdown like
// every other background loop in this app.
func (t *HealthTicker) Run(ctx context.Context) {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.tick(ctx)
		}
	}
}

func (t *HealthTicker) tick(ctx context.Context) {
	if t.hub.SubscriberCount(healthTopic) == 0 {
		return
	}
	payload := healthPayload{Status: "ok", DB: "ok", Version: t.gitSHA}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := t.db.PingContext(pingCtx); err != nil {
		payload.Status, payload.DB = "degraded", "unreachable"
	}
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("marshal health broadcast payload", "error", err)
		return
	}
	t.hub.Broadcast(realtime.Envelope{Topic: healthTopic, Type: realtime.MessageTypeUpdate, Payload: data})
}
