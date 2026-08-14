package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/devianse/pet-project/backend/internal/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestHandleHealth_ReportsDBOkAndVersion(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	conn, err := db.Open(databaseURL)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	t.Setenv("GIT_SHA", "abc1234")

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handleHealth(conn)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status  string `json:"status"`
		DB      string `json:"db"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}
	if resp.DB != "ok" {
		t.Fatalf("expected db ok, got %q", resp.DB)
	}
	if resp.Version != "abc1234" {
		t.Fatalf("expected version abc1234, got %q", resp.Version)
	}
}

func TestHandleHealth_VersionUnknownWhenGitSHAUnset(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	conn, err := db.Open(databaseURL)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	t.Setenv("GIT_SHA", "")

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handleHealth(conn)(rec, req)

	var resp struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Version != "unknown" {
		t.Fatalf("expected version unknown, got %q", resp.Version)
	}
}

func TestHandleHealth_DBDownReportsUnhealthy(t *testing.T) {
	// sql.Open (unlike db.Open) connects lazily and never errors here —
	// the handler's own PingContext call is what has to notice the DB is
	// unreachable, exactly like a real outage would surface.
	badConn, err := sql.Open("pgx", "postgres://postgres:postgres@localhost:1/nonexistent?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { badConn.Close() })

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handleHealth(badConn)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for a down DB, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status string `json:"status"`
		DB     string `json:"db"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.DB != "unreachable" {
		t.Fatalf("expected db unreachable, got %q", resp.DB)
	}
	if resp.Status != "degraded" {
		t.Fatalf("expected status degraded, got %q", resp.Status)
	}
}
