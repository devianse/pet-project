package datenight

import (
	"context"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/db"
)

// setupStore connects to the local Postgres, ensures the datenight
// tables exist, and clears them so each test starts empty. Skipped (not
// failed) if DATABASE_URL isn't set — same convention as
// notes/store_test.go.
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
	if err := auth.NewStore(conn).EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure auth schema: %v", err)
	}
	// date_night_proposals FKs to date_night_activities, so clear it
	// first regardless of which test needs which table.
	if _, err := conn.ExecContext(context.Background(), "DELETE FROM date_night_proposals"); err != nil {
		t.Fatalf("clearing date_night_proposals table: %v", err)
	}
	if _, err := conn.ExecContext(context.Background(), "DELETE FROM date_night_activities"); err != nil {
		t.Fatalf("clearing date_night_activities table: %v", err)
	}
	return store
}

func TestStore_CreateActivityThenListActivities(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	desc := "Takeout sushi + whatever's queued up"
	created, err := store.CreateActivity(ctx, "Sushi Movie Night", &desc, CategoryFood)
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}
	if created.Name != "Sushi Movie Night" || created.Category != CategoryFood {
		t.Fatalf("unexpected created activity: %+v", created)
	}
	if created.Description == nil || *created.Description != desc {
		t.Fatalf("expected description to round-trip, got %+v", created.Description)
	}

	if _, err := store.CreateActivity(ctx, "Hiking", nil, CategoryOutdoor); err != nil {
		t.Fatalf("CreateActivity (no description): %v", err)
	}

	listed, err := store.ListActivities(ctx)
	if err != nil {
		t.Fatalf("ListActivities: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 activities, got %d", len(listed))
	}
	// Newest first.
	if listed[0].Name != "Hiking" || listed[1].Name != "Sushi Movie Night" {
		t.Fatalf("expected newest-first order [Hiking, Sushi Movie Night], got %+v", listed)
	}
	if listed[0].Description != nil {
		t.Fatalf("expected nil description for Hiking, got %+v", listed[0].Description)
	}
}

func TestStore_DeleteActivity(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	created, err := store.CreateActivity(ctx, "Temporary", nil, CategoryCozy)
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}

	found, err := store.DeleteActivity(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteActivity: %v", err)
	}
	if !found {
		t.Fatal("expected DeleteActivity to report found=true for an existing id")
	}

	found, err = store.DeleteActivity(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteActivity (second time): %v", err)
	}
	if found {
		t.Fatal("expected DeleteActivity to report found=false for an already-deleted id")
	}
}
