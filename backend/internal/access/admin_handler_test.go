// backend/internal/access/admin_handler_test.go
package access

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
