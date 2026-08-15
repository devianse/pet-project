// Phase 1 scaffold, now with the Notes and Watchlist features wired in as
// deliberate detours ahead of the shell-first plan (see
// docs/superpowers/specs/2026-08-08-notes-design.md and
// docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md),
// plus real per-user JWT auth gating the whole app (see
// docs/superpowers/specs/2026-08-10-jwt-auth-design.md).
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/devianse/pet-project/backend/internal/access"
	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/datenight"
	"github.com/devianse/pet-project/backend/internal/db"
	"github.com/devianse/pet-project/backend/internal/notes"
	"github.com/devianse/pet-project/backend/internal/ops"
	"github.com/devianse/pet-project/backend/internal/realtime"
	"github.com/devianse/pet-project/backend/internal/watchlist"
	"github.com/joho/godotenv"
)

// shutdownTimeout bounds how long graceful shutdown waits for in-flight
// requests to finish before forcing the listener closed.
const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// ctx is cancelled on SIGINT/SIGTERM and drives graceful shutdown
	// below — passed to every long-lived background loop (currently just
	// the Telegram poller) so nothing outlives the server itself.
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
	startTelegramBot(ctx, logger, notesStore)

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

	datenightStore := datenight.NewStore(conn)
	if err := datenightStore.EnsureSchema(context.Background()); err != nil {
		logger.Error("failed to ensure date night schema", "error", err)
		os.Exit(1)
	}
	datenightHandler := datenight.NewHandler(datenightStore)

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
	accessStore := access.NewStore(conn)
	if err := accessStore.EnsureSchema(context.Background()); err != nil {
		logger.Error("failed to ensure access schema", "error", err)
		os.Exit(1)
	}

	// wsAuthenticator reuses the same session-cookie validation REST already
	// uses — see realtime.Authenticator's doc comment for why this is a
	// seam, not a hardcoded dependency.
	wsAuthenticator := realtime.AuthenticatorFunc(func(r *http.Request) (realtime.Identity, error) {
		claims, err := auth.ClaimsFromRequest(r, []byte(jwtSecret))
		if err != nil {
			return realtime.Identity{}, err
		}
		userID, err := claims.UserID()
		if err != nil {
			return realtime.Identity{}, err
		}
		return realtime.Identity{UserID: userID, Role: claims.Role}, nil
	})

	// opsTopicAuthorizer is the first real per-topic policy: ops.* is
	// admin-only (mirrors RequireRole(accessStore, "admin") REST already
	// uses for /api/admin/...), everything else stays open to any
	// authenticated identity until a future consumer needs its own rule.
	opsTopicAuthorizer := realtime.TopicAuthorizerFunc(func(_ context.Context, identity realtime.Identity, topic string) bool {
		if strings.HasPrefix(topic, "ops.") {
			return identity.Role == "admin"
		}
		return true
	})
	realtimeHub := realtime.NewHub(opsTopicAuthorizer)
	wsHandler := realtime.NewHandler(realtimeHub, wsAuthenticator)

	// healthTicker is ops panel live-update's other half (audit log
	// broadcasts from AdminHandler.logAction below) — only does DB work
	// while someone's actually subscribed to ops.health, see
	// internal/ops's own doc comment. Runs until ctx (SIGINT/SIGTERM) is
	// cancelled, same lifecycle as the rest of this process's background
	// work.
	gitSHA := os.Getenv("GIT_SHA")
	if gitSHA == "" {
		gitSHA = "unknown"
	}
	healthTicker := ops.NewHealthTicker(conn, realtimeHub, gitSHA)
	go healthTicker.Run(ctx)

	authHandler := auth.NewHandler(authStore, []byte(jwtSecret), secureCookies, accessStore)
	requireAuth := auth.Require([]byte(jwtSecret))
	requireFeature := func(key string) func(http.Handler) http.Handler {
		return access.RequireFeature(accessStore, key)
	}

	adminHandler := access.NewAdminHandler(accessStore, authStore, realtimeHub)
	requireAdmin := func(h http.Handler) http.Handler {
		return requireAuth(access.RequireRole(accessStore, "admin")(h))
	}

	// loginLimiter caps login attempts per IP: bcrypt-costed and, since
	// JWT auth replaced Caddy basic-auth, publicly reachable and
	// unauthenticated — see PLANNING.md's Security TODO. 5 requests/min
	// with a burst of 5 tolerates a genuine mistyped-password retry
	// without leaving the endpoint open to unlimited guessing.
	loginLimiter := newIPRateLimiter(rateEvery(time.Minute/5), 5)
	go loginLimiter.startCleanup(ctx, 10*time.Minute, time.Hour)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth(conn))
	mux.Handle("GET /api/ws", wsHandler)
	mux.Handle("POST /api/auth/login", rateLimitMiddleware(loginLimiter, http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/me", authHandler.Me)
	mux.HandleFunc("PATCH /api/me", authHandler.UpdateMe)
	mux.Handle("GET /api/notes", requireAuth(requireFeature("notes")(http.HandlerFunc(notesHandler.List))))
	mux.Handle("POST /api/notes", requireAuth(requireFeature("notes")(http.HandlerFunc(notesHandler.Create))))
	mux.Handle("DELETE /api/notes/{id}", requireAuth(requireFeature("notes")(http.HandlerFunc(notesHandler.Delete))))
	mux.Handle("GET /api/watchlist", requireAuth(requireFeature("watchlist")(http.HandlerFunc(watchlistHandler.List))))
	mux.Handle("POST /api/watchlist", requireAuth(requireFeature("watchlist")(http.HandlerFunc(watchlistHandler.Create))))
	mux.Handle("PATCH /api/watchlist/{id}", requireAuth(requireFeature("watchlist")(http.HandlerFunc(watchlistHandler.SetViewed))))
	mux.Handle("DELETE /api/watchlist/{id}", requireAuth(requireFeature("watchlist")(http.HandlerFunc(watchlistHandler.Delete))))

	mux.Handle("GET /api/datenight/activities", requireAuth(requireFeature("date-night")(http.HandlerFunc(datenightHandler.ListActivities))))
	mux.Handle("POST /api/datenight/activities", requireAuth(requireFeature("date-night")(http.HandlerFunc(datenightHandler.CreateActivity))))
	mux.Handle("DELETE /api/datenight/activities/{id}", requireAuth(requireFeature("date-night")(http.HandlerFunc(datenightHandler.DeleteActivity))))
	mux.Handle("GET /api/datenight/proposals", requireAuth(requireFeature("date-night")(http.HandlerFunc(datenightHandler.ListProposals))))
	mux.Handle("POST /api/datenight/proposals", requireAuth(requireFeature("date-night")(http.HandlerFunc(datenightHandler.CreateProposal))))
	mux.Handle("POST /api/datenight/proposals/{id}/accept", requireAuth(requireFeature("date-night")(http.HandlerFunc(datenightHandler.AcceptProposal))))
	mux.Handle("POST /api/datenight/proposals/{id}/decline", requireAuth(requireFeature("date-night")(http.HandlerFunc(datenightHandler.DeclineProposal))))

	mux.Handle("GET /api/admin/users", requireAdmin(http.HandlerFunc(adminHandler.ListUsers)))
	mux.Handle("POST /api/admin/users", requireAdmin(http.HandlerFunc(adminHandler.CreateUser)))
	mux.Handle("POST /api/admin/users/{id}/features/{key}", requireAdmin(http.HandlerFunc(adminHandler.GrantFeature)))
	mux.Handle("DELETE /api/admin/users/{id}/features/{key}", requireAdmin(http.HandlerFunc(adminHandler.RevokeFeature)))
	mux.Handle("PUT /api/admin/users/{id}/role", requireAdmin(http.HandlerFunc(adminHandler.UpdateRole)))
	mux.Handle("PUT /api/admin/users/{id}/active", requireAdmin(http.HandlerFunc(adminHandler.SetActive)))
	mux.Handle("POST /api/admin/users/{id}/reset-password", requireAdmin(http.HandlerFunc(adminHandler.ResetPassword)))
	mux.Handle("GET /api/admin/audit-log", requireAdmin(http.HandlerFunc(adminHandler.AuditLog)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	srv := newServer(addr, maxBytesMiddleware(mux))

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
			realtimeHub.Shutdown(shutdownCtx)
		}()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
		wg.Wait()
	}
}

// handleHealth reports basic liveness (always "ok" if the process is
// serving requests at all), DB connectivity (a real PingContext, not just
// "the pool object exists"), and the running build's git SHA — kept on
// this existing unauthenticated endpoint since health checks are
// conventionally public (load balancers, uptime monitors) and none of
// this is sensitive. GIT_SHA is a plain runtime env var, not baked in at
// build time — see infra/docker-compose.yml and the deploy runbook, which
// set it from `git rev-parse --short HEAD` the same way TMDB_READ_ACCESS_
// TOKEN and friends are already passed through.
func handleHealth(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		overallStatus, dbStatus, httpStatus := "ok", "ok", http.StatusOK
		if err := conn.PingContext(r.Context()); err != nil {
			overallStatus, dbStatus, httpStatus = "degraded", "unreachable", http.StatusServiceUnavailable
		}
		version := os.Getenv("GIT_SHA")
		if version == "" {
			version = "unknown"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  overallStatus,
			"db":      dbStatus,
			"version": version,
		})
	}
}
