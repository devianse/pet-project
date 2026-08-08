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

- `backend/.env.example` — `PORT` the Go server listens on
- `frontend/.env.example` — `API_PROXY_TARGET` (where Vite's dev proxy
  forwards `/api` requests) and `FRONTEND_PORT` (what port Vite itself
  runs on, default 3000)

`.env` files are gitignored; `.env.example` is the checked-in template.

## Other commands

```
make build-backend    # go build ./...
make build-frontend   # tsc -b && vite build
make lint-frontend     # oxlint
```
