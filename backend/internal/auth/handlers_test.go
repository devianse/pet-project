package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestHandler(t *testing.T) (*Handler, *Store) {
	t.Helper()
	store := setupStore(t)
	handler := NewHandler(store, []byte("test-secret"), false)
	return handler, store
}

func TestHandler_Login_Success(t *testing.T) {
	handler, store := newTestHandler(t)
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
	handler, store := newTestHandler(t)
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
	handler, _ := newTestHandler(t)

	body, _ := json.Marshal(loginRequest{Username: "nobody", Password: "whatever"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandler_Logout_ClearsCookie(t *testing.T) {
	handler, _ := newTestHandler(t)

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
	handler, store := newTestHandler(t)
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
	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()

	handler.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
