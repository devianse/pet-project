# Movie/TV Sharing List Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a shared, no-auth `/watchlist` page where pasting an IMDb link resolves the title via TMDb and adds it to one Postgres-backed list, with viewed toggling, expandable detail, and removal.

**Architecture:** A new `backend/internal/watchlist` Go package (types + Postgres store + a TMDb HTTP client + HTTP handlers), wired into the existing `net/http` mux in `cmd/api/main.go`, mirroring how `internal/notes` is structured and wired today. A new `frontend/src/features/watchlist/Page.tsx` React feature, wired into `App.tsx`'s routes and `AppShell.tsx`'s nav, using the existing 1st-Pouf primitives and the same fetch-wrapper pattern already established in `shared/api.ts`.

**Tech Stack:** Go 1.26.5, `database/sql` + `pgx/v5` stdlib driver, stdlib `net/http` (no TMDb SDK), Postgres. React 19 + Vite, `react-router-dom`, 1st-Pouf components.

## Global Constraints

- Go module is `github.com/devianse/pet-project/backend`, Go 1.26.5 — match existing `go.mod`.
- No ORM, no query builder — raw SQL via `database/sql` + the `pgx/v5` stdlib driver, matching `internal/notes`.
- No third-party TMDb SDK — stdlib `net/http` only, matching the project's no-extra-dependency convention.
- `frontend/src/components/pouf/` is off-limits for manual edits (vendored/CLI-managed) — this plan only *consumes* existing pouf components, never modifies files under that path.
- Use the `@/*` path alias (resolves to `frontend/src/*`) for pouf imports, e.g. `@/components/pouf/Button`.
- **Never commit without being asked.** Each task's final "Commit" step is a suggested boundary, not something to run automatically — get explicit confirmation before running any `git commit`, every time, per `CLAUDE.md`.
- A PR isn't ready unless `npm audit` (frontend), `govulncheck ./...` (backend), and `gitleaks detect --source . -v` all pass clean — run by hand (`make audit-frontend`, `make audit-backend`, `make scan-secrets`); no CI enforces this yet.
- Backend tests that need a real Postgres connection skip (not fail) when `DATABASE_URL` isn't set in the environment, matching `internal/notes`' existing test pattern — this is expected, not a bug, in a sandboxed run.
- `TMDB_READ_ACCESS_TOKEN` (a TMDb v4 Read Access Token, obtained manually beforehand per the design spec's "Prerequisites" section) must be present in `backend/.env` for the backend to start; only a placeholder goes in `backend/.env.example`, never the real value.

---

### Task 1: Watchlist data model & Postgres store

**Files:**
- Create: `backend/internal/watchlist/watchlist.go`
- Test: `backend/internal/watchlist/store_test.go`

**Interfaces:**
- Consumes: `db.Open(dsn string) (*sql.DB, error)` from `backend/internal/db/db.go` (already exists).
- Produces: `type TMDbMatch struct { MediaType, Title, ReleaseYear, PosterPath, Overview, Genres string; TMDbID int; VoteAverage float64 }` — the shape a TMDb lookup resolves to, consumed by Task 2's client and Task 3's handler. `type WatchlistItem struct { ID int64; ImdbID, MediaType, Title, Overview, Genres string; TMDbID int; ReleaseYear, PosterPath *string; VoteAverage float64; Viewed bool; CreatedAt time.Time }` with `json` tags matching the design spec's field names. `type Store struct{}` with `NewStore(conn *sql.DB) *Store`, `(*Store) EnsureSchema(ctx) error`, `(*Store) List(ctx) ([]WatchlistItem, error)`, `(*Store) Insert(ctx, imdbID string, m *TMDbMatch) (*WatchlistItem, error)`, `(*Store) SetViewed(ctx, id int64, viewed bool) (bool, error)`, `(*Store) Delete(ctx, id int64) (bool, error)`. `var ErrDuplicateImdbID error`.

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/watchlist/store_test.go
package watchlist

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/db"
)

// setupStore connects to the local Postgres pointed at by DATABASE_URL,
// ensures the watchlist_items table exists, and clears it so each test
// starts empty. Skipped (not failed) if DATABASE_URL isn't set, matching
// internal/notes' existing test pattern.
func setupStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping test that needs a real Postgres instance")
	}

	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	store := NewStore(conn)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}
	if _, err := conn.ExecContext(context.Background(), "DELETE FROM watchlist_items"); err != nil {
		t.Fatalf("clearing watchlist_items table: %v", err)
	}
	return store
}

func testMatch() *TMDbMatch {
	return &TMDbMatch{
		MediaType:   "movie",
		TMDbID:      278,
		Title:       "The Shawshank Redemption",
		ReleaseYear: "1994",
		PosterPath:  "/q6y0Go1tsGEsmtFryDOJo3dEmqu.jpg",
		Overview:    "Framed in the 1940s...",
		VoteAverage: 8.7,
		Genres:      "Drama, Crime",
	}
}

func TestStore_InsertThenList(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	created, err := store.Insert(ctx, "tt0111161", testMatch())
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if created.ImdbID != "tt0111161" || created.Title != "The Shawshank Redemption" {
		t.Fatalf("unexpected created item: %+v", created)
	}
	if created.ReleaseYear == nil || *created.ReleaseYear != "1994" {
		t.Fatalf("expected release year 1994, got %+v", created.ReleaseYear)
	}
	if created.Viewed {
		t.Fatal("expected new item to default to unviewed")
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 item, got %d", len(listed))
	}
}

