// Phase 1 scaffold, now with the Notes and Watchlist features wired in as
// deliberate detours ahead of the shell-first plan (see
// docs/superpowers/specs/2026-08-08-notes-design.md and
// docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md),
// plus real per-user JWT auth gating the whole app (see
// docs/superpowers/specs/2026-08-10-jwt-auth-design.md).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/devianse/pet-project/backend/internal/db"
	"github.com/joho/godotenv"
)

// shutdownTimeout bounds how long graceful shutdown waits for in-flight
// requests to finish before forcing the listener closed.
const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// ctx is cancelled on SIGINT/SIGTERM and drives graceful shutdown
	// below — passed to every long-lived background loop (App's own
	// background work, plus the Telegram poller) so nothing outlives the
	// server itself.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		logger.Error("JWT_SECRET is not set")
		os.Exit(1)
	}

	tmdbToken := os.Getenv("TMDB_READ_ACCESS_TOKEN")
	if tmdbToken == "" {
		logger.Error("TMDB_READ_ACCESS_TOKEN is not set")
		os.Exit(1)
	}

	gitSHA := os.Getenv("GIT_SHA")
	if gitSHA == "" {
		gitSHA = "unknown"
	}

	cfg := Config{
		JWTSecret:     jwtSecret,
		SecureCookies: os.Getenv("ENV") == "production",
		TMDBToken:     tmdbToken,
		GitSHA:        gitSHA,
	}

	conn, err := db.Open(databaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	app, err := newApp(conn, cfg)
	if err != nil {
		logger.Error("failed to build app", "error", err)
		os.Exit(1)
	}
	app.StartBackgroundWork(ctx)

	startTelegramBot(ctx, logger, app.NotesStore, app.RemindersStore)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	srv := newServer(addr, maxBytesMiddleware(app.Mux))

	// ListenAndServe runs in its own goroutine so the main goroutine is
	// free to block on ctx below and drive shutdown — srv.Shutdown from
	// the same goroutine that's still inside ListenAndServe would
	// deadlock.
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections", "timeout", shutdownTimeout)
		// Independent timeout, not ctx — ctx is already cancelled at
		// this point (that's what unblocked this select case), and
		// Shutdown needs its own live deadline to bound the drain.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.RealtimeHub.Shutdown(shutdownCtx)
		}()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
		wg.Wait()
	}
}
