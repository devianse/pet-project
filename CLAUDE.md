# pet-projects

Personal portfolio platform: Go backend + React (Vite) SPA, built as a
shell that domain projects (Family Shopping List, Image Processing) plug
into later. Full architecture and reasoning: [`PLANNING.md`](./PLANNING.md).
This file is orientation for me, not a duplicate of it — check `PLANNING.md`
before assuming a decision hasn't been made.

## Current phase

Phase 1 (barebones scaffold) is done: Go API + React SPA wired end to end
via Vite's dev proxy, env var config in place. Phase 2 (auth, WebSockets)
has NOT started — don't assume either exists. Phase 3 (domain projects)
is further out still. See `PLANNING.md`'s phase breakdown for the full
picture.

## Layout

```
backend/   Go API (cmd/api is the entrypoint binary)
frontend/  React SPA (Vite)
```

`infra/` and `docs/` are deliberately not created yet — nothing to put in
either until deployment/real docs are actually needed. Not an oversight.

## Commands

```
make dev-backend      # Go API on :8080, needs its own terminal
make dev-frontend     # Vite dev server on :3000, proxies /api -> backend
make build-backend    # go build ./...
make build-frontend   # tsc -b && vite build
make lint-frontend    # oxlint
```

Both dev servers block and must run in separate terminals simultaneously
— see `README.md`'s "Running locally" section for the full explanation.

## Conventions

- **Never commit without being asked.** Staging/committing is the user's
  call, every time — not implied by "this looks done."
- **Env vars**: each app has a `.env.example` (checked in) and `.env`
  (gitignored, real values). Backend loads `.env` via `godotenv` with
  `PORT` as the only var so far; frontend's `vite.config.ts` reads
  `API_PROXY_TARGET` and `FRONTEND_PORT` via `loadEnv` (Node-side only,
  never shipped to the browser bundle — no `VITE_` prefix needed).
- Frontend dev server defaults to port 3000 (not Vite's default 5173) —
  a deliberate choice, not a leftover.

## Secrets

`.claude/settings.json` denies my Read/Grep/Glob on `.env` files and any
`secrets/` path. This is partial protection (Bash `cat backend/.env`
still works) — full sandboxing was deliberately deferred since nothing
sensitive lives in the repo yet. Revisit when phase 2 adds real
credentials (JWT secret, DB connection string).
