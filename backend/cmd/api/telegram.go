// backend/cmd/api/telegram.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/devianse/pet-project/backend/internal/notes"
	"github.com/devianse/pet-project/backend/internal/telegram"
)

// startTelegramBot wires up and launches the Telegram poller if
// TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID are both set. Unlike
// DATABASE_URL/JWT_SECRET/TMDB_READ_ACCESS_TOKEN, this integration is
// optional: nothing else in the app depends on the bot existing, so a
// missing token disables this feature rather than failing startup.
//
// ctx should be the process's shutdown context (cancelled on
// SIGINT/SIGTERM), not context.Background() — the poller loop only
// returns when its context is cancelled, so without this the goroutine
// would outlive the rest of the server on every graceful shutdown.
func startTelegramBot(ctx context.Context, logger *slog.Logger, notesStore *notes.Store) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatIDStr == "" {
		logger.Info("telegram bot not configured, skipping (TELEGRAM_BOT_TOKEN/TELEGRAM_CHAT_ID unset)")
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		logger.Error("TELEGRAM_CHAT_ID is not a valid integer", "value", chatIDStr, "error", err)
		os.Exit(1)
	}

	router := telegram.NewRouter()
	router.Handle("/notes", "list all notes", notesListCommand(notesStore))
	router.Handle("/newnote", "add a new note: /newnote <text>", notesNewCommand(notesStore))
	router.Handle("/help", "list available commands", helpCommand(router))

	client := telegram.NewRealClient(token)
	// Populates Telegram's native command menu (the "/" button next to
	// the message box) with the same prefix/description pairs /help
	// prints — best-effort: a failure here doesn't stop the bot from
	// working, it just leaves the menu stale or empty, so it's logged
	// rather than fatal.
	if err := client.SetMyCommands(ctx, router.Commands()); err != nil {
		logger.Error("telegram: failed to set command menu", "error", err)
	}

	poller := telegram.NewPoller(client, chatID, router)
	go poller.Run(ctx)
	logger.Info("telegram bot poller started")
}

// helpCommand replies with every registered command and its description,
// one per line — generated from router.Commands() so it can never drift
// out of sync with what's actually registered (including itself, since
// Router.Handle appends before Dispatch is ever called).
func helpCommand(router *telegram.Router) telegram.CommandFunc {
	return func(_ context.Context, _ string) (string, error) {
		return router.HelpText(), nil
	}
}

// maxReplyChars caps how long a /notes reply is allowed to get, well under
// Telegram's ~4096-char sendMessage limit. Notes have no LIMIT applied by
// notes.Store.List and individual notes can be up to 10000 chars via the
// HTTP API, so an unbounded join can exceed Telegram's cap — sendMessage
// then returns ok:false, which the poller only logs and swallows, so the
// user would get no reply at all with no visible sign anything went
// wrong. Truncating to whole lines under this budget keeps replies always
// deliverable.
const maxReplyChars = 3900

// notesListCommand replies with every note's content, one per line (most
// recent first, same order GET /api/notes returns), or a placeholder if
// there are none. Reuses notes.Store.List — the same read path the HTTP
// API uses, called directly rather than through an HTTP round-trip since
// both live in the same process.
func notesListCommand(store *notes.Store) telegram.CommandFunc {
	return func(ctx context.Context, _ string) (string, error) {
		all, err := store.List(ctx)
		if err != nil {
			return "", err
		}
		if len(all) == 0 {
			return "no notes yet", nil
		}
		lines := make([]string, len(all))
		for i, n := range all {
			lines[i] = fmt.Sprintf("- %s", n.Content)
		}
		return joinCapped(lines, maxReplyChars), nil
	}
}

// joinCapped joins lines with "\n", but if the full join would exceed
// maxChars, includes only as many whole leading lines as fit and appends
// a trailing "... (N more)" line noting how many were left out — rather
// than silently truncating mid-line or exceeding Telegram's message-length
// cap and getting the whole reply dropped.
func joinCapped(lines []string, maxChars int) string {
	full := strings.Join(lines, "\n")
	if len(full) <= maxChars {
		return full
	}

	var kept []string
	length := 0
	for i, line := range lines {
		// +1 accounts for the "\n" that would join this line to the
		// previous one (not needed before the first line).
		added := len(line)
		if i > 0 {
			added++
		}
		if length+added > maxChars {
			break
		}
		kept = append(kept, line)
		length += added
	}

	remaining := len(lines) - len(kept)
	return strings.Join(kept, "\n") + fmt.Sprintf("\n... (%d more)", remaining)
}

// notesNewCommand creates one note from args and replies with confirmation.
// Reuses notes.Store.InsertBatch — the same create path POST /api/notes
// uses. Empty args (no text after "/newnote ") reply with a usage message
// rather than creating an empty note, mirroring notes.validateContent's
// "content must not be empty" rule without duplicating notes' unexported
// validator.
func notesNewCommand(store *notes.Store) telegram.CommandFunc {
	return func(ctx context.Context, args string) (string, error) {
		content := strings.TrimSpace(args)
		if content == "" {
			return "usage: /newnote <text>", nil
		}
		if _, err := store.InsertBatch(ctx, []string{content}); err != nil {
			return "", err
		}
		return "note added", nil
	}
}
