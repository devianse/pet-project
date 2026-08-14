# pet-projects

Orientation for me, not a duplicate of the other docs — see
[`README.md`](./README.md) for what this is and how to run it,
[`PLANNING.md`](./PLANNING.md) for the full architecture and reasoning,
[`CONTEXT.md`](./CONTEXT.md) for domain terminology, and
[`docs/adr/`](./docs/adr/) for durable architectural decisions (why, not
just what — `planning/decisions.md` covers the rest as a lighter-weight
open-questions log). Check all before assuming a decision hasn't been
made, or a term is undefined.

## Current phase

Phase 1 (barebones scaffold) is done. Phase 2's first half — real JWT
auth (invite-only accounts, httpOnly session cookie, `admin`/`user`
roles) — is done and merged to `main`. Phase 2's second half,
WebSockets, has NOT started — don't assume it exists. Several small
features (Notes, design system, pre-deployment security hardening,
Movie/TV Sharing List, Date Night) have landed ahead of phase 2/3 as
deliberate detours, alongside a first VPS deployment (live at
`mikelab.dev`) — see `planning/history.md` for the current sequence, not
just the original phase breakdown in `PLANNING.md`.

## Layout

See `README.md`'s "Layout" section for the directory tree. `docs/`
already holds design specs (`docs/superpowers/specs/`); `infra/` gets
its first content (Caddyfile, docker-compose.yml) as part of the
pre-deployment security hardening work. `planning/` holds history/
decisions/security-TODO notes split out of `PLANNING.md` for size — see
that file's own pointer table for what's where.

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
  (backend), and `gitleaks detect --source . -v` all pass clean.** No CI
  enforces this yet — the `closing-out-a-feature` skill runs these
  automatically (plus planning-doc updates and dev-resource cleanup)
  once a feature branch is done, before the merge/PR decision. See
  README's "Other commands" for install/usage of each if running by
  hand.
- **Env vars**: each app has a `.env.example` (checked in) and `.env`
  (gitignored, real values) — see README's "Env config" section for the
  actual var list. Backend loads its via `godotenv`; frontend's
  `vite.config.ts` reads its via `loadEnv` (Node-side only, never shipped
  to the browser bundle — no `VITE_` prefix needed).
- Frontend dev server defaults to port 3000 (not Vite's default 5173) —
  a deliberate choice, not a leftover.
- **Backend Go code**: apply the `golang-design-patterns` skill —
  Design mode when a task involves a new constructor, package shape, or
  architecture choice (during planning or a `subagent-driven-development`
  task); Review mode as one lens of the final whole-branch review before
  merge, alongside `code-review`/`security-review`.

## Secrets

`.claude/settings.json` denies my Read/Grep/Glob on `.env` files and any
`secrets/` path. This is partial protection (Bash `cat backend/.env`
still works) — full sandboxing was deliberately deferred since nothing
sensitive lives in the repo yet. Revisit when phase 2 adds real
credentials (JWT secret, DB connection string).
