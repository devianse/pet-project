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
	"fmt"
	"net/http"
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
	var updates []Update
	if err := json.Unmarshal(result, &updates); err != nil {
		return nil, fmt.Errorf("decoding telegram updates: %w", err)
	}
	return updates, nil
}

func (c *RealClient) post(ctx context.Context, path string, body []byte) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/bot%s%s", c.baseURL, c.token, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
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
