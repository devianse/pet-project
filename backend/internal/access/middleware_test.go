// backend/internal/access/middleware_test.go
package access

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devianse/pet-project/backend/internal/auth"
)

func TestRequireFeature_AdminBypassesWithoutGrant(t *testing.T) {
	// &Store{} is safe here: admin's path in HasFeature returns before
	// ever touching store.conn, so no real DB connection is needed for
	// this pure-unit case.
	store := &Store{}
	terminal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RequireFeature(store, "notes")(terminal)

	secret := []byte("test-secret")
	token, err := auth.SignToken(secret, 1, "access-admin", "admin")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	rec := httptest.NewRecorder()

	chained := auth.Require(secret)(handler)
	chained.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin bypass, got %d", rec.Code)
	}
}

func TestRequireFeature_RejectsMissingClaims(t *testing.T) {
	store := &Store{}
	terminal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RequireFeature(store, "notes")(terminal)

	// No auth.Require wrapping this request — no claims in context at all,
	// which RequireFeature must treat as unauthorized rather than panic.
	req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no claims in context, got %d", rec.Code)
	}
}

func TestRequireFeature_NonAdminNeedsGrant(t *testing.T) {
	store, authStore := setupAccessStore(t)
	ctx := context.Background()
	userID := createTestUser(t, authStore, "access-mw", "user")

	terminal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RequireFeature(store, "notes")(terminal)

	secret := []byte("test-secret")
	token, err := auth.SignToken(secret, userID, "access-mw", "user")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	req := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
		r.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
		return r
	}
	chained := auth.Require(secret)(handler)

	rec := httptest.NewRecorder()
	chained.ServeHTTP(rec, req())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 before grant, got %d", rec.Code)
	}

	if err := store.Grant(ctx, userID, "notes"); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	rec = httptest.NewRecorder()
	chained.ServeHTTP(rec, req())
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after grant, got %d", rec.Code)
	}
}
