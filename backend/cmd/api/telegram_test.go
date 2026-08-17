// backend/cmd/api/telegram_test.go
package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devianse/pet-project/backend/internal/db"
	"github.com/devianse/pet-project/backend/internal/notes"
	"github.com/devianse/pet-project/backend/internal/reminders"
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

func TestNotesListCommand_CapsReplyUnderTelegramLimitWithManyNotes(t *testing.T) {
	store := setupNotesStore(t)

	// 40 notes of 200 chars each join to ~8000+ chars, comfortably over
	// maxReplyChars (3900) but with each individual line well under it —
	// so truncation lands on a whole-line boundary rather than dropping
	// everything, exercising the "keep as many whole lines as fit" path.
	contents := make([]string, 40)
	for i := range contents {
		contents[i] = strings.Repeat("x", 200)
	}
	if _, err := store.InsertBatch(context.Background(), contents); err != nil {
		t.Fatalf("seeding notes: %v", err)
	}
	cmd := notesListCommand(store)

	reply, err := cmd(context.Background(), "")
	if err != nil {
		t.Fatalf("notesListCommand: %v", err)
	}
	if len(reply) >= 4096 {
		t.Fatalf("expected reply under telegram's 4096-char cap, got %d chars", len(reply))
	}
	if !strings.Contains(reply, "more") {
		t.Fatalf("expected truncated reply to mention there's more, got %q", reply)
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

func TestHelpCommand_ListsAllRegisteredCommandsViaRouter(t *testing.T) {
	router := telegram.NewRouter()
	router.Handle("/notes", "list all notes", func(_ context.Context, _ string) (string, error) { return "", nil })
	router.Handle("/help", "list available commands", helpCommand(router))

	reply := router.Dispatch(context.Background(), "/help")
	want := "/notes — list all notes\n/help — list available commands"
	if reply != want {
		t.Fatalf("expected %q, got %q", want, reply)
	}
}

func TestNotesNewCommand_BareCommandViaRouter(t *testing.T) {
	store := setupNotesStore(t)
	router := telegram.NewRouter()
	router.Handle("/newnote", "add a new note: /newnote <text>", notesNewCommand(store))

	reply := router.Dispatch(context.Background(), "/newnote")
	if reply != "usage: /newnote <text>" {
		t.Fatalf("bare /newnote should return usage message, got %q", reply)
	}
}

// setupRemindersStore mirrors setupNotesStore — same skip-if-no-
// DATABASE_URL convention.
func setupRemindersStore(t *testing.T) *reminders.Store {
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

	store := reminders.NewStore(conn)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	if _, err := conn.ExecContext(context.Background(), "DELETE FROM reminders"); err != nil {
		t.Fatalf("clearing reminders table: %v", err)
	}
	return store
}

func TestRemindersUpcomingCommand_EmptyStoreRepliesPlaceholder(t *testing.T) {
	store := setupRemindersStore(t)
	cmd := remindersUpcomingCommand(store)

	reply, err := cmd(context.Background(), "")
	if err != nil {
		t.Fatalf("remindersUpcomingCommand: %v", err)
	}
	if reply != "nothing upcoming" {
		t.Fatalf("expected placeholder reply, got %q", reply)
	}
}

func TestRemindersUpcomingCommand_ListsPendingSoonestFirst(t *testing.T) {
	store := setupRemindersStore(t)
	ctx := context.Background()
	now := time.Now()
	if _, err := store.Schedule(ctx, "sub:later", "Grandpa's internet", now.Add(6*24*time.Hour)); err != nil {
		t.Fatalf("seeding later reminder: %v", err)
	}
	if _, err := store.Schedule(ctx, "sub:sooner", "VPS balance top-up", now.Add(2*24*time.Hour)); err != nil {
		t.Fatalf("seeding sooner reminder: %v", err)
	}
	cmd := remindersUpcomingCommand(store)

	reply, err := cmd(ctx, "")
	if err != nil {
		t.Fatalf("remindersUpcomingCommand: %v", err)
	}
	if !strings.HasPrefix(reply, "📌 Upcoming\n- VPS balance top-up") {
		t.Fatalf("expected VPS reminder listed first, got %q", reply)
	}
	if !strings.Contains(reply, "Grandpa's internet") {
		t.Fatalf("expected both reminders listed, got %q", reply)
	}
}

func TestHumanRelative(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		due  time.Time
		want string
	}{
		{"two days out", now.Add(48 * time.Hour), "in 2 days (Aug 19)"},
		{"one day out", now.Add(24 * time.Hour), "in 1 day (Aug 18)"},
		{"due today", now, "today (Aug 17)"},
		{"overdue", now.Add(-time.Hour), "overdue (Aug 17)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := humanRelative(now, c.due); got != c.want {
				t.Fatalf("humanRelative(%v, %v) = %q, want %q", now, c.due, got, c.want)
			}
		})
	}
}
