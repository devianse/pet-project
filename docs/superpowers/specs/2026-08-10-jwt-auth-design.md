# Real JWT auth (phase 2) — design

## Status

Deferred item from `PLANNING.md`'s Security TODO section (added during
the pre-deployment security hardening pass,
`docs/superpowers/specs/2026-08-09-pre-deployment-security-design.md`):
"Real JWT auth (phase 2) replacing the Caddy basic-auth stopgap." This
doc is that phase-2 work, scoped via `superpowers:brainstorming`.

## Approach

Site-wide Caddy `basic_auth` (one shared credential pair) currently
gates the whole app as a pre-deployment stopgap. This replaces it with
real per-user login: a `users` table, bcrypt password hashing, and a
signed JWT held in an httpOnly cookie. Scope decided through Q&A against
the actual current state of the repo (reviewed `main.go`, `server.go`,
`App.tsx`, `shared/api.ts`, the Caddyfile and `docker-compose.yml`):

- **Multi-user, invite-only.** Real accounts (not a single hardcoded
  login), but no public registration endpoint — the admin (Mike) seeds
  accounts via a CLI command. Fits the eventual Family Shopping List use
  case (family/friends each get an account) without the attack surface
  of open self-registration on a personal site.
- **Two roles to start: `admin` and `user`.** Enough for "I want to see
  some things other users can't at all" today. Finer-grained
  per-user (not per-role) feature gating is an explicitly deferred
  future task — noted here so the role claim doesn't block it, not
  designed now (YAGNI).
- **Existing data (Notes, Watchlist) stays shared**, not partitioned per
  user — auth gates *access* to the app, it doesn't scope query results.
  No schema change to either feature's tables. Matches how they already
  behave and fits "family shares one list."
- **Single long-lived JWT (no refresh token).** One cookie, ~14 day
  expiry, reissued on login. No refresh endpoint/rotation/revocation
  logic — real scope for a low-traffic personal site with no attacker
  incentive yet. Logout just clears the cookie.
- **httpOnly cookie, not localStorage.** JS never touches the token —
  immune to XSS token theft. `SameSite=Strict` is sufficient CSRF
  protection given Caddy already makes frontend+API same-origin; no
  separate CSRF token needed.
- **Basic-auth is removed**, not layered underneath. It was explicitly a
  stopgap in `PLANNING.md`/README until this existed — keeping both
  would mean two credential systems to maintain.

**Alternatives considered and rejected:** open self-registration
(rejected — bigger attack surface than needed for an invite-only
personal site); an admin HTTP endpoint for account creation (rejected —
another authenticated surface to secure now, when a CLI command does the
job with less exposure); access+refresh token pair (rejected — real
complexity, no current traffic/threat model justifies it); localStorage
token storage (rejected — XSS-readable, no benefit here since there's no
non-browser client yet); per-user data partitioning on Notes/Watchlist
now (rejected — no current need, would be schema churn on two working
features ahead of any actual requirement).

## Data model

New `users` table (owned by a new `internal/auth` package, following the
existing per-feature package + `EnsureSchema` pattern from
`internal/notes` and `internal/watchlist`):

```sql
CREATE TABLE IF NOT EXISTS users (
    id             SERIAL PRIMARY KEY,
    username       TEXT UNIQUE NOT NULL,
    display_name   TEXT,
    password_hash  TEXT NOT NULL,
    role           TEXT NOT NULL DEFAULT 'user',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at  TIMESTAMPTZ
);
```

- `username` is the login identifier (no email usage — not needed for an
  invite-only site with no email-sending capability).
- `display_name` is nullable; UI falls back to `username` when unset.
  Deliberately left as a plain field to expand on later, not designed
  further now.
- `role` is `'admin'` or `'user'`, enforced at the application layer (no
  DB check constraint needed at this scale — CLI is the only writer).
- `last_login_at` updated on each successful login; gives the admin
  visibility into whether an invited account has actually been used.
- No `updated_at`, no soft-delete/`is_active` flag — YAGNI. Revoking an
  account is a `DELETE` via direct DB access or a future CLI subcommand,
  not designed now.

## Backend

### `internal/auth` package

- `Store` (mirrors `notes.Store`/`watchlist.Store` shape): `EnsureSchema`,
  `FindByUsername`, `UpdateLastLogin`, `CreateUser` (used by the CLI).
- JWT signing/verification using `golang-jwt/jwt/v5` (new dependency).
  Claims: `sub` (user id), `username`, `role`, `exp` (~14 days out).
  Secret from a new `JWT_SECRET` env var (backend `.env`/`.env.example`,
  `infra/.env`, `infra/docker-compose.yml`).
