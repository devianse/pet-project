// Package reminders is a generic "fire a Telegram message at a future
// time, once" scheduling primitive — see
// docs/superpowers/specs/2026-08-17-reminders-design.md. It deliberately
// knows nothing about recurrence/cadence: a consumer that wants a
// recurring reminder calls Schedule again for the next occurrence after
// this one fires. Delivery itself lives in Ticker (ticker.go); this file
// is just the store.
package reminders

import (
	"context"
	"database/sql"
	"time"
)

// Reminder is one scheduled row.
type Reminder struct {
	ID        int
	Source    string
	Message   string
	DueAt     time.Time
	Status    string
	CreatedAt time.Time
}

type Store struct {
	conn *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{conn: conn}
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	_, err := s.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS reminders (
			id         SERIAL PRIMARY KEY,
			source     TEXT NOT NULL,
			message    TEXT NOT NULL,
			due_at     TIMESTAMPTZ NOT NULL,
			status     TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

// Schedule creates a new pending reminder. source is a free-form
// identifier for the owning feature/entity (e.g. "subscription:42") —
// it's how Cancel/Reschedule later find their own reminder without this
// package knowing that schema exists.
func (s *Store) Schedule(ctx context.Context, source, message string, dueAt time.Time) (int, error) {
	var id int
	err := s.conn.QueryRowContext(ctx, `
		INSERT INTO reminders (source, message, due_at, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id
	`, source, message, dueAt).Scan(&id)
	return id, err
}

// ListPending returns every reminder still awaiting delivery, soonest due
// first. Used by both Ticker (filters to due<=now itself) and the
// /reminders command (shows everything upcoming, due or not).
func (s *Store) ListPending(ctx context.Context) ([]Reminder, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT id, source, message, due_at, status, created_at
		FROM reminders
		WHERE status = 'pending'
		ORDER BY due_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Reminder{}
	for rows.Next() {
		var r Reminder
		if err := rows.Scan(&r.ID, &r.Source, &r.Message, &r.DueAt, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Cancel marks any pending reminder for source as cancelled. A no-op (not
// an error) if none matches — a consumer racing its own cancel against an
// already-sent reminder shouldn't need special-case handling.
func (s *Store) Cancel(ctx context.Context, source string) error {
	_, err := s.conn.ExecContext(ctx, `
		UPDATE reminders SET status = 'cancelled'
		WHERE source = $1 AND status = 'pending'
	`, source)
	return err
}

// Reschedule moves a pending reminder's due_at forward (or back). Same
// no-op-if-no-match behavior as Cancel.
func (s *Store) Reschedule(ctx context.Context, source string, newDueAt time.Time) error {
	_, err := s.conn.ExecContext(ctx, `
		UPDATE reminders SET due_at = $2
		WHERE source = $1 AND status = 'pending'
	`, source, newDueAt)
	return err
}

// MarkSent marks a reminder as delivered. Called by Ticker after a
// successful send.
func (s *Store) MarkSent(ctx context.Context, id int) error {
	_, err := s.conn.ExecContext(ctx, `UPDATE reminders SET status = 'sent' WHERE id = $1`, id)
	return err
}
