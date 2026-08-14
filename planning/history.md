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
10. **Admin grants UI** (done, on the `grants` branch, not yet merged) —
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
11. **Role promote/demote** (done, on the `access` branch, not yet
    merged) — closes the "changing an existing user's role" follow-up
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