- Password hashing via `golang.org/x/crypto/bcrypt`.
- `Handler` with three methods wired into `main.go`'s mux:
  - `POST /api/auth/login` — body `{username, password}`. On success:
    sign JWT, `Set-Cookie` (httpOnly, SameSite=Strict, Path=/, ~14 day
    Max-Age), update `last_login_at`, respond
    `{username, display_name, role}`. On failure (unknown username or
    bad password): generic 401 "invalid credentials" — no
    username-enumeration hints from timing or message differences.
    The `Secure` flag is conditional: on in production, off in local
    dev — a `Secure` cookie is silently dropped by the browser over
    plain `http://localhost`, which is how both `make dev-backend` and
    `make dev-frontend` run. Gated by a new `ENV` env var
    (`production`/`development`, default `development`) rather than
    inferring from the request, so behavior doesn't depend on how a
    request happened to arrive.
  - `POST /api/auth/logout` — sets the cookie with an immediate past
    expiry, clearing it. No body.
  - `GET /api/me` — parses the cookie, verifies the JWT, returns
    `{username, display_name, role}`; 401 if the cookie is missing,
    invalid, or expired.
- `Require` middleware: verifies the cookie the same way `/api/me` does,
  401s on failure, otherwise injects parsed claims into request context
  via `context.WithValue` (a typed key, not a raw string) so downstream
  handlers can read user id/role if they need to.

### Wiring into `main.go`

`Require` wraps every existing route (`/api/notes`, `/api/watchlist`,
and anything added later) — added once, applied via the same mux-wrapping
pattern `maxBytesMiddleware` already uses in `server.go`, not per-route.
`GET /api/health` stays unauthenticated (container healthchecks and any
future uptime monitoring hit it without a session). The three
`/api/auth/*` routes and `/api/me` are also unauthenticated by nature
(login must be reachable logged-out; `/api/me` is how the frontend
*discovers* logged-out state).

### `cmd/createuser` CLI

New `cmd/createuser/main.go`, following `cmd/api`'s pattern of loading
`DATABASE_URL` via `godotenv`/env. Flags: `-username`, `-role`
(default `user`); prompts for password on stdin (not a flag — avoids it
landing in shell history). Hashes with bcrypt, inserts via `auth.Store`,
rejects duplicate usernames with a clear error. Run via
`go run ./cmd/createuser -username=mike -role=admin` locally, or
`docker compose exec backend ...` / `docker compose run --rm backend ...`
against the deployed stack.

## Frontend

- New `AuthContext` (`frontend/src/shared/auth.tsx` or similar) — on
  mount, calls `GET /api/me` once; exposes `{user, loading, refresh,
  logout}`. `user` is `null` until a successful `/api/me` resolves.
- New `/login` route + `features/auth/Page.tsx` — username/password
  form, `POST /api/auth/login`; on success calls context's `refresh()`
  and navigates to `/`. Shows the generic "invalid credentials" error
  inline on failure.
- `RequireAuth` wrapper in `App.tsx` around the whole `<Routes>` tree
  (everything is gated, not per-route): while `loading`, render nothing
  (or a minimal spinner); if `user` is `null`, redirect to `/login`;
  otherwise render the routes normally. `/login` itself is the one route
  outside this wrapper.
- `AppShell` nav gets a logout control showing `display_name ||
  username`; posts `/api/auth/logout`, clears context state, redirects
  to `/login`.
- `shared/api.ts`: no per-call header changes needed (cookie sent
  automatically by the browser), but add one shared behavior — any `401`
  response clears auth context and redirects to `/login` (covers a
  session that expired mid-use, not just app-load state).

## Rollout / migration

Ordering matters here: the deploy that removes basic-auth must also seed
the first admin account in the same pass, so the live site is never
gate-less between the two.

1. `infra/Caddyfile`: remove the `basic_auth` block entirely.
2. `infra/docker-compose.yml` / `infra/.env.example`: drop
   `BASIC_AUTH_USER`/`BASIC_AUTH_PASSWORD_HASH`, add `JWT_SECRET`
   (generated fresh for production, not reused from local dev).
3. `backend/.env.example`: add `JWT_SECRET` and `ENV` (documented default
   `development`; production `docker-compose.yml`/`infra/.env` sets
   `ENV=production` so the login cookie gets `Secure`).
4. Deploy the new stack (`docker compose up --build -d`), then
   immediately `docker compose exec backend go run ./cmd/createuser
   -username=mike -role=admin` (or a pre-built binary if `go run` isn't
   available in the runtime image) before considering the deploy done —
   the site is otherwise reachable with no way to log in and no way to
   gate it.
5. `README.md`: update the "Deploying" section's basic-auth paragraph
   describing what gates the site now.
6. `PLANNING.md`: check off "Real JWT auth (phase 2)" in the Security
   TODO section.

## Testing

- **Backend**: table-driven tests for `POST /api/auth/login` (correct
  password, wrong password, unknown username — all paths return the
  same generic error), `GET /api/me` with a valid/missing/expired
  cookie, `Require` middleware 401 behavior on a wrapped test route,
  `cmd/createuser` (hash round-trips via `bcrypt.CompareHashAndPassword`,
  duplicate username rejected).
- **Frontend**: `RequireAuth` redirects to `/login` when logged out and
  renders children when logged in; login form's success path (redirect
  to `/`) and failure path (inline error, stays on `/login`).
- **Manual, post-deploy**: confirm `/api/notes`/`/api/watchlist` 401
  without a cookie, login sets a cookie and unlocks the app, logout
  clears it, basic-auth challenge no longer appears.
