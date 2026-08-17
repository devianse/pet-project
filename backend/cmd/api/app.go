package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devianse/pet-project/backend/internal/access"
	"github.com/devianse/pet-project/backend/internal/auth"
	"github.com/devianse/pet-project/backend/internal/datenight"
	"github.com/devianse/pet-project/backend/internal/notes"
	"github.com/devianse/pet-project/backend/internal/ops"
	"github.com/devianse/pet-project/backend/internal/realtime"
	"github.com/devianse/pet-project/backend/internal/reminders"
	"github.com/devianse/pet-project/backend/internal/watchlist"
)

// Config holds every already-read, already-validated env-derived value
// newApp needs to wire the app. Reading and validating os.Getenv values
// stays main()'s job — Config lets newApp (and its tests) take those
// values as plain data instead of touching the process environment
// itself.
type Config struct {
	JWTSecret     string
	SecureCookies bool
	TMDBToken     string
	GitSHA        string
}

// App is everything main() needs after newApp has finished wiring: the
// fully-routed mux to serve, plus the pieces with a lifecycle of their
// own (background goroutines to start, a shutdown to drive).
type App struct {
	Mux            *http.ServeMux
	RealtimeHub    *realtime.Hub
	HealthTicker   *ops.HealthTicker
	NotesStore     *notes.Store
	RemindersStore *reminders.Store
	loginLimiter   *ipRateLimiter
}

// newApp constructs every store, ensures every schema, builds every
// handler, and registers every route — the whole wiring main() used to
// do inline. Splitting it out gives the router a seam: tests can build a
// real, fully-wired App against a test DB and exercise any route
// (including /api/ws) through httptest, without running the binary.
func newApp(conn *sql.DB, cfg Config) (*App, error) {
	ctx := context.Background()

	notesStore := notes.NewStore(conn)
	if err := notesStore.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensuring notes schema: %w", err)
	}
	notesHandler := notes.NewHandler(notesStore)

	remindersStore := reminders.NewStore(conn)
	if err := remindersStore.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensuring reminders schema: %w", err)
	}

	tmdbClient, err := watchlist.NewRealTMDbClient(ctx, cfg.TMDBToken)
	if err != nil {
		return nil, fmt.Errorf("initializing tmdb client: %w", err)
	}

	watchlistStore := watchlist.NewStore(conn)
	if err := watchlistStore.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensuring watchlist schema: %w", err)
	}
	watchlistHandler := watchlist.NewHandler(watchlistStore, tmdbClient)

	datenightStore := datenight.NewStore(conn)
	if err := datenightStore.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensuring date night schema: %w", err)
	}
	datenightHandler := datenight.NewHandler(datenightStore)

	authStore := auth.NewStore(conn)
	if err := authStore.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensuring users schema: %w", err)
	}
	accessStore := access.NewStore(conn)
	if err := accessStore.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("ensuring access schema: %w", err)
	}

	// wsAuthenticator reuses the same session-cookie validation REST already
	// uses — see realtime.Authenticator's doc comment for why this is a
	// seam, not a hardcoded dependency.
	wsAuthenticator := realtime.AuthenticatorFunc(func(r *http.Request) (realtime.Identity, error) {
		claims, err := auth.ClaimsFromRequest(r, []byte(cfg.JWTSecret))
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
	// internal/ops's own doc comment. Started by main() once this App is
	// returned; its lifecycle (running until ctx is cancelled) belongs to
	// the process, not to construction.
	healthTicker := ops.NewHealthTicker(conn, realtimeHub, cfg.GitSHA)

	authHandler := auth.NewHandler(authStore, []byte(cfg.JWTSecret), cfg.SecureCookies, accessStore)
	requireAuth := auth.Require([]byte(cfg.JWTSecret))
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

	return &App{
		Mux:            mux,
		RealtimeHub:    realtimeHub,
		HealthTicker:   healthTicker,
		NotesStore:     notesStore,
		RemindersStore: remindersStore,
		loginLimiter:   loginLimiter,
	}, nil
}

// StartBackgroundWork launches every goroutine App owns (the health
// ticker's broadcast loop, the login limiter's stale-visitor cleanup),
// bound to ctx — cancel ctx (SIGINT/SIGTERM in main()) to stop them.
// Split from newApp so tests can build an App and hit its Mux without
// spinning up background loops they don't need.
func (a *App) StartBackgroundWork(ctx context.Context) {
	go a.HealthTicker.Run(ctx)
	go a.loginLimiter.startCleanup(ctx, 10*time.Minute, time.Hour)
}
