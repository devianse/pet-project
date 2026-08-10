# pet-projects

Personal portfolio platform: a Go backend + React (Vite) SPA, built as a
shell that domain projects (Family Shopping List, Image Processing) plug
into later. See [`PLANNING.md`](./PLANNING.md) for the full architecture
and reasoning.

## Layout

```
backend/   Go API (cmd/api is the entrypoint binary)
frontend/  React SPA (Vite)
infra/     deploy config (Caddyfile, docker-compose) — added once deployment starts
```

Two independent apps, each managing its own dependencies (`go.mod`,
`package.json`) in its own subfolder — no monorepo tooling needed at this
size.

`frontend/src/components/pouf/` is vendored, CLI-managed code pulled from
the 1st-Pouf design-system registry — not hand-authored, don't edit it
directly (the next CLI run would silently overwrite manual changes). Add
to it with `npx shadcn@latest add https://1st-pouf.worksonmy.dev/r/<name>.json`.

## Running locally

`make dev-backend` and `make dev-frontend` each start a long-running,
blocking process (`go run` / `vite`) — the terminal they run in is stuck
printing logs until you `Ctrl+C` it. Since the frontend proxies `/api` to
the backend, you need both running at once, so they need separate
terminals (or terminal tabs/panes):

```
# terminal 1
make dev-backend
# → api listening addr=:8080

# terminal 2
make dev-frontend
# → Local: http://localhost:3000/
```

Then open `http://localhost:3000` — the SPA loads from Vite, and any
`/api/*` call it makes is proxied through to the Go backend on `:8080`.
Hitting `http://localhost:8080/api/health` directly also works, but the
frontend never talks to that port itself in dev, it talks to `:3000` and
lets Vite forward `/api` — same as Caddy will do in production.

There's no single `make dev` that starts both, because backgrounding one
of them muddles its logs into the other's terminal. That's a fine
tradeoff for two processes; worth revisiting if a third long-running dev
process shows up later.

Env config: copy `.env.example` to `.env` in each app's folder and adjust
as needed.

- `backend/.env.example` — `PORT` the Go server listens on,
  `TMDB_READ_ACCESS_TOKEN` for the Watchlist feature's TMDb API calls
- `frontend/.env.example` — `API_PROXY_TARGET` (where Vite's dev proxy
  forwards `/api` requests) and `FRONTEND_PORT` (what port Vite itself
  runs on, default 3000)

`.env` files are gitignored; `.env.example` is the checked-in template.

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
   — deny everything else by default. Note: this covers the SSH port and
   anything the host itself listens on, but Docker inserts its own
   `iptables` rules ahead of `ufw`'s — a container that publishes a host
   port is reachable regardless of `ufw`. That's fine here (only 80/443
   are published, which `ufw` already allows), but don't assume `ufw`
   alone would block a future `ports:` addition to `docker-compose.yml`.
3. **fail2ban.** `apt install fail2ban` (or equivalent), watching sshd for
   repeated failed login attempts.
4. **Deploy.** Clone the whole repo onto the VPS (not just `infra/` —
   `infra/docker-compose.yml`'s builds need `frontend/` and `backend/` as
   build context too), set real values in `infra/.env` (a real `DOMAIN`,
   a freshly generated `JWT_SECRET`, real Postgres credentials — never
   the `.env.example` placeholders), then `cd infra && docker compose up
   --build -d`. Immediately after, seed the first admin account —
   `docker compose exec backend createuser -username=<name>
   -role=admin` — before considering the deploy done: nothing else gates
   the site once basic-auth is gone.
5. **DNS.** Point the domain's A record at the VPS's public IP before
   step 4's first request — Caddy requests its TLS certificate on first
   contact and needs the domain already resolving.

Real per-user JWT auth (login page, httpOnly session cookie) gates the
whole app — see `docs/superpowers/specs/2026-08-10-jwt-auth-design.md`.
Accounts are invite-only: no public registration endpoint, seed one
with `go run ./cmd/createuser -username=<name> -role=admin|user` (run
locally against `DATABASE_URL`), or against the deployed database via
`docker compose exec backend createuser -username=<name>
-role=admin|user` — the runtime image ships the `createuser` binary
alongside `api`, no Go toolchain needed inside the container.
