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

Phase 1 (barebones scaffold) is done. Phase 2 is done in full: real JWT
auth (invite-only accounts, httpOnly session cookie, `admin`/`user`
roles) *and* the WebSockets shell (`internal/realtime`, `GET /api/ws`,
PR #21) are both merged to `main` and in production use — the Ops panel
already consumes `ops.health`/`ops.audit` over it, so don't treat
WebSockets as unbuilt.

Beyond that, several features landed ahead of phase 2/3 as deliberate
detours: Notes, the design system, pre-deployment security hardening,
Movie/TV Sharing List, Date Night, the admin grants panel, the audit
log/Ops panel, the Telegram bot, graceful shutdown, Postgres
backup/restore, and the reminders primitive — alongside a first VPS
deployment (live at `mikelab.dev`). See `planning/history.md` for the
current sequence, not just the original phase breakdown in
`PLANNING.md`.

Open threads worth knowing before you start: WebSockets has no consumers
beyond Ops yet; the WS authenticator still trusts the role from the JWT
claim rather than re-reading it from the database (see
`planning/security-todo.md`); notes are still ownerless/shared; and the
Subscriptions feature has an approved spec and plan but no code.

## Layout

See `README.md`'s "Layout" section for the directory tree. `docs/`
holds the ADRs (`docs/adr/`); the design specs and implementation plans
under `docs/superpowers/` are **gitignored and exist only on the
author's machine** — they are not in a fresh clone and not on GitHub, so
treat a `docs/superpowers/...` link elsewhere in these docs as
unresolvable unless you are on that machine. `infra/` is populated
(Caddyfile, docker-compose, backup/restore scripts) and the stack is
deployed. `planning/` holds history/decisions/security-TODO notes split
out of `PLANNING.md` for size — see that file's own pointer table for
what's where.

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
- **One scope, one branch, one worktree.** Independent changes (a dep
  bump, a docs refresh, a behaviour fix) get separate branches even when
  found in one sitting — they have different review and revert stories.
  `git worktree add -b fix/<scope> ../pp-wt-<scope> origin/main` keeps
  them building in parallel without stashing; the close-out skill's
  Step 5 removes the worktree once the branch is pushed.
- **Never commit without being asked.** Staging/committing is the user's
  call, every time — not implied by "this looks done."
- **A PR isn't ready unless `npm audit` (frontend), `govulncheck ./...`
  (backend), and `gitleaks detect --source . -v` all pass clean.** No CI
  enforces this yet — the `closing-out-a-feature` skill runs these
  automatically, then pushes the branch, opens the PR, and updates
  planning docs in their final already-shipped state (citing the real
  PR number) before handing off to `finishing-a-development-branch` for
  the squash-merge/keep decision — see that skill's own doc for why the
  PR is opened first. See README's "Other commands" for install/usage
  of each if running by hand.
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
