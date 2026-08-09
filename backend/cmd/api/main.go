// Phase 1 scaffold, now with the Notes and Watchlist features wired in as
// deliberate detours ahead of the shell-first plan (see
// docs/superpowers/specs/2026-08-08-notes-design.md and
// docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md).
// Still no auth.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/devianse/pet-project/backend/internal/db"
	"github.com/devianse/pet-project/backend/internal/notes"
	"github.com/devianse/pet-project/backend/internal/watchlist"
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

	tmdbToken := os.Getenv("TMDB_READ_ACCESS_TOKEN")
	if tmdbToken == "" {
		logger.Error("TMDB_READ_ACCESS_TOKEN is not set")
		os.Exit(1)
	}
	tmdbClient, err := watchlist.NewRealTMDbClient(context.Background(), tmdbToken)
	if err != nil {
		logger.Error("failed to initialize tmdb client", "error", err)
		os.Exit(1)
	}

	watchlistStore := watchlist.NewStore(conn)
	if err := watchlistStore.EnsureSchema(context.Background()); err != nil {
		logger.Error("failed to ensure watchlist schema", "error", err)
		os.Exit(1)
	}
	watchlistHandler := watchlist.NewHandler(watchlistStore, tmdbClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/notes", notesHandler.List)
	mux.HandleFunc("POST /api/notes", notesHandler.Create)
	mux.HandleFunc("DELETE /api/notes/{id}", notesHandler.Delete)
	mux.HandleFunc("GET /api/watchlist", watchlistHandler.List)
	mux.HandleFunc("POST /api/watchlist", watchlistHandler.Create)
	mux.HandleFunc("PATCH /api/watchlist/{id}", watchlistHandler.SetViewed)
	mux.HandleFunc("DELETE /api/watchlist/{id}", watchlistHandler.Delete)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	srv := newServer(addr, maxBytesMiddleware(mux))
	logger.Info("api listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
