# Pre-Deployment Security Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the obvious security gaps (server hardening, infra config, secret/dependency scanning, VPS baseline) before the first VPS deployment, which happens without phase-2 auth existing yet.

**Architecture:** Backend gets explicit HTTP server timeouts and a request-body size cap. A new `infra/` gets its first content: a Caddy reverse-proxy config with security headers and a basic-auth stopgap, Dockerfiles for the backend and for a Caddy image that also serves the built frontend, and a `docker-compose.yml` wiring Caddy + backend + Postgres together with Postgres/backend not exposed to the host. Dependency/secret scanning (`npm audit`, `govulncheck`, `gitleaks`) becomes `make` targets, run by hand per the `CLAUDE.md` PR-readiness convention already in place. A VPS hardening runbook (SSH keys, `ufw`, `fail2ban`) is documented in README for when the VPS is provisioned.

**Tech Stack:** Go 1.26 stdlib `net/http`, Docker + Docker Compose, Caddy 2, Postgres 16, gitleaks, `govulncheck`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-08-09-pre-deployment-security-design.md` — follow its component boundaries exactly; anything listed under its "Deliberately out of scope" section (real JWT auth, CI/CD, rate limiting beyond body-size cap, CORS, CSP beyond the baseline here, WebSocket auth, backups) is NOT part of this plan.
- No CI exists yet — all scans (`npm audit`, `govulncheck`, `gitleaks detect`) are manual `make` targets, not GitHub Actions.
- Postgres and the backend container must not publish ports to the host — only Caddy is reachable from outside the Docker network (spec component 3).
- Caddy config values (basic-auth credentials, domain) come from env vars, never hardcoded into `infra/Caddyfile` (spec component 2).
- Follow existing backend conventions: stdlib `net/http`, no framework, `slog` for logging, table-driven Go tests matching the style in `backend/internal/notes/handlers_test.go`.

---

## File Structure

```
backend/
  cmd/api/
    main.go          (MODIFY — wire in newServer + maxBytesMiddleware)
    server.go         (NEW — HTTP server timeouts + body-size middleware)
    server_test.go     (NEW)
  Dockerfile           (NEW — multi-stage Go build)
infra/
  Caddyfile             (NEW — reverse proxy, security headers, basic auth)
  Dockerfile             (NEW — builds frontend, packages into a Caddy image)
  docker-compose.yml     (NEW — caddy + backend + postgres, 3 services)
  .env.example            (NEW — DOMAIN, BASIC_AUTH_*, POSTGRES_*)
Makefile               (MODIFY — add scan-secrets, audit-frontend, audit-backend)
README.md              (MODIFY — document new make targets + install steps, new "Deploying" section with VPS hardening runbook)
```

---

### Task 1: Backend server hardening (timeouts + body-size cap)

**Files:**
- Create: `backend/cmd/api/server.go`
- Create: `backend/cmd/api/server_test.go`
- Modify: `backend/cmd/api/main.go:55-64`

**Interfaces:**
- Produces: `maxBytesMiddleware(next http.Handler) http.Handler` — wraps a handler, rejecting requests whose `Content-Length` exceeds `maxRequestBodyBytes` with `413`, and capping unknown-length bodies via `http.MaxBytesReader`.
- Produces: `newServer(addr string, handler http.Handler) *http.Server` — builds an `*http.Server` with explicit `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout`.
- Produces: `const maxRequestBodyBytes = 1 << 20` (1MB).

- [ ] **Step 1: Write the failing tests**

Create `backend/cmd/api/server_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMaxBytesMiddleware_RejectsOversizedContentLength(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	body := strings.Repeat("a", maxRequestBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(body))
	rec := httptest.NewRecorder()

	maxBytesMiddleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
	if called {
		t.Fatal("expected next handler not to be called for an oversized body")
	}
}

func TestMaxBytesMiddleware_AllowsBodyWithinLimit(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader("small body"))
	rec := httptest.NewRecorder()

	maxBytesMiddleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Fatal("expected next handler to be called for a body within the limit")
	}
}

