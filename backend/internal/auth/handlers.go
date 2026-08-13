package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/devianse/pet-project/backend/internal/platform"
)

// FeatureLister is the one thing Handler needs from package access to
// populate meResponse.Features. Defined here (not imported from access)
// so package auth never imports package access — access.RequireFeature
// already imports auth for ClaimsFromContext, and auth importing access
// back would cycle. access.Store satisfies this interface structurally;
// only cmd/api/main.go and test files need to import both packages to
// wire the concrete type in.
type FeatureLister interface {
	ListAllForUser(ctx context.Context, userID int64, role string) ([]string, error)
}

type Handler struct {
	store         *Store
	secret        []byte
	secureCookies bool
	features      FeatureLister
}

func NewHandler(store *Store, secret []byte, secureCookies bool, features FeatureLister) *Handler {
	return &Handler{store: store, secret: secret, secureCookies: secureCookies, features: features}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type meResponse struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	DisplayName *string  `json:"display_name"`
	Role        string   `json:"role"`
	Features    []string `json:"features"`
}

// dummyHash is a valid bcrypt hash of no real password. Used to make
// Login pay the same bcrypt cost whether the username exists or not,
// closing a timing side-channel that would otherwise let a caller
// distinguish "unknown username" from "wrong password" by response time.
const dummyHash = "$2a$10$CwTycUXWue0Thq9StjUM0uJ8G8xtObW/gxSbeMhWaBaFTUKtBBWnu"

// buildMeResponse resolves the caller's feature set and assembles the
// shared response shape both Login and Me return — kept in one place so
// the two paths can't silently drift into reporting different fields for
// the same user.
func (h *Handler) buildMeResponse(ctx context.Context, user *User) (meResponse, error) {
	features, err := h.features.ListAllForUser(ctx, user.ID, user.Role)
	if err != nil {
		return meResponse{}, err
	}
	return meResponse{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Features:    features,
	}, nil
}

// Login authenticates a username/password pair and, on success, sets the
// session cookie. Every failure path (unknown username, wrong password)
// returns the same generic 401 message — no signal that would let a
// caller enumerate valid usernames.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed JSON body", http.StatusBadRequest)
		return
	}

	user, err := h.store.FindByUsername(r.Context(), req.Username)
	if err != nil {
		slog.Error("find user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	hashToCheck := dummyHash
	if user != nil {
		hashToCheck = user.PasswordHash
	}
	passwordOK := VerifyPassword(hashToCheck, req.Password)

	if user == nil || !passwordOK {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := SignToken(h.secret, user.ID, user.Username, user.Role)
	if err != nil {
		slog.Error("sign token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	setAuthCookie(w, token, h.secureCookies)

	// A failure updating last_login_at shouldn't fail the login itself —
	// it's a nice-to-have audit field, not part of the auth decision.
	if err := h.store.UpdateLastLogin(r.Context(), user.ID); err != nil {
		slog.Error("update last login", "error", err)
	}

	resp, err := h.buildMeResponse(r.Context(), user)
	if err != nil {
		slog.Error("building me response", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	platform.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	clearAuthCookie(w, h.secureCookies)
	w.WriteHeader(http.StatusNoContent)
}

// Me looks the user up fresh from the store rather than trusting the JWT
// claims verbatim, so a display_name/role change since the token was
// issued shows up immediately instead of only after the next login.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims, err := claimsFromRequest(r, h.secret)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, err := claims.UserID()
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.store.FindByID(r.Context(), userID)
	if err != nil {
		slog.Error("find user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	resp, err := h.buildMeResponse(r.Context(), user)
	if err != nil {
		slog.Error("building me response", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	platform.WriteJSON(w, http.StatusOK, resp)
}
