package notes

import (
	"context"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/db"
)

// setupStore connects to the local Postgres started for this plan's
// Task 1, ensures the notes table exists, and clears it so each test
// starts from an empty table. Tests are skipped (not failed) if
// DATABASE_URL isn't set, so `go test ./...` doesn't break when nobody
// has started the local Postgres container.
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
	if _, err := conn.ExecContext(context.Background(), "DELETE FROM notes"); err != nil {
		t.Fatalf("clearing notes table: %v", err)
	}
	return store
}

func TestStore_InsertBatchThenList(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	created, err := store.InsertBatch(ctx, []string{"buy milk", "walk the dog"})
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(created))
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 notes from List, got %d", len(listed))
	}
	// Both rows share one transaction's created_at (Postgres now() is
	// transaction-scoped), so id DESC is the real tiebreaker for
	// "newest-first" within one batch — the later item in the batch
	// gets the higher id and should list first.
	if listed[0].Content != "walk the dog" || listed[1].Content != "buy milk" {
		t.Fatalf("expected newest-first order [walk the dog, buy milk], got %+v", listed)
	}
}

func TestStore_Delete(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	created, err := store.InsertBatch(ctx, []string{"temporary note"})
	if err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}
	id := created[0].ID

	found, err := store.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !found {
		t.Fatal("expected Delete to report found=true for an existing id")
	}

	found, err = store.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete (second time): %v", err)
	}
	if found {
		t.Fatal("expected Delete to report found=false for an already-deleted id")
	}
}
