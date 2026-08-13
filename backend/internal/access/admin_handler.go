// backend/internal/access/admin_handler.go
package access

import (
	"context"
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
