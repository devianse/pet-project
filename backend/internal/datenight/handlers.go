package datenight

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/devianse/pet-project/backend/internal/platform"
)

const (
	maxActivityNameLength        = 200
	maxActivityDescriptionLength = 2000
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) ListActivities(w http.ResponseWriter, r *http.Request) {
	activities, err := h.store.ListActivities(r.Context())
	if err != nil {
		slog.Error("list date night activities", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	platform.WriteJSON(w, http.StatusOK, activities)
}

type createActivityRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Category    Category `json:"category"`
}

func (h *Handler) CreateActivity(w http.ResponseWriter, r *http.Request) {
	var req createActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed JSON body", http.StatusBadRequest)
		return
	}
	if err := validateActivityInput(req.Name, req.Description, req.Category); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	activity, err := h.store.CreateActivity(r.Context(), req.Name, req.Description, req.Category)
	if err != nil {
		slog.Error("create date night activity", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	platform.WriteJSON(w, http.StatusOK, activity)
}

func (h *Handler) DeleteActivity(w http.ResponseWriter, r *http.Request) {
	id, ok := platform.IDParam(r)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	found, err := h.store.DeleteActivity(r.Context(), id)
	if errors.Is(err, ErrActivityInUse) {
		http.Error(w, "that activity is part of a date proposal — it can't be removed", http.StatusConflict)
		return
	}
	if err != nil {
		slog.Error("delete date night activity", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateActivityInput(name string, description *string, category Category) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name must not be empty")
	}
	if len(name) > maxActivityNameLength {
		return errors.New("name exceeds maximum length")
	}
	if description != nil && len(*description) > maxActivityDescriptionLength {
		return errors.New("description exceeds maximum length")
	}
	if !IsValidCategory(category) {
		return errors.New("invalid category")
	}
	return nil
}
