# Build history

Chronological record of what's actually shipped, since it no longer
matches `PLANNING.md`'s shell-first order one-for-one — see that file's
"Framing" section for the intended order and why detours happened.

1. **Phase 1 scaffold** — Go API + React SPA wired end to end via Vite's
   dev proxy, env var config. Matches the original plan.
2. **Notes** (done) — a small no-auth CRUD page, pulled forward onto
   Postgres ahead of phase 3's "real" domain projects, as a low-stakes
   practice feature. See
   `docs/superpowers/specs/2026-08-08-notes-design.md`.
3. **Design system** (done) — adopted 1st-Pouf (shadcn registry) as the
   visual identity for the shell + Notes, rather than hand-rolling
   tokens. See `docs/superpowers/specs/2026-08-09-design-system.md`.
4. **Pre-deployment security hardening** (done) — Caddy/Postgres
   infra config, backend timeout/body-size hardening, basic-auth stopgap
   ahead of real JWT auth, manual dependency/secret scans. Needed
   because step 6 below deploys without phase-2 auth existing yet. See
   `docs/superpowers/specs/2026-08-09-pre-deployment-security-design.md`.
5. **Movie/TV Sharing List** (done) — another pre-phase-3 detour,
   same pattern as Notes: a shareable watchlist, paste an IMDb link, get
   a preview card (title, description, poster image) resolved via TMDb,
   mark items as viewed/expand for detail. See
   `docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md`
   and `docs/superpowers/plans/2026-08-09-movie-tv-sharing-list.md`.
6. **First VPS deployment attempt** (done, 2026-08-09) — live at
   `https://mikelab.dev`, domain bought on reg.ru, deployed on a Cloudzy
   Frankfurt VPS (1 vCPU/2GB/60GB, hourly billing as a ~3-week trial —
   see `PLANNING.md`'s "Deployment target" section for the planned move
   to Timeweb once that runs out). Server hardened first (non-root sudo
   user, root SSH disabled, password auth disabled, SSH moved off port
   22, `ufw` deny-by-default with only SSH/80/443 open), then Docker +
   Compose installed, repo cloned, production `infra/.env` filled in
   (fresh Postgres password, real TMDb token, Caddy basic-auth hash),
   DNS A record pointed at the VPS, `docker compose up -d --build`.
   Caddy obtained a real Let's Encrypt cert automatically, all 3
   containers healthy on first boot. Full step-by-step in
   `infra/deployment-runbook/` — written to make the Timeweb move a
   checklist, not a from-scratch redo. One real bug caught and fixed
   during this pass: `infra/docker-compose.yml` wasn't passing
   `TMDB_READ_ACCESS_TOKEN` into the backend container at all (backend
   exits on startup without it) — added alongside this deployment.
