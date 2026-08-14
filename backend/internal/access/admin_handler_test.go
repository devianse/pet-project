// backend/internal/access/admin_handler_test.go
package access

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/devianse/pet-project/backend/internal/auth"
)

func TestAdminHandler_ListUsers_ReturnsActualGrantsNotBypass(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	userID := createTestUser(t, authStore, "access-mike", "user")
	adminID := createTestUser(t, authStore, "access-admin", "admin")

	if err := accessStore.Grant(ctx, userID, "notes"); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	handler := NewAdminHandler(accessStore, authStore)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()
	handler.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp []adminUserResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	var mike, admin *adminUserResponse
	for i := range resp {
		switch resp[i].ID {
		case userID:
			mike = &resp[i]
		case adminID:
			admin = &resp[i]
		}
	}
	if mike == nil || admin == nil {
		t.Fatalf("expected both fixture users in the response, got %+v", resp)
	}
	if len(mike.Features) != 1 || mike.Features[0] != "notes" {
		t.Fatalf("expected access-mike to show [notes], got %v", mike.Features)
	}
	// Admin's real feature_access rows are empty — this endpoint must use
	// ListForUser (actual grants), not ListAllForUser (bypass-inflated).
	if len(admin.Features) != 0 {
		t.Fatalf("expected admin's actual grants to be empty (no bypass inflation), got %v", admin.Features)
	}
}

func TestAdminHandler_GrantFeature_ThenListForUserShowsIt(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	userID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+strconv.FormatInt(userID, 10)+"/features/notes", nil)
	req.SetPathValue("id", strconv.FormatInt(userID, 10))
	req.SetPathValue("key", "notes")
	rec := httptest.NewRecorder()
	handler.GrantFeature(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	features, err := accessStore.ListForUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(features) != 1 || features[0] != "notes" {
		t.Fatalf("expected [notes] after grant, got %v", features)
	}
}

func TestAdminHandler_GrantFeature_UnknownKeyRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	userID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+strconv.FormatInt(userID, 10)+"/features/bogus", nil)
	req.SetPathValue("id", strconv.FormatInt(userID, 10))
	req.SetPathValue("key", "bogus")
	rec := httptest.NewRecorder()
	handler.GrantFeature(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown feature key, got %d", rec.Code)
	}
}

func TestAdminHandler_RevokeFeature_RemovesGrant(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	userID := createTestUser(t, authStore, "access-mike", "user")
	if err := accessStore.Grant(ctx, userID, "notes"); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	handler := NewAdminHandler(accessStore, authStore)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(userID, 10)+"/features/notes", nil)
	req.SetPathValue("id", strconv.FormatInt(userID, 10))
	req.SetPathValue("key", "notes")
	rec := httptest.NewRecorder()
	handler.RevokeFeature(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	features, err := accessStore.ListForUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(features) != 0 {
		t.Fatalf("expected no features after revoke, got %v", features)
	}
}

func TestAdminHandler_RevokeFeature_NothingToRevokeIsStillNoContent(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	userID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(userID, 10)+"/features/notes", nil)
	req.SetPathValue("id", strconv.FormatInt(userID, 10))
	req.SetPathValue("key", "notes")
	rec := httptest.NewRecorder()
	handler.RevokeFeature(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 even with nothing to revoke, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_GrantFeature_InvalidIDRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	handler := NewAdminHandler(accessStore, authStore)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/not-a-number/features/notes", nil)
	req.SetPathValue("id", "not-a-number")
	req.SetPathValue("key", "notes")
	rec := httptest.NewRecorder()
	handler.GrantFeature(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-numeric id, got %d", rec.Code)
	}
}

func TestAdminHandler_GrantFeature_UnknownUserIDRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	handler := NewAdminHandler(accessStore, authStore)

	// A numeric id that parses fine but matches no row — distinct from
	// the malformed-id case above, and from Grant's own FK constraint
	// (which would otherwise surface as a generic 500, not a clean 404).
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/999999999/features/notes", nil)
	req.SetPathValue("id", "999999999")
	req.SetPathValue("key", "notes")
	rec := httptest.NewRecorder()
	handler.GrantFeature(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown user id, got %d: %s", rec.Code, rec.Body.String())
	}
}

// performUpdateRole builds a PUT /api/admin/users/{id}/role request,
// wrapped in auth.Require so callerID's claims land in the request
// context the same way the real middleware chain (requireAdmin ->
// auth.Require) would produce them — UpdateRole reads the caller's own
// id off those claims to enforce the self-demote guard.
func performUpdateRole(t *testing.T, handler *AdminHandler, callerID, targetID int64, callerRole, body string) *httptest.ResponseRecorder {
	t.Helper()
	secret := []byte("test-secret")
	token, err := auth.SignToken(secret, callerID, "caller", callerRole)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+strconv.FormatInt(targetID, 10)+"/role", strings.NewReader(body))
	req.SetPathValue("id", strconv.FormatInt(targetID, 10))
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	rec := httptest.NewRecorder()

	chained := auth.Require(secret)(http.HandlerFunc(handler.UpdateRole))
	chained.ServeHTTP(rec, req)
	return rec
}

