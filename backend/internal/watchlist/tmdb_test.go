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
