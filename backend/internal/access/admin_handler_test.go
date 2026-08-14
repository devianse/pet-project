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
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	userID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performGrantFeature(t, handler, callerID, userID, "admin", "notes")

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
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	userID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performGrantFeature(t, handler, callerID, userID, "admin", "bogus")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown feature key, got %d", rec.Code)
	}
}

func TestAdminHandler_RevokeFeature_RemovesGrant(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	userID := createTestUser(t, authStore, "access-mike", "user")
	if err := accessStore.Grant(ctx, userID, "notes"); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	handler := NewAdminHandler(accessStore, authStore)
	rec := performRevokeFeature(t, handler, callerID, userID, "admin", "notes")

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
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	userID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performRevokeFeature(t, handler, callerID, userID, "admin", "notes")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 even with nothing to revoke, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_GrantFeature_InvalidIDRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	handler := NewAdminHandler(accessStore, authStore)

	secret := []byte("test-secret")
	token, err := auth.SignToken(secret, callerID, "caller", "admin")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/not-a-number/features/notes", nil)
	req.SetPathValue("id", "not-a-number")
	req.SetPathValue("key", "notes")
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	rec := httptest.NewRecorder()
	auth.Require(secret)(http.HandlerFunc(handler.GrantFeature)).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-numeric id, got %d", rec.Code)
	}
}

func TestAdminHandler_GrantFeature_UnknownUserIDRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	handler := NewAdminHandler(accessStore, authStore)

	// A numeric id that parses fine but matches no row — distinct from
	// the malformed-id case above, and from Grant's own FK constraint
	// (which would otherwise surface as a generic 500, not a clean 404).
	rec := performGrantFeature(t, handler, callerID, 999999999, "admin", "notes")

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
	callerID := createTestUser(t, authStore, "access-admin", "admin")

	handler := NewAdminHandler(accessStore, authStore)
	body := `{"username":"access-newbie","password":"s3cret-pass","role":"user"}`
	rec := performCreateUser(t, handler, callerID, "admin", body)

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
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	body := `{"username":"access-mike","password":"s3cret-pass","role":"user"}`
	rec := performCreateUser(t, handler, callerID, "admin", body)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a duplicate username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_CreateUser_UnknownRoleRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	callerID := createTestUser(t, authStore, "access-admin", "admin")

	handler := NewAdminHandler(accessStore, authStore)
	body := `{"username":"access-newbie","password":"s3cret-pass","role":"superuser"}`
	rec := performCreateUser(t, handler, callerID, "admin", body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown role, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_CreateUser_EmptyPasswordRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	callerID := createTestUser(t, authStore, "access-admin", "admin")

	handler := NewAdminHandler(accessStore, authStore)
	body := `{"username":"access-newbie","password":"","role":"user"}`
	rec := performCreateUser(t, handler, callerID, "admin", body)

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
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performResetPassword(t, handler, callerID, targetID, "admin", `{"password":"new-s3cret"}`)

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
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performResetPassword(t, handler, callerID, targetID, "admin", `{"password":""}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty password, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_ResetPassword_UnknownUserIDRejected(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	callerID := createTestUser(t, authStore, "access-admin", "admin")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performResetPassword(t, handler, callerID, 999999999, "admin", `{"password":"new-s3cret"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown user id, got %d: %s", rec.Code, rec.Body.String())
	}
}

// performGrantFeature/performRevokeFeature/performCreateUser/
// performResetPassword mirror performUpdateRole/performSetActive's shape
// (real claims in context via auth.Require) now that every mutating admin
// handler records the caller as the audit log's actor.
func performGrantFeature(t *testing.T, handler *AdminHandler, callerID, targetID int64, callerRole, key string) *httptest.ResponseRecorder {
	t.Helper()
	secret := []byte("test-secret")
	token, err := auth.SignToken(secret, callerID, "caller", callerRole)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+strconv.FormatInt(targetID, 10)+"/features/"+key, nil)
	req.SetPathValue("id", strconv.FormatInt(targetID, 10))
	req.SetPathValue("key", key)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	rec := httptest.NewRecorder()

	chained := auth.Require(secret)(http.HandlerFunc(handler.GrantFeature))
	chained.ServeHTTP(rec, req)
	return rec
}

func performRevokeFeature(t *testing.T, handler *AdminHandler, callerID, targetID int64, callerRole, key string) *httptest.ResponseRecorder {
	t.Helper()
	secret := []byte("test-secret")
	token, err := auth.SignToken(secret, callerID, "caller", callerRole)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(targetID, 10)+"/features/"+key, nil)
	req.SetPathValue("id", strconv.FormatInt(targetID, 10))
	req.SetPathValue("key", key)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	rec := httptest.NewRecorder()

	chained := auth.Require(secret)(http.HandlerFunc(handler.RevokeFeature))
	chained.ServeHTTP(rec, req)
	return rec
}

func performCreateUser(t *testing.T, handler *AdminHandler, callerID int64, callerRole, body string) *httptest.ResponseRecorder {
	t.Helper()
	secret := []byte("test-secret")
	token, err := auth.SignToken(secret, callerID, "caller", callerRole)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	rec := httptest.NewRecorder()

	chained := auth.Require(secret)(http.HandlerFunc(handler.CreateUser))
	chained.ServeHTTP(rec, req)
	return rec
}

