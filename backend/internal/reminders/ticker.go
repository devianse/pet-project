// backend/internal/reminders/ticker.go
package reminders

import (
	"context"
	"log/slog"
	"time"
)

const (
	defaultInterval = time.Hour
	// tickTimeout bounds each individual external call (ListPending,
	// SendMessage, MarkSent) in tick — Run's own interval spaces ticks
	// out, but this guards against a single call hanging and stalling
	// the reminder loop indefinitely (see internal/ops/health.go's pattern).
	tickTimeout = 5 * time.Second
)

// sender is the one thing Ticker needs from telegram.Client — same
// "define the interface at the consumer" pattern as internal/ops's
// pinger/hub. telegram.Client and *telegram.RealClient both satisfy this
// structurally.
type sender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// store is the one thing Ticker needs from *reminders.Store.
type store interface {
	ListPending(ctx context.Context) ([]Reminder, error)
	MarkSent(ctx context.Context, id int) error
}

// Ticker periodically checks for due reminders and delivers them via
// sender, to chatID. Mirrors internal/ops.HealthTicker's functional-
// options shape.
type Ticker struct {
	store    store
	sender   sender
	chatID   int64
	interval time.Duration
}

type Option func(*Ticker)

// WithInterval overrides the default 1-hour tick interval — mainly
// useful for tests, which need a much shorter cadence than production.
func WithInterval(d time.Duration) Option {
	return func(t *Ticker) { t.interval = d }
}

func NewTicker(store store, sender sender, chatID int64, opts ...Option) *Ticker {
	t := &Ticker{store: store, sender: sender, chatID: chatID, interval: defaultInterval}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Run blocks, ticking on t.interval, until ctx is cancelled — wired
// alongside the Telegram poller in cmd/api/telegram.go (go ticker.Run(ctx)),
// so it exits cleanly on server shutdown like every other background loop.
func (t *Ticker) Run(ctx context.Context) {
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

func (t *Ticker) tick(ctx context.Context) {
	listCtx, cancel := context.WithTimeout(ctx, tickTimeout)
	defer cancel()
	pending, err := t.store.ListPending(listCtx)
	if err != nil {
		slog.Error("reminders: listing pending reminders", "error", err)
		return
	}
	now := time.Now()
	for _, r := range pending {
		if r.DueAt.After(now) {
			// ListPending is sorted soonest-due-first, but a later row
			// could in principle still be due if rows are inserted
			// out of order — continue rather than break, at negligible
			// cost given this app's reminder volume.
			continue
		}
		sendCtx, sendCancel := context.WithTimeout(ctx, tickTimeout)
		err := t.sender.SendMessage(sendCtx, t.chatID, r.Message)
		sendCancel()
		if err != nil {
			slog.Error("reminders: sending reminder", "id", r.ID, "error", err)
			continue
		}
		markCtx, markCancel := context.WithTimeout(ctx, tickTimeout)
		err = t.store.MarkSent(markCtx, r.ID)
		markCancel()
		if err != nil {
			slog.Error("reminders: marking reminder sent", "id", r.ID, "error", err)
		}
	}
}
