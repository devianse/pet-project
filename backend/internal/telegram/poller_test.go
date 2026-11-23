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
	router.Handle("/ping", "ping", func(_ context.Context, _ string) (string, error) { return "pong", nil })

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
	router.Handle("/ping", "ping", func(_ context.Context, _ string) (string, error) { return "pong", nil })

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
	router.Handle("/ping", "ping", func(_ context.Context, _ string) (string, error) { return "pong", nil })

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

// offsetRecorder records the offset each GetUpdates call was made with,
// which is the only way to observe that SkipBacklog actually advanced the
// poller's in-memory offset.
type offsetRecorder struct {
	mu       sync.Mutex
	offsets  []int64
	backlog  []Update
	cancel   context.CancelFunc
	sentText []string
}

func (o *offsetRecorder) GetUpdates(_ context.Context, offset int64, _ int) ([]Update, error) {
	o.mu.Lock()
	o.offsets = append(o.offsets, offset)
	n := len(o.offsets)
	o.mu.Unlock()
	if n == 1 {
		return o.backlog, nil
	}
	if o.cancel != nil {
		o.cancel()
	}
	return nil, nil
}

func (o *offsetRecorder) SendMessage(_ context.Context, _ int64, text string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sentText = append(o.sentText, text)
	return nil
}

// Without this, every restart re-ran the backlog: /newnote is a write,
// so a deploy duplicated notes typed since the last restart.
func TestPoller_SkipBacklog_AdvancesOffsetWithoutDispatching(t *testing.T) {
	rec := &offsetRecorder{backlog: []Update{
		{UpdateID: 5, Message: &Message{Chat: Chat{ID: 555}, Text: "/ping"}},
		{UpdateID: 6, Message: &Message{Chat: Chat{ID: 555}, Text: "/ping"}},
	}}
	router := NewRouter()
	router.Handle("/ping", "ping", func(_ context.Context, _ string) (string, error) { return "pong", nil })

	p := NewPoller(rec, 555, router)
	if err := p.SkipBacklog(context.Background()); err != nil {
		t.Fatalf("SkipBacklog: %v", err)
	}
	rec.mu.Lock()
	sent := append([]string(nil), rec.sentText...)
	rec.mu.Unlock()
	if len(sent) != 0 {
		t.Fatalf("SkipBacklog must discard, not dispatch, got replies %v", sent)
	}

	ctx, cancel := context.WithCancel(context.Background())
	rec.cancel = cancel
	p.Run(ctx)

	rec.mu.Lock()
	offsets := append([]int64(nil), rec.offsets...)
	rec.mu.Unlock()
	if len(offsets) < 2 {
		t.Fatalf("expected a SkipBacklog call plus at least one Run call, got %v", offsets)
	}
	if offsets[0] != -1 {
		t.Fatalf("expected SkipBacklog to request offset -1, got %d", offsets[0])
	}
	if last := offsets[len(offsets)-1]; last != 7 {
		t.Fatalf("expected Run to resume at offset 7 (past the backlog), got %d", last)
	}
}

func TestPoller_SkipBacklog_NoBacklogLeavesOffsetAtZero(t *testing.T) {
	rec := &offsetRecorder{}
	p := NewPoller(rec, 555, NewRouter())
	if err := p.SkipBacklog(context.Background()); err != nil {
		t.Fatalf("SkipBacklog: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rec.cancel = cancel
	p.Run(ctx)
	rec.mu.Lock()
	offsets := append([]int64(nil), rec.offsets...)
	rec.mu.Unlock()
	if len(offsets) >= 2 && offsets[1] != 0 {
		t.Fatalf("expected offset 0 with an empty backlog, got %d", offsets[1])
	}
}