func TestNewServer_SetsTimeouts(t *testing.T) {
	srv := newServer(":8080", http.NewServeMux())

	if srv.Addr != ":8080" {
		t.Fatalf("expected addr :8080, got %q", srv.Addr)
	}
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("expected ReadHeaderTimeout 5s, got %v", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 10*time.Second {
		t.Fatalf("expected ReadTimeout 10s, got %v", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 10*time.Second {
		t.Fatalf("expected WriteTimeout 10s, got %v", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 60*time.Second {
		t.Fatalf("expected IdleTimeout 60s, got %v", srv.IdleTimeout)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./cmd/api/... -v`
Expected: FAIL — `undefined: maxBytesMiddleware`, `undefined: newServer`, `undefined: maxRequestBodyBytes` (compile error, not a runtime failure).

- [ ] **Step 3: Write the implementation**

Create `backend/cmd/api/server.go`:

```go
package main

import (
	"net/http"
	"time"
)

// maxRequestBodyBytes caps incoming request bodies before they reach
// JSON decoding — a large-body DoS shouldn't get as far as parsing.
const maxRequestBodyBytes = 1 << 20 // 1MB

// maxBytesMiddleware rejects requests whose declared Content-Length
// exceeds maxRequestBodyBytes outright, and wraps the body reader so an
// unknown-length (e.g. chunked) body that exceeds the cap fails on read
// instead of being decoded in full first.
func maxBytesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxRequestBodyBytes {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// newServer builds an http.Server with explicit timeouts. The zero-value
// server used by a bare http.ListenAndServe call has none, which leaves
// it exposed to slow-client (slowloris-style) connections that never
// finish sending headers or body.
func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./cmd/api/... -v`
Expected: PASS (all three tests).

- [ ] **Step 5: Wire the new server/middleware into `main.go`**

In `backend/cmd/api/main.go`, replace:

```go
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
```

with:

```go
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
```

- [ ] **Step 6: Run the full backend test suite**

Run: `cd backend && go build ./... && go test ./...`
Expected: build succeeds; `cmd/api` tests PASS; `internal/notes` and `internal/db` tests either PASS or SKIP (they skip without `DATABASE_URL` set, per `backend/internal/notes/store_test.go`) — no FAIL.

- [ ] **Step 7: Commit**

```bash
git add backend/cmd/api/server.go backend/cmd/api/server_test.go backend/cmd/api/main.go
git commit -m "feat(backend): add HTTP server timeouts and request body size cap"
```

---

### Task 2: Backend Dockerfile

**Files:**
- Create: `backend/Dockerfile`

**Interfaces:**
- Consumes: `backend/go.mod`, `backend/go.sum`, `backend/cmd/api` (from Task 1).
- Produces: a Docker image that runs the `api` binary on `$PORT` (default 8080), consumed by `infra/docker-compose.yml` in Task 4.

- [ ] **Step 1: Write the Dockerfile**

Create `backend/Dockerfile`:

```dockerfile
# --- build stage ---
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=build /out/api /usr/local/bin/api
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
```

Runs as a non-root user (`app`, uid 10001) inside the container — cheap hardening with no functional cost.

- [ ] **Step 2: Build the image**

Run: `docker build -t pet-projects-backend ./backend`
Expected: build completes with exit code 0, final line `Successfully tagged pet-projects-backend:latest` (or equivalent BuildKit success output).

- [ ] **Step 3: Verify the binary starts and serves health**

Run:
```bash
docker run --rm -d --name pp-backend-test -p 18080:8080 \
  -e PORT=8080 \
  -e DATABASE_URL="postgres://postgres:postgres@host.docker.internal:5432/notes?sslmode=disable" \
  pet-projects-backend
sleep 1
curl -sf http://localhost:18080/api/health
docker logs pp-backend-test
docker stop pp-backend-test
```
Expected: either `{"status":"ok"}` from `curl` if a local Postgres happens to be reachable at that DSN, or (more likely, no local Postgres running) the container logs `failed to connect to database` and exits — either outcome confirms the binary runs; a DB connection failure here is expected and not a Task 2 defect (Task 4's `docker compose` stack provides the real Postgres). If the binary doesn't start at all (e.g. `exec format error`, missing file), that IS a Task 2 defect — fix the Dockerfile.

- [ ] **Step 4: Commit**

```bash
git add backend/Dockerfile
git commit -m "feat(backend): add production Dockerfile"
```

---

### Task 3: Caddy config and image (security headers, basic auth, frontend build)

**Files:**
- Create: `infra/Caddyfile`
- Create: `infra/Dockerfile`

**Interfaces:**
- Consumes: `frontend/package.json`'s `build` script (`tsc -b && vite build`, outputs to `frontend/dist`), env vars `$DOMAIN`, `$BASIC_AUTH_USER`, `$BASIC_AUTH_PASSWORD_HASH` (supplied by `infra/docker-compose.yml` in Task 4).
- Produces: a Docker image (built from repo root) serving the SPA and reverse-proxying `/api/*` to a service named `backend` on port 8080 — the service name Task 4's compose file must use.

- [ ] **Step 1: Write the Caddyfile**

Create `infra/Caddyfile`:

```
{$DOMAIN} {
	encode gzip

	header {
		Content-Security-Policy "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'"
		X-Frame-Options "DENY"
		X-Content-Type-Options "nosniff"
		Referrer-Policy "strict-origin-when-cross-origin"
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
	}

	basic_auth {
		{$BASIC_AUTH_USER} {$BASIC_AUTH_PASSWORD_HASH}
	}

	handle /api/* {
		reverse_proxy backend:8080
	}

	handle {
		root * /srv
		try_files {path} /index.html
		file_server
	}
}
```

`style-src` allows `'unsafe-inline'` because 1st-Pouf/shadcn components may set inline `style` attributes directly; everything else stays locked to `'self'`. `{$BASIC_AUTH_PASSWORD_HASH}` must be a bcrypt hash (produced via `caddy hash-password`, see Task 4 verification), never a plaintext password — Caddy's `basic_auth` directive expects a hash, so a plaintext value here would simply fail every login attempt, not silently accept it insecurely.

- [ ] **Step 2: Write the image Dockerfile**

Create `infra/Dockerfile` (built with the repo root as context, so it can reach both `frontend/` and this Caddyfile):

```dockerfile
# --- frontend build stage ---
FROM node:22-alpine AS frontend-build
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- caddy stage ---
FROM caddy:2-alpine
COPY infra/Caddyfile /etc/caddy/Caddyfile
COPY --from=frontend-build /app/dist /srv
```

- [ ] **Step 3: Build the image standalone**

Run (from repo root): `docker build -t pet-projects-caddy -f infra/Dockerfile .`
Expected: build completes with exit code 0 — confirms the frontend actually builds inside the image and the Caddyfile is syntactically valid (Caddy validates its config on image build via its own entrypoint checks at container start, verified fully in Task 4).

- [ ] **Step 4: Commit**

```bash
git add infra/Caddyfile infra/Dockerfile
git commit -m "feat(infra): add Caddy config with security headers and basic-auth stopgap"
```

---

### Task 4: docker-compose stack (wiring, no host-exposed DB/backend)

**Files:**
- Create: `infra/docker-compose.yml`
- Create: `infra/.env.example`

**Interfaces:**
- Consumes: `backend/Dockerfile` (Task 2), `infra/Dockerfile` + `infra/Caddyfile` (Task 3).
- Produces: a runnable 3-container stack (`caddy`, `backend`, `postgres`) reachable only via Caddy on host ports 80/443.

- [ ] **Step 1: Write `infra/.env.example`**

Create `infra/.env.example`:

```
# Domain Caddy requests a TLS cert for. Use "localhost" for local testing
# (Caddy falls back to its internal CA — curl needs -k against it); use
# your real domain once deploying to the VPS.
DOMAIN=localhost

# Caddy basic-auth stopgap credentials (pre-real-auth). Generate the hash
# with: docker run --rm caddy:2-alpine caddy hash-password --plaintext '<your-password>'
BASIC_AUTH_USER=admin
BASIC_AUTH_PASSWORD_HASH=

POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=notes
```

- [ ] **Step 2: Write `infra/docker-compose.yml`**

Create `infra/docker-compose.yml`:

```yaml
services:
  caddy:
    build:
      context: ..
      dockerfile: infra/Dockerfile
    ports:
      - "80:80"
      - "443:443"
    environment:
      DOMAIN: ${DOMAIN}
      BASIC_AUTH_USER: ${BASIC_AUTH_USER}
      BASIC_AUTH_PASSWORD_HASH: ${BASIC_AUTH_PASSWORD_HASH}
    volumes:
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      - backend

  backend:
    build:
      context: ../backend
    environment:
      PORT: "8080"
      DATABASE_URL: postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable
    depends_on:
      - postgres

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
    volumes:
      - pg_data:/var/lib/postgresql/data

volumes:
  caddy_data:
  caddy_config:
  pg_data:
```

Note what's absent: no `ports:` on `backend` or `postgres` — neither is reachable from the host or the public internet, only from other containers on the compose-created network, and only `caddy` publishes to the host.

- [ ] **Step 3: Bring the stack up locally**

```bash
cd infra
cp .env.example .env
# Generate a real bcrypt hash and put it in .env's BASIC_AUTH_PASSWORD_HASH:
docker run --rm caddy:2-alpine caddy hash-password --plaintext 'test-password-123'
# paste the printed hash into infra/.env's BASIC_AUTH_PASSWORD_HASH=
docker compose up --build -d
```
Expected: all three containers start and stay running (`docker compose ps` shows `caddy`, `backend`, `postgres` all `Up`/`running`, no restart loops).

- [ ] **Step 4: Verify basic auth is enforced**

Run: `curl -sk -o /dev/null -w "%{http_code}\n" https://localhost/`
Expected: `401`

- [ ] **Step 5: Verify basic auth + security headers on success**

Run: `curl -sk -u admin:test-password-123 -D - -o /dev/null https://localhost/`
Expected: `HTTP/2 200`, with `content-security-policy`, `x-frame-options`, `x-content-type-options`, `referrer-policy`, and `strict-transport-security` all present in the response headers.

- [ ] **Step 6: Verify `/api/*` proxies to the backend**

Run: `curl -sk -u admin:test-password-123 https://localhost/api/health`
Expected: `{"status":"ok"}`

- [ ] **Step 7: Verify SPA routing survives a hard refresh**

Run: `curl -sk -u admin:test-password-123 -o /dev/null -w "%{http_code}\n" https://localhost/notes`
Expected: `200` (served `index.html` via `try_files`, not a 404).

- [ ] **Step 8: Verify Postgres and the backend are not reachable from the host**

Run: `nc -zv -w 2 localhost 5432; nc -zv -w 2 localhost 8080`
Expected: both connections refused/fail — confirms no host port mapping leaked through.

- [ ] **Step 9: Tear down**

Run: `docker compose down -v`
Expected: containers and named volumes removed cleanly.

- [ ] **Step 10: Commit**

```bash
git add infra/docker-compose.yml infra/.env.example
git commit -m "feat(infra): add docker-compose stack with no host-exposed DB/backend"
```

---

### Task 5: Manual scan tooling + VPS hardening runbook (docs)

**Files:**
- Modify: `Makefile`
- Modify: `README.md`

**Interfaces:**
- Consumes: nothing from earlier tasks (documentation-only, though it documents the scans `CLAUDE.md` already requires before a PR is ready).
- Produces: `make scan-secrets`, `make audit-frontend`, `make audit-backend` targets; a README "Deploying" section.

- [ ] **Step 1: Add the scan targets to the Makefile**

In `Makefile`, update the `.PHONY` line and add three targets:

```makefile
.PHONY: dev-backend dev-frontend build-backend build-frontend lint-frontend scan-secrets audit-frontend audit-backend
```

```makefile
scan-secrets:
	@command -v gitleaks >/dev/null 2>&1 || { echo "gitleaks not found — see README's 'Other commands' section for install instructions"; exit 1; }
	gitleaks detect --source . -v

audit-frontend:
	cd frontend && npm audit

audit-backend:
	@command -v govulncheck >/dev/null 2>&1 || { echo "govulncheck not found — run: go install golang.org/x/vuln/cmd/govulncheck@latest"; exit 1; }
	cd backend && govulncheck ./...
```

- [ ] **Step 2: Verify the targets run**

Run: `make audit-frontend`
Expected: `npm audit` output (a vulnerability report or "found 0 vulnerabilities"); exit code may be non-zero if `npm audit` finds issues — that's `npm audit` surfacing real findings, not a broken target. Report any findings rather than silently ignoring them.

Run: `go install golang.org/x/vuln/cmd/govulncheck@latest && make audit-backend`
Expected: `govulncheck` output ("No vulnerabilities found" or a findings report).

Run: install gitleaks per Step 3 below, then `make scan-secrets`
Expected: `gitleaks detect` output ("no leaks found" or a findings report — if it finds anything in git history, stop and report it rather than proceeding).

- [ ] **Step 3: Document the new commands + install steps in README**

In `README.md`'s "Other commands" section, replace:

```
## Other commands

```
make build-backend    # go build ./...
make build-frontend   # tsc -b && vite build
make lint-frontend     # oxlint
```
```

with:

```
## Other commands

```
make build-backend    # go build ./...
make build-frontend   # tsc -b && vite build
make lint-frontend    # oxlint
make audit-frontend   # npm audit — known-vulnerability check on JS deps
make audit-backend    # govulncheck ./... — known-vulnerability check on Go deps
make scan-secrets     # gitleaks detect — scans git history for committed secrets
```

`audit-backend` needs `govulncheck` installed once:
`go install golang.org/x/vuln/cmd/govulncheck@latest`

`scan-secrets` needs `gitleaks` installed once — it's a standalone binary,
not a project dependency: `brew install gitleaks`, or download a release
from [github.com/gitleaks/gitleaks](https://github.com/gitleaks/gitleaks/releases).

Per `CLAUDE.md`'s convention, a PR isn't considered ready until all three
scans pass clean.
```

- [ ] **Step 4: Add a README "Deploying" section with the VPS hardening runbook**

Add a new section to `README.md`, after "Other commands":

```
## Deploying

The full stack (`infra/docker-compose.yml`) is Caddy + backend + Postgres,
with only Caddy reachable from outside the container network — see
`docs/superpowers/specs/2026-08-09-pre-deployment-security-design.md` for
the reasoning. Steps to provision a fresh VPS:

1. **SSH: key-only login.** Generate a key pair locally if you don't have
   one (`ssh-keygen -t ed25519`), get the public key onto the VPS (many
   providers accept it at provisioning time; otherwise `ssh-copy-id`
   once using the initial password), then in `/etc/ssh/sshd_config` set
   `PasswordAuthentication no` and `PubkeyAuthentication yes`, and
   `systemctl restart sshd`. Verify a *second* terminal can still log in
   with the key before closing the first — don't skip this check.
2. **Firewall.** `ufw allow 22 && ufw allow 80 && ufw allow 443 && ufw enable`
   — deny everything else by default.
3. **fail2ban.** `apt install fail2ban` (or equivalent), watching sshd for
   repeated failed login attempts.
4. **Deploy.** Copy `infra/` to the VPS (or clone the repo there), set
   real values in `infra/.env` (a real `DOMAIN`, a freshly generated
   `BASIC_AUTH_PASSWORD_HASH`, real Postgres credentials — never the
   `.env.example` placeholders), then `cd infra && docker compose up
   --build -d`.
5. **DNS.** Point the domain's A record at the VPS's public IP before
   step 4's first request — Caddy requests its TLS certificate on first
   contact and needs the domain already resolving.

Basic auth (site-wide, one shared credential pair) gates the whole app
until phase 2's real JWT auth replaces it — see `PLANNING.md`'s Security
TODO section.
```

- [ ] **Step 5: Commit**

```bash
git add Makefile README.md
git commit -m "docs: add scan make targets and VPS deployment runbook"
```

---

## Self-Review Notes

- **Spec coverage:** all 5 components from `docs/superpowers/specs/2026-08-09-pre-deployment-security-design.md` map to a task — backend hardening → Task 1; Caddyfile → Task 3; docker-compose → Task 4; manual scans → Task 5; VPS runbook → Task 5. Backend/Caddy Dockerfiles (Tasks 2–3) weren't named components in the spec but are required for the spec's own docker-compose component to be buildable — added as the natural completion of that component, not new scope.
- **Type/name consistency:** `maxBytesMiddleware`, `newServer`, `maxRequestBodyBytes` are defined once in Task 1 and used identically in `main.go`'s Step 5; the `backend:8080` proxy target in Task 3's Caddyfile matches the service name `backend` defined in Task 4's compose file; `BASIC_AUTH_USER`/`BASIC_AUTH_PASSWORD_HASH`/`DOMAIN` env var names match across Caddyfile, `.env.example`, and `docker-compose.yml`.
- **No placeholders:** all code/config blocks are complete and copy-pasteable; the one intentionally-empty value (`BASIC_AUTH_PASSWORD_HASH=` in `.env.example`) is a real secret placeholder for a gitignored file, not a plan placeholder — Task 4 Step 3 shows exactly how to generate and fill it in.
