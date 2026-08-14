// Package telegram is the platform's Telegram Bot API integration — v1
// covers inbound commands only (see docs/superpowers/specs/2026-08-14-
// telegram-bot-design.md). Three pieces: Client (this file, talks to
// Telegram), Router (command dispatch), Poller (the long-poll loop that
// ties them together).
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiBaseURL = "https://api.telegram.org"

// Chat identifies a Telegram chat/conversation.
type Chat struct {
	ID int64 `json:"id"`
}

// Message is the subset of Telegram's Message object this package uses.
type Message struct {
	Chat Chat   `json:"chat"`
	Text string `json:"text"`
}

// Update is one item from getUpdates. Message is nil for update types this
// package doesn't handle (edited messages, channel posts, etc.) — callers
// must check for nil before touching it.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// Client is what Poller depends on, so tests can supply a fake instead of
// calling the real Telegram Bot API.
type Client interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
	GetUpdates(ctx context.Context, offset int64, timeoutSeconds int) ([]Update, error)
}

// RealClient calls the actual Telegram Bot API.
type RealClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewRealClient builds a client for the real Telegram Bot API using token
// (from @BotFather).
func NewRealClient(token string) *RealClient {
	return newRealClient(apiBaseURL, token)
}

// newRealClient takes an explicit baseURL so tests can point it at an
// httptest.Server instead of the real api.telegram.org.
func newRealClient(baseURL, token string) *RealClient {
	return &RealClient{
		// getUpdates' own timeoutSeconds param can run up to Telegram's
		// ~50s long-poll cap, so the HTTP client's timeout must
		// comfortably exceed the longest timeoutSeconds this package
		// ever passes (see poller.go's longPollTimeoutSeconds).
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    baseURL,
		token:      token,
	}
}

type sendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

// apiEnvelope is the {"ok": ..., "result": ..., "description": ...} shape
// every Telegram Bot API response uses.
type apiEnvelope struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

// SendMessage posts text to chatID via Telegram's sendMessage endpoint.
func (c *RealClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	body, err := json.Marshal(sendMessageRequest{ChatID: chatID, Text: text})
	if err != nil {
		return err
	}
	_, err = c.post(ctx, "/sendMessage", body)
	return err
}

// botCommand is one entry in the setMyCommands payload. Telegram requires
// command (its "/" stripped) to be lowercase and command_scope-unique;
// v1's commands already satisfy that so no validation is done here.
type botCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// SetMyCommands populates Telegram's native command menu (the "/" button
// next to the message box, and autocomplete suggestions) via the
// setMyCommands endpoint. Intended to be called once at startup, after
// every command is registered on Router — see cmd/api/telegram.go.
func (c *RealClient) SetMyCommands(ctx context.Context, commands []Command) error {
	botCommands := make([]botCommand, len(commands))
	for i, cmd := range commands {
		// Telegram's command field excludes the leading "/" and any
		// trailing argument placeholder — Router's prefixes carry both
		// (e.g. "/newnote ", with a trailing space commands like /notes
		// don't have), so both are stripped here rather than asking
		// Router to store two forms of the same prefix.
		botCommands[i] = botCommand{
			Command:     strings.TrimSpace(strings.TrimPrefix(cmd.Prefix, "/")),
			Description: cmd.Description,
		}
	}
	body, err := json.Marshal(map[string]any{"commands": botCommands})
	if err != nil {
		return err
	}
	_, err = c.post(ctx, "/setMyCommands", body)
	return err
}

// GetUpdates long-polls Telegram's getUpdates endpoint, blocking up to
// timeoutSeconds server-side for a new update. offset should be one past
// the highest update_id already processed, so Telegram doesn't redeliver
// it.
func (c *RealClient) GetUpdates(ctx context.Context, offset int64, timeoutSeconds int) ([]Update, error) {
	body, err := json.Marshal(map[string]any{"offset": offset, "timeout": timeoutSeconds})
	if err != nil {
		return nil, err
	}
	result, err := c.post(ctx, "/getUpdates", body)
	if err != nil {
		return nil, err
	}

	// Decode the array one element at a time rather than in one
	// json.Unmarshal call: if a single element fails to decode (a type
	// mismatch on some field), decoding the whole array in one shot would
	// fail the entire batch. Poller.Run would then retry with the same
	// offset, refetching that same malformed element forever.
	//
	// So each element is decoded individually. If full decode fails, we
	// still attempt a lightweight decode of just update_id (the one field
	// unlikely to be the cause of a type mismatch on "some field") and
	// return an Update carrying only that ID with a nil Message — Poller
	// already treats a nil Message as a no-op, but still advances its
	// offset past every update in the returned slice, so this is enough
	// to stop the malformed element from being refetched forever without
	// any change to poller.go. Only if even that minimal parse fails do
	// we drop the element entirely (offset can't safely advance past an
	// update whose ID is unknown).
	var raw []json.RawMessage
	if err := json.Unmarshal(result, &raw); err != nil {
		return nil, fmt.Errorf("decoding telegram updates: %w", err)
	}

	updates := make([]Update, 0, len(raw))
	for _, r := range raw {
		var u Update
		if err := json.Unmarshal(r, &u); err != nil {
			var idOnly struct {
				UpdateID int64 `json:"update_id"`
			}
			if idErr := json.Unmarshal(r, &idOnly); idErr != nil {
				slog.Error("telegram: dropping unparseable update, offset cannot advance past it", "error", err, "raw", string(r))
				continue
			}
			slog.Error("telegram: skipping malformed update", "error", err, "update_id", idOnly.UpdateID, "raw", string(r))
			updates = append(updates, Update{UpdateID: idOnly.UpdateID})
			continue
		}
		updates = append(updates, u)
	}
	return updates, nil
}

func (c *RealClient) post(ctx context.Context, path string, body []byte) (json.RawMessage, error) {
	reqURL := fmt.Sprintf("%s/bot%s%s", c.baseURL, c.token, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// *url.Error's Error() string embeds the full request URL, which
		// contains the bot token — unwrap to the inner error so the
		// token never reaches logs (this error flows straight into
		// poller.go's slog.Error on every network blip).
		var ue *url.Error
		if errors.As(err, &ue) {
			return nil, fmt.Errorf("calling telegram %s: %w", path, ue.Err)
		}
		return nil, fmt.Errorf("calling telegram %s: %w", path, err)
	}
	defer resp.Body.Close()

	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("decoding telegram response from %s: %w", path, err)
	}
	if !env.OK {
		return nil, fmt.Errorf("telegram API error from %s: %s", path, env.Description)
	}
	return env.Result, nil
}
