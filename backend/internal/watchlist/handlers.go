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
