// Phase 1 scaffold, now with the Notes feature wired in as a deliberate
// detour ahead of the shell-first plan (see
// docs/superpowers/specs/2026-08-08-notes-design.md). Still no auth.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/devianse/pet-project/backend/internal/db"
	"github.com/devianse/pet-project/backend/internal/notes"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// .env is for local dev convenience only. In production the real
	// env vars are set by the host/systemd/container, so a missing file
	// here is expected and not an error worth failing startup over.
	if err := godotenv.Load(); err != nil {
		logger.Info("no .env file found, using process env")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	conn, err := db.Open(databaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	notesStore := notes.NewStore(conn)
	if err := notesStore.EnsureSchema(context.Background()); err != nil {
		logger.Error("failed to ensure notes schema", "error", err)
		os.Exit(1)
	}
	notesHandler := notes.NewHandler(notesStore)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/notes", notesHandler.List)
	mux.HandleFunc("POST /api/notes", notesHandler.Create)
	mux.HandleFunc("DELETE /api/notes/{id}", notesHandler.Delete)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	logger.Info("api listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
