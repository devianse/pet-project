# pet-projects

Orientation for me, not a duplicate of the other docs — see
[`README.md`](./README.md) for what this is and how to run it, and
[`PLANNING.md`](./PLANNING.md) for the full architecture and reasoning.
Check both before assuming a decision hasn't been made.

## Current phase

Phase 1 (barebones scaffold) is done: Go API + React SPA wired end to end
via Vite's dev proxy, env var config in place. Phase 2 (auth, WebSockets)
has NOT started — don't assume either exists. Phase 3 (domain projects)
is further out still. See `PLANNING.md`'s phase breakdown for the full
picture.

## Layout

See `README.md`'s "Layout" section for the directory tree. `infra/` and
`docs/` are deliberately not created yet — nothing to put in either until
deployment/real docs are actually needed. Not an oversight.

## Commands

See `README.md`'s "Running locally" and "Other commands" sections for the
full `make` target list — both dev servers block and must run in separate
terminals simultaneously, which README explains in detail.

## Conventions

- **Never commit without being asked.** Staging/committing is the user's
  call, every time — not implied by "this looks done."
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
