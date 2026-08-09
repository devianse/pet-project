package watchlist

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/db"
)

// setupStore connects to the local Postgres pointed at by DATABASE_URL,
// ensures the watchlist_items table exists, and clears it so each test
// starts empty. Skipped (not failed) if DATABASE_URL isn't set, matching
// internal/notes' existing test pattern.
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
	if _, err := conn.ExecContext(context.Background(), "DELETE FROM watchlist_items"); err != nil {
		t.Fatalf("clearing watchlist_items table: %v", err)
	}
	return store
}

func testMatch() *TMDbMatch {
	return &TMDbMatch{
		MediaType:   "movie",
		TMDbID:      278,
		Title:       "The Shawshank Redemption",
		ReleaseYear: "1994",
		PosterPath:  "/q6y0Go1tsGEsmtFryDOJo3dEmqu.jpg",
		Overview:    "Framed in the 1940s...",
		VoteAverage: 8.7,
		Genres:      "Drama, Crime",
	}
}

func TestStore_InsertThenList(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	created, err := store.Insert(ctx, "tt0111161", testMatch())
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if created.ImdbID != "tt0111161" || created.Title != "The Shawshank Redemption" {
		t.Fatalf("unexpected created item: %+v", created)
	}
	if created.ReleaseYear == nil || *created.ReleaseYear != "1994" {
		t.Fatalf("expected release year 1994, got %+v", created.ReleaseYear)
	}
	if created.Viewed {
		t.Fatal("expected new item to default to unviewed")
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 item, got %d", len(listed))
	}
}

func TestStore_Insert_RejectsDuplicateImdbID(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	if _, err := store.Insert(ctx, "tt0111161", testMatch()); err != nil {
		t.Fatalf("first Insert: %v", err)
	}

	_, err := store.Insert(ctx, "tt0111161", testMatch())
	if !errors.Is(err, ErrDuplicateImdbID) {
		t.Fatalf("expected ErrDuplicateImdbID, got %v", err)
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected duplicate insert to leave exactly 1 item, got %d", len(listed))
	}
}

func TestStore_SetViewed(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	created, err := store.Insert(ctx, "tt0111161", testMatch())
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	found, err := store.SetViewed(ctx, created.ID, true)
	if err != nil {
		t.Fatalf("SetViewed: %v", err)
	}
	if !found {
		t.Fatal("expected SetViewed to report found=true for an existing id")
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !listed[0].Viewed {
		t.Fatal("expected item to be marked viewed")
	}
}

func TestStore_SetViewed_NotFound(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	found, err := store.SetViewed(ctx, 999999, true)
	if err != nil {
		t.Fatalf("SetViewed: %v", err)
	}
	if found {
		t.Fatal("expected SetViewed to report found=false for a nonexistent id")
	}
}

func TestStore_Delete(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	created, err := store.Insert(ctx, "tt0111161", testMatch())
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	found, err := store.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !found {
		t.Fatal("expected Delete to report found=true for an existing id")
	}

	found, err = store.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete (second time): %v", err)
	}
	if found {
		t.Fatal("expected Delete to report found=false for an already-deleted id")
	}
}
