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
func startTelegramBot(logger *slog.Logger, notesStore *notes.Store) {
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
	router.Handle("/notes", notesListCommand(notesStore))
	router.Handle("/newnote ", notesNewCommand(notesStore))

	client := telegram.NewRealClient(token)
	poller := telegram.NewPoller(client, chatID, router)
	go poller.Run(context.Background())
	logger.Info("telegram bot poller started")
}

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
		return strings.Join(lines, "\n"), nil
	}
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
