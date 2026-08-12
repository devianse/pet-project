package datenight

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devianse/pet-project/backend/internal/auth"
)

func TestLoadPair(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"valid pair", "alice,bob", false},
		{"trims whitespace", " alice , bob ", false},
		{"empty string", "", true},
		{"only one name", "alice", true},
		{"three names", "alice,bob,carol", true},
		{"duplicate names", "alice,alice", true},
		{"empty first name", ",bob", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadPair(tc.raw)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestPair_Contains(t *testing.T) {
	pair, err := LoadPair("alice,bob")
	if err != nil {
		t.Fatalf("LoadPair: %v", err)
	}
	if !pair.Contains("alice") || !pair.Contains("bob") {
		t.Fatal("expected both pair members to be contained")
	}
	if pair.Contains("carol") {
		t.Fatal("expected a non-member to not be contained")
	}
}

func TestPair_Other(t *testing.T) {
	pair, err := LoadPair("alice,bob")
	if err != nil {
		t.Fatalf("LoadPair: %v", err)
	}
	other, ok := pair.Other("alice")
	if !ok || other != "bob" {
		t.Fatalf("expected (bob, true), got (%q, %v)", other, ok)
	}
	other, ok = pair.Other("bob")
	if !ok || other != "alice" {
		t.Fatalf("expected (alice, true), got (%q, %v)", other, ok)
	}
	_, ok = pair.Other("carol")
	if ok {
		t.Fatal("expected ok=false for a non-member")
	}
}

func TestRequirePair(t *testing.T) {
	secret := []byte("test-secret")
	pair, err := LoadPair("alice,bob")
	if err != nil {
		t.Fatalf("LoadPair: %v", err)
	}

	terminal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := auth.Require(secret)(RequirePair(pair)(terminal))

	tokenFor := func(username string) string {
		token, err := auth.SignToken(secret, 1, username, "user")
		if err != nil {
			t.Fatalf("SignToken: %v", err)
		}
		return token
	}

	// auth_token mirrors auth's private cookie name — see auth/cookie.go.
	newRequestWithCookie := func(token string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/datenight/activities", nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
		}
		return req
	}

	cases := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"pair member", tokenFor("alice"), http.StatusOK},
		{"other pair member", tokenFor("bob"), http.StatusOK},
		{"non-member", tokenFor("carol"), http.StatusForbidden},
		{"no cookie", "", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, newRequestWithCookie(tc.token))
			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, rec.Code)
			}
		})
	}
}
