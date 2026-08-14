// backend/internal/telegram/router.go
package telegram

import (
	"context"
	"log/slog"
	"strings"
)

// CommandFunc handles one matched command. args is the message text with
// the matched prefix trimmed off. Returning an error logs it and replies
// with a generic message — command handlers should log their own
// context-specific detail before returning, the same convention this
// codebase's HTTP handlers already follow (e.g. notes.Handler.List).
type CommandFunc func(ctx context.Context, args string) (reply string, err error)

// unknownCommandReply is sent when a recognized chat sends text that
// doesn't match any registered command prefix — distinct from the
// silent-drop behavior for an unrecognized chat ID, which Poller handles
// before text ever reaches Dispatch.
const unknownCommandReply = "unknown command"

type registeredCommand struct {
	prefix  string
	handler CommandFunc
}

// Router dispatches incoming message text to a registered command handler
// by prefix match — no argument-parsing library needed at v1's size.
type Router struct {
	commands []registeredCommand
}

func NewRouter() *Router {
	return &Router{}
}

// Handle registers fn to run for any message text starting with prefix.
// If two registered prefixes both match a given text, the one registered
// first wins — none of v1's commands overlap, so this hasn't come up in
// practice.
func (r *Router) Handle(prefix string, fn CommandFunc) {
	r.commands = append(r.commands, registeredCommand{prefix: prefix, handler: fn})
}

// Dispatch runs the first matching command handler for text and returns
// its reply, or unknownCommandReply if nothing matches.
func (r *Router) Dispatch(ctx context.Context, text string) string {
	for _, c := range r.commands {
		if !strings.HasPrefix(text, c.prefix) {
			continue
		}
		reply, err := c.handler(ctx, strings.TrimPrefix(text, c.prefix))
		if err != nil {
			slog.Error("telegram command handler failed", "prefix", c.prefix, "error", err)
			return "error handling command"
		}
		return reply
	}
	return unknownCommandReply
}
