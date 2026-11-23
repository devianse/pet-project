// backend/internal/telegram/poller.go
package telegram

import (
	"context"
	"log/slog"
	"time"
)

// longPollTimeoutSeconds is how long each getUpdates call blocks
// server-side waiting for a new update before returning empty — Telegram
// caps this around 50s. Long enough that the poller is idle most of the
// time (roughly one open request at a time, not a tight loop); short
// enough to reconnect promptly if the connection drops.
const longPollTimeoutSeconds = 30

// initialBackoff/maxBackoff bound the retry delay after a getUpdates
// failure (network blip, Telegram API error): starts fast, caps so a
// prolonged outage doesn't wait forever between retries. var, not const,
// so tests can shrink them.
var (
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
)

// Poller repeatedly long-polls client for new updates and dispatches
// messages from allowedChatID to router. Run never returns on its own
// (except when ctx is cancelled) — call it in a goroutine and let it run
// for the process lifetime.
type Poller struct {
	client        Client
	allowedChatID int64
	router        *Router
	offset        int64
}

func NewPoller(client Client, allowedChatID int64, router *Router) *Poller {
	return &Poller{client: client, allowedChatID: allowedChatID, router: router}
}

// Run loops until ctx is cancelled. Each iteration calls GetUpdates; on
// failure it logs and retries with exponential backoff (reset to
// initialBackoff after any success). On success it processes every update
// in order and advances the offset past all of them, matched or not — an
// update from an unrecognized chat still gets marked processed, so it
// isn't redelivered forever.
func (p *Poller) Run(ctx context.Context) {
	backoff := initialBackoff
	for {
		if ctx.Err() != nil {
			return
		}

		updates, err := p.client.GetUpdates(ctx, p.offset, longPollTimeoutSeconds)
		if err != nil {
			slog.Error("telegram getUpdates failed, retrying", "error", err, "backoff", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff *= 2; backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = initialBackoff

		for _, u := range updates {
			p.offset = u.UpdateID + 1
			p.handleUpdate(ctx, u)
		}
	}
}

// SkipBacklog discards updates that queued up while the poller was not
// running, and must be called once before Run.
//
// Telegram's getUpdates keeps unacknowledged updates for ~24h, and the
// offset lives in memory, so a restart starts from 0 and the whole
// backlog is redelivered. For this bot that is not a cosmetic problem:
// /newnote is a write, so every deploy re-executed the notes typed
// since the last restart and duplicated them.
//
// Skipping rather than persisting the offset is deliberate — the
// container filesystem is rebuilt on deploy and there is no store worth
// writing to for a single integer, and "commands sent while the bot was
// down are dropped" is the right semantics for read-mostly commands.
// If a future command is important enough to require guaranteed
// delivery, this is the place to swap in a persisted offset.
func (p *Poller) SkipBacklog(ctx context.Context) error {
	updates, err := p.client.GetUpdates(ctx, -1, 0)
	if err != nil {
		return err
	}
	for _, u := range updates {
		if u.UpdateID+1 > p.offset {
			p.offset = u.UpdateID + 1
		}
	}
	return nil
}

func (p *Poller) handleUpdate(ctx context.Context, u Update) {
	if u.Message == nil {
		return // update type this package doesn't handle (edited message, etc.)
	}
	chatID := u.Message.Chat.ID
	if chatID != p.allowedChatID {
		slog.Warn("rejected telegram command from unrecognized chat", "chat_id", chatID)
		return
	}

	reply := p.router.Dispatch(ctx, u.Message.Text)
	if reply == "" {
		return
	}
	if err := p.client.SendMessage(ctx, chatID, reply); err != nil {
		slog.Error("telegram sendMessage failed", "error", err)
	}
}
