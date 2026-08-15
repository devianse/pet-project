// backend/internal/ops/health_test.go
package ops

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/devianse/pet-project/backend/internal/realtime"
)

// fakePinger is a *sql.DB test double — HealthTicker only needs
// PingContext, so the fake only implements that, no real database
// involved (mirrors the "define the interface at the consumer" pattern
// used across the backend, e.g. internal/access's userLister).
type fakePinger struct {
	err error

	mu sync.Mutex
	// sawDeadline records whether the ctx passed to PingContext carried a
	// deadline — HealthTicker must bound each ping itself rather than
	// handing the long-lived Run context straight through, since nothing
	// else bounds how long a hung DB connection could block one tick.
	sawDeadline bool
}

func (f *fakePinger) PingContext(ctx context.Context) error {
	_, hasDeadline := ctx.Deadline()
	f.mu.Lock()
	f.sawDeadline = hasDeadline
	f.mu.Unlock()
	return f.err
}

func (f *fakePinger) deadlineSeen() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sawDeadline
}

// fakeHub is a test double for the hub interface (SubscriberCount +
// Broadcast) — records broadcasts and lets tests control subscriber
// count without a real realtime.Hub.
type fakeHub struct {
	mu        sync.Mutex
	count     int
	envelopes []realtime.Envelope
}

func (f *fakeHub) SubscriberCount(topic string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count
}

func (f *fakeHub) Broadcast(env realtime.Envelope) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envelopes = append(f.envelopes, env)
}

func (f *fakeHub) snapshot() []realtime.Envelope {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]realtime.Envelope, len(f.envelopes))
	copy(out, f.envelopes)
	return out
}

func TestHealthTicker_SkipsBroadcastWhenNoSubscribers(t *testing.T) {
	hub := &fakeHub{count: 0}
	ticker := NewHealthTicker(&fakePinger{}, hub, "abc123", WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	go ticker.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	if got := len(hub.snapshot()); got != 0 {
		t.Fatalf("expected no broadcasts with 0 subscribers, got %d", got)
	}
}

func TestHealthTicker_BroadcastsWhenSubscribed(t *testing.T) {
	hub := &fakeHub{count: 1}
	ticker := NewHealthTicker(&fakePinger{}, hub, "abc123", WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	go ticker.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	envs := hub.snapshot()
	if len(envs) == 0 {
		t.Fatal("expected at least one broadcast with a subscriber present")
	}
	env := envs[0]
	if env.Topic != "ops.health" {
		t.Fatalf("expected topic ops.health, got %q", env.Topic)
	}
	if env.Type != realtime.MessageTypeUpdate {
		t.Fatalf("expected type update, got %q", env.Type)
	}
	var payload healthPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshaling payload: %v", err)
	}
	if payload.Status != "ok" || payload.DB != "ok" || payload.Version != "abc123" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestHealthTicker_ReportsDegradedOnPingFailure(t *testing.T) {
	hub := &fakeHub{count: 1}
	ticker := NewHealthTicker(&fakePinger{err: errors.New("connection refused")}, hub, "abc123", WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	go ticker.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	envs := hub.snapshot()
	if len(envs) == 0 {
		t.Fatal("expected at least one broadcast")
	}
	var payload healthPayload
	if err := json.Unmarshal(envs[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshaling payload: %v", err)
	}
	if payload.Status != "degraded" || payload.DB != "unreachable" {
		t.Fatalf("expected degraded/unreachable, got %+v", payload)
	}
}

func TestHealthTicker_PingIsBoundedByItsOwnTimeout(t *testing.T) {
	pinger := &fakePinger{}
	hub := &fakeHub{count: 1}
	ticker := NewHealthTicker(pinger, hub, "abc123", WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	go ticker.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	if !pinger.deadlineSeen() {
		t.Fatal("expected PingContext to receive a context with its own deadline, got one with none")
	}
}

func TestHealthTicker_RunReturnsOnContextCancel(t *testing.T) {
	hub := &fakeHub{count: 0}
	ticker := NewHealthTicker(&fakePinger{}, hub, "abc123", WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ticker.Run(ctx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return within 1s of context cancellation")
	}
}
