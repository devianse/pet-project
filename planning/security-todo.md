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
- **WebSocket auth** (phase 2 feature, doesn't exist yet)
- **Postgres backup strategy** / secrets rotation
