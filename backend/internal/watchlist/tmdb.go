package watchlist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
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
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
		token:      token,
	}

	c.movieGenres = c.fetchGenreMapOrEmpty(ctx, "/genre/movie/list")
	c.tvGenres = c.fetchGenreMapOrEmpty(ctx, "/genre/tv/list")
	return c, nil
}

// fetchGenreMapOrEmpty fetches a genre id->name map, logging and falling
// back to an empty map on failure. A TMDb outage during startup should
// degrade the genres field on watchlist items, not take down the whole
// server — Notes and /api/health don't depend on TMDb at all.
func (c *RealTMDbClient) fetchGenreMapOrEmpty(ctx context.Context, path string) map[int]string {
	m, err := c.fetchGenreMap(ctx, path)
	if err != nil {
		slog.Warn("fetching tmdb genre list, continuing with empty genre names", "path", path, "error", err)
		return map[int]string{}
	}
	return m
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
