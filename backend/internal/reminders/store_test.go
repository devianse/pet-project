package reminders

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/devianse/pet-project/backend/internal/db"
)

// setupStore mirrors internal/notes/store_test.go's setupStore — same
// skip-if-no-DATABASE_URL convention, duplicated here since it's
// test-only plumbing each package owns independently.
func setupStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping test that needs a real Postgres instance")
	}

	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	store := NewStore(conn)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	if _, err := conn.ExecContext(context.Background(), "DELETE FROM reminders"); err != nil {
		t.Fatalf("clearing reminders table: %v", err)
	}
	return store
}

func TestStore_ScheduleThenListPending(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	due := time.Now().Add(24 * time.Hour).Truncate(time.Second)

	id, err := store.Schedule(ctx, "sub:1", "Netflix renews tomorrow", due)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if id == 0 {
		t.Fatal("expected a non-zero id")
	}

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending reminder, got %d", len(pending))
	}
	got := pending[0]
	if got.ID != id || got.Source != "sub:1" || got.Message != "Netflix renews tomorrow" || got.Status != "pending" {
		t.Fatalf("unexpected reminder: %+v", got)
	}
	if !got.DueAt.Equal(due) {
		t.Fatalf("expected due_at %v, got %v", due, got.DueAt)
	}
}

func TestStore_ListPendingOrdersBySoonestDueFirst(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	if _, err := store.Schedule(ctx, "sub:later", "later", now.Add(48*time.Hour)); err != nil {
		t.Fatalf("Schedule later: %v", err)
	}
	if _, err := store.Schedule(ctx, "sub:sooner", "sooner", now.Add(24*time.Hour)); err != nil {
		t.Fatalf("Schedule sooner: %v", err)
	}

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending reminders, got %d", len(pending))
	}
	if pending[0].Source != "sub:sooner" || pending[1].Source != "sub:later" {
		t.Fatalf("expected sooner-first order, got %+v", pending)
	}
}

func TestStore_CancelStopsReminderFromListingAsPending(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	if _, err := store.Schedule(ctx, "sub:1", "msg", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if err := store.Cancel(ctx, "sub:1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after cancel, got %d", len(pending))
	}
}

func TestStore_CancelOnUnknownSourceIsNoOp(t *testing.T) {
	store := setupStore(t)
	if err := store.Cancel(context.Background(), "does-not-exist"); err != nil {
		t.Fatalf("expected Cancel on unknown source to be a no-op, got error: %v", err)
	}
}

func TestStore_RescheduleUpdatesDueAt(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	newDue := time.Now().Add(72 * time.Hour).Truncate(time.Second)

	if _, err := store.Schedule(ctx, "sub:1", "msg", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if err := store.Reschedule(ctx, "sub:1", newDue); err != nil {
		t.Fatalf("Reschedule: %v", err)
	}

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 1 || !pending[0].DueAt.Equal(newDue) {
		t.Fatalf("expected rescheduled due_at %v, got %+v", newDue, pending)
	}
}

func TestStore_RescheduleOnUnknownSourceIsNoOp(t *testing.T) {
	store := setupStore(t)
	if err := store.Reschedule(context.Background(), "does-not-exist", time.Now()); err != nil {
		t.Fatalf("expected Reschedule on unknown source to be a no-op, got error: %v", err)
	}
}

func TestStore_MarkSentRemovesFromListPending(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	id, err := store.Schedule(ctx, "sub:1", "msg", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if err := store.MarkSent(ctx, id); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after MarkSent, got %d", len(pending))
	}
}