func performResetPassword(t *testing.T, handler *AdminHandler, callerID, targetID int64, callerRole, body string) *httptest.ResponseRecorder {
	t.Helper()
	secret := []byte("test-secret")
	token, err := auth.SignToken(secret, callerID, "caller", callerRole)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+strconv.FormatInt(targetID, 10)+"/reset-password", strings.NewReader(body))
	req.SetPathValue("id", strconv.FormatInt(targetID, 10))
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	rec := httptest.NewRecorder()

	chained := auth.Require(secret)(http.HandlerFunc(handler.ResetPassword))
	chained.ServeHTTP(rec, req)
	return rec
}

func TestAdminHandler_GrantFeature_LogsAuditEntry(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performGrantFeature(t, handler, callerID, targetID, "admin", "notes")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	entries, err := accessStore.ListAuditLog(ctx)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	if entries[0].ActorUsername != "access-admin" || entries[0].Action != "grant_feature" {
		t.Fatalf("unexpected audit entry: %+v", entries[0])
	}
	if entries[0].TargetUsername == nil || *entries[0].TargetUsername != "access-mike" {
		t.Fatalf("expected target access-mike, got %v", entries[0].TargetUsername)
	}
	if entries[0].Detail != "feature=notes" {
		t.Fatalf("expected detail %q, got %q", "feature=notes", entries[0].Detail)
	}
}

func TestAdminHandler_RevokeFeature_LogsAuditEntry(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")
	if err := accessStore.Grant(ctx, targetID, "notes"); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	handler := NewAdminHandler(accessStore, authStore)
	rec := performRevokeFeature(t, handler, callerID, targetID, "admin", "notes")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	entries, err := accessStore.ListAuditLog(ctx)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(entries) != 1 || entries[0].Action != "revoke_feature" || entries[0].Detail != "feature=notes" {
		t.Fatalf("unexpected audit entries: %+v", entries)
	}
}

func TestAdminHandler_CreateUser_LogsAuditEntry(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performCreateUser(t, handler, callerID, "admin", `{"username":"access-newbie","password":"s3cret-pass","role":"user"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	entries, err := accessStore.ListAuditLog(ctx)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(entries) != 1 || entries[0].Action != "create_user" {
		t.Fatalf("unexpected audit entries: %+v", entries)
	}
	if entries[0].TargetUsername == nil || *entries[0].TargetUsername != "access-newbie" {
		t.Fatalf("expected target access-newbie, got %v", entries[0].TargetUsername)
	}
}

func TestAdminHandler_SetActive_LogsAuditEntry(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performSetActive(t, handler, callerID, targetID, "admin", `{"is_active":false}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	entries, err := accessStore.ListAuditLog(ctx)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(entries) != 1 || entries[0].Action != "set_active" || entries[0].Detail != "is_active=false" {
		t.Fatalf("unexpected audit entries: %+v", entries)
	}
}

func TestAdminHandler_ResetPassword_LogsAuditEntry(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performResetPassword(t, handler, callerID, targetID, "admin", `{"password":"new-s3cret"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	entries, err := accessStore.ListAuditLog(ctx)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(entries) != 1 || entries[0].Action != "reset_password" {
		t.Fatalf("unexpected audit entries: %+v", entries)
	}
	if entries[0].TargetUsername == nil || *entries[0].TargetUsername != "access-mike" {
		t.Fatalf("expected target access-mike, got %v", entries[0].TargetUsername)
	}
}

func TestAdminHandler_UpdateRole_LogsAuditEntry(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	callerID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")

	handler := NewAdminHandler(accessStore, authStore)
	rec := performUpdateRole(t, handler, callerID, targetID, "admin", `{"role":"admin"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	entries, err := accessStore.ListAuditLog(ctx)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(entries) != 1 || entries[0].Action != "update_role" || entries[0].Detail != "role=admin" {
		t.Fatalf("unexpected audit entries: %+v", entries)
	}
}

func TestAdminHandler_AuditLog_ReturnsEntries(t *testing.T) {
	accessStore, authStore := setupAccessStore(t)
	ctx := context.Background()
	actorID := createTestUser(t, authStore, "access-admin", "admin")
	targetID := createTestUser(t, authStore, "access-mike", "user")
	if err := accessStore.LogAction(ctx, actorID, "grant_feature", &targetID, "feature=notes"); err != nil {
		t.Fatalf("LogAction: %v", err)
	}

	handler := NewAdminHandler(accessStore, authStore)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-log", nil)
	rec := httptest.NewRecorder()
	handler.AuditLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp []auditEntryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(resp))
	}
	if resp[0].ActorUsername != "access-admin" || resp[0].Action != "grant_feature" {
		t.Fatalf("unexpected entry: %+v", resp[0])
	}
	if resp[0].TargetUsername == nil || *resp[0].TargetUsername != "access-mike" {
		t.Fatalf("expected target access-mike, got %v", resp[0].TargetUsername)
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
