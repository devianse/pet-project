package main

import (
	"context"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/db"
)

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
	if _, err := conn.ExecContext(context.Background(), "DELETE FROM users"); err != nil {
		t.Fatalf("clearing users table: %v", err)
	}
	return store
}

func TestCreateUser_Success(t *testing.T) {
	store := setupStore(t)

	if err := createUser(context.Background(), store, "cli-mike", "s3cret-pass", "admin"); err != nil {
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
}

func TestCreateUser_RejectsDuplicateUsername(t *testing.T) {
	store := setupStore(t)

	if err := createUser(context.Background(), store, "cli-dupe", "pass-one", "user"); err != nil {
		t.Fatalf("createUser (first): %v", err)
	}
	if err := createUser(context.Background(), store, "cli-dupe", "pass-two", "user"); err == nil {
		t.Fatal("expected an error creating a duplicate username")
	}
}
