// backend/internal/access/access.go
package access

import (
	"context"
	"database/sql"
	"time"
)

// Store owns the features/feature_access tables. Mirrors every other
// feature package's Store shape (auth.Store, datenight.Store, ...).
type Store struct {
	conn *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{conn: conn}
}

// EnsureSchema creates both tables and seeds features from KnownFeatures.
// Idempotent — safe to call on every startup, same pattern every other
// feature package uses (no migration framework in this repo).
func (s *Store) EnsureSchema(ctx context.Context) error {
	if _, err := s.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS features (
			key   TEXT PRIMARY KEY,
			label TEXT NOT NULL
		)
	`); err != nil {
		return err
	}
	if _, err := s.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS feature_access (
			user_id     INT NOT NULL REFERENCES users(id),
			feature_key TEXT NOT NULL REFERENCES features(key),
			granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (user_id, feature_key)
		)
	`); err != nil {
		return err
	}
	for _, f := range KnownFeatures {
		if _, err := s.conn.ExecContext(ctx, `
			INSERT INTO features (key, label) VALUES ($1, $2)
			ON CONFLICT (key) DO NOTHING
		`, f.Key, f.Label); err != nil {
			return err
		}
	}
	if _, err := s.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS admin_audit_log (
			id             BIGSERIAL PRIMARY KEY,
			actor_id       BIGINT NOT NULL REFERENCES users(id),
			action         TEXT NOT NULL,
			target_user_id BIGINT REFERENCES users(id),
			detail         TEXT NOT NULL DEFAULT '',
			created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return err
	}
	return nil
}

// AuditEntry is one row of the admin-action log, joined with the actor's
// and (if any) target's username for display — the audit page has exactly
// one reader (the admin UI table), so there's no need to expose raw ids
// and make every caller do that join itself.
type AuditEntry struct {
	ID             int64
	ActorUsername  string
	Action         string
	TargetUsername *string
	Detail         string
	CreatedAt      time.Time
}

// auditEntryQuery is the join every audit-entry read uses — ListAuditLog's
// bulk read and LogAction's single-row read-back after insert share it, so
// the two can't drift into reporting different fields for the same row
// (same reasoning as AdminHandler.toResponse's shared-shape comment).
const auditEntryQuery = `
	SELECT l.id, actor.username, l.action, target.username, l.detail, l.created_at
	FROM admin_audit_log l
	JOIN users actor ON actor.id = l.actor_id
	LEFT JOIN users target ON target.id = l.target_user_id
`

func scanAuditEntry(row *sql.Row) (AuditEntry, error) {
	var e AuditEntry
	err := row.Scan(&e.ID, &e.ActorUsername, &e.Action, &e.TargetUsername, &e.Detail, &e.CreatedAt)
	return e, err
}

// LogAction records one admin action and returns the created entry (joined
// with actor/target usernames, same shape ListAuditLog returns) so callers
// — AdminHandler, to broadcast it over ops.audit — don't need a second
// query to get displayable data back. targetUserID is nil for actions with
// no single target (e.g. create_user, where the newly created user *is*
// the subject but recording it as target_user_id would be redundant with
// what's already in detail — kept nil-able for actions like a future
// bulk operation that has no single target at all).
func (s *Store) LogAction(ctx context.Context, actorID int64, action string, targetUserID *int64, detail string) (AuditEntry, error) {
	var id int64
	err := s.conn.QueryRowContext(ctx, `
		INSERT INTO admin_audit_log (actor_id, action, target_user_id, detail)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, actorID, action, targetUserID, detail).Scan(&id)
	if err != nil {
		return AuditEntry{}, err
	}
	row := s.conn.QueryRowContext(ctx, auditEntryQuery+` WHERE l.id = $1`, id)
	return scanAuditEntry(row)
}

// ListAuditLog returns the most recent 100 audit entries, newest first —
// far more than this app's admin-action volume needs, so a hard limit
// keeps the query trivial without a pagination UI.
func (s *Store) ListAuditLog(ctx context.Context) ([]AuditEntry, error) {
	rows, err := s.conn.QueryContext(ctx, auditEntryQuery+`
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorUsername, &e.Action, &e.TargetUsername, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Grant is idempotent — granting an already-granted feature is a no-op,
// not an error, so callers don't need to check first.
func (s *Store) Grant(ctx context.Context, userID int64, key string) error {
	_, err := s.conn.ExecContext(ctx, `
		INSERT INTO feature_access (user_id, feature_key) VALUES ($1, $2)
		ON CONFLICT (user_id, feature_key) DO NOTHING
	`, userID, key)
	return err
}

// Revoke reports whether a grant actually existed, so the CLI can tell
// "revoked" from "there was nothing to revoke" apart.
func (s *Store) Revoke(ctx context.Context, userID int64, key string) (bool, error) {
	res, err := s.conn.ExecContext(ctx, `
		DELETE FROM feature_access WHERE user_id = $1 AND feature_key = $2
	`, userID, key)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// ListForUser returns exactly what's in feature_access for userID — no
// admin bypass. Use ListAllForUser when the caller needs the resolved
// (admin-aware) set instead.
func (s *Store) ListForUser(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT feature_key FROM feature_access WHERE user_id = $1 ORDER BY feature_key
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// ListAllForUser is the resolved feature set for /api/me and the admin
// panel's users list: admin gets every KnownFeatures key verbatim
// (bypassing the table entirely), everyone else gets their real grants.
func (s *Store) ListAllForUser(ctx context.Context, userID int64, role string) ([]string, error) {
	if role == "admin" {
		keys := make([]string, len(KnownFeatures))
		for i, f := range KnownFeatures {
			keys[i] = f.Key
		}
		return keys, nil
	}
	return s.ListForUser(ctx, userID)
}

// HasFeature is the single-feature check RequireFeature's middleware
// uses. role comes from the caller's JWT claims, which can be stale for
// the lifetime of the token — so an admin claim only short-circuits to
// true after currentRole confirms the DB still says admin. A demoted
// user's very next request loses the bypass, no re-login required. Non-
// admin claims skip that extra query entirely and go straight to the
// feature_access check, which already re-reads the DB every time.
func HasFeature(ctx context.Context, store *Store, userID int64, role, key string) (bool, error) {
	if role == "admin" {
		stillAdmin, err := store.currentRole(ctx, userID)
		if err != nil {
			return false, err
		}
		if stillAdmin == "admin" {
			return true, nil
		}
	}
	var exists bool
	err := store.conn.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM feature_access WHERE user_id = $1 AND feature_key = $2)
	`, userID, key).Scan(&exists)
	return exists, err
}

// currentRole reads a user's role fresh from the users table — the same
// re-check /api/me already does by loading the full user row. Used to
// verify an "admin" JWT claim is still true before letting it bypass a
// feature gate.
func (s *Store) currentRole(ctx context.Context, userID int64) (string, error) {
	var role string
	err := s.conn.QueryRowContext(ctx, `SELECT role FROM users WHERE id = $1`, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return role, err
}
