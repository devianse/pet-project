package watchlist

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// TMDbMatch is what a TMDb lookup resolves an IMDb id to — produced by a
// TMDbClient (see tmdb.go) and consumed by Store.Insert. Kept separate
// from WatchlistItem because a match hasn't been assigned a database id,
// viewed flag, or created_at yet.
type TMDbMatch struct {
	MediaType   string // "movie" | "tv"
	TMDbID      int
	Title       string
	ReleaseYear string // "" if TMDb has no date for this title
	PosterPath  string // "" if TMDb has no poster for this title
	Overview    string
	VoteAverage float64
	Genres      string // comma-joined genre names, "" if none resolved
}

type WatchlistItem struct {
	ID          int64     `json:"id"`
	ImdbID      string    `json:"imdb_id"`
	MediaType   string    `json:"media_type"`
	TMDbID      int       `json:"tmdb_id"`
	Title       string    `json:"title"`
	ReleaseYear *string   `json:"release_year"`
	PosterPath  *string   `json:"poster_path"`
	Overview    string    `json:"overview"`
	VoteAverage float64   `json:"vote_average"`
	Genres      string    `json:"genres"`
	Viewed      bool      `json:"viewed"`
	CreatedAt   time.Time `json:"created_at"`
}

// ErrDuplicateImdbID is returned by Insert when imdb_id is already on the
// list — checked proactively (so the caller gets a friendly error) and
// backed by the table's UNIQUE constraint (as the backstop against a race
// between two concurrent inserts of the same link).
var ErrDuplicateImdbID = errors.New("imdb id already on the list")

type Store struct {
	conn *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{conn: conn}
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	_, err := s.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS watchlist_items (
			id           SERIAL PRIMARY KEY,
			imdb_id      TEXT NOT NULL UNIQUE,
			media_type   TEXT NOT NULL,
			tmdb_id      INTEGER NOT NULL,
			title        TEXT NOT NULL,
			release_year TEXT,
			poster_path  TEXT,
			overview     TEXT,
			vote_average REAL,
			genres       TEXT,
			viewed       BOOLEAN NOT NULL DEFAULT false,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

func (s *Store) List(ctx context.Context) ([]WatchlistItem, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT id, imdb_id, media_type, tmdb_id, title, release_year,
		       poster_path, overview, vote_average, genres, viewed, created_at
		FROM watchlist_items
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []WatchlistItem{}
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// rowScanner is satisfied by both *sql.Rows and *sql.Row, so scanItem
// serves List's multi-row loop and Insert's RETURNING single row alike.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (WatchlistItem, error) {
	var item WatchlistItem
	var releaseYear, posterPath sql.NullString
	err := row.Scan(
		&item.ID, &item.ImdbID, &item.MediaType, &item.TMDbID, &item.Title,
		&releaseYear, &posterPath, &item.Overview, &item.VoteAverage,
		&item.Genres, &item.Viewed, &item.CreatedAt,
	)
	if err != nil {
		return WatchlistItem{}, err
	}
	if releaseYear.Valid {
		item.ReleaseYear = &releaseYear.String
	}
	if posterPath.Valid {
		item.PosterPath = &posterPath.String
	}
	return item, nil
}

// Insert checks imdb_id uniqueness proactively, then inserts. The UNIQUE
// constraint on imdb_id is the backstop against a race between two
// concurrent inserts of the same link.
func (s *Store) Insert(ctx context.Context, imdbID string, m *TMDbMatch) (*WatchlistItem, error) {
	var exists bool
	if err := s.conn.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM watchlist_items WHERE imdb_id = $1)`, imdbID,
	).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateImdbID
	}

	row := s.conn.QueryRowContext(ctx, `
		INSERT INTO watchlist_items
			(imdb_id, media_type, tmdb_id, title, release_year, poster_path, overview, vote_average, genres)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, imdb_id, media_type, tmdb_id, title, release_year,
		          poster_path, overview, vote_average, genres, viewed, created_at
	`, imdbID, m.MediaType, m.TMDbID, m.Title, nullIfEmpty(m.ReleaseYear),
		nullIfEmpty(m.PosterPath), m.Overview, m.VoteAverage, m.Genres)

	item, err := scanItem(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateImdbID
		}
		return nil, err
	}
	return &item, nil
}

func (s *Store) SetViewed(ctx context.Context, id int64, viewed bool) (bool, error) {
	res, err := s.conn.ExecContext(ctx,
		`UPDATE watchlist_items SET viewed = $1 WHERE id = $2`, viewed, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) Delete(ctx context.Context, id int64) (bool, error) {
	res, err := s.conn.ExecContext(ctx, `DELETE FROM watchlist_items WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
