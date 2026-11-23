// backend/internal/auth/middleware_test.go
package auth

import (
	"context"
	"errors"
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

// fakeLiveness is a stub Liveness so RequireActive can be tested without
// a database — the store's own IsActive is covered by store_test.go.
type fakeLiveness struct {
	active bool
	err    error
}

func (f fakeLiveness) IsActive(ctx context.Context, id int64) (bool, error) {
	return f.active, f.err
}

// chained runs Require then RequireActive the way cmd/api wires them.
func chained(secret []byte, liveness Liveness, next http.Handler) http.Handler {
	return Require(secret)(RequireActive(liveness)(next))
}

func TestRequireActive_AllowsActiveUser(t *testing.T) {
	secret := []byte("test-secret")
	tokenString, err := SignToken(secret, 7, "mike", "user")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tokenString})
	rec := httptest.NewRecorder()

	chained(secret, fakeLiveness{active: true}, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !called {
		t.Fatalf("expected 200 from next handler, got %d (called=%v)", rec.Code, called)
	}
}

// The regression this whole change exists for: a valid, unexpired cookie
// must stop working the moment the user is deactivated, since the token
// itself has no revocation path and lives for TokenTTL (14 days).
func TestRequireActive_BlocksDeactivatedUser(t *testing.T) {
	secret := []byte("test-secret")
	tokenString, err := SignToken(secret, 7, "mike", "admin")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tokenString})
	rec := httptest.NewRecorder()

	chained(secret, fakeLiveness{active: false}, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a deactivated user, got %d", rec.Code)
	}
	if called {
		t.Fatal("next handler must not run for a deactivated user")
	}
}

// A deleted user row reports (false, nil) from the store; a lookup error
// must fail closed too rather than wave the request through.
func TestRequireActive_FailsClosed(t *testing.T) {
	secret := []byte("test-secret")
	tokenString, err := SignToken(secret, 7, "mike", "user")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run when the liveness check fails")
	})

	for name, liveness := range map[string]Liveness{
		"missing row": fakeLiveness{active: false},
		"db error":    fakeLiveness{err: errors.New("connection reset")},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
			req.AddCookie(&http.Cookie{Name: cookieName, Value: tokenString})
			rec := httptest.NewRecorder()
			chained(secret, liveness, next).ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
		})
	}
}
