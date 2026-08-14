// backend/internal/access/access_test.go
package access

import (
	"context"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/db"
)

// setupAccessStore returns a ready *Store plus the auth.Store sharing the
// same connection, so tests can create real users to grant features to.
// Fixture usernames used by this file: access-mike, access-nastya,
// access-admin, access-mw, access-newbie — distinct from every other
// package's fixtures so
// `go test ./...`'s concurrent packages never collide on the shared
// users table.
func setupAccessStore(t *testing.T) (*Store, *auth.Store) {
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

	accessStore := NewStore(conn)
	if err := accessStore.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensuring access schema: %v", err)
	}

	usernames := []string{"access-mike", "access-nastya", "access-admin", "access-mw", "access-newbie"}
	cleanup := func() {
		for _, u := range usernames {
			conn.Exec(`DELETE FROM feature_access WHERE user_id IN (SELECT id FROM users WHERE username = $1)`, u)
			conn.Exec(`DELETE FROM admin_audit_log WHERE actor_id IN (SELECT id FROM users WHERE username = $1) OR target_user_id IN (SELECT id FROM users WHERE username = $1)`, u)
			conn.Exec(`DELETE FROM users WHERE username = $1`, u)
		}
	}
	t.Cleanup(cleanup)
	cleanup()

	return accessStore, authStore
}

func createTestUser(t *testing.T, authStore *auth.Store, username, role string) int64 {
	t.Helper()
	user, err := authStore.CreateUser(context.Background(), username, "password123", role)
	if err != nil {
		t.Fatalf("creating test user %q: %v", username, err)
	}
	return user.ID
}

func TestEnsureSchema_SeedsKnownFeatures(t *testing.T) {
	store, _ := setupAccessStore(t)
	ctx := context.Background()

	var count int
	if err := store.conn.QueryRowContext(ctx, `SELECT count(*) FROM features`).Scan(&count); err != nil {
		t.Fatalf("counting features: %v", err)
	}
	if count < len(KnownFeatures) {
		t.Fatalf("expected at least %d seeded features, got %d", len(KnownFeatures), count)
	}
}

func TestGrantAndListForUser(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	userID := createTestUser(t, authStore, "access-mike", "user")

	if err := store.Grant(ctx, userID, "notes"); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	got, err := store.ListForUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(got) != 1 || got[0] != "notes" {
		t.Fatalf("expected [notes], got %v", got)
	}
}

func TestGrant_IsIdempotent(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	userID := createTestUser(t, authStore, "access-mike", "user")

	if err := store.Grant(ctx, userID, "notes"); err != nil {
		t.Fatalf("first Grant: %v", err)
	}
	if err := store.Grant(ctx, userID, "notes"); err != nil {
		t.Fatalf("second Grant (should be idempotent): %v", err)
	}

	got, err := store.ListForUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 grant after double Grant, got %v", got)
	}
}

func TestRevoke(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	userID := createTestUser(t, authStore, "access-nastya", "user")

	if err := store.Grant(ctx, userID, "watchlist"); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	revoked, err := store.Revoke(ctx, userID, "watchlist")
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if !revoked {
		t.Fatal("expected revoked=true for an existing grant")
	}

	revoked, err = store.Revoke(ctx, userID, "watchlist")
	if err != nil {
		t.Fatalf("second Revoke: %v", err)
	}
	if revoked {
		t.Fatal("expected revoked=false when nothing was granted")
	}
}

func TestListAllForUser_AdminGetsEveryKnownFeature(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	userID := createTestUser(t, authStore, "access-admin", "admin")

	got, err := store.ListAllForUser(ctx, userID, "admin")
	if err != nil {
		t.Fatalf("ListAllForUser: %v", err)
	}
	if len(got) != len(KnownFeatures) {
		t.Fatalf("expected admin to get all %d known features, got %v", len(KnownFeatures), got)
	}
}

func TestListAllForUser_RegularUserGetsOnlyGrants(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	userID := createTestUser(t, authStore, "access-nastya", "user")

	if err := store.Grant(ctx, userID, "watchlist"); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	got, err := store.ListAllForUser(ctx, userID, "user")
	if err != nil {
		t.Fatalf("ListAllForUser: %v", err)
	}
	if len(got) != 1 || got[0] != "watchlist" {
		t.Fatalf("expected [watchlist], got %v", got)
	}
}

func TestHasFeature(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	adminID := createTestUser(t, authStore, "access-admin", "admin")
	userID := createTestUser(t, authStore, "access-mike", "user")

	adminHas, err := HasFeature(ctx, store, adminID, "admin", "notes")
	if err != nil {
		t.Fatalf("HasFeature (admin): %v", err)
	}
	if !adminHas {
		t.Fatal("expected admin to bypass gating and have every feature")
	}

	userHasBefore, err := HasFeature(ctx, store, userID, "user", "notes")
	if err != nil {
		t.Fatalf("HasFeature (before grant): %v", err)
	}
	if userHasBefore {
		t.Fatal("expected ungranted user to not have the feature")
	}

	if err := store.Grant(ctx, userID, "notes"); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	userHasAfter, err := HasFeature(ctx, store, userID, "user", "notes")
	if err != nil {
		t.Fatalf("HasFeature (after grant): %v", err)
	}
	if !userHasAfter {
		t.Fatal("expected granted user to have the feature")
	}
}

