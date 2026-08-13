// backend/internal/access/access.go
package access

import (
	"context"
	"database/sql"
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
	return nil
}

// Grant is idempotent — granting an already-granted feature is a no-op,
// not an error, so callers (the grantaccess CLI especially) don't need to
// check first.
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

// ListAllForUser is the resolved feature set for /api/me and the
// grantaccess -list flag: admin gets every KnownFeatures key verbatim
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
// uses. admin short-circuits to true without touching the DB.
func HasFeature(ctx context.Context, store *Store, userID int64, role, key string) (bool, error) {
	if role == "admin" {
		return true, nil
	}
	var exists bool
	err := store.conn.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM feature_access WHERE user_id = $1 AND feature_key = $2)
	`, userID, key).Scan(&exists)
	return exists, err
}
