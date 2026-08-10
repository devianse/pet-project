package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/devianse/pet-project/backend/internal/platform"
)

const maxContentLength = 10000

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	notes, err := h.store.List(r.Context())
	if err != nil {
		slog.Error("list notes", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	platform.WriteJSON(w, http.StatusOK, notes)
}

type createRequest struct {
	Items []string `json:"items"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed JSON body", http.StatusBadRequest)
		return
	}
	if len(req.Items) == 0 {
		http.Error(w, "items must not be empty", http.StatusBadRequest)
		return
	}
	// All-or-nothing: validate every item before inserting any of them,
	// so the frontend never has to reconcile a partially-saved batch.
	for _, item := range req.Items {
		if err := validateContent(item); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	notes, err := h.store.InsertBatch(r.Context(), req.Items)
	if err != nil {
		slog.Error("insert notes", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	platform.WriteJSON(w, http.StatusOK, notes)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := platform.IDParam(r)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	found, err := h.store.Delete(r.Context(), id)
	if err != nil {
		slog.Error("delete note", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return errors.New("content must not be empty")
	}
	if len(content) > maxContentLength {
		return fmt.Errorf("content exceeds maximum length of %d characters", maxContentLength)
	}
	return nil
}
