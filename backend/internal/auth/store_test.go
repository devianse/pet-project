package auth

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/db"
)

// setupStore connects to the local Postgres pointed at by DATABASE_URL,
// ensures the users table exists, and clears this package's own fixture
// rows so each test starts from a known state. Skipped (not failed) if
// DATABASE_URL isn't set, matching internal/notes' existing test pattern.
//
// The users table is also touched by cmd/createuser's tests against the
// same DATABASE_URL, and `go test ./...` runs different packages'
// binaries concurrently by default — an unscoped `DELETE FROM users`
// here would wipe rows cmd/createuser's tests just inserted (and vice
// versa) regardless of whether the fixture usernames themselves
// collide. Deleting only this package's own known fixture usernames
// keeps the two packages' tests from ever touching each other's rows.
func setupStore(t *testing.T) (*Store, *sql.DB) {
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
	if _, err := conn.ExecContext(context.Background(),
		"DELETE FROM users WHERE username IN ('mike', 'alice', 'bob', 'dupe')"); err != nil {
		t.Fatalf("clearing users table: %v", err)
	}
	return store, conn
}

func TestStore_CreateUserThenFindByUsername(t *testing.T) {
	store, _ := setupStore(t)
	ctx := context.Background()

	created, err := store.CreateUser(ctx, "mike", "hashed-password", "admin")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.Username != "mike" || created.Role != "admin" {
		t.Fatalf("unexpected created user: %+v", created)
	}
	if created.DisplayName != nil {
		t.Fatalf("expected nil display_name for a freshly created user, got %v", *created.DisplayName)
	}

	found, err := store.FindByUsername(ctx, "mike")
	if err != nil {
		t.Fatalf("FindByUsername: %v", err)
	}
	if found == nil || found.ID != created.ID {
		t.Fatalf("expected to find the created user, got %+v", found)
	}
}

func TestStore_FindByUsername_NotFound(t *testing.T) {
	store, _ := setupStore(t)

	found, err := store.FindByUsername(context.Background(), "nobody")
	if err != nil {
		t.Fatalf("FindByUsername: %v", err)
	}
	if found != nil {
		t.Fatalf("expected nil for an unknown username, got %+v", found)
	}
}

func TestStore_FindByID(t *testing.T) {
	store, _ := setupStore(t)
	ctx := context.Background()

	created, err := store.CreateUser(ctx, "alice", "hashed-password", "user")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	found, err := store.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil || found.Username != "alice" {
		t.Fatalf("expected to find alice by id, got %+v", found)
	}
}

func TestStore_UpdateLastLogin(t *testing.T) {
	store, _ := setupStore(t)
	ctx := context.Background()

	created, err := store.CreateUser(ctx, "bob", "hashed-password", "user")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.LastLoginAt != nil {
		t.Fatalf("expected nil last_login_at for a freshly created user, got %v", *created.LastLoginAt)
	}

	if err := store.UpdateLastLogin(ctx, created.ID); err != nil {
		t.Fatalf("UpdateLastLogin: %v", err)
	}

	found, err := store.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.LastLoginAt == nil {
		t.Fatal("expected last_login_at to be set after UpdateLastLogin")
	}
}

func TestStore_CreateUser_RejectsDuplicateUsername(t *testing.T) {
	store, _ := setupStore(t)
	ctx := context.Background()

	if _, err := store.CreateUser(ctx, "dupe", "hashed-password", "user"); err != nil {
		t.Fatalf("CreateUser (first): %v", err)
	}
	if _, err := store.CreateUser(ctx, "dupe", "another-hash", "user"); err == nil {
		t.Fatal("expected an error creating a user with a duplicate username")
	}
}
