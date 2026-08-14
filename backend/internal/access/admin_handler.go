// backend/internal/access/admin_handler.go
package access

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/platform"
	"github.com/jackc/pgx/v5/pgconn"
)

// userLister is the one thing AdminHandler needs from package auth,
// narrowed to just ListUsers — same "define the interface at the
// consumer" pattern auth.FeatureLister already uses for the reverse
// dependency. *auth.Store satisfies this structurally.
type userLister interface {
	ListUsers(ctx context.Context) ([]*auth.User, error)
	FindByID(ctx context.Context, id int64) (*auth.User, error)
	UpdateRole(ctx context.Context, id int64, role string) (*auth.User, error)
	CreateUser(ctx context.Context, username, passwordHash, role string) (*auth.User, error)
	SetActive(ctx context.Context, id int64, isActive bool) (*auth.User, error)
	SetPasswordHash(ctx context.Context, id int64, passwordHash string) (*auth.User, error)
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
	IsActive    bool     `json:"is_active"`
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
		converted, err := h.toResponse(r.Context(), u)
		if err != nil {
			slog.Error("list features for user", "user_id", u.ID, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp[i] = converted
	}
	platform.WriteJSON(w, http.StatusOK, resp)
}

// toResponse assembles the shared adminUserResponse shape, shared by
// ListUsers and CreateUser so the two paths can't drift into reporting
// different fields for the same user (same reasoning as auth.Handler's
// buildMeResponse).
func (h *AdminHandler) toResponse(ctx context.Context, u *auth.User) (adminUserResponse, error) {
	features, err := h.accessStore.ListForUser(ctx, u.ID)
	if err != nil {
		return adminUserResponse{}, err
	}
	return adminUserResponse{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		IsActive:    u.IsActive,
		Features:    features,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
	}, nil
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

// actorFromClaims extracts the caller's user id from the JWT claims
// auth.Require already verified, writing 401 if they're missing. Shared by
// every mutating handler below so each can record itself as the audit
// log's actor; UpdateRole/SetActive also reuse it for their self-guard
// checks instead of extracting claims twice.
func (h *AdminHandler) actorFromClaims(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return 0, false
	}
	callerID, err := claims.UserID()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return 0, false
	}
	return callerID, true
}

// logAction is a thin wrapper around accessStore.LogAction that only logs
// the failure — an audit-write error shouldn't turn an otherwise-successful
// admin action into a 500 for the caller, since the mutation itself already
// committed.
func (h *AdminHandler) logAction(ctx context.Context, actorID int64, action string, targetUserID *int64, detail string) {
	if err := h.accessStore.LogAction(ctx, actorID, action, targetUserID, detail); err != nil {
		slog.Error("log audit action", "action", action, "actor_id", actorID, "error", err)
	}
}

type auditEntryResponse struct {
	ActorUsername  string  `json:"actor_username"`
	Action         string  `json:"action"`
	TargetUsername *string `json:"target_username"`
	Detail         string  `json:"detail"`
	CreatedAt      string  `json:"created_at"`
}

// AuditLog returns the most recent admin-action log entries, newest first.
func (h *AdminHandler) AuditLog(w http.ResponseWriter, r *http.Request) {
	entries, err := h.accessStore.ListAuditLog(r.Context())
	if err != nil {
		slog.Error("list audit log", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	resp := make([]auditEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = auditEntryResponse{
			ActorUsername:  e.ActorUsername,
			Action:         e.Action,
			TargetUsername: e.TargetUsername,
			Detail:         e.Detail,
			CreatedAt:      e.CreatedAt.Format(time.RFC3339),
		}
	}
	platform.WriteJSON(w, http.StatusOK, resp)
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
	actorID, ok := h.actorFromClaims(w, r)
	if !ok {
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
	h.logAction(r.Context(), actorID, "grant_feature", &userID, "feature="+key)
	w.WriteHeader(http.StatusNoContent)
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// CreateUser is the UI equivalent of cmd/createuser — same validation
// (non-empty username/password, a known role) and same bcrypt hashing,
// duplicated rather than shared with the CLI since one lives in package
// main and the other in package access, and it's a handful of lines.
// Unlike the CLI, there's no -display-name flag: the created user sets
// their own via the profile popover after first login (see
// planning/decisions.md's "user lifecycle management" entry).
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		http.Error(w, "username must not be empty", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "password must not be empty", http.StatusBadRequest)
		return
	}
	if req.Role != "admin" && req.Role != "user" {
		http.Error(w, `role must be "admin" or "user"`, http.StatusBadRequest)
		return
	}
	actorID, ok := h.actorFromClaims(w, r)
	if !ok {
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("hash password", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	user, err := h.users.CreateUser(r.Context(), req.Username, hash, req.Role)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "username already exists", http.StatusConflict)
			return
		}
		slog.Error("create user", "username", req.Username, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.logAction(r.Context(), actorID, "create_user", &user.ID, "role="+req.Role)

	resp, err := h.toResponse(r.Context(), user)
	if err != nil {
		slog.Error("list features for user", "user_id", user.ID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	platform.WriteJSON(w, http.StatusOK, resp)
}

type setActiveRequest struct {
	IsActive bool `json:"is_active"`
}

// SetActive activates/deactivates user {id} — a soft, reversible
// alternative to deleting the row (see planning/decisions.md). Mirrors
// UpdateRole's shape exactly, including the self-guard: an admin can't
// lock themselves out through this endpoint, checked server-side since a
// UI-only disabled control is trivially bypassed with curl.
func (h *AdminHandler) SetActive(w http.ResponseWriter, r *http.Request) {
	userID, ok := platform.IDParam(r)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req setActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed JSON body", http.StatusBadRequest)
		return
	}

	callerID, ok := h.actorFromClaims(w, r)
	if !ok {
		return
	}
	if callerID == userID {
		http.Error(w, "can't deactivate your own account", http.StatusForbidden)
		return
	}

	if !h.requireExistingUser(w, r, userID) {
		return
	}
	if _, err := h.users.SetActive(r.Context(), userID, req.IsActive); err != nil {
		slog.Error("set active", "user_id", userID, "is_active", req.IsActive, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.logAction(r.Context(), callerID, "set_active", &userID, "is_active="+strconv.FormatBool(req.IsActive))
	w.WriteHeader(http.StatusNoContent)
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

// ResetPassword overwrites user {id}'s password with an admin-supplied
// value. No self-guard needed (unlike UpdateRole/SetActive) — an admin
// resetting their own password isn't a privilege-loss risk the way
// self-demote/self-deactivate are. The temp password itself is never
// stored or logged; only its bcrypt hash persists.
func (h *AdminHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := platform.IDParam(r)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed JSON body", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "password must not be empty", http.StatusBadRequest)
		return
	}
	actorID, ok := h.actorFromClaims(w, r)
	if !ok {
		return
	}

	if !h.requireExistingUser(w, r, userID) {
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("hash password", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if _, err := h.users.SetPasswordHash(r.Context(), userID, hash); err != nil {
		slog.Error("reset password", "user_id", userID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.logAction(r.Context(), actorID, "reset_password", &userID, "")
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
	actorID, ok := h.actorFromClaims(w, r)
	if !ok {
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
	h.logAction(r.Context(), actorID, "revoke_feature", &userID, "feature="+key)
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

	callerID, ok := h.actorFromClaims(w, r)
	if !ok {
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
	h.logAction(r.Context(), callerID, "update_role", &userID, "role="+req.Role)
	w.WriteHeader(http.StatusNoContent)
}