func TestStore_Insert_RejectsDuplicateImdbID(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	if _, err := store.Insert(ctx, "tt0111161", testMatch()); err != nil {
		t.Fatalf("first Insert: %v", err)
	}

	_, err := store.Insert(ctx, "tt0111161", testMatch())
	if !errors.Is(err, ErrDuplicateImdbID) {
		t.Fatalf("expected ErrDuplicateImdbID, got %v", err)
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected duplicate insert to leave exactly 1 item, got %d", len(listed))
	}
}

func TestStore_SetViewed(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	created, err := store.Insert(ctx, "tt0111161", testMatch())
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	found, err := store.SetViewed(ctx, created.ID, true)
	if err != nil {
		t.Fatalf("SetViewed: %v", err)
	}
	if !found {
		t.Fatal("expected SetViewed to report found=true for an existing id")
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !listed[0].Viewed {
		t.Fatal("expected item to be marked viewed")
	}
}

func TestStore_SetViewed_NotFound(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	found, err := store.SetViewed(ctx, 999999, true)
	if err != nil {
		t.Fatalf("SetViewed: %v", err)
	}
	if found {
		t.Fatal("expected SetViewed to report found=false for a nonexistent id")
	}
}

func TestStore_Delete(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	created, err := store.Insert(ctx, "tt0111161", testMatch())
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	found, err := store.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !found {
		t.Fatal("expected Delete to report found=true for an existing id")
	}

	found, err = store.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete (second time): %v", err)
	}
	if found {
		t.Fatal("expected Delete to report found=false for an already-deleted id")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/watchlist/... -v`
Expected: FAIL — build failure (`watchlist.go` doesn't exist yet, so `NewStore`, `TMDbMatch`, `ErrDuplicateImdbID` etc. are undefined).

- [ ] **Step 3: Write the implementation**

```go
// backend/internal/watchlist/watchlist.go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/watchlist/... -v`
Expected: PASS if `DATABASE_URL` is set to a reachable Postgres instance; SKIP ("DATABASE_URL not set...") otherwise — either outcome means Step 3's code is correct, per this project's existing test convention.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/watchlist/watchlist.go backend/internal/watchlist/store_test.go
git commit -m "feat(watchlist): add data model and Postgres store"
```

---

### Task 2: TMDb client

**Files:**
- Create: `backend/internal/watchlist/tmdb.go`
- Test: `backend/internal/watchlist/tmdb_test.go`

**Interfaces:**
- Consumes: `TMDbMatch` from Task 1 (`watchlist.go`).
- Produces: `type TMDbClient interface { FindByIMDbID(ctx context.Context, imdbID string) (*TMDbMatch, error) }` — consumed by Task 3's `Handler`. `var ErrTMDbNotFound error`. `func NewRealTMDbClient(ctx context.Context, token string) (*RealTMDbClient, error)` — the real implementation, consumed by Task 4's `main.go`.

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/watchlist/tmdb_test.go
package watchlist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newFakeTMDbServer(t *testing.T, findBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/genre/movie/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"genres":[{"id":18,"name":"Drama"},{"id":80,"name":"Crime"}]}`))
	})
	mux.HandleFunc("/genre/tv/list", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"genres":[{"id":10765,"name":"Sci-Fi & Fantasy"}]}`))
	})
	mux.HandleFunc("/find/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(findBody))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestRealTMDbClient_FindByIMDbID_MovieMatch(t *testing.T) {
	server := newFakeTMDbServer(t, `{
		"movie_results": [{
			"id": 278,
			"title": "The Shawshank Redemption",
			"release_date": "1994-09-23",
			"poster_path": "/q6y0Go1tsGEsmtFryDOJo3dEmqu.jpg",
			"overview": "Framed in the 1940s...",
			"vote_average": 8.7,
			"genre_ids": [18, 80]
		}],
		"tv_results": []
	}`)

	client, err := newRealTMDbClient(context.Background(), server.URL, "test-token")
	if err != nil {
		t.Fatalf("newRealTMDbClient: %v", err)
	}

	match, err := client.FindByIMDbID(context.Background(), "tt0111161")
	if err != nil {
		t.Fatalf("FindByIMDbID: %v", err)
	}
	if match.MediaType != "movie" {
		t.Fatalf("expected media type movie, got %s", match.MediaType)
	}
	if match.Title != "The Shawshank Redemption" {
		t.Fatalf("unexpected title: %s", match.Title)
	}
	if match.ReleaseYear != "1994" {
		t.Fatalf("expected release year 1994, got %s", match.ReleaseYear)
	}
	if match.Genres != "Drama, Crime" {
		t.Fatalf("expected genres 'Drama, Crime', got %q", match.Genres)
	}
}

func TestRealTMDbClient_FindByIMDbID_TVMatch(t *testing.T) {
	server := newFakeTMDbServer(t, `{
		"movie_results": [],
		"tv_results": [{
			"id": 1399,
			"name": "Game of Thrones",
			"first_air_date": "2011-04-17",
			"poster_path": "/u3bZgnGQ9T01sWNhyveQz0wH0Hl.jpg",
			"overview": "Seven noble families fight...",
			"vote_average": 8.4,
			"genre_ids": [10765]
		}]
	}`)

	client, err := newRealTMDbClient(context.Background(), server.URL, "test-token")
	if err != nil {
		t.Fatalf("newRealTMDbClient: %v", err)
	}

	match, err := client.FindByIMDbID(context.Background(), "tt0944947")
	if err != nil {
		t.Fatalf("FindByIMDbID: %v", err)
	}
	if match.MediaType != "tv" {
		t.Fatalf("expected media type tv, got %s", match.MediaType)
	}
	if match.Title != "Game of Thrones" {
		t.Fatalf("unexpected title: %s", match.Title)
	}
	if match.Genres != "Sci-Fi & Fantasy" {
		t.Fatalf("expected genres 'Sci-Fi & Fantasy', got %q", match.Genres)
	}
}

func TestRealTMDbClient_FindByIMDbID_NoMatch(t *testing.T) {
	server := newFakeTMDbServer(t, `{"movie_results": [], "tv_results": []}`)

	client, err := newRealTMDbClient(context.Background(), server.URL, "test-token")
	if err != nil {
		t.Fatalf("newRealTMDbClient: %v", err)
	}

	_, err = client.FindByIMDbID(context.Background(), "tt9999999999")
	if !errors.Is(err, ErrTMDbNotFound) {
		t.Fatalf("expected ErrTMDbNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/watchlist/... -run TestRealTMDbClient -v`
