package main

import (
	"context"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/db"
)

// setupStore clears only this package's own fixture usernames rather
// than the whole users table — internal/auth's tests share the same
// DATABASE_URL/table, and go test runs different packages' binaries
// concurrently by default, so an unscoped DELETE here would race with
// internal/auth's tests regardless of whether the fixture usernames
// themselves collide.
func setupStore(t *testing.T) *auth.Store {
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

	store := auth.NewStore(conn)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	if _, err := conn.ExecContext(context.Background(),
		"DELETE FROM users WHERE username IN ('cli-mike', 'cli-dupe', 'cli-ada')"); err != nil {
		t.Fatalf("clearing users table: %v", err)
	}
	return store
}

func TestCreateUser_Success(t *testing.T) {
	store := setupStore(t)

	if err := createUser(context.Background(), store, "cli-mike", "s3cret-pass", "admin", ""); err != nil {
		t.Fatalf("createUser: %v", err)
	}

	found, err := store.FindByUsername(context.Background(), "cli-mike")
	if err != nil {
		t.Fatalf("FindByUsername: %v", err)
	}
	if found == nil || found.Role != "admin" {
		t.Fatalf("expected cli-mike to exist with role admin, got %+v", found)
	}
	if !auth.VerifyPassword(found.PasswordHash, "s3cret-pass") {
		t.Fatal("expected the stored hash to verify against the original password")
	}
	if found.DisplayName != nil {
		t.Fatalf("expected no display_name when -display-name is omitted, got %v", *found.DisplayName)
	}
}

func TestCreateUser_WithDisplayName(t *testing.T) {
	store := setupStore(t)

	if err := createUser(context.Background(), store, "cli-ada", "s3cret-pass", "user", "  Ada  "); err != nil {
		t.Fatalf("createUser: %v", err)
	}

	found, err := store.FindByUsername(context.Background(), "cli-ada")
	if err != nil {
		t.Fatalf("FindByUsername: %v", err)
	}
	if found == nil || found.DisplayName == nil || *found.DisplayName != "Ada" {
		t.Fatalf("expected display_name %q (trimmed), got %+v", "Ada", found)
	}
}

func TestCreateUser_RejectsDuplicateUsername(t *testing.T) {
	store := setupStore(t)

	if err := createUser(context.Background(), store, "cli-dupe", "pass-one", "user", ""); err != nil {
		t.Fatalf("createUser (first): %v", err)
	}
	if err := createUser(context.Background(), store, "cli-dupe", "pass-two", "user", ""); err == nil {
		t.Fatal("expected an error creating a duplicate username")
	}
}
