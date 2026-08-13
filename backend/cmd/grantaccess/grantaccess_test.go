// backend/cmd/grantaccess/grantaccess_test.go
package main

import (
	"context"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/access"
	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/db"
)

// Fixture usernames used by this file: grantaccess-mike, grantaccess-admin
// — distinct from every other package's fixtures.
func setupStores(t *testing.T) (*auth.Store, *access.Store) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	conn, err := db.Open(databaseURL)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	authStore := auth.NewStore(conn)
	if err := authStore.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensuring auth schema: %v", err)
	}
	accessStore := access.NewStore(conn)
	if err := accessStore.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensuring access schema: %v", err)
	}

	usernames := []string{"grantaccess-mike", "grantaccess-admin"}
	cleanup := func() {
		for _, u := range usernames {
			conn.Exec(`DELETE FROM feature_access WHERE user_id IN (SELECT id FROM users WHERE username = $1)`, u)
			conn.Exec(`DELETE FROM users WHERE username = $1`, u)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	return authStore, accessStore
}

func TestValidateMode(t *testing.T) {
	cases := []struct {
		name    string
		grant   string
		revoke  string
		list    bool
		wantErr bool
	}{
		{"grant only", "notes", "", false, false},
		{"revoke only", "", "notes", false, false},
		{"list only", "", "", true, false},
		{"none set", "", "", false, true},
		{"grant and revoke", "notes", "watchlist", false, true},
		{"grant and list", "notes", "", true, true},
		{"all three", "notes", "watchlist", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMode(tc.grant, tc.revoke, tc.list)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestRun_GrantThenList(t *testing.T) {
	authStore, accessStore := setupStores(t)
	ctx := context.Background()
	if _, err := authStore.CreateUser(ctx, "grantaccess-mike", "password123", "user"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	out, err := run(ctx, authStore, accessStore, "grantaccess-mike", "notes", "", false)
	if err != nil {
		t.Fatalf("run (grant): %v", err)
	}
	if out != `granted "notes" to "grantaccess-mike"` {
		t.Fatalf("unexpected grant output: %q", out)
	}

	out, err = run(ctx, authStore, accessStore, "grantaccess-mike", "", "", true)
	if err != nil {
		t.Fatalf("run (list): %v", err)
	}
	want := `"grantaccess-mike" (role user): notes`
	if out != want {
		t.Fatalf("expected %q, got %q", want, out)
	}
}

func TestRun_RevokeUngrantedFeature(t *testing.T) {
	authStore, accessStore := setupStores(t)
	ctx := context.Background()
	if _, err := authStore.CreateUser(ctx, "grantaccess-mike", "password123", "user"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	out, err := run(ctx, authStore, accessStore, "grantaccess-mike", "", "notes", false)
	if err != nil {
		t.Fatalf("run (revoke): %v", err)
	}
	want := `"grantaccess-mike" did not have "notes" granted`
	if out != want {
		t.Fatalf("expected %q, got %q", want, out)
	}
}

func TestRun_RejectsUnknownFeatureKey(t *testing.T) {
	authStore, accessStore := setupStores(t)
	ctx := context.Background()
	if _, err := authStore.CreateUser(ctx, "grantaccess-mike", "password123", "user"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err := run(ctx, authStore, accessStore, "grantaccess-mike", "not-a-real-feature", "", false)
	if err == nil {
		t.Fatal("expected error for unknown feature key, got nil")
	}
}

func TestRun_RejectsUnknownUsername(t *testing.T) {
	authStore, accessStore := setupStores(t)
	ctx := context.Background()

	_, err := run(ctx, authStore, accessStore, "no-such-grantaccess-user", "notes", "", false)
	if err == nil {
		t.Fatal("expected error for unknown username, got nil")
	}
}

func TestRun_ListAdminGetsEveryKnownFeature(t *testing.T) {
	authStore, accessStore := setupStores(t)
	ctx := context.Background()
	if _, err := authStore.CreateUser(ctx, "grantaccess-admin", "password123", "admin"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	out, err := run(ctx, authStore, accessStore, "grantaccess-admin", "", "", true)
	if err != nil {
		t.Fatalf("run (list): %v", err)
	}
	keys := make([]string, len(access.KnownFeatures))
	for i, f := range access.KnownFeatures {
		keys[i] = f.Key
	}
	want := `"grantaccess-admin" (role admin): ` + joinComma(keys)
	if out != want {
		t.Fatalf("expected %q, got %q", want, out)
	}
}

func joinComma(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ", "
		}
		out += item
	}
	return out
}
