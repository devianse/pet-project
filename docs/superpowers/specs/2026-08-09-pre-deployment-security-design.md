# Pre-deployment security hardening — design

## Status

Not on `PLANNING.md` yet — this is the decision record for that gap.
Motivated by an imminent VPS deploy today: plan is to deploy the current
shell (Notes feature, no auth) before phase 2's real JWT auth exists, so
this pass closes the obvious gaps that opens up. Scope is deliberately
"easy wins now" — anything that genuinely depends on auth existing, a
live VPS, or a CI pipeline is deferred to a `PLANNING.md` TODO section,
not built here.

## Approach

No alternative approaches considered — the scope here was pinned down
question-by-question against the actual current state of the repo
(reviewed `notes.go`, `handlers.go`, `main.go`, `db.go`, `Makefile`,
`.gitignore`; confirmed queries are already parameterized via pgx
placeholders, `.env`/`.env.*` are already gitignored, and `infra/`
genuinely doesn't exist yet). This doc just assembles the agreed items
into one buildable unit.

**Deliberately out of scope, deferred to `PLANNING.md`:**
- Real JWT auth (phase 2) replacing the basic-auth stopgap below
- CI/CD pipeline (automating the manual scans below)
- Rate limiting beyond the body-size cap
- CORS policy (not needed yet — Caddy makes frontend/API same-origin)
- CSP tightening as the frontend grows past its current shape
- WebSocket auth (phase 2 feature, doesn't exist yet)
- Postgres backup strategy / secrets rotation

## Components

### 1. Backend hardening (`backend/cmd/api/main.go`)

Replace the bare `http.ListenAndServe(addr, mux)` call with an explicit
`http.Server`:

```go
srv := &http.Server{
    Addr:              addr,
    Handler:           mux,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      10 * time.Second,
    IdleTimeout:       60 * time.Second,
}
```

Closes the slowloris-style risk of a connection that never finishes
sending headers/body.

Add a small middleware wrapping the mux that applies
`http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)` before handlers run
— caps payload size (e.g. 1MB) before JSON decoding/validation even
starts, so a huge body can't be forced through parsing first. Applies to
all routes uniformly rather than per-handler.

### 2. New `infra/Caddyfile`

First real content for `infra/` (README already documents it as "added
once deployment starts" — that's now). Shape, per `PLANNING.md`'s
existing target:

- Reverse-proxies `/api/*` to the Go backend; serves the built SPA
  static files otherwise, with `try_files {path} /index.html` for
  client-side routing.
- Security headers block applied site-wide: `Content-Security-Policy`,
  `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: strict-origin-when-cross-origin`,
  `Strict-Transport-Security` (Caddy's automatic HTTPS makes HSTS safe
  to set immediately).
- `basicauth` directive wrapping the whole site as the pre-real-auth
  stopgap — username/password read from env vars passed into the Caddy
  container (not hardcoded into the Caddyfile), so the file itself stays
  committable.

### 3. New `infra/docker-compose.yml`

Three services matching `PLANNING.md`'s target shape: Caddy, backend,
Postgres.

- Postgres gets **no host port mapping** — reachable only on the
  internal Docker network the other two containers share. Closes
  "database open to the internet" by construction rather than relying on
  a firewall rule to remember.
- Backend also gets no host port mapping — only Caddy is reachable from
  outside the container network, matching the "Caddy is the only public
  entrypoint" shape `PLANNING.md` already describes.

### 4. Manual security scans — documented, not automated

No CI exists yet (deferred), so these are commands run by hand before a
PR is considered mergeable:

- `npm audit` (frontend) — known-vulnerability check on JS dependencies.
- `govulncheck ./...` (backend) — known-vulnerability check on Go
  dependencies, using Go's official vuln database.
- `gitleaks detect --source . -v` — scans full git history for
  committed secrets (API keys, tokens, anything that slipped past
  `.gitignore`).

**Gitleaks install**: no package manager entry in this repo (it's a
standalone binary, not an npm/Go dependency) — install via `brew install
gitleaks` or download a release binary from
[github.com/gitleaks/gitleaks](https://github.com/gitleaks/gitleaks/releases).
Documented in README's "Other commands" section alongside the commands
themselves, not vendored into the repo.

**`CLAUDE.md` gets a new convention**: a PR is not ready to merge unless
`npm audit`, `govulncheck ./...`, and `gitleaks detect` all pass clean —
stated as a rule alongside the existing "never commit without being
asked" convention, so it's enforced by habit/review until CI exists to
enforce it mechanically.

### 5. VPS hardening runbook

No VPS exists yet, so this is a provider-agnostic checklist rather than
applied config — written into README (new "Deploying" section, or
extending "Other commands") to follow today once the VPS is
provisioned:

- SSH: key-only login, disable password authentication in `sshd_config`.
- `ufw` firewall: allow only 22 (SSH), 80/443 (HTTP/HTTPS); deny
  everything else by default.
- `fail2ban` installed, watching sshd.
- Docker Compose containers run as non-root where the base image
  supports it (Caddy/Postgres official images already default
  sensibly; confirm at setup time rather than pre-deciding here).

### 6. `PLANNING.md` security TODO section

New section listing the deferred items from "Approach" above, so they
don't get lost: real JWT auth replacing basic-auth, CI/CD automating the
three manual scans (+ Dependabot/Renovate for ongoing dependency
updates), rate limiting, CORS policy once relevant, CSP tightening,
WebSocket auth, Postgres backup strategy.

## Testing

Infra config (Caddyfile, docker-compose.yml) has no unit-test surface —
verified by running the stack locally via `docker compose up` and
confirming: security headers present on responses (`curl -I`), basic
auth challenge appears, Postgres port not reachable from the host,
`/api/*` still proxies correctly, SPA routes survive a hard refresh.

Backend changes (`http.Server` timeouts, `MaxBytesReader` middleware)
get Go tests: a request under the body-size cap succeeds, one over it
gets rejected with an appropriate status before reaching handler logic.