7. **Real JWT auth** (phase 2, first half — done, merged to `main` via
   the `jwt` branch, PR #4). Replaces the Caddy basic-auth stopgap from
   step 4: a `users` table (invite-only, no public registration —
   accounts are seeded via a new `cmd/createuser` CLI), bcrypt password
   hashes, a signed JWT held in an httpOnly `SameSite=Strict` session
   cookie, `admin`/`user` roles (fine-grained per-user permissions
   deliberately deferred at the time — since resolved, see
   `planning/decisions.md`), and the whole SPA gated behind login. Full
   design/build
   record: `docs/superpowers/specs/2026-08-10-jwt-auth-design.md` and
   `docs/superpowers/plans/2026-08-10-jwt-auth.md`. Built via
   `superpowers:subagent-driven-development` — 14 tasks, a final
   whole-branch review caught one real Critical (the deploy runbook's
   admin-seeding command didn't actually work against the built Docker
   image — fixed) plus a mobile-nav logout gap, both fixed before this
   was considered done. One accepted gap: the frontend auth flow
   (login/logout/redirect) was verified via backend curl calls + code
   review, not an actual rendered browser session — no browser
   automation tool was available in that session. (Since resolved by
   usage, not a dedicated pass — see `planning/decisions.md`.)
8. **Date Night** (done) — another pre-phase-3 detour, a playful
   propose/respond page scoped to exactly two paired accounts
   (`DATE_NIGHT_USERNAMES`, matched against seeded account usernames):
   one person proposes a day, time slot, and activity, the other accepts
   or declines. It's the first feature to actually use per-user identity
   from the JWT auth step 7 landed — Notes and Watchlist are open behind
   the login wall but don't attribute actions by user. Shipped as an
   explicit v1, with Phase 2 work (WebSockets for live updates, Telegram
   notify on propose/accept/decline) documented but deliberately
   deferred in its own spec. See
   `docs/superpowers/specs/2026-08-12-date-night-design.md` and
   `docs/superpowers/plans/2026-08-12-date-night.md`.
9. **User profile / `/api/me` QOL** (done) — closed the `display_name`
    write path left open by the feature-visibility design: a
    `PATCH /api/me` endpoint (full-replace semantics — the frontend
    always resends both fields) plus a `cmd/createuser -display-name`
    flag, and a new user-editable `avatar_color` column (one of six
    pouf "brand" tones). Frontend gained an avatar-triggered popover
    (hand-authored over raw `@radix-ui/react-popover`, since pouf's own
    `DropdownMenu` only supports static click rows) replacing
    `AppShell`'s old plain username+logout block — editable display
    name, read-only `@username`, "member since" date, an avatar-color
    swatch picker, logout. Deliberately skipped speculative fields
    (email, timezone, bio/status, avatar image upload) as YAGNI. Minor
    intersection fixed at the same time: Date Night's "proposed by"
    label now prefers `display_name` over `username` (`COALESCE(u.
    display_name, u.username, 'someone')` in `internal/datenight/
    proposals.go`), keeping the `proposed_by_username` field name as-is
    since only the resolved value changed. No design doc — scoped and
    approved conversationally as a bounded QOL pass, not an
    architectural change.
10. **Admin grants UI** (done, merged to `main` via the `grants` branch,
    PR #12) —
    closes the "admin web UI for managing grants" follow-up deferred by
    the feature-visibility design (step 9's sibling decision; see
    `planning/decisions.md`). Backend: DB-re-checked `access.RequireRole`
    middleware (same staleness-safe pattern as `HasFeature`'s admin
    bypass) and `access.AdminHandler` (list users with their granted
    features; idempotent grant/revoke per user/feature). Frontend: a
    `/admin` page, nav-gated to admins only, showing a matrix of users ×
    known features with a per-cell toggle button (Grant/Granted,
    per-cell pending state so one in-flight toggle doesn't block the
    rest of the table); heading reads "Admin" with an "Access" sub-label
    left room for future admin sections (e.g. the still-open role
    promote/demote follow-up) to land alongside it. See
    `docs/superpowers/specs/2026-08-14-admin-grants-ui-design.md` and
    `docs/superpowers/plans/2026-08-14-admin-grants-ui.md`. Built via
    `superpowers:subagent-driven-development`, 10 tasks; one fix wave
    off the final whole-branch review (refetch-on-failure in the toggle
    handler, compile-time-exhaustive feature-label map). Same accepted
    gap as the `jwt` branch (step 7): the rendered frontend was checked
    via a curl-driven server-side E2E pass and static task review, not
    an actual browser click-through — no browser automation tool was
    available in that session.
11. **Role promote/demote** (done, merged to `main` via the `access`
    branch, PR #13) — closes the "changing an existing user's role" follow-up
    left open by step 10, sequenced onto the same `/admin` page's Access
    section. Backend: `auth.Store.UpdateRole` and a
    `PUT /api/admin/users/{id}/role` handler on `access.AdminHandler`,
    validating the role value and 404ing on an unknown user id like the
    grant/revoke handlers already do. Self-demote is blocked twice: the
    frontend disables the control on the caller's own row, and the
    handler independently 403s if the target id matches the caller's id
    from JWT claims (a UI-only guard is trivially bypassed with curl,
    and `RequireRole` already re-verifies role server-side for the same
    reason). Frontend: the grants matrix gained a "Role" column with a
    `user`/`admin` `<select>` per row — a dropdown rather than a toggle
    button like the feature cells, since the role list may grow later
    (unlike Grant/Granted's fixed boolean). Bounded-path work (brainstormed,
    short in-chat design, no spec doc): the existing admin grants flow
    was extended, not a new subsystem. TDD throughout on the backend
    (store + handler tests, each watched fail before the code existed);
    no frontend test infra exists in this repo yet, so the UI change
    followed the same untested convention as the rest of `/admin`.
12. **User lifecycle management** (done, merged to `main` via the
    `user-lifecycle` branch, PR #15) — the next admin-panel feature after a brainstorm of
    what else belongs there once gating/grants existed (see
    `planning/decisions.md`), grouping user creation, deactivate/
    reactivate, and admin-triggered password reset into one pass since
    all three mutate the same `users` row and extend the same `/admin`
    Access section as steps 10-11. Backend: a new `users.is_active`
    column (`ADD COLUMN IF NOT EXISTS`, defaults `true`), `auth.Store.
    SetActive`/`SetPasswordHash`, `auth.Handler.Login` rejecting a
    deactivated user through the same generic 401 as a wrong password
    (no signal that distinguishes the two), and three new
    `access.AdminHandler` endpoints: `CreateUser` (`POST /api/admin/
    users`, mirrors `cmd/createuser`'s validation/bcrypt hashing, 409 on
    a duplicate username), `SetActive` (`PUT /api/admin/users/{id}/
    active`, self-deactivate blocked server-side off JWT claims — same
    guard shape as `UpdateRole`'s self-demote block), and
    `ResetPassword` (`POST /api/admin/users/{id}/reset-password`, no
    self-guard needed since it isn't a privilege-loss risk; the
    plaintext password is never stored or logged, only its hash).
    Frontend: a "Create user" form card above the grants matrix
    (username/password/role), an Active/Deactivated toggle per row
    (disabled on the caller's own row, tone `up`/`down`), and a per-row
    inline password-reset form (type-or-Generate, client-side generator
    is a convenience only — the server never sees or trusts it) that
    shows the new password once in a dismissible banner after a
    successful reset, matching the chosen "admin sets a temp password,
    shown once" design (self-service email reset was considered and
    explicitly deferred — needs email infra this app doesn't have yet).
    Deactivation is soft-delete only: hard delete was ruled out up front
    as bad practice for DB management, so there's no cascade-delete
    surface to reason about. Bounded-path work (brainstormed, short
    in-chat design, no spec doc) — an extension of the existing admin
    flow, not a new subsystem. TDD throughout on the backend (store +
    handler tests for every new method/endpoint, each watched fail
    first); frontend followed the same untested convention as the rest
    of `/admin`. `govulncheck`/`npm audit`/`gitleaks` all ran clean
    (`govulncheck` surfaced 6 pre-existing Go-toolchain stdlib
    vulnerabilities unrelated to this branch — no new dependency was
    added). Same accepted gap as steps 10-11: no browser click-through,
    verified via backend tests and static review only.
13. **Audit trail + ops/system panel** (done, on the `audit-ops` branch,
    not yet merged) — the first of the two deferred admin-panel
    observability options from step 12's decision (the other, app data
    moderation, stays deferred — see `planning/decisions.md`). A
    read-only introspection page added to the same `/admin` panel as
    steps 10-12: an admin-action audit log (who granted/revoked
    features, created/deactivated/reactivated a user, reset a password,
    or changed a role, and when) plus basic system health (DB
    connectivity, deployed git SHA). Backend: a new `admin_audit_log`
    table (actor, action, optional target user, free-text detail,
    timestamp), `access.Store.LogAction`/`ListAuditLog` (newest-first,
    capped at 100 rows), and a `GET /api/admin/audit-log` handler.
    Every existing mutating admin handler (`GrantFeature`,
    `RevokeFeature`, `CreateUser`, `SetActive`, `ResetPassword`,
    `UpdateRole`) now calls a shared `logAction` helper after a
    successful write — a logging failure doesn't fail the request
    itself. `cmd/api`'s `/api/health` handler was extended to ping the
    DB and report a `GIT_SHA`-sourced version string (falls back to
    "unknown" locally where that env var isn't set); `infra/
    docker-compose.yml` and both deployment-runbook docs pass `GIT_SHA`
    through at deploy time. Frontend: a new "Ops" section on `/admin`
    below the existing Access table — a system-health card and an
    audit-log table (When/Actor/Action/Target/Detail). Bounded-path
    work (brainstormed, short in-chat design approved, no spec doc) —
    an extension of the existing admin flow, not a new subsystem. TDD
    throughout on the backend (store + handler tests, each watched fail
    first, run against real Postgres); frontend followed the same
    untested convention as the rest of `/admin`. Verified end-to-end
    with a curl-driven session against a locally running backend (real
    JWT cookie, grant + deactivate actions, confirmed both showed up
    correctly ordered in the audit log) — same accepted browser-
    click-through gap as steps 10-12. `govulncheck`/`npm audit`/
    `gitleaks` all ran clean (`govulncheck` surfaced the same 6
    pre-existing Go-toolchain stdlib vulnerabilities as step 12,
    confirmed present even with this branch's changes stashed out — not
    introduced here; no new dependency was added).
14. **Telegram bot v1** (done, on the `telegram` branch, not yet merged)
    — a commands-in Telegram bot for the app, implemented per
    `docs/superpowers/specs/2026-08-14-telegram-bot-design.md` and
    `docs/superpowers/plans/2026-08-14-telegram-bot-v1.md` via
    subagent-driven development, no worktree (branch checked out
    directly). New `backend/internal/telegram` package: a `Client`
    (`SendMessage`/`GetUpdates` against the Telegram Bot API,
    long-polling with a 30s timeout), a `Router` (prefix-matched
    command dispatch, "unknown command" fallback), and a `Poller`
    (offset-tracked polling loop with exponential backoff on transport
    errors, run as a background goroutine mirroring the existing
    `loginLimiter.startCleanup` pattern). `cmd/api` wires it up via
    `startTelegramBot`, gated on `TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID`
    both being set (optional-start: the server runs normally with the
    bot disabled if either is missing) and restricted to a single
    allowed chat ID. Two commands ship: `/notes` (lists the notes
    feature's entries, capped under Telegram's ~4096-char message limit
    with a truncation trailer) and `/newnote <text>` (creates one). No
    DB schema change — reuses the existing `internal/notes` store.

    The final whole-branch review (most capable model) caught and fixed
    one Critical and three Important issues before merge-readiness: a
    transport-error path that leaked the bot token into logs via an
    embedded request URL (fixed by unwrapping `*url.Error` before
    logging); `infra/docker-compose.yml` never forwarding the two new
    env vars into the container (the plan's "no compose changes"
    constraint turned out to be a plan bug, not a real constraint —
    ruled and fixed, since it left the feature permanently inert in
    production); the `/notes` reply exceeding Telegram's message-length
    cap with no visible error; and one malformed update in a batch
    permanently wedging the poller (contradicted the spec's own "one
    bad update doesn't block subsequent ones"). All four fixed in one
    consolidated fix wave, re-reviewed clean.

    Closing this branch out also surfaced and fixed a pre-existing,
    unrelated test-isolation gap: bumping the Go toolchain
    1.26.5→1.26.6 (clearing 6 stdlib CVEs `govulncheck` flagged, present
    on `main` too) invalidated Go's test cache, which exposed that
    several `internal/access` tests asserted `ListAuditLog`'s *global*
    unfiltered count — never reliable against a shared local dev
    Postgres that also backs real admin-panel usage — and that
    `internal/auth`'s test cleanup could hit a foreign-key violation
    deleting fixture users referenced by stale `admin_audit_log` rows.
    Both fixed (scoped the audit-log assertions to their own test
    actor; clear referencing audit rows before the user delete),
    verified stable across repeated uncached full-suite runs.
    `govulncheck`/`npm audit`/`gitleaks` all clean.

    Task 5 of the plan (manual verification against the real Telegram
    API) was explicitly not automated — needs a real bot token and a
    real Telegram account, left for manual follow-up.
15. **Backend process lifecycle hardening** (done, on the
    `backend-graceful-shutdown` branch, not yet merged) — a small
    follow-up prompted by a `golang-design-patterns` review-mode audit
    of the backend, done ahead of phase 2's WebSockets work so
    shutdown/lifecycle plumbing exists before long-lived socket
    connections make it more consequential. Three fixes: `cmd/api`'s
    `main` now derives a root `context.Context` from
    `signal.NotifyContext` (SIGINT/SIGTERM) and threads it through the
    Telegram poller and the login-rate-limiter cleanup loop (both
    previously started with `context.Background()`, so neither could be
    stopped independently of killing the whole process); `srv.ListenAndServe`
    now runs in its own goroutine so the main goroutine can call
    `srv.Shutdown` (10s drain timeout) on that same signal, closing the
    DB pool only after the server has actually stopped instead of
    racing it; and `internal/db.Open`'s startup `Ping` gained a 5s
    timeout, matching every other external call in the codebase (TMDb,
    Telegram) instead of being the one unbounded one. No schema change,
    no new dependency, no env var change. `govulncheck`/`npm
    audit`/`gitleaks` all ran clean.

    Deliberately left for later: connection-pool sizing
    (`SetMaxOpenConns`/`SetMaxIdleConns`/`SetConnMaxLifetime`) — flagged
    by the same audit but judged low-value at this app's current
    traffic (a handful of invited users), not worth bundling in here.
16. **Telegram bot: `/help` + native command menu, dev+prod deployment**
    (done, merged to `main` via PR #19 "telegram help menus") — a
    follow-up to step 14's v1. `Router.Handle` gained a `description`
    parameter stored alongside each prefix; `Router.Commands()` and
    `Router.HelpText()` read that list back, so the in-chat `/help`
    reply is generated from whatever's actually registered rather than
    hand-maintained. `unknownCommandReply` now points users at `/help`.
    `Client.SetMyCommands` wraps Telegram's `setMyCommands` endpoint
    (stripping the leading `/` and any trailing arg-placeholder space
    from each prefix) and is called once at startup from
    `startTelegramBot`, right after registering `/notes`, `/newnote`,
    and `/help` — this populates Telegram's native command-menu button
    (the "/" icon next to the message box) with the same descriptions,
    best-effort (logged, not fatal, if it fails).

    Deployment: dev bot created via `@BotFather` and verified
    end-to-end locally first (`/notes`/`/newnote`/`/help`, confirmed
    via the local `make dev-backend` process). Then, per the two-bot
    decision (`decisions.md`), a separate prod bot was created and its
    token plus the (unchanged) chat ID added to `infra/.env` on the
    VPS; `docker compose up -d` picked up the new env values without a
    rebuild (no code changed on the VPS side, `infra/.env` already had
    both vars wired into `docker-compose.yml` from step 14). Verified
    live against the prod bot the same way as dev.
17. **Home page redesign** (done, not yet committed) — replaced the
    Phase 1 wiring-check stub (title + raw backend-health text, now
    that Admin's Ops section owns health) with a combinatoric,
    mobile-first cover page. Bounded-path work (brainstormed, short
    in-chat design approved, no spec doc). `frontend/src/features/home/`
    picks four slots independently at random on every mount — persona
    (`personas.ts`: corporate-SaaS, glitch/error, Geocities '99,
    guilt-trip visitor counter, pirate, noir detective, hype announcer,
    fortune cookie, plus a wholesome control case), gimmick
    (`Gimmick.tsx`: do-not-click button, absurd cookie-consent trio,
    fake terminal, avatar-poke, magic 8-ball, rate-my-vibe, stuck-at-99%
    loading bar), avatar entrance (`entrances.ts`: 8 framer-motion
    choreographies), and background accent (`Accent.tsx`: confetti,
    gradient blob, scanlines, sparkles, bubbles, pulsing grid, calm) —
    3,528 total combinations, re-rolled fresh on every visit. All
    motion respects `useReducedMotion()` (matching the existing
    `pouf/disclosure.tsx` convention), stays transform/opacity-only for
    phone performance, and touch-adapts the one hover-dependent bit
    (the dodging "Close" button jumps on tap instead). New `home-*`
    keyframes live in `index.css` since vendored `pouf.css` is
    off-limits for hand edits. `tsc -b`, `oxlint`, and `vite build` all
    ran clean; no frontend test framework exists in this repo yet, so
    verification was build/typecheck plus a manual look at the running
    dev server rather than an automated or browser-driven check (no
    browser automation tool was available in this session).
18. **Postgres backup strategy** (done, on the `something` branch,
    pushed, PR not yet created) — a manual, run-on-demand way to pull a
    Postgres backup off the VPS onto the operator's dev machine and
    restore one, the concrete capability needed before an eventual VPS
    migration. See
    `docs/superpowers/specs/2026-08-15-postgres-backup-design.md` and
    `docs/superpowers/plans/2026-08-15-postgres-backup.md`. Two
    standalone scripts: `infra/backup.sh` SSHes into the VPS, runs
    `pg_dump -Fc` inside the already-running `postgres` container via
    `docker compose exec`, streams the result straight to a local
    timestamped file under `~/pet-projects-backups/` (override via
    `BACKUP_DIR`, deliberately outside the repo clone so a dump full of
    usernames/password hashes/notes content can never land inside a
    `git add -A`) with `umask 077` so it lands owner-only, then prunes
    to the 5 most recent local dumps per db. `infra/restore.sh` runs
    `pg_restore --clean --if-exists` against a target container
    (`docker compose`'s `postgres` service by default, or an explicit
    container name/ID for local testing) via `docker exec`. No cron, no
    wrapper around deploy, no new accounts/credentials, no cloud
    storage — both scripts are standalone commands run by hand, per the
    design doc's explicit non-goals. Built via
    `superpowers:subagent-driven-development`, 3 tasks (work in place on
    `something`, no worktree, per explicit direction), each with an
    independent implementer + task review (all clean, 2 minors
    deferred). A final whole-branch review caught and fixed 2 Important
    issues before merge-readiness: the migration walkthrough as written
    told the operator to run `restore.sh` on their dev machine, where it
    can't find the target container at all (`restore.sh` resolves via
    `docker compose ps` against whatever Docker daemon it runs on — the
    walkthrough now scps the dump to the new VPS and runs `restore.sh`
    there); and neither script was mentioned anywhere in git-tracked
    docs (`README.md`'s "Other commands" now lists both, plus the
    previously-undocumented `make redeploy` target). Also fixed: a
    stale runbook-path reference in `restore.sh`'s header, a unit-test
    file whose "not -e" comment was silently defeated by sourcing
    `backup.sh` (which re-enables `set -e`), and `restore.sh`'s
    container-resolution hardened against `docker compose` itself
    erroring (not just returning empty) with new test coverage for that
    path. All fixes re-reviewed clean. Both shell test suites pass,
    including `restore_test.sh`'s real dump → wipe → restore round-trip
    against a throwaway `postgres:16-alpine` container. `make
    test-backend` unaffected (no Go code touched).

    One real mid-implementation surprise: the new runbook page
    (`infra/deployment-runbook/06-restore-backup.md`, numbered 06 since
    04/05 were already taken by files added after the plan was written)
    could not be committed — `infra/deployment-runbook/` turned out to
    already be entirely gitignored (`.gitignore:34`, "Personal
    reference, not project docs", grouped with `CHEATSHEET.local/`),
    despite `PLANNING.md`/this file describing it elsewhere as reviewed
    project documentation for the Timeweb migration checklist (steps 6
    and 7's build entries above). `git log --all` confirms the folder
    has never been committed on any branch. Escalated directly rather
    than guessed at; decided (2026-08-15) to leave it gitignored for
    now — the new page and its edits to `00-index.md`/`04-known-gaps.md`
    exist on disk only, uncommitted. Revisit whether the whole
    `infra/deployment-runbook/` gitignore rule should be lifted — it
    looks like a leftover from when the folder was a single
    `.local.md`-suffixed file, not revisited when it became a real
    multi-page directory (see "Still open" in `planning/decisions.md`
    if this hasn't been resolved by the time this is read).
19. **Ops panel live-update** (done, on the `ops` branch, not yet
    committed) — closes the deferred follow-up from step 13: the
    `/admin` panel's health card and audit log now push updates over
    the WebSockets shell instead of only fetching once on mount.
    Bounded-path work (brainstormed, short in-chat design approved, no
    spec doc) — the shell already existed; this is its first real
    consumer. Backend: `realtime.Hub.SubscriberCount(topic)` (lets a
    background producer skip work when nobody's listening); a new
    `internal/ops` package with `HealthTicker` (functional-options
    constructor matching `realtime.Hub`'s own pattern, `WithInterval`)
    that ticks every 15s, skips the tick entirely if `ops.health` has
    zero subscribers, otherwise pings the DB — bounded by its own 5s
    `context.WithTimeout` derived from the ticker's long-lived run
    context, not the run context directly — and broadcasts a
    status/db/version payload; `access.Store.LogAction` now returns the
    created `AuditEntry` (read back via a shared `auditEntryQuery`
    constant + `scanAuditEntry` helper, also reused by `ListAuditLog`)
    instead of just an error, and `access.AdminHandler` gained a
    `broadcaster` interface (`Broadcast(realtime.Envelope)`, defined at
    the consumer per the codebase's existing pattern, nil-safe so every
    pre-existing test call site stays valid passing `nil`) so every
    successful admin mutation broadcasts the new audit entry on
    `ops.audit`. `cmd/api/main.go` wires an admin-only topic authorizer
    for `ops.*`, constructs and runs the `HealthTicker` alongside the
    server's other background goroutines, and passes the hub into
    `NewAdminHandler`. Frontend: `features/admin/Page.tsx` subscribes to
    both topics via the existing `useRealtimeTopic` hook, replacing
    nothing about the mount-time fetch (still the initial load) but
    updating state live as broadcasts arrive.

    A `golang-design-patterns` review pass (explicitly requested)
    surfaced one real finding on the first draft: `HealthTicker.tick`
    was passing the long-lived `Run` context straight into
    `PingContext`, so a hung DB connection could block indefinitely and
    silently stall every future 15s broadcast. Fixed via TDD (a new
    `TestHealthTicker_PingIsBoundedByItsOwnTimeout` watched RED via a
    `fakePinger` that records whether the context it received carried a
    deadline, then GREEN via the per-tick `context.WithTimeout`
    described above) before the branch was considered review-clean. No
    other findings — no `init()`/global state, resources already
    bounded, options pattern already consistent with the rest of the
    codebase.

    Verified: full backend suite (`go test -p 1 ./...` against a real
    Postgres) green except the pre-existing, unrelated
    `TestHandler_Me_Features_Admin` failure in `internal/auth`
    (confirmed via `git stash` to fail identically on unmodified code);
    `gofmt -l .` / `go vet ./...` clean; frontend `tsc -b`, `oxlint`,
    `vite build` all clean. Same accepted browser-click-through gap as
    every other `/admin` change (steps 10-13): verified by test suite
    and code review, not a rendered browser session.
20. **Scheduled reminders** (done, merged to `main` via the `sheduler`
    branch, PR #23) — the generic "fire a Telegram message at a future
    time, once" primitive scoped out while brainstorming the
    subscriptions/finance tracker (`decisions.md`), built ahead of that
    tracker itself per the same generic-capability-first pattern the
    WebSockets shell used for the Ops panel (step 19). Design:
    `docs/superpowers/specs/2026-08-17-reminders-design.md`. New
    `internal/reminders` package: a Postgres-backed `Store`
    (`Schedule`/`Cancel`/`Reschedule`/`ListPending`/`MarkSent`,
    `Cancel`/`Reschedule` idempotent no-ops when no pending row matches
    `source`) and a `Ticker` (functional-options, mirrors
    `internal/ops.HealthTicker`'s shape, including its per-call
    `context.WithTimeout` pattern — added in a final-review fix round,
    see below) that polls hourly for due+pending reminders and delivers
    them via the existing `internal/telegram.Client.SendMessage` — no
    new delivery mechanism, no cadence/recurrence logic (that stays
    with whichever feature owns a recurring obligation, starting with
    the subscriptions tracker). A new `/reminders` command on the
    existing Telegram `Router`, alongside `/notes`/`/newnote`/`/help`,
    lists pending reminders soonest-first with human-relative due dates
    ("in 2 days (Aug 19)"/"today"/"overdue"), reading `ListPending`
    directly rather than through the ticker. No new HTTP endpoint, no
    frontend surface — every consumer of this package is backend Go
    code, per the design's non-goals.

    Built via subagent-driven-development: four tasks (Store, Ticker,
    `/reminders` command, startup wiring), each independently
    implemented and reviewed clean. The plan's own sample code for
    `humanRelative`'s day calculation (`int()` truncation) turned out
    to fail the plan's own "overdue" test case (hand-verified: `int()`
    truncates `-0.04` toward zero instead of down, misclassifying an
    hour-overdue reminder as "today"); the implementing agent caught it
    and switched to `math.Floor`, confirmed correct against all four
    test cases by the task reviewer — a plan defect, not an
    implementation one. The final whole-branch review (run at max
    capability) found one real gap: `Ticker.tick` had no per-call
    timeout, the one place it didn't actually mirror `HealthTicker`'s
    pattern despite claiming to — fixed and re-reviewed clean in one
    round.

    While closing out the branch, also fixed a pre-existing, unrelated
    test-isolation bug surfaced by the full-suite run:
    `TestHandler_Me_Features_Admin` in `internal/auth` hardcoded user
    ID 1, which only held against a freshly-seeded `users` table — noted
    as a known pre-existing failure as far back as step 19, now fixed
    to use the ID `CreateUser` actually returns.

    Verified: full backend suite (`go test -p 1 ./...` against a real
    Postgres) green, including the `internal/auth` fix above; `gofmt`/
    `go vet` clean; `gitleaks detect`, frontend `npm audit`, backend
    `govulncheck` all clean. No new dependencies, no new env vars, no
    schema/CLI drift beyond the new `reminders` table (via the same
    idempotent `EnsureSchema` pattern every other store uses). Same
    accepted gap as the original Telegram bot v1 plan (step 14): the
    manual end-to-end smoke test against a real Telegram bot (send
    `/reminders`, confirm the hourly ticker actually delivers) needs a
    human with bot access and has not been run as of this entry.
