// Phase 1 scaffold — just proves the backend runs and answers a request.
// No auth, no DB, no domains yet: those are phase 2 and phase 3.
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)

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