func TestLogActionAndListAuditLog(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	actorID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")

	if err := store.LogAction(ctx, actorID, "update_role", &targetID, "role: user -> admin"); err != nil {
		t.Fatalf("LogAction: %v", err)
	}

	allEntries, err := store.ListAuditLog(ctx)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	// ListAuditLog is unfiltered across the whole table, which in a shared
	// dev database also accumulates real admin-panel activity outside
	// these tests. Scope to entries this test itself produced (by actor)
	// rather than asserting a global count, so real activity elsewhere in
	// admin_audit_log never breaks this assertion.
	entries := filterByActor(allEntries, "access-admin")
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry for access-admin, got %d", len(entries))
	}
	got := entries[0]
	if got.ActorUsername != "access-admin" {
		t.Fatalf("expected actor username access-admin, got %q", got.ActorUsername)
	}
	if got.Action != "update_role" {
		t.Fatalf("expected action update_role, got %q", got.Action)
	}
	if got.TargetUsername == nil || *got.TargetUsername != "access-mike" {
		t.Fatalf("expected target username access-mike, got %v", got.TargetUsername)
	}
	if got.Detail != "role: user -> admin" {
		t.Fatalf("expected detail %q, got %q", "role: user -> admin", got.Detail)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
}

func TestLogAction_TargetUserOptional(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	actorID := createTestUser(t, authStore, "access-admin", "admin")

	if err := store.LogAction(ctx, actorID, "create_user", nil, "username=access-newbie"); err != nil {
		t.Fatalf("LogAction: %v", err)
	}

	allEntries, err := store.ListAuditLog(ctx)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	// See TestLogActionAndListAuditLog for why this scopes by actor
	// instead of asserting a global count.
	entries := filterByActor(allEntries, "access-admin")
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry for access-admin, got %d", len(entries))
	}
	if entries[0].TargetUsername != nil {
		t.Fatalf("expected nil target username, got %v", *entries[0].TargetUsername)
	}
}

func TestListAuditLog_NewestFirst(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	actorID := createTestUser(t, authStore, "access-admin", "admin")

	if err := store.LogAction(ctx, actorID, "grant_feature", nil, "feature=notes"); err != nil {
		t.Fatalf("first LogAction: %v", err)
	}
	if err := store.LogAction(ctx, actorID, "revoke_feature", nil, "feature=notes"); err != nil {
		t.Fatalf("second LogAction: %v", err)
	}

	allEntries, err := store.ListAuditLog(ctx)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	// See TestLogActionAndListAuditLog for why this scopes by actor
	// instead of asserting a global count. Ordering (newest-first) is
	// preserved by filtering without re-sorting.
	entries := filterByActor(allEntries, "access-admin")
	if len(entries) != 2 {
		t.Fatalf("expected 2 audit entries for access-admin, got %d", len(entries))
	}
	if entries[0].Action != "revoke_feature" || entries[1].Action != "grant_feature" {
		t.Fatalf("expected newest-first order, got %v then %v", entries[0].Action, entries[1].Action)
	}
}

// filterByActor scopes a ListAuditLog result to entries produced by a
// specific actor username. ListAuditLog itself is intentionally
// unfiltered (see access.go), which in a shared dev database also
// surfaces real admin-panel activity outside any given test run — so
// tests that assert exact counts scope down to their own fixture actor
// first, keeping unrelated activity elsewhere in the table from ever
// breaking these assertions.
func filterByActor(entries []AuditEntry, actorUsername string) []AuditEntry {
	scoped := make([]AuditEntry, 0, len(entries))
	for _, e := range entries {
		if e.ActorUsername == actorUsername {
			scoped = append(scoped, e)
		}
	}
	return scoped
}

// TestHasFeature_DemotedAdminLosesBypassImmediately guards the gap a stale
// JWT claim would otherwise leave open: an "admin" claims.Role that's no
// longer true in the DB must not bypass gating, even though the token
// itself is still valid and unexpired.
func TestHasFeature_DemotedAdminLosesBypassImmediately(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	userID := createTestUser(t, authStore, "access-admin", "admin")

	if _, err := store.conn.ExecContext(ctx, `UPDATE users SET role = 'user' WHERE id = $1`, userID); err != nil {
		t.Fatalf("demoting user: %v", err)
	}

	// Simulates a request carrying a JWT signed before the demotion —
	// claims.Role is still "admin" even though the DB says otherwise.
	has, err := HasFeature(ctx, store, userID, "admin", "notes")
	if err != nil {
		t.Fatalf("HasFeature: %v", err)
	}
	if has {
		t.Fatal("expected demoted user's stale admin claim to not bypass gating")
	}

	if err := store.Grant(ctx, userID, "notes"); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	hasAfterGrant, err := HasFeature(ctx, store, userID, "admin", "notes")
	if err != nil {
		t.Fatalf("HasFeature (after grant): %v", err)
	}
	if !hasAfterGrant {
		t.Fatal("expected demoted user to still see features granted directly")
	}
}
