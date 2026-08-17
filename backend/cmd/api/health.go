package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
)

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
