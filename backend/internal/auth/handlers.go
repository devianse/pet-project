package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Handler struct {
	store         *Store
	secret        []byte
	secureCookies bool
}

func NewHandler(store *Store, secret []byte, secureCookies bool) *Handler {
	return &Handler{store: store, secret: secret, secureCookies: secureCookies}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type meResponse struct {
	Username    string  `json:"username"`
	DisplayName *string `json:"display_name"`
	Role        string  `json:"role"`
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
	if user == nil || !VerifyPassword(user.PasswordHash, req.Password) {
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

	writeJSON(w, http.StatusOK, meResponse{Username: user.Username, DisplayName: user.DisplayName, Role: user.Role})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	clearAuthCookie(w, h.secureCookies)
	w.WriteHeader(http.StatusNoContent)
}

// Me looks the user up fresh from the store rather than trusting the JWT
// claims verbatim, so a display_name/role change since the token was
// issued shows up immediately instead of only after the next login.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	claims, err := ParseToken(h.secret, cookie.Value)
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
	writeJSON(w, http.StatusOK, meResponse{Username: user.Username, DisplayName: user.DisplayName, Role: user.Role})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
