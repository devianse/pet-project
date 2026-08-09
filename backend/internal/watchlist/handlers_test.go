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
