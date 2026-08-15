# Security TODO (deferred out of the pre-deployment hardening pass)

Recorded during the pre-deployment security brainstorm
(`docs/superpowers/specs/2026-08-09-pre-deployment-security-design.md`)
as genuinely dependent on later phases, not skipped by oversight:

- ~~Real JWT auth (phase 2) replacing the Caddy basic-auth stopgap~~ —
  done, see `docs/superpowers/specs/2026-08-10-jwt-auth-design.md`
- **CI/CD pipeline** automating what's manual for now: `npm audit`,
  `govulncheck ./...`, `gitleaks detect` on every push/PR, plus
  Dependabot/Renovate for ongoing dependency updates
- ~~Rate limiting on `POST /api/auth/login`~~ — done: an in-process
  per-IP token-bucket middleware (`backend/cmd/api/ratelimit.go`, 5
  requests/min, burst 5), chosen over a Caddy-plugin or fail2ban layer
  because it needs no custom Caddy build and stays testable in the same
  suite as the rest of `internal/auth`. Trusts `X-Forwarded-For` from
  Caddy, which is safe only because the backend has no host port mapping
  in `infra/docker-compose.yml` — revisit if that ever changes. State is
  per-process/per-replica (fine for the current single-VPS, single-
  instance topology per `PLANNING.md`'s "No Kubernetes / load balancer"
  note; wouldn't be if that changes). Broader request-volume limiting
  beyond this one endpoint is still deferred until real traffic patterns
  exist.
- **CORS policy** — not needed yet (Caddy makes frontend/API
  same-origin), revisit if that changes
- **CSP tightening** as the frontend grows past its current shape
- ~~WebSocket auth~~ — done, see
  `docs/superpowers/specs/2026-08-15-websockets-shell-design.md`. The
  `GET /api/ws` upgrade reuses the same session-cookie validation as
  REST via `realtime.Authenticator`; per-topic authorization goes
  through `realtime.TopicAuthorizer` on every subscribe.
- ~~Postgres backup strategy~~ — done, see
  `docs/superpowers/specs/2026-08-15-postgres-backup-design.md` and
  `docs/superpowers/plans/2026-08-15-postgres-backup.md` (manual
  `infra/backup.sh`/`infra/restore.sh`, pull-to-dev-machine, no cloud
  storage)
- **Secrets rotation**
