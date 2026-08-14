// backend/internal/access/admin_handler.go
package access

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/platform"
)

// userLister is the one thing AdminHandler needs from package auth,
// narrowed to just ListUsers — same "define the interface at the
// consumer" pattern auth.FeatureLister already uses for the reverse
// dependency. *auth.Store satisfies this structurally.
type userLister interface {
	ListUsers(ctx context.Context) ([]*auth.User, error)
	FindByID(ctx context.Context, id int64) (*auth.User, error)
	UpdateRole(ctx context.Context, id int64, role string) (*auth.User, error)
}

// AdminHandler serves the admin-only user/feature-grant management API
// (GET/POST/DELETE /api/admin/users...). Gating is the route wiring's
// job (RequireRole(accessStore, "admin") in cmd/api/main.go) — this
// handler does no role checking itself.
type AdminHandler struct {
	accessStore *Store
	users       userLister
}

func NewAdminHandler(accessStore *Store, users userLister) *AdminHandler {
	return &AdminHandler{accessStore: accessStore, users: users}
}

type adminUserResponse struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	DisplayName *string  `json:"display_name"`
	Role        string   `json:"role"`
	Features    []string `json:"features"`
	CreatedAt   string   `json:"created_at"`
}

// ListUsers returns every user with their actual feature_access rows —
// ListForUser, not ListAllForUser, so an admin row shows real grants
// rather than the bypass-inflated "admin has everything" view (see
// docs/superpowers/specs/2026-08-14-admin-grants-ui-design.md).
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.ListUsers(r.Context())
	if err != nil {
		slog.Error("list users", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := make([]adminUserResponse, len(users))
	for i, u := range users {
		features, err := h.accessStore.ListForUser(r.Context(), u.ID)
		if err != nil {
			slog.Error("list features for user", "user_id", u.ID, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp[i] = adminUserResponse{
			ID:          u.ID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Role:        u.Role,
			Features:    features,
			CreatedAt:   u.CreatedAt.Format(time.RFC3339),
		}
	}
	platform.WriteJSON(w, http.StatusOK, resp)
}

// knownFeatureKeysCSV mirrors cmd/grantaccess's knownFeatureKeys helper.
// Duplicated rather than shared: one lives in package main, the other in
// package access, and it's three lines — not worth an exported helper
// for two call sites across a package boundary.
func knownFeatureKeysCSV() string {
	keys := make([]string, len(KnownFeatures))
	for i, f := range KnownFeatures {
		keys[i] = f.Key
	}
	return strings.Join(keys, ", ")
}

// requireExistingUser 404s if id doesn't match a real user, so a typo'd
// or stale id in the admin UI gets a clean not-found instead of falling
// through to Grant's feature_access foreign-key violation (a generic
// 500). Shared by GrantFeature and RevokeFeature.
func (h *AdminHandler) requireExistingUser(w http.ResponseWriter, r *http.Request, userID int64) bool {
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		slog.Error("find user", "user_id", userID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return false
	}
	if user == nil {
		http.Error(w, "no such user", http.StatusNotFound)
		return false
	}
	return true
}

// GrantFeature grants the {key} feature to user {id}. Idempotent, like
// accessStore.Grant and cmd/grantaccess's -grant flag.
func (h *AdminHandler) GrantFeature(w http.ResponseWriter, r *http.Request) {
	userID, ok := platform.IDParam(r)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	key := r.PathValue("key")
	if !IsKnownFeature(key) {
		http.Error(w, "unknown feature key (known: "+knownFeatureKeysCSV()+")", http.StatusBadRequest)
		return
	}
	if !h.requireExistingUser(w, r, userID) {
		return
	}
	if err := h.accessStore.Grant(r.Context(), userID, key); err != nil {
		slog.Error("grant feature", "user_id", userID, "key", key, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RevokeFeature revokes the {key} feature from user {id}. Also
// idempotent — revoking a feature the user never had is still a 204,
// matching accessStore.Revoke's bool-not-error signature for "nothing to
// revoke".
func (h *AdminHandler) RevokeFeature(w http.ResponseWriter, r *http.Request) {
	userID, ok := platform.IDParam(r)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	key := r.PathValue("key")
	if !IsKnownFeature(key) {
		http.Error(w, "unknown feature key (known: "+knownFeatureKeysCSV()+")", http.StatusBadRequest)
		return
	}
	if !h.requireExistingUser(w, r, userID) {
		return
	}
	if _, err := h.accessStore.Revoke(r.Context(), userID, key); err != nil {
		slog.Error("revoke feature", "user_id", userID, "key", key, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

// UpdateRole promotes/demotes user {id} to the given role. Mirrors
// GrantFeature/RevokeFeature's shape (validate input, 404 via
// requireExistingUser, write, 204) with one addition: an admin can't
// change their own role through this endpoint — a UI-only guard would
// be trivially bypassed with curl, and RequireRole already re-verifies
// role server-side for the same "don't trust a stale/spoofed claim"
// reason, so the self-check belongs here too, not just in the frontend.
func (h *AdminHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	userID, ok := platform.IDParam(r)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed JSON body", http.StatusBadRequest)
		return
	}
	if req.Role != "admin" && req.Role != "user" {
		http.Error(w, `role must be "admin" or "user"`, http.StatusBadRequest)
		return
	}

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	callerID, err := claims.UserID()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if callerID == userID {
		http.Error(w, "can't change your own role", http.StatusForbidden)
		return
	}

	if !h.requireExistingUser(w, r, userID) {
		return
	}
	if _, err := h.users.UpdateRole(r.Context(), userID, req.Role); err != nil {
		slog.Error("update role", "user_id", userID, "role", req.Role, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
