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
	PasswordHash string
	Role         string
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
	return err
}

const selectUserColumns = `id, username, display_name, password_hash, role, created_at, last_login_at`

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.LastLoginAt)
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

func (s *Store) CreateUser(ctx context.Context, username, passwordHash, role string) (*User, error) {
	row := s.conn.QueryRowContext(ctx, `
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, $3)
		RETURNING `+selectUserColumns, username, passwordHash, role)
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

func (s *Store) UpdateLastLogin(ctx context.Context, id int64) error {
	_, err := s.conn.ExecContext(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	return err
}
