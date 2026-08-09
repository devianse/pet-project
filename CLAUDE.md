# pet-projects

Orientation for me, not a duplicate of the other docs — see
[`README.md`](./README.md) for what this is and how to run it, and
[`PLANNING.md`](./PLANNING.md) for the full architecture and reasoning.
Check both before assuming a decision hasn't been made.

## Current phase

Phase 1 (barebones scaffold) is done. Phase 2 (auth, WebSockets) has NOT
started — don't assume either exists. Several small features (Notes,
design system, pre-deployment security hardening, an upcoming Movie
Sharing list) have landed ahead of phase 2/3 as deliberate detours before
a first VPS deployment attempt — see `PLANNING.md`'s "Actual build order
so far" section for the current sequence, not just the original
phase breakdown further down that file.

## Layout

See `README.md`'s "Layout" section for the directory tree. `docs/`
already holds design specs (`docs/superpowers/specs/`); `infra/` gets
its first content (Caddyfile, docker-compose.yml) as part of the
pre-deployment security hardening work.

## Commands

See `README.md`'s "Running locally" and "Other commands" sections for the
full `make` target list — both dev servers block and must run in separate
terminals simultaneously, which README explains in detail.

## Conventions

- **`@/*` path alias**: resolves to `frontend/src/*` (set up in
  `tsconfig.app.json` and `vite.config.ts`). Use it for imports rather
  than relative paths, especially across `frontend/src/components/pouf/`.
- **`frontend/src/components/pouf/` is off-limits for manual edits** —
  it's vendored/CLI-managed (see README's "Layout" section). Regenerate
  or add to it via the shadcn CLI, don't hand-edit.
- **Never commit without being asked.** Staging/committing is the user's
  call, every time — not implied by "this looks done."
- **A PR isn't ready unless `npm audit` (frontend), `govulncheck ./...`
  (backend), and `gitleaks detect --source . -v` all pass clean.** Run
  by hand before proposing a PR as done — no CI enforces this yet. See
  README's "Other commands" for install/usage of each.
- **Env vars**: each app has a `.env.example` (checked in) and `.env`
  (gitignored, real values) — see README's "Env config" section for the
  actual var list. Backend loads its via `godotenv`; frontend's
  `vite.config.ts` reads its via `loadEnv` (Node-side only, never shipped
  to the browser bundle — no `VITE_` prefix needed).
- Frontend dev server defaults to port 3000 (not Vite's default 5173) —
  a deliberate choice, not a leftover.

## Secrets

`.claude/settings.json` denies my Read/Grep/Glob on `.env` files and any
`secrets/` path. This is partial protection (Bash `cat backend/.env`
still works) — full sandboxing was deliberately deferred since nothing
sensitive lives in the repo yet. Revisit when phase 2 adds real
credentials (JWT secret, DB connection string).
