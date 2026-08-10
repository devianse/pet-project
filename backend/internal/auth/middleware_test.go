// backend/internal/auth/middleware_test.go
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequire_RejectsMissingCookie(t *testing.T) {
	secret := []byte("test-secret")
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	rec := httptest.NewRecorder()

	Require(secret)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if called {
		t.Fatal("expected next handler not to be called without a cookie")
	}
}

func TestRequire_RejectsInvalidCookie(t *testing.T) {
	secret := []byte("test-secret")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "not-a-real-token"})
	rec := httptest.NewRecorder()

	Require(secret)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequire_AllowsValidCookieAndInjectsClaims(t *testing.T) {
	secret := []byte("test-secret")
	tokenString, err := SignToken(secret, 7, "mike", "admin")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	var gotClaims *Claims
	var gotOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims, gotOK = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tokenString})
	rec := httptest.NewRecorder()

	Require(secret)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !gotOK || gotClaims.Username != "mike" {
		t.Fatalf("expected claims injected into context, got ok=%v claims=%+v", gotOK, gotClaims)
	}
}
