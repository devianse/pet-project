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

	"github.com/devianse/pet-project/backend/internal/auth"
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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		logger.Error("JWT_SECRET is not set")
		os.Exit(1)
	}
	secureCookies := os.Getenv("ENV") == "production"

	authStore := auth.NewStore(conn)
	if err := authStore.EnsureSchema(context.Background()); err != nil {
		logger.Error("failed to ensure users schema", "error", err)
		os.Exit(1)
	}
	authHandler := auth.NewHandler(authStore, []byte(jwtSecret), secureCookies)
	requireAuth := auth.Require([]byte(jwtSecret))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/me", authHandler.Me)
	mux.Handle("GET /api/notes", requireAuth(http.HandlerFunc(notesHandler.List)))
	mux.Handle("POST /api/notes", requireAuth(http.HandlerFunc(notesHandler.Create)))
	mux.Handle("DELETE /api/notes/{id}", requireAuth(http.HandlerFunc(notesHandler.Delete)))
	mux.Handle("GET /api/watchlist", requireAuth(http.HandlerFunc(watchlistHandler.List)))
	mux.Handle("POST /api/watchlist", requireAuth(http.HandlerFunc(watchlistHandler.Create)))
	mux.Handle("PATCH /api/watchlist/{id}", requireAuth(http.HandlerFunc(watchlistHandler.SetViewed)))
	mux.Handle("DELETE /api/watchlist/{id}", requireAuth(http.HandlerFunc(watchlistHandler.Delete)))

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
