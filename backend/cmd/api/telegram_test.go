// backend/cmd/api/telegram_test.go
package main

import (
	"context"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/db"
	"github.com/devianse/pet-project/backend/internal/notes"
	"github.com/devianse/pet-project/backend/internal/telegram"
)

// setupNotesStore mirrors notes/store_test.go's setupStore — same
// skip-if-no-DATABASE_URL convention, duplicated here rather than
// exported from the notes package since it's test-only plumbing.
func setupNotesStore(t *testing.T) *notes.Store {
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

	store := notes.NewStore(conn)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	if _, err := conn.ExecContext(context.Background(), "DELETE FROM notes"); err != nil {
		t.Fatalf("clearing notes table: %v", err)
	}
	return store
}

func TestNotesListCommand_EmptyStoreRepliesPlaceholder(t *testing.T) {
	store := setupNotesStore(t)
	cmd := notesListCommand(store)

	reply, err := cmd(context.Background(), "")
	if err != nil {
		t.Fatalf("notesListCommand: %v", err)
	}
	if reply != "no notes yet" {
		t.Fatalf("expected placeholder reply, got %q", reply)
	}
}

func TestNotesListCommand_ListsExistingNotes(t *testing.T) {
	store := setupNotesStore(t)
	if _, err := store.InsertBatch(context.Background(), []string{"buy milk", "walk dog"}); err != nil {
		t.Fatalf("seeding notes: %v", err)
	}
	cmd := notesListCommand(store)

	reply, err := cmd(context.Background(), "")
	if err != nil {
		t.Fatalf("notesListCommand: %v", err)
	}
	if reply != "- walk dog\n- buy milk" {
		t.Fatalf("unexpected reply: %q", reply)
	}
}

func TestNotesNewCommand_CreatesNoteFromArgs(t *testing.T) {
	store := setupNotesStore(t)
	cmd := notesNewCommand(store)

	reply, err := cmd(context.Background(), "buy milk")
	if err != nil {
		t.Fatalf("notesNewCommand: %v", err)
	}
	if reply != "note added" {
		t.Fatalf("unexpected reply: %q", reply)
	}

	all, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("listing notes: %v", err)
	}
	if len(all) != 1 || all[0].Content != "buy milk" {
		t.Fatalf("expected one note %q, got %+v", "buy milk", all)
	}
}

func TestNotesNewCommand_EmptyArgsRepliesUsage(t *testing.T) {
	store := setupNotesStore(t)
	cmd := notesNewCommand(store)

	reply, err := cmd(context.Background(), "   ")
	if err != nil {
		t.Fatalf("notesNewCommand: %v", err)
	}
	if reply != "usage: /newnote <text>" {
		t.Fatalf("unexpected reply: %q", reply)
	}

	all, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("listing notes: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected no note created for empty args, got %+v", all)
	}
}

func TestNotesNewCommand_BareCommandViaRouter(t *testing.T) {
	store := setupNotesStore(t)
	router := telegram.NewRouter()
	router.Handle("/newnote", notesNewCommand(store))

	reply := router.Dispatch(context.Background(), "/newnote")
	if reply != "usage: /newnote <text>" {
		t.Fatalf("bare /newnote should return usage message, got %q", reply)
	}
}
