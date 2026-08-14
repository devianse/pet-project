package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type User struct {
	ID           int64
	Username     string
	DisplayName  *string
	AvatarColor  *string
	PasswordHash string
	Role         string
	IsActive     bool
	CreatedAt    time.Time
	LastLoginAt  *time.Time
}

type Store struct {
	conn *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{conn: conn}
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	_, err := s.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id             SERIAL PRIMARY KEY,
			username       TEXT UNIQUE NOT NULL,
			display_name   TEXT,
			password_hash  TEXT NOT NULL,
			role           TEXT NOT NULL DEFAULT 'user',
			created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_login_at  TIMESTAMPTZ
		)
	`)
	if err != nil {
		return err
	}
	// Added after the table's initial release, so CREATE TABLE IF NOT
	// EXISTS above won't add it to a table that already exists in
	// prod — ADD COLUMN IF NOT EXISTS instead, same pattern as
	// watchlist.go's post-release columns.
	_, err = s.conn.ExecContext(ctx, `
		ALTER TABLE users
			ADD COLUMN IF NOT EXISTS avatar_color TEXT
	`)
	if err != nil {
		return err
	}
	// Same post-release ADD COLUMN pattern as avatar_color above — an
	// admin-triggered "deactivate" (blocks login, no data loss) needs
	// somewhere to record it. Defaults true so every existing row reads
	// as active with no backfill step.
	_, err = s.conn.ExecContext(ctx, `
		ALTER TABLE users
			ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true
	`)
	return err
}

const selectUserColumns = `id, username, display_name, avatar_color, password_hash, role, is_active, created_at, last_login_at`

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.AvatarColor, &u.PasswordHash, &u.Role, &u.IsActive, &u.CreatedAt, &u.LastLoginAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) FindByUsername(ctx context.Context, username string) (*User, error) {
	row := s.conn.QueryRowContext(ctx, `SELECT `+selectUserColumns+` FROM users WHERE username = $1`, username)
	return scanUser(row)
}

func (s *Store) FindByID(ctx context.Context, id int64) (*User, error) {
	row := s.conn.QueryRowContext(ctx, `SELECT `+selectUserColumns+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}

// ListUsers returns every user, ordered by username, for the admin
// grants UI (access.AdminHandler). No pagination — the user base is
// small and invite-only.
func (s *Store) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := s.conn.QueryContext(ctx, `SELECT `+selectUserColumns+` FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.AvatarColor, &u.PasswordHash, &u.Role, &u.IsActive, &u.CreatedAt, &u.LastLoginAt); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash, role string) (*User, error) {
	return s.CreateUserWithDisplayName(ctx, username, passwordHash, role, nil)
}

// CreateUserWithDisplayName is CreateUser plus an optional display_name,
// split out rather than added as a CreateUser parameter so the many
// existing CreateUser call sites across other packages' tests don't all
// need updating for a field most of them don't care about.
func (s *Store) CreateUserWithDisplayName(ctx context.Context, username, passwordHash, role string, displayName *string) (*User, error) {
	row := s.conn.QueryRowContext(ctx, `
		INSERT INTO users (username, password_hash, role, display_name)
		VALUES ($1, $2, $3, $4)
		RETURNING `+selectUserColumns, username, passwordHash, role, displayName)
	user, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	// scanUser returns (nil, nil) on sql.ErrNoRows, which RETURNING never
	// produces on a successful insert — a nil here would mean the row
	// really did fail to scan for some other reason, so surface it.
	if user == nil {
		return nil, errors.New("insert returned no row")
	}
	return user, nil
}

// UpdateRole sets a user's role. Callers (access.AdminHandler) are
// responsible for validating role is a known value ("admin"/"user")
// before calling — this is a plain write, same division of labor as
// UpdateProfile's caller filling in "unchanged" fields.
func (s *Store) UpdateRole(ctx context.Context, id int64, role string) (*User, error) {
	row := s.conn.QueryRowContext(ctx, `
		UPDATE users SET role = $1
		WHERE id = $2
		RETURNING `+selectUserColumns, role, id)
	user, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("update returned no row")
	}
	return user, nil
}

// SetActive sets a user's is_active flag. A false value blocks future
// logins (checked in Handler.Login) without touching any other row —
// the deliberately chosen alternative to a hard delete (see
// docs/adr/0002-soft-delete-users.md). Callers (access.AdminHandler) are
// responsible for the self-deactivate guard, same division of labor as
// UpdateRole.
func (s *Store) SetActive(ctx context.Context, id int64, isActive bool) (*User, error) {
	row := s.conn.QueryRowContext(ctx, `
		UPDATE users SET is_active = $1
		WHERE id = $2
		RETURNING `+selectUserColumns, isActive, id)
	user, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("update returned no row")
	}
	return user, nil
}

// SetPasswordHash overwrites a user's password_hash — the store side of
// an admin-triggered password reset. Callers are responsible for hashing
// the new password (auth.HashPassword) before calling, same as
// CreateUser already expects a pre-hashed value.
func (s *Store) SetPasswordHash(ctx context.Context, id int64, passwordHash string) (*User, error) {
	row := s.conn.QueryRowContext(ctx, `
		UPDATE users SET password_hash = $1
		WHERE id = $2
		RETURNING `+selectUserColumns, passwordHash, id)
	user, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("update returned no row")
	}
	return user, nil
}

func (s *Store) UpdateLastLogin(ctx context.Context, id int64) error {
	_, err := s.conn.ExecContext(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	return err
}

// UpdateProfile is a full replace of the self-service profile fields, not
// a partial patch: both displayName and avatarColor are set to whatever
// is passed (nil clears the column to NULL). The handler layer is
// responsible for filling in "unchanged" values from the current row
// before calling this, since JSON can't distinguish "omitted" from
// "explicitly null" without pointer-to-pointer decoding this app doesn't
// otherwise need.
func (s *Store) UpdateProfile(ctx context.Context, id int64, displayName, avatarColor *string) (*User, error) {
	row := s.conn.QueryRowContext(ctx, `
		UPDATE users SET display_name = $1, avatar_color = $2
		WHERE id = $3
		RETURNING `+selectUserColumns, displayName, avatarColor, id)
	user, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("update returned no row")
	}
	return user, nil
}
