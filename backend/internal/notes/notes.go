package notes

import (
	"context"
	"database/sql"
	"time"
)

type Note struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	conn *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{conn: conn}
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	_, err := s.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS notes (
			id         SERIAL PRIMARY KEY,
			content    TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

func (s *Store) List(ctx context.Context) ([]Note, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT id, content, created_at FROM notes
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Content, &n.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

// InsertBatch inserts all contents in one transaction — either every
// item lands or none do, matching the handler's all-or-nothing
// validation. It returns the full list afterward so callers never need
// separate insert-result and list-result shapes.
func (s *Store) InsertBatch(ctx context.Context, contents []string) ([]Note, error) {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, c := range contents {
		if _, err := tx.ExecContext(ctx, `INSERT INTO notes (content) VALUES ($1)`, c); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.List(ctx)
}

func (s *Store) Delete(ctx context.Context, id int64) (bool, error) {
	res, err := s.conn.ExecContext(ctx, `DELETE FROM notes WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
