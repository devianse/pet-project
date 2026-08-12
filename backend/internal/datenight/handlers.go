package datenight

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/devianse/pet-project/backend/internal/auth"
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

type proposalsResponse struct {
	Current *Proposal  `json:"current"`
	History []Proposal `json:"history"`
}

func (h *Handler) ListProposals(w http.ResponseWriter, r *http.Request) {
	proposals, err := h.store.ListProposals(r.Context())
	if err != nil {
		slog.Error("list date night proposals", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	resp := proposalsResponse{History: []Proposal{}}
	if len(proposals) > 0 {
		current := proposals[0]
		resp.Current = &current
		resp.History = proposals[1:]
	}
	platform.WriteJSON(w, http.StatusOK, resp)
}

type createProposalRequest struct {
	ActivityID  int64       `json:"activity_id"`
	Date        string      `json:"date"`
	TimeSlot    TimeSlot    `json:"time_slot"`
	EnergyLevel EnergyLevel `json:"energy_level"`
	Moods       []Mood      `json:"moods"`
}

func (h *Handler) CreateProposal(w http.ResponseWriter, r *http.Request) {
	var req createProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed JSON body", http.StatusBadRequest)
		return
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		http.Error(w, "date must be in YYYY-MM-DD format", http.StatusBadRequest)
		return
	}
	if err := validateProposalInput(req.TimeSlot, req.EnergyLevel, req.Moods); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, err := claims.UserID()
	if err != nil {
		http.Error(w, "invalid session", http.StatusUnauthorized)
		return
	}

	proposal, err := h.store.CreateProposal(r.Context(), req.ActivityID, date, req.TimeSlot, req.EnergyLevel, req.Moods, userID)
	if errors.Is(err, ErrUnknownActivity) {
		http.Error(w, "unknown activity_id", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("create date night proposal", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	platform.WriteJSON(w, http.StatusOK, proposal)
}

func (h *Handler) AcceptProposal(w http.ResponseWriter, r *http.Request) {
	h.setProposalStatus(w, r, StatusAccepted)
}

func (h *Handler) DeclineProposal(w http.ResponseWriter, r *http.Request) {
	h.setProposalStatus(w, r, StatusDeclined)
}

func (h *Handler) setProposalStatus(w http.ResponseWriter, r *http.Request, status ProposalStatus) {
	id, ok := platform.IDParam(r)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, err := claims.UserID()
	if err != nil {
		http.Error(w, "invalid session", http.StatusUnauthorized)
		return
	}

	proposal, err := h.store.SetProposalStatus(r.Context(), id, status, userID)
	if errors.Is(err, ErrProposalNotActionable) {
		http.Error(w, "that proposal isn't yours to answer, or it's no longer the current pending one", http.StatusConflict)
		return
	}
	if err != nil {
		slog.Error("set date night proposal status", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	platform.WriteJSON(w, http.StatusOK, proposal)
}

func validateProposalInput(slot TimeSlot, energy EnergyLevel, moods []Mood) error {
	if !IsValidTimeSlot(slot) {
		return errors.New("invalid time_slot")
	}
	if !IsValidEnergyLevel(energy) {
		return errors.New("invalid energy_level")
	}
	if len(moods) == 0 {
		return errors.New("at least one mood is required")
	}
	for _, m := range moods {
		if !IsValidMood(m) {
			return fmt.Errorf("invalid mood: %s", m)
		}
	}
	return nil
}