func TestAdminHandler_UpdateRole_ChangesRole(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performUpdateRole(t, handler, callerID, targetID, "admin", `{"role":"admin"}`)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	updated, err := authStore.FindByID(ctx, targetID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if updated.Role != "admin" {
		t.Fatalf("expected role %q, got %q", "admin", updated.Role)
	}
}

func TestAdminHandler_UpdateRole_UnknownRoleRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performUpdateRole(t, handler, callerID, targetID, "admin", `{"role":"superuser"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown role, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_UpdateRole_UnknownUserIDRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	callerID := createTestUser(t, authStore, "access-admin", "admin")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performUpdateRole(t, handler, callerID, 999999999, "admin", `{"role":"admin"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown user id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_CreateUser_CreatesActiveUser(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()

	handler := NewAdminHandler(accessStore, authStore)
	body := `{"username":"access-newbie","password":"s3cret-pass","role":"user"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.CreateUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp adminUserResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Username != "access-newbie" || resp.Role != "user" || !resp.IsActive {
		t.Fatalf("unexpected created user: %+v", resp)
	}

	created, err := authStore.FindByUsername(ctx, "access-newbie")
	if err != nil {
		t.Fatalf("FindByUsername: %v", err)
	}
	if created == nil {
		t.Fatal("expected the user to actually be persisted")
	}
}

func TestAdminHandler_CreateUser_DuplicateUsernameRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	body := `{"username":"access-mike","password":"s3cret-pass","role":"user"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.CreateUser(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a duplicate username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_CreateUser_UnknownRoleRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)

	handler := NewAdminHandler(accessStore, authStore)
	body := `{"username":"access-newbie","password":"s3cret-pass","role":"superuser"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.CreateUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown role, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_CreateUser_EmptyPasswordRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)

	handler := NewAdminHandler(accessStore, authStore)
	body := `{"username":"access-newbie","password":"","role":"user"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.CreateUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty password, got %d: %s", rec.Code, rec.Body.String())
	}
}

// performSetActive mirrors performUpdateRole's shape (real claims in
// context via auth.Require) since SetActive enforces the same
// self-deactivate guard UpdateRole enforces for self-demote.
func performSetActive(t *testing.T, handler *AdminHandler, callerID, targetID int64, callerRole, body string) *httptest.ResponseRecorder {
	t.Helper()
	secret := []byte("test-secret")
	token, err := auth.SignToken(secret, callerID, "caller", callerRole)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+strconv.FormatInt(targetID, 10)+"/active", strings.NewReader(body))
	req.SetPathValue("id", strconv.FormatInt(targetID, 10))
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	rec := httptest.NewRecorder()

	chained := auth.Require(secret)(http.HandlerFunc(handler.SetActive))
	chained.ServeHTTP(rec, req)
	return rec
}

func TestAdminHandler_SetActive_DeactivatesUser(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performSetActive(t, handler, callerID, targetID, "admin", `{"is_active":false}`)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	updated, err := authStore.FindByID(ctx, targetID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if updated.IsActive {
		t.Fatalf("expected is_active false, got %+v", updated)
	}
}

func TestAdminHandler_SetActive_SelfDeactivateRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performSetActive(t, handler, callerID, callerID, "admin", `{"is_active":false}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when an admin targets their own id, got %d: %s", rec.Code, rec.Body.String())
	}

	unchanged, err := authStore.FindByID(ctx, callerID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !unchanged.IsActive {
		t.Fatalf("expected caller to stay active, got %+v", unchanged)
	}
}

func TestAdminHandler_SetActive_UnknownUserIDRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	callerID := createTestUser(t, authStore, "access-admin", "admin")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performSetActive(t, handler, callerID, 999999999, "admin", `{"is_active":false}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown user id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_ResetPassword_SetsNewPassword(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+strconv.FormatInt(targetID, 10)+"/reset-password", strings.NewReader(`{"password":"new-s3cret"}`))
	req.SetPathValue("id", strconv.FormatInt(targetID, 10))
	rec := httptest.NewRecorder()
	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	updated, err := authStore.FindByID(ctx, targetID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !auth.VerifyPassword(updated.PasswordHash, "new-s3cret") {
		t.Fatal("expected the new password to verify against the stored hash")
	}
}

func TestAdminHandler_ResetPassword_EmptyPasswordRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+strconv.FormatInt(targetID, 10)+"/reset-password", strings.NewReader(`{"password":""}`))
	req.SetPathValue("id", strconv.FormatInt(targetID, 10))
	rec := httptest.NewRecorder()
	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty password, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_ResetPassword_UnknownUserIDRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)

	handler := NewAdminHandler(accessStore, authStore)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/999999999/reset-password", strings.NewReader(`{"password":"new-s3cret"}`))
	req.SetPathValue("id", "999999999")
	rec := httptest.NewRecorder()
	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown user id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_UpdateRole_SelfChangeRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performUpdateRole(t, handler, callerID, callerID, "admin", `{"role":"user"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when an admin targets their own id, got %d: %s", rec.Code, rec.Body.String())
	}

	unchanged, err := authStore.FindByID(ctx, callerID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if unchanged.Role != "admin" {
		t.Fatalf("expected role to stay %q, got %q", "admin", unchanged.Role)
	}
}