Expected: FAIL — build failure (`newRealTMDbClient`, `ErrTMDbNotFound` etc. don't exist yet).

- [ ] **Step 3: Write the implementation**

```go
// backend/internal/watchlist/tmdb.go
package watchlist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrTMDbNotFound is returned by TMDbClient.FindByIMDbID when TMDb has
// neither a movie nor a TV match for the given id.
var ErrTMDbNotFound = errors.New("no tmdb match found for imdb id")

const tmdbBaseURL = "https://api.themoviedb.org/3"

// TMDbClient is the interface Handler depends on, so tests can supply a
// fake instead of making real network calls.
type TMDbClient interface {
	FindByIMDbID(ctx context.Context, imdbID string) (*TMDbMatch, error)
}

// RealTMDbClient calls the actual TMDb API.
type RealTMDbClient struct {
	httpClient  *http.Client
	baseURL     string
	token       string
	movieGenres map[int]string
	tvGenres    map[int]string
}

// NewRealTMDbClient fetches TMDb's movie and TV genre lists once, up
// front, and caches them for the client's lifetime — /find only returns
// numeric genre_ids, and re-fetching the (effectively static) name list
// on every request would be wasted work.
func NewRealTMDbClient(ctx context.Context, token string) (*RealTMDbClient, error) {
	return newRealTMDbClient(ctx, tmdbBaseURL, token)
}

// newRealTMDbClient takes an explicit baseURL so tests can point it at an
// httptest.Server instead of the real themoviedb.org.
func newRealTMDbClient(ctx context.Context, baseURL, token string) (*RealTMDbClient, error) {
	c := &RealTMDbClient{
		httpClient: http.DefaultClient,
		baseURL:    baseURL,
		token:      token,
	}

	movieGenres, err := c.fetchGenreMap(ctx, "/genre/movie/list")
	if err != nil {
		return nil, fmt.Errorf("fetching movie genres: %w", err)
	}
	tvGenres, err := c.fetchGenreMap(ctx, "/genre/tv/list")
	if err != nil {
		return nil, fmt.Errorf("fetching tv genres: %w", err)
	}
	c.movieGenres = movieGenres
	c.tvGenres = tvGenres
	return c, nil
}

type genreListResponse struct {
	Genres []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
}

func (c *RealTMDbClient) fetchGenreMap(ctx context.Context, path string) (map[int]string, error) {
	var resp genreListResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	m := make(map[int]string, len(resp.Genres))
	for _, g := range resp.Genres {
		m[g.ID] = g.Name
	}
	return m, nil
}

type findResponse struct {
	MovieResults []struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		ReleaseDate string  `json:"release_date"`
		PosterPath  string  `json:"poster_path"`
		Overview    string  `json:"overview"`
		VoteAverage float64 `json:"vote_average"`
		GenreIDs    []int   `json:"genre_ids"`
	} `json:"movie_results"`
	TVResults []struct {
		ID           int     `json:"id"`
		Name         string  `json:"name"`
		FirstAirDate string  `json:"first_air_date"`
		PosterPath   string  `json:"poster_path"`
		Overview     string  `json:"overview"`
		VoteAverage  float64 `json:"vote_average"`
		GenreIDs     []int   `json:"genre_ids"`
	} `json:"tv_results"`
}

// FindByIMDbID looks up an external IMDb id. TMDb's /find returns exactly
// one match per media type (or none) — there's no "which of several
// candidates" ambiguity to resolve, unlike a fuzzy title search.
func (c *RealTMDbClient) FindByIMDbID(ctx context.Context, imdbID string) (*TMDbMatch, error) {
	var resp findResponse
	path := fmt.Sprintf("/find/%s?external_source=imdb_id", imdbID)
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("tmdb find: %w", err)
	}

	if len(resp.MovieResults) > 0 {
		r := resp.MovieResults[0]
		return &TMDbMatch{
			MediaType:   "movie",
			TMDbID:      r.ID,
			Title:       r.Title,
			ReleaseYear: yearFromDate(r.ReleaseDate),
			PosterPath:  r.PosterPath,
			Overview:    r.Overview,
			VoteAverage: r.VoteAverage,
			Genres:      resolveGenres(r.GenreIDs, c.movieGenres),
		}, nil
	}
	if len(resp.TVResults) > 0 {
		r := resp.TVResults[0]
		return &TMDbMatch{
			MediaType:   "tv",
			TMDbID:      r.ID,
			Title:       r.Name,
			ReleaseYear: yearFromDate(r.FirstAirDate),
			PosterPath:  r.PosterPath,
			Overview:    r.Overview,
			VoteAverage: r.VoteAverage,
			Genres:      resolveGenres(r.GenreIDs, c.tvGenres),
		}, nil
	}
	return nil, ErrTMDbNotFound
}

func (c *RealTMDbClient) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tmdb request to %s failed: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// yearFromDate extracts "1994" from TMDb's "1994-09-23"-shaped date
// strings. TMDb sometimes omits the date entirely, hence the length guard.
func yearFromDate(date string) string {
	if len(date) < 4 {
		return ""
	}
	return date[:4]
}

func resolveGenres(ids []int, names map[int]string) string {
	resolved := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := names[id]; ok {
			resolved = append(resolved, name)
		}
	}
	return strings.Join(resolved, ", ")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/watchlist/... -run TestRealTMDbClient -v`
Expected: PASS — these tests hit a local `httptest.Server`, not the real TMDb API, so they always run (no `DATABASE_URL`/network dependency).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/watchlist/tmdb.go backend/internal/watchlist/tmdb_test.go
git commit -m "feat(watchlist): add TMDb client"
```

---

### Task 3: HTTP handlers and link parsing

**Files:**
- Create: `backend/internal/watchlist/handlers.go`
- Test: `backend/internal/watchlist/handlers_test.go`

**Interfaces:**
- Consumes: `Store` and `TMDbMatch`/`ErrDuplicateImdbID` from Task 1; `TMDbClient`/`ErrTMDbNotFound` from Task 2.
- Produces: `func NewHandler(store *Store, tmdb TMDbClient) *Handler` with methods `List`, `Create`, `SetViewed`, `Delete` (each `func(w http.ResponseWriter, r *http.Request)`) — consumed by Task 4's `main.go` route registration.

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/watchlist/handlers_test.go
package watchlist

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

type fakeTMDbClient struct {
	match *TMDbMatch
	err   error
}

func (f *fakeTMDbClient) FindByIMDbID(ctx context.Context, imdbID string) (*TMDbMatch, error) {
	return f.match, f.err
}

func TestParseIMDbID(t *testing.T) {
	cases := []struct {
		name    string
		link    string
		wantID  string
		wantErr bool
	}{
		{"valid", "https://www.imdb.com/title/tt0111161/", "tt0111161", false},
		{"valid without www", "https://imdb.com/title/tt0111161", "tt0111161", false},
		{"valid with query suffix", "https://www.imdb.com/title/tt0111161/?ref_=nv_sr_srsg_0", "tt0111161", false},
		{"wrong site", "https://www.kinopoisk.ru/film/326/", "", true},
		{"not a url", "The Shawshank Redemption", "", true},
		{"empty", "", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := parseIMDbID(tc.link)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if id != tc.wantID {
				t.Fatalf("expected id %q, got %q", tc.wantID, id)
			}
		})
	}
}

func TestHandler_Create_RejectsNonIMDbLink(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store, &fakeTMDbClient{})

	body, _ := json.Marshal(createRequest{Link: "https://www.kinopoisk.ru/film/326/"})
	req := httptest.NewRequest(http.MethodPost, "/api/watchlist", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_Create_RejectsWhenTMDbHasNoMatch(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store, &fakeTMDbClient{err: ErrTMDbNotFound})

	body, _ := json.Marshal(createRequest{Link: "https://www.imdb.com/title/tt0000000/"})
	req := httptest.NewRequest(http.MethodPost, "/api/watchlist", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_CreateThenList(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store, &fakeTMDbClient{match: testMatch()})

	body, _ := json.Marshal(createRequest{Link: "https://www.imdb.com/title/tt0111161/"})
	req := httptest.NewRequest(http.MethodPost, "/api/watchlist", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var created WatchlistItem
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if created.ImdbID != "tt0111161" {
		t.Fatalf("expected imdb_id tt0111161, got %s", created.ImdbID)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/watchlist", nil)
	listRec := httptest.NewRecorder()
	handler.List(listRec, listReq)

	var listed []WatchlistItem
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decoding list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 item, got %d", len(listed))
	}
}

func TestHandler_Create_RejectsDuplicate(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store, &fakeTMDbClient{match: testMatch()})

	body, _ := json.Marshal(createRequest{Link: "https://www.imdb.com/title/tt0111161/"})

	req1 := httptest.NewRequest(http.MethodPost, "/api/watchlist", bytes.NewReader(body))
	rec1 := httptest.NewRecorder()
	handler.Create(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first create to succeed, got %d: %s", rec1.Code, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/watchlist", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	handler.Create(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected second create to be rejected as duplicate, got %d", rec2.Code)
	}
}

func TestHandler_SetViewed(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store, &fakeTMDbClient{})
	ctx := context.Background()

	created, err := store.Insert(ctx, "tt0111161", testMatch())
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	idStr := strconv.FormatInt(created.ID, 10)

	body, _ := json.Marshal(setViewedRequest{Viewed: true})
	req := httptest.NewRequest(http.MethodPatch, "/api/watchlist/"+idStr, bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	rec := httptest.NewRecorder()

	handler.SetViewed(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_SetViewed_NotFound(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store, &fakeTMDbClient{})

	body, _ := json.Marshal(setViewedRequest{Viewed: true})
	req := httptest.NewRequest(http.MethodPatch, "/api/watchlist/999999", bytes.NewReader(body))
	req.SetPathValue("id", "999999")
	rec := httptest.NewRecorder()

	handler.SetViewed(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandler_Delete(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store, &fakeTMDbClient{})
	ctx := context.Background()

	created, err := store.Insert(ctx, "tt0111161", testMatch())
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	idStr := strconv.FormatInt(created.ID, 10)

	req := httptest.NewRequest(http.MethodDelete, "/api/watchlist/"+idStr, nil)
	req.SetPathValue("id", idStr)
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestHandler_Delete_NotFound(t *testing.T) {
	store := setupStore(t)
	handler := NewHandler(store, &fakeTMDbClient{})

	req := httptest.NewRequest(http.MethodDelete, "/api/watchlist/999999", nil)
	req.SetPathValue("id", "999999")
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/watchlist/... -v`
Expected: FAIL — build failure (`NewHandler`, `createRequest`, `setViewedRequest`, `parseIMDbID` don't exist yet).

- [ ] **Step 3: Write the implementation**

```go
// backend/internal/watchlist/handlers.go
package watchlist

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
)

var imdbTitleURLPattern = regexp.MustCompile(`(?i)^https?://(www\.)?imdb\.com/title/(tt\d+)`)

// parseIMDbID extracts the tt-id from an IMDb title URL, e.g.
// "https://www.imdb.com/title/tt0111161/" -> "tt0111161". Anything else
// (a different site, plain text, a malformed URL) is rejected — no
// fallback fuzzy search, matching the design's IMDb-links-only scope.
func parseIMDbID(link string) (string, error) {
	matches := imdbTitleURLPattern.FindStringSubmatch(link)
	if matches == nil {
		return "", errors.New("link must be an IMDb title URL, e.g. https://www.imdb.com/title/tt0111161/")
	}
	return matches[2], nil
}

type Handler struct {
	store *Store
	tmdb  TMDbClient
}

func NewHandler(store *Store, tmdb TMDbClient) *Handler {
	return &Handler{store: store, tmdb: tmdb}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.List(r.Context())
	if err != nil {
		slog.Error("list watchlist items", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type createRequest struct {
	Link string `json:"link"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed JSON body", http.StatusBadRequest)
		return
	}

	imdbID, err := parseIMDbID(req.Link)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	match, err := h.tmdb.FindByIMDbID(r.Context(), imdbID)
	if errors.Is(err, ErrTMDbNotFound) {
		http.Error(w, "no title found for that link", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("tmdb lookup", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	item, err := h.store.Insert(r.Context(), imdbID, match)
	if errors.Is(err, ErrDuplicateImdbID) {
		http.Error(w, "already on the list", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("insert watchlist item", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

type setViewedRequest struct {
	Viewed bool `json:"viewed"`
}

func (h *Handler) SetViewed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req setViewedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed JSON body", http.StatusBadRequest)
		return
	}

	found, err := h.store.SetViewed(r.Context(), id, req.Viewed)
	if err != nil {
		slog.Error("set viewed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	found, err := h.store.Delete(r.Context(), id)
	if err != nil {
		slog.Error("delete watchlist item", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/watchlist/... -v`
Expected: PASS (the `TestParseIMDbID`/`fakeTMDbClient`-based tests always run; the `setupStore`-based ones PASS with `DATABASE_URL` set, SKIP otherwise).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/watchlist/handlers.go backend/internal/watchlist/handlers_test.go
git commit -m "feat(watchlist): add HTTP handlers and IMDb link parsing"
```

---

### Task 4: Wire the feature into the server

**Files:**
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/.env.example`

**Interfaces:**
- Consumes: `watchlist.NewStore`, `watchlist.NewRealTMDbClient`, `watchlist.NewHandler` (Tasks 1–3), plus the existing `db.Open` and the existing `notes` wiring already in `main.go` as a template to follow.
- Produces: nothing new for later tasks — this task only wires existing pieces together and exposes the routes at `/api/watchlist` and `/api/watchlist/{id}`.

- [ ] **Step 1: Add the TMDb env var placeholder**

Modify `backend/.env.example`, appending a line so the convention documents itself (the real value stays in the gitignored `backend/.env`, never here):

```
PORT=8080
DATABASE_URL=postgres://postgres:postgres@localhost:5432/notes?sslmode=disable
TMDB_READ_ACCESS_TOKEN=your-tmdb-v4-read-access-token
```

- [ ] **Step 2: Wire watchlist into `main.go`**

Modify `backend/cmd/api/main.go`:

1. Update the file's top-of-file comment to mention the new feature, alongside the existing Notes mention:

```go
// Phase 1 scaffold, now with the Notes and Watchlist features wired in as
// deliberate detours ahead of the shell-first plan (see
// docs/superpowers/specs/2026-08-08-notes-design.md and
// docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md).
// Still no auth.
package main
```

2. Add the import:

```go
	"github.com/devianse/pet-project/backend/internal/watchlist"
```

3. After the existing `notesHandler := notes.NewHandler(notesStore)` line, add:

```go
	tmdbToken := os.Getenv("TMDB_READ_ACCESS_TOKEN")
	if tmdbToken == "" {
		logger.Error("TMDB_READ_ACCESS_TOKEN is not set")
		os.Exit(1)
	}
	tmdbClient, err := watchlist.NewRealTMDbClient(context.Background(), tmdbToken)
	if err != nil {
		logger.Error("failed to initialize tmdb client", "error", err)
		os.Exit(1)
	}

	watchlistStore := watchlist.NewStore(conn)
	if err := watchlistStore.EnsureSchema(context.Background()); err != nil {
		logger.Error("failed to ensure watchlist schema", "error", err)
		os.Exit(1)
	}
	watchlistHandler := watchlist.NewHandler(watchlistStore, tmdbClient)
```

4. Alongside the existing `mux.HandleFunc("DELETE /api/notes/{id}", ...)` line, add:

```go
	mux.HandleFunc("GET /api/watchlist", watchlistHandler.List)
	mux.HandleFunc("POST /api/watchlist", watchlistHandler.Create)
	mux.HandleFunc("PATCH /api/watchlist/{id}", watchlistHandler.SetViewed)
	mux.HandleFunc("DELETE /api/watchlist/{id}", watchlistHandler.Delete)
```

- [ ] **Step 3: Verify the backend builds and existing tests still pass**

Run: `cd backend && go build ./...`
Expected: succeeds with no errors.

Run: `cd backend && go test ./...`
Expected: PASS/SKIP for every package (same as Task 3's Step 4) — this task adds no new tests of its own, since it's pure wiring; its correctness gate is a clean build plus the existing suite staying green.

- [ ] **Step 4: Manually verify against a running server**

With `backend/.env` containing a real `DATABASE_URL` and `TMDB_READ_ACCESS_TOKEN`:

Run: `make dev-backend`
Expected: log line `api listening addr=:8080`, no startup errors.

Run (in another terminal): `curl -s -X POST localhost:8080/api/watchlist -H 'Content-Type: application/json' -d '{"link":"https://www.imdb.com/title/tt0111161/"}'`
Expected: a `200` JSON response with `"title":"The Shawshank Redemption"` and a non-null `poster_path`.

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/api/main.go backend/.env.example
git commit -m "feat(watchlist): wire feature into the server"
```

---

### Task 5: Frontend API client functions

**Files:**
- Modify: `frontend/src/shared/api.ts`

**Interfaces:**
- Produces: `type WatchlistItem = { id: number; imdb_id: string; media_type: 'movie' | 'tv'; tmdb_id: number; title: string; release_year: string | null; poster_path: string | null; overview: string; vote_average: number; genres: string; viewed: boolean; created_at: string }`, `getWatchlist(): Promise<WatchlistItem[]>`, `addToWatchlist(link: string): Promise<WatchlistItem>`, `setWatchlistItemViewed(id: number, viewed: boolean): Promise<void>`, `removeFromWatchlist(id: number): Promise<void>` — all consumed by Task 6's `Page.tsx`.

- [ ] **Step 1: Add the type and functions**

Append to `frontend/src/shared/api.ts`:

```ts
export type WatchlistItem = {
  id: number
  imdb_id: string
  media_type: 'movie' | 'tv'
  tmdb_id: number
  title: string
  release_year: string | null
  poster_path: string | null
  overview: string
  vote_average: number
  genres: string
  viewed: boolean
  created_at: string
}

export async function getWatchlist(): Promise<WatchlistItem[]> {
  const res = await fetch('/api/watchlist')
  if (!res.ok) {
    throw new Error(`failed to load watchlist: ${res.status}`)
  }
  return res.json()
}

export async function addToWatchlist(link: string): Promise<WatchlistItem> {
  const res = await fetch('/api/watchlist', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ link }),
  })
  if (!res.ok) {
    const message = await res.text()
    throw new Error(message || `failed to add link: ${res.status}`)
  }
  return res.json()
}

export async function setWatchlistItemViewed(id: number, viewed: boolean): Promise<void> {
  const res = await fetch(`/api/watchlist/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ viewed }),
  })
  if (!res.ok) {
    throw new Error(`failed to update watchlist item: ${res.status}`)
  }
}

export async function removeFromWatchlist(id: number): Promise<void> {
  const res = await fetch(`/api/watchlist/${id}`, { method: 'DELETE' })
  if (!res.ok) {
    throw new Error(`failed to remove watchlist item: ${res.status}`)
  }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd frontend && npx tsc -b --noEmit`
Expected: no type errors. (Nothing imports these functions yet, so this only confirms `api.ts` itself is well-typed.)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/shared/api.ts
git commit -m "feat(watchlist): add frontend API client functions"
```

---

### Task 6: Watchlist page component

**Files:**
- Create: `frontend/src/features/watchlist/Page.tsx`

**Interfaces:**
- Consumes: `WatchlistItem`, `getWatchlist`, `addToWatchlist`, `setWatchlistItemViewed`, `removeFromWatchlist` from Task 5; `Card`, `RowCard` from `@/components/pouf/surface`; `Field`, `Input` from `@/components/pouf/Input`; `Button`, `IconButton` from `@/components/pouf/Button`; `Icon` from `@/components/pouf/Icon`; `Stack`, `Row` from `@/components/pouf/layout`.
- Produces: `export default function WatchlistPage()` — consumed by Task 7's `App.tsx`.

- [ ] **Step 1: Write the component**

```tsx
// frontend/src/features/watchlist/Page.tsx
import { useEffect, useState } from 'react'
import {
  addToWatchlist,
  getWatchlist,
  removeFromWatchlist,
  setWatchlistItemViewed,
  type WatchlistItem,
} from '../../shared/api'
import { Card, RowCard } from '@/components/pouf/surface'
import { Field, Input } from '@/components/pouf/Input'
import { Button, IconButton } from '@/components/pouf/Button'
import { Icon } from '@/components/pouf/Icon'
import { Stack, Row } from '@/components/pouf/layout'

const POSTER_BASE = 'https://image.tmdb.org/t/p/w185'

export default function WatchlistPage() {
  const [items, setItems] = useState<WatchlistItem[]>([])
  const [link, setLink] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set())

  useEffect(() => {
    getWatchlist()
      .then(setItems)
      .catch(() => setError('failed to load watchlist'))
  }, [])

  async function handleAdd() {
    if (link.trim() === '') return
    setError(null)
    setSubmitting(true)
    try {
      const created = await addToWatchlist(link.trim())
      setItems((prev) => [created, ...prev])
      setLink('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to add link')
    } finally {
      setSubmitting(false)
    }
  }

  function toggleExpand(id: number) {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  async function toggleViewed(item: WatchlistItem) {
    const nextViewed = !item.viewed
    setItems((prev) => prev.map((i) => (i.id === item.id ? { ...i, viewed: nextViewed } : i)))
    try {
      await setWatchlistItemViewed(item.id, nextViewed)
    } catch {
      setItems((prev) => prev.map((i) => (i.id === item.id ? { ...i, viewed: item.viewed } : i)))
      setError('failed to update viewed status')
    }
  }

  async function remove(id: number) {
    setError(null)
    try {
      await removeFromWatchlist(id)
      setItems((prev) => prev.filter((i) => i.id !== id))
    } catch {
      setError('failed to remove item')
    }
  }

  return (
    <Stack gap={5}>
      <h1 className="text-2xl font-black text-ink">Watchlist</h1>
      {error && (
        <p className="font-bold text-(--on-accent) bg-orange rounded-xl px-3 py-2 self-start">
          {error}
        </p>
      )}

      <Card>
        <Stack gap={3}>
          <Field label="IMDb link">
            {(id, describedBy) => (
              <Input
                id={id}
                describedBy={describedBy}
                value={link}
                onChange={setLink}
                placeholder="https://www.imdb.com/title/tt0111161/"
              />
            )}
          </Field>
          <Row justify="end">
            <Button onClick={handleAdd} tone="purple" loading={submitting}>
              <Icon name="add" /> Add
            </Button>
          </Row>
        </Stack>
      </Card>

      <Stack gap={2}>
        {items.map((item) => {
          const expanded = expandedIds.has(item.id)
          return (
            <RowCard key={item.id}>
              <Row gap={3} align="top">
                {item.poster_path ? (
                  <img
                    src={`${POSTER_BASE}${item.poster_path}`}
                    alt=""
                    className="w-16 rounded-control shrink-0"
                  />
                ) : (
                  <div className="w-16 h-24 rounded-control bg-bg shrink-0 flex items-center justify-center">
                    <Icon name="photo" />
                  </div>
                )}
                <Stack gap={2}>
                  <Row justify="between">
                    <Row gap={2}>
                      <span className="font-black text-ink">{item.title}</span>
                      {item.release_year && <span className="text-muted">({item.release_year})</span>}
                      <span className="text-xs font-black uppercase px-2 py-1 rounded-full bg-blue text-(--on-accent)">
                        {item.media_type === 'tv' ? 'TV' : 'Movie'}
                      </span>
                    </Row>
                    <Row gap={1}>
                      <IconButton
                        variant={item.viewed ? 'solid' : 'quiet'}
                        tone="mint"
                        size="sm"
                        onClick={() => toggleViewed(item)}
                        label={
                          item.viewed
                            ? `Mark "${item.title}" as unwatched`
                            : `Mark "${item.title}" as watched`
                        }
                        icon={<Icon name="ok" />}
                      />
                      <IconButton
                        variant="quiet"
                        size="sm"
                        onClick={() => toggleExpand(item.id)}
                        label={
                          expanded
                            ? `Collapse details for "${item.title}"`
                            : `Expand details for "${item.title}"`
                        }
                        icon={<Icon name="expand" />}
                      />
                      <IconButton
                        variant="quiet"
                        size="sm"
                        onClick={() => remove(item.id)}
                        label={`Remove "${item.title}" from watchlist`}
                        icon={<Icon name="remove" />}
                      />
                    </Row>
                  </Row>
                  {expanded && (
                    <Stack gap={1}>
                      <p className="text-ink">{item.overview}</p>
                      {item.genres && <p className="text-muted">{item.genres}</p>}
                      <p className="text-muted">TMDb rating: {item.vote_average.toFixed(1)}</p>
                    </Stack>
                  )}
                </Stack>
              </Row>
            </RowCard>
          )
        })}
      </Stack>

      <p className="text-xs text-muted">
        This product uses the TMDB API but is not endorsed or certified by TMDB.
      </p>
    </Stack>
  )
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd frontend && npx tsc -b --noEmit`
Expected: no type errors. (No dedicated frontend test suite exists in this project — per the design spec's Testing section, type-checking plus Task 7's manual run-through are this feature's frontend verification, matching how Notes and the rest of the app are covered today.)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/watchlist/Page.tsx
git commit -m "feat(watchlist): add Watchlist page component"
```

---

### Task 7: Routing and navigation

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/AppShell.tsx`

**Interfaces:**
- Consumes: `WatchlistPage` from Task 6.
- Produces: nothing new for later tasks — this is the final piece making the feature reachable in the running app.

- [ ] **Step 1: Add the route**

Modify `frontend/src/App.tsx`:

```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import HomePage from './features/home/Page'
import ShoppingListPage from './features/shopping-list/Page'
import ImageProcessingPage from './features/image-processing/Page'
import NotesPage from './features/notes/Page'
import WatchlistPage from './features/watchlist/Page'
import { AppShell } from './components/AppShell'

// Phase 1 shell: nav + routing only. Auth-gating, layout polish, etc. are
// phase 2 once there's something worth protecting. Notes and Watchlist
// are phase-2/3 domain features pulled forward as deliberate detours —
// see docs/superpowers/specs/2026-08-08-notes-design.md and
// docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md. Nav
// chrome comes from AppShell (Tailwind + 1st-Pouf) — see
// docs/superpowers/specs/2026-08-09-design-system.md.
export default function App() {
  return (
    <BrowserRouter>
      <AppShell>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/shopping-list" element={<ShoppingListPage />} />
          <Route path="/image-processing" element={<ImageProcessingPage />} />
          <Route path="/notes" element={<NotesPage />} />
          <Route path="/watchlist" element={<WatchlistPage />} />
        </Routes>
      </AppShell>
    </BrowserRouter>
  )
}
```

- [ ] **Step 2: Add the nav item**

Modify `frontend/src/components/AppShell.tsx`'s `NAV_ITEMS` array:

```tsx
const NAV_ITEMS: { href: string; label: string; icon: IconName }[] = [
  { href: '/', label: 'Home', icon: 'home' },
  { href: '/shopping-list', label: 'Shopping List', icon: 'cart' },
  { href: '/image-processing', label: 'Image Processing', icon: 'photo' },
  { href: '/notes', label: 'Notes', icon: 'log' },
  { href: '/watchlist', label: 'Watchlist', icon: 'play' },
]
```

- [ ] **Step 3: Verify it compiles**

Run: `cd frontend && npx tsc -b --noEmit`
Expected: no type errors.

- [ ] **Step 4: Manually verify in the running app**

With both dev servers up (`make dev-backend`, `make dev-frontend`, per README):

1. Open `http://localhost:3000` — a "Watchlist" nav item appears.
2. Click it, paste `https://www.imdb.com/title/tt0111161/`, click Add.
3. Expected: a card appears with poster, "The Shawshank Redemption", "(1994)", and a "Movie" badge.
4. Click the card's expand button — overview, genres, and TMDb rating appear.
5. Click the viewed toggle, then the remove button — the card updates, then disappears.
6. Paste the same link again, click Add — expect an inline "already on the list" error (only reachable after re-adding a title still present; if the previous step removed it, adding it twice in a row without removing demonstrates this instead).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/AppShell.tsx
git commit -m "feat(watchlist): wire page into routing and navigation"
```

---

### Task 8: Docs and pre-merge scans

**Files:**
- Modify: `README.md`
- Modify: `PLANNING.md`

**Interfaces:** None — documentation only, plus the project's standing verification gate.

- [ ] **Step 1: Document the new env var**

Modify `README.md`'s "Env config" section, changing:

```
- `backend/.env.example` — `PORT` the Go server listens on
```

to:

```
- `backend/.env.example` — `PORT` the Go server listens on,
  `TMDB_READ_ACCESS_TOKEN` for the Watchlist feature's TMDb API calls
```

- [ ] **Step 2: Resolve `PLANNING.md`'s step 5 and open question**

Modify `PLANNING.md`'s "Actual build order so far" step 5, changing:

```
5. **Movie/TV Sharing List** (next, not yet designed) — another
   pre-phase-3 detour, same pattern as Notes: a shareable watchlist,
   insert a link, get a preview card (title, description, poster image),
   mark items as viewed/expand for detail. Likely needs a third-party
   metadata source (IMDb vs. Kinopoisk under consideration — access/rate
   limits from Russia unverified for either) — that's a real design
   question for its own brainstorming pass when this is picked up, not
   resolved here.
```

to:

```
5. **Movie/TV Sharing List** (in progress) — another pre-phase-3 detour,
   same pattern as Notes: a shareable watchlist, paste an IMDb link, get
   a preview card (title, description, poster image) resolved via TMDb,
   mark items as viewed/expand for detail. See
   `docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md`
   and `docs/superpowers/plans/2026-08-09-movie-tv-sharing-list.md`.
```

Modify the "Open questions" section, changing:

```
- **Movie Sharing's metadata source** — IMDb vs. Kinopoisk (or another
  option), access/rate-limit/availability-from-Russia unverified for
  either. Needs its own brainstorming pass when that feature is picked
  up (step 5 above).
```

to:

```
- ~~Movie Sharing's metadata source~~ — resolved, TMDb (see
  `docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md`).
  Link parsing is IMDb-URL-only; TMDb's `/find` resolves the id. TMDb's
  Russia/Belarus IP block doesn't reach a Cloudzy-hosted backend, since
  the block is IP-based and Cloudzy has no Russia region.
```

- [ ] **Step 3: Run the full pre-merge verification suite**

Run: `cd backend && go test ./...`
Expected: PASS/SKIP across all packages.

Run: `make audit-backend`
Expected: `govulncheck` reports no known vulnerabilities.

Run: `make audit-frontend`
Expected: `npm audit` reports no vulnerabilities (or only pre-existing ones unrelated to this feature — no new dependency was added).

Run: `make scan-secrets`
Expected: `gitleaks` reports no leaks (in particular: confirm `backend/.env.example`'s new line is the placeholder, not the real token).

- [ ] **Step 4: Commit**

```bash
git add README.md PLANNING.md
git commit -m "docs: document watchlist env var, resolve PLANNING.md step 5"
```

## Self-Review

**Spec coverage:** Every section of `docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md` maps to a task — Prerequisites (noted in Global Constraints and Task 4), Data model (Task 1), TMDb integration (Task 2), API (Task 3 handlers + Task 4 routes), Validation (Task 3), Frontend (Tasks 5–7), Testing (each task's own verification step), Explicitly out of scope (nothing here builds Kinopoisk support, the URL-shortener merge, image proxying, or cast/runtime — confirmed absent from every task).

**Placeholder scan:** No `TBD`/`TODO`/"add appropriate handling"-style steps — every step above has literal code, exact commands, and exact expected output.

**Type consistency:** `TMDbMatch` (Task 1) is used identically in Task 2 (`FindByIMDbID` returns `*TMDbMatch`) and Task 1's own `Insert(ctx, imdbID string, m *TMDbMatch)`. `TMDbClient` (Task 2) is the exact type `Handler` (Task 3) depends on via `NewHandler(store *Store, tmdb TMDbClient)`. `WatchlistItem`'s JSON field names (Task 1) match `WatchlistItem` in `shared/api.ts` (Task 5) field-for-field. `createRequest{Link}` and `setViewedRequest{Viewed}` (Task 3) match the JSON bodies `addToWatchlist`/`setWatchlistItemViewed` (Task 5) send (`{link}`/`{viewed}`).
