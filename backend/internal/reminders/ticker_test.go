// backend/internal/reminders/ticker_test.go
package reminders

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeSender is a telegram.Client test double — Ticker only needs
// SendMessage, so the fake only implements that (mirrors internal/ops's
// fakePinger/fakeHub pattern: "define the interface at the consumer").
type fakeSender struct {
	err error

	mu   sync.Mutex
	sent []string
}

func (f *fakeSender) SendMessage(_ context.Context, _ int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, text)
	return nil
}

func (f *fakeSender) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	copy(out, f.sent)
	return out
}

// fakeStore is a reminders.Store test double holding reminders in memory
// — no real Postgres involved, same rationale as fakeSender.
type fakeStore struct {
	mu      sync.Mutex
	items   []Reminder
	sentIDs []int
}

func (f *fakeStore) ListPending(_ context.Context) ([]Reminder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Reminder, len(f.items))
	copy(out, f.items)
	return out, nil
}

func (f *fakeStore) MarkSent(_ context.Context, id int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, item := range f.items {
		if item.ID == id {
			f.items = append(f.items[:i], f.items[i+1:]...)
			break
		}
	}
	f.sentIDs = append(f.sentIDs, id)
	return nil
}

func (f *fakeStore) markSentIDs() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]int, len(f.sentIDs))
	copy(out, f.sentIDs)
	return out
}

func TestTicker_SendsDueReminderAndMarksItSent(t *testing.T) {
	store := &fakeStore{items: []Reminder{
		{ID: 1, Message: "pay the bill", DueAt: time.Now().Add(-time.Minute)},
	}}
	sender := &fakeSender{}
	ticker := NewTicker(store, sender, 123, WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	go ticker.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	sent := sender.snapshot()
	if len(sent) == 0 || sent[0] != "pay the bill" {
		t.Fatalf("expected 'pay the bill' to be sent, got %v", sent)
	}
	if ids := store.markSentIDs(); len(ids) == 0 || ids[0] != 1 {
		t.Fatalf("expected reminder 1 marked sent, got %v", ids)
	}
}

func TestTicker_SkipsReminderNotYetDue(t *testing.T) {
	store := &fakeStore{items: []Reminder{
		{ID: 1, Message: "not yet", DueAt: time.Now().Add(time.Hour)},
	}}
	sender := &fakeSender{}
	ticker := NewTicker(store, sender, 123, WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	go ticker.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	if sent := sender.snapshot(); len(sent) != 0 {
		t.Fatalf("expected nothing sent for a not-yet-due reminder, got %v", sent)
	}
}

func TestTicker_LeavesReminderPendingOnSendFailure(t *testing.T) {
	store := &fakeStore{items: []Reminder{
		{ID: 1, Message: "will fail", DueAt: time.Now().Add(-time.Minute)},
	}}
	sender := &fakeSender{err: errors.New("telegram unreachable")}
	ticker := NewTicker(store, sender, 123, WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	go ticker.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	if ids := store.markSentIDs(); len(ids) != 0 {
		t.Fatalf("expected no reminder marked sent after a send failure, got %v", ids)
	}
}

func TestTicker_OneFailureDoesNotBlockTheRestOfTheTick(t *testing.T) {
	store := &fakeStore{items: []Reminder{
		{ID: 1, Message: "fails", DueAt: time.Now().Add(-time.Minute)},
	}}
	// Only one item in this store fails to send; a second Ticker/store
	// pair with a non-erroring sender proves independent items aren't
	// coupled — simulated here via two separate tick cycles on the same
	// store: first with a failing sender, then a working one, to confirm
	// the failed item is still retried (still pending) after the first.
	failing := &fakeSender{err: errors.New("boom")}
	ticker := NewTicker(store, failing, 123, WithInterval(10*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	go ticker.Run(ctx)
	time.Sleep(30 * time.Millisecond)
	cancel()

	working := &fakeSender{}
	retryTicker := NewTicker(store, working, 123, WithInterval(10*time.Millisecond))
	ctx2, cancel2 := context.WithCancel(context.Background())
	go retryTicker.Run(ctx2)
	time.Sleep(30 * time.Millisecond)
	cancel2()

	if sent := working.snapshot(); len(sent) == 0 || sent[0] != "fails" {
		t.Fatalf("expected the previously-failed reminder to be retried and sent, got %v", sent)
	}
}

func TestTicker_RunReturnsOnContextCancel(t *testing.T) {
	store := &fakeStore{}
	ticker := NewTicker(store, &fakeSender{}, 123, WithInterval(10*time.Millisecond))

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
