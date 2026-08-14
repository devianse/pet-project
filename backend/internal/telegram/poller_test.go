// backend/internal/telegram/poller_test.go
package telegram

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeClient is a test double for Client. getUpdatesFunc is called on
// every GetUpdates invocation, letting each test script exactly what
// happens on each poll iteration (including cancelling ctx to stop Run).
type fakeClient struct {
	mu             sync.Mutex
	getUpdatesFunc func(callN int) ([]Update, error)
	callN          int
	sentMessages   []sentMessage
}

type sentMessage struct {
	chatID int64
	text   string
}

func (f *fakeClient) GetUpdates(_ context.Context, _ int64, _ int) ([]Update, error) {
	f.mu.Lock()
	n := f.callN
	f.callN++
	f.mu.Unlock()
	return f.getUpdatesFunc(n)
}

func (f *fakeClient) SendMessage(_ context.Context, chatID int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sentMessages = append(f.sentMessages, sentMessage{chatID: chatID, text: text})
	return nil
}

func TestPoller_Run_DispatchesFromAllowedChatAndReplies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fc := &fakeClient{}
	fc.getUpdatesFunc = func(n int) ([]Update, error) {
		if n == 0 {
			return []Update{{UpdateID: 1, Message: &Message{Chat: Chat{ID: 555}, Text: "/ping"}}}, nil
		}
		cancel()
		return nil, nil
	}

	router := NewRouter()
	router.Handle("/ping", func(_ context.Context, _ string) (string, error) { return "pong", nil })

	p := NewPoller(fc, 555, router)
	p.Run(ctx)

	if len(fc.sentMessages) != 1 || fc.sentMessages[0] != (sentMessage{chatID: 555, text: "pong"}) {
		t.Fatalf("expected one reply {555, pong}, got %+v", fc.sentMessages)
	}
}

func TestPoller_Run_DropsMessageFromUnrecognizedChatSilently(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fc := &fakeClient{}
	fc.getUpdatesFunc = func(n int) ([]Update, error) {
		if n == 0 {
			return []Update{{UpdateID: 1, Message: &Message{Chat: Chat{ID: 999}, Text: "/ping"}}}, nil
		}
		cancel()
		return nil, nil
	}

	router := NewRouter()
	router.Handle("/ping", func(_ context.Context, _ string) (string, error) { return "pong", nil })

	p := NewPoller(fc, 555, router)
	p.Run(ctx)

	if len(fc.sentMessages) != 0 {
		t.Fatalf("expected no reply sent for unrecognized chat, got %+v", fc.sentMessages)
	}
}

func TestPoller_Run_AdvancesOffsetPastEveryUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fc := &fakeClient{}
	fc.getUpdatesFunc = func(n int) ([]Update, error) {
		if n == 0 {
			return []Update{
				{UpdateID: 10, Message: &Message{Chat: Chat{ID: 999}, Text: "/ping"}}, // wrong chat, still advances offset
				{UpdateID: 11, Message: &Message{Chat: Chat{ID: 555}, Text: "/ping"}},
			}, nil
		}
		cancel()
		return nil, nil
	}

	router := NewRouter()
	router.Handle("/ping", func(_ context.Context, _ string) (string, error) { return "pong", nil })

	p := NewPoller(fc, 555, router)

	// p.offset is unexported, asserted directly since this test lives in
	// the same package — it's the only observable proof that an update
	// from an unrecognized chat still gets marked processed.
	p.Run(ctx)

	if p.offset != 12 {
		t.Fatalf("expected offset advanced to 12 (past update_id 11), got %d", p.offset)
	}
}

func TestPoller_Run_RetriesWithBackoffOnGetUpdatesError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fc := &fakeClient{}
	calls := 0
	fc.getUpdatesFunc = func(n int) ([]Update, error) {
		calls++
		if n == 0 {
			return nil, errors.New("network blip")
		}
		cancel()
		return nil, nil
	}

	p := NewPoller(fc, 555, NewRouter())
	originalBackoff := initialBackoff
	t.Cleanup(func() {
		initialBackoff = originalBackoff
	})
	initialBackoff = time.Millisecond // override package var for a fast test; see poller.go
	p.Run(ctx)

	if calls < 2 {
		t.Fatalf("expected Run to retry after an error, got %d call(s)", calls)
	}
}
