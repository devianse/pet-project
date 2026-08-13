package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeFeatureLister is a test double for auth.FeatureLister. A real
// access.Store can't be used here: access imports auth (for
// ClaimsFromContext), so a package-auth internal test file importing
// access would close an import cycle. Seeded per-test via direct map
// writes instead of real Grant calls.
type fakeFeatureLister struct {
	features map[int64][]string // userID -> resolved feature set
}

func (f *fakeFeatureLister) ListAllForUser(ctx context.Context, userID int64, role string) ([]string, error) {
	return f.features[userID], nil
}

func newTestHandler(t *testing.T) (*Handler, *Store, *fakeFeatureLister) {
	t.Helper()
	store, _ := setupStore(t)
	features := &fakeFeatureLister{features: map[int64][]string{}}
	handler := NewHandler(store, []byte("test-secret"), false, features)
	return handler, store, features
}

func TestHandler_Login_Success(t *testing.T) {
	handler, store, _ := newTestHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := store.CreateUser(ctx, "mike", hash, "admin"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	body, _ := json.Marshal(loginRequest{Username: "mike", Password: "s3cret-pass"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != cookieName || cookies[0].Value == "" {
		t.Fatalf("expected an auth_token cookie to be set, got %+v", cookies)
	}

	var resp meResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Username != "mike" || resp.Role != "admin" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestHandler_Login_WrongPassword(t *testing.T) {
	handler, store, _ := newTestHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := store.CreateUser(ctx, "mike", hash, "admin"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	body, _ := json.Marshal(loginRequest{Username: "mike", Password: "wrong-pass"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandler_Login_UnknownUsername(t *testing.T) {
	handler, _, _ := newTestHandler(t)

	body, _ := json.Marshal(loginRequest{Username: "nobody", Password: "whatever"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// TestHandler_Login_UnknownUsername_PaysBcryptCost guards against a
// regression of the timing side-channel: an unknown username must still
// run VerifyPassword (against the dummy hash) rather than short-circuiting,
// so its response time is comparable to a known-username/wrong-password
// login rather than near-instant. We can't reliably assert exact timing
// in a unit test, but we can assert both paths take at least roughly the
// same, bcrypt-dominated order of magnitude.
func TestHandler_Login_UnknownUsername_PaysBcryptCost(t *testing.T) {
	handler, store, _ := newTestHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := store.CreateUser(ctx, "mike", hash, "admin"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	timeLogin := func(body loginRequest) time.Duration {
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		start := time.Now()
		handler.Login(rec, req)
		elapsed := time.Since(start)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
		return elapsed
	}

	wrongPassword := timeLogin(loginRequest{Username: "mike", Password: "wrong-pass"})
	unknownUsername := timeLogin(loginRequest{Username: "nobody", Password: "whatever"})

	// bcrypt at the default cost dominates both paths (tens of
	// milliseconds); a fixed field lookup or short-circuit would be
	// orders of magnitude faster. Guard against a regression back to
	// "unknown username returns near-instantly" rather than asserting a
	// tight timing equivalence, which would be flaky.
	const minBcryptFloor = 5 * time.Millisecond
	if unknownUsername < minBcryptFloor {
		t.Fatalf("unknown-username login returned in %v, expected it to pay a bcrypt-comparable cost (>= %v) like the wrong-password path (%v)", unknownUsername, minBcryptFloor, wrongPassword)
	}
}

func TestHandler_Logout_ClearsCookie(t *testing.T) {
	handler, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("expected a cookie-clearing Set-Cookie (negative MaxAge), got %+v", cookies)
	}
}

func TestHandler_Me_WithValidCookie(t *testing.T) {
	handler, store, _ := newTestHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	user, err := store.CreateUser(ctx, "mike", hash, "admin")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	tokenString, err := SignToken([]byte("test-secret"), user.ID, user.Username, user.Role)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tokenString})
	rec := httptest.NewRecorder()

	handler.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp meResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Username != "mike" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestHandler_Me_WithoutCookie(t *testing.T) {
	handler, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()

	handler.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandler_Me_Features_Admin(t *testing.T) {
	handler, store, features := newTestHandler(t)
	_, err := store.CreateUser(context.Background(), "mike", "password123", "admin")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// Mirrors access.KnownFeatures' keys as of this writing — hand-kept in
	// sync the same way frontend/src/shared/api.ts's FeatureKey union is
	// (see backend/internal/access/features.go's doc comment). This test
	// verifies Handler forwards whatever FeatureLister returns, not the
	// real admin-bypass logic itself (that's Task 1's job).
	features.features[1] = []string{"notes", "watchlist", "date-night", "shopping-list", "image-processing"}
	token, err := SignToken([]byte("test-secret"), 1, "mike", "admin")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	rec := httptest.NewRecorder()
	handler.Me(rec, req)

	var resp meResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.Features) != 5 {
		t.Fatalf("expected 5 features, got %v", resp.Features)
	}
}

func TestHandler_Me_Features_UserWithGrant(t *testing.T) {
	handler, store, features := newTestHandler(t)
	user, err := store.CreateUser(context.Background(), "bob", "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	features.features[user.ID] = []string{"notes"}
	token, err := SignToken([]byte("test-secret"), user.ID, "bob", "user")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	rec := httptest.NewRecorder()
	handler.Me(rec, req)

	var resp meResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.Features) != 1 || resp.Features[0] != "notes" {
		t.Fatalf("expected [notes], got %v", resp.Features)
	}
}
