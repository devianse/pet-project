package datenight

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type Category string

const (
	CategoryFood      Category = "food"
	CategoryOutdoor   Category = "outdoor"
	CategoryCozy      Category = "cozy"
	CategoryAdventure Category = "adventure"
	CategoryCulture   Category = "culture"
)

// validCategories is the fixed set activities can be tagged with.
// Expanding it is a code change (a new const + entry here), not a data
// migration or admin UI — see the design spec's "Data model" section.
var validCategories = map[Category]bool{
	CategoryFood:      true,
	CategoryOutdoor:   true,
	CategoryCozy:      true,
	CategoryAdventure: true,
	CategoryCulture:   true,
}

func IsValidCategory(c Category) bool {
	return validCategories[c]
}

type Activity struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Category    Category  `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
}

type Store struct {
	conn *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{conn: conn}
}

// ErrActivityInUse is returned by DeleteActivity when a proposal still
// references the activity. The FK is deliberately RESTRICT-by-default
// (no ON DELETE clause): a proposal's activity is what the proposal
// MEANS, so cascading would silently delete date history, and nulling it
// out would leave "Unknown activity" rows in the History tab.
var ErrActivityInUse = errors.New("activity is referenced by a proposal")

// EnsureSchema creates both of the feature's tables. They're created
// together because date_night_proposals FKs to date_night_activities and
// the tests clear both, so a half-built schema is never a valid state.
func (s *Store) EnsureSchema(ctx context.Context) error {
	if _, err := s.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS date_night_activities (
			id          SERIAL PRIMARY KEY,
			name        TEXT NOT NULL,
			description TEXT,
			category    TEXT NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return err
	}
	// moods is a comma-joined TEXT column (e.g. "romantic,playful"), not a
	// Postgres array type — the same pattern watchlist_items.genres already
	// uses, chosen for consistency over introducing array-type scanning.
	// This deviates from the design spec's `text[]`; see Self-review notes.
	//
	// proposed_by_user_id carries no FK to users(id) even though the spec
	// lists one: the store tests insert synthetic ids (1, 2) against a
	// database whose users table belongs to another package's schema, and
	// a real FK would couple this package's tests to auth's fixtures for
	// no safety this feature actually needs.
	_, err := s.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS date_night_proposals (
			id                   SERIAL PRIMARY KEY,
			activity_id          INTEGER NOT NULL REFERENCES date_night_activities(id),
			date                 DATE NOT NULL,
			time_slot            TEXT NOT NULL,
			energy_level         TEXT NOT NULL,
			moods                TEXT NOT NULL,
			status               TEXT NOT NULL DEFAULT 'pending',
			proposed_by_user_id  BIGINT NOT NULL,
			created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

func (s *Store) ListActivities(ctx context.Context) ([]Activity, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT id, name, description, category, created_at
		FROM date_night_activities
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activities := []Activity{}
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.Category, &a.CreatedAt); err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, rows.Err()
}

func (s *Store) CreateActivity(ctx context.Context, name string, description *string, category Category) (Activity, error) {
	var a Activity
	err := s.conn.QueryRowContext(ctx, `
		INSERT INTO date_night_activities (name, description, category)
		VALUES ($1, $2, $3)
		RETURNING id, name, description, category, created_at
	`, name, description, category).Scan(&a.ID, &a.Name, &a.Description, &a.Category, &a.CreatedAt)
	return a, err
}

func (s *Store) DeleteActivity(ctx context.Context, id int64) (bool, error) {
	res, err := s.conn.ExecContext(ctx, `DELETE FROM date_night_activities WHERE id = $1`, id)
	if err != nil {
		// 23503 = foreign_key_violation: a proposal still points here.
		// Same pgconn.PgError shape watchlist's Insert uses for 23505.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return false, ErrActivityInUse
		}
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
