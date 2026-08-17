package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// testConn opens a real test-DB connection, skipping when DATABASE_URL
// isn't set — same gating convention health_test.go already uses.
func testConn(t *testing.T) *sql.DB {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	conn, err := db.Open(databaseURL)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func testConfig() Config {
	return Config{
		JWTSecret:     "test-jwt-secret",
		SecureCookies: false,
		TMDBToken:     "test-token",
		GitSHA:        "test-sha",
	}
}

// TestNewApp_RoutesAreReachableThroughRealRouter is the payoff for
// splitting newApp out of main(): before this, exercising /api/ws (or any
// other route) required running the actual binary — there was no seam to
// build a real, fully-wired router in a test. Login through a real
// /api/auth/login call, then dial /api/ws with the resulting session
// cookie, confirming the whole chain (mux → requireAuth-equivalent WS
// authenticator → hub) works end to end.
func TestNewApp_RoutesAreReachableThroughRealRouter(t *testing.T) {
	conn := testConn(t)
	cfg := testConfig()

	app, err := newApp(conn, cfg)
	if err != nil {
		t.Fatalf("newApp: %v", err)
	}

	authStore := auth.NewStore(conn)
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	// Scoped delete before insert (not an unscoped DELETE FROM users) so
	// this test is safe to rerun and doesn't race other packages' fixture
	// rows — see planning/decisions.md's notes/multi-tenancy entry on the
	// test-isolation convention this mirrors.
	username := "app-test-ws-user"
	if _, err := conn.ExecContext(context.Background(), "DELETE FROM users WHERE username = $1", username); err != nil {
		t.Fatalf("cleaning up fixture user: %v", err)
	}
	if _, err := authStore.CreateUser(context.Background(), username, hash, "user"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	srv := httptest.NewServer(app.Mux)
	defer srv.Close()

	loginBody, _ := json.Marshal(map[string]string{
		"username": username,
		"password": "correct horse battery staple",
	})
	resp, err := http.Post(srv.URL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", resp.StatusCode)
	}
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("login response set no cookies")
	}

	cookieHeader := ""
	for i, c := range cookies {
		if i > 0 {
			cookieHeader += "; "
		}
		cookieHeader += c.Name + "=" + c.Value
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	wsURL := "ws" + srv.URL[len("http"):] + "/api/ws"
	client, wsResp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": {cookieHeader}},
	})
	if err != nil {
		t.Fatalf("ws Dial with valid session cookie: %v (resp: %v)", err, wsResp)
	}
	defer client.Close(websocket.StatusNormalClosure, "")
}

// TestNewApp_WSRejectsUnauthenticated confirms the router's WS route
// enforces the same session-cookie auth as the REST routes, reachable
// through the same real mux newApp builds.
func TestNewApp_WSRejectsUnauthenticated(t *testing.T) {
	conn := testConn(t)
	app, err := newApp(conn, testConfig())
	if err != nil {
		t.Fatalf("newApp: %v", err)
	}

	srv := httptest.NewServer(app.Mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	wsURL := "ws" + srv.URL[len("http"):] + "/api/ws"
	_, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected Dial to fail without a session cookie")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
