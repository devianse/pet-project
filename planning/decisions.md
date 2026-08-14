# Open questions / not yet decided

Decision log split out of `PLANNING.md` for size — see that file for
the framing and roadmap these decisions feed into.

## Resolved

- ~~Exact repo/folder structure inside `pet-projects/`~~ — resolved by
  what actually exists: `backend/`, `frontend/`, `docs/`, `infra/`
  (added in `planning/history.md` step 4) at the root, each app
  managing its own deps. See `README.md`'s "Layout" section for the
  tree; not revisited here since it's just describing reality at this
  point, not a pending decision.
- ~~VPS provider~~ — resolved, Cloudzy (see `PLANNING.md`'s "Deployment
  target" section).
- ~~Movie Sharing's metadata source~~ — resolved, TMDb (see
  `docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md`).
  Link parsing is IMDb-URL-only; TMDb's `/find` resolves the id. TMDb's
  Russia/Belarus IP block doesn't reach a Cloudzy-hosted backend, since
  the block is IP-based and Cloudzy has no Russia region.
- ~~`jwt` branch merge~~ — merged to `main` via PR #4. (This bullet
  previously said "not merged yet, by deliberate choice" — stale; the
  merge happened shortly after that note was written.)
- ~~Fine-grained per-user permissions~~ — designed **and implemented**:
  see `docs/superpowers/specs/2026-08-13-feature-visibility-design.md`,
  landed via the `visibility` branch (PR #9). A `features`/
  `feature_access` table pair plus a `RequireFeature` backend middleware
  and frontend route guard, granted per user via a new `cmd/grantaccess`
  CLI (same pattern as `cmd/createuser`); `admin` bypasses automatically
  (and, as of 2026-08-13, that bypass re-checks the DB rather than
  trusting a possibly-stale JWT claim — closes the window where a
  demoted admin kept bypass access until their token expired). Also
  retired Date Night's bespoke `DATE_NIGHT_USERNAMES` env var onto the
  same mechanism. That design explicitly deferred two follow-ups; one
  (`display_name` write path) is now resolved — see `planning/
  history.md` step 9 — the other (admin web UI for grants) is still
  open below.
- ~~Browser-level verification of the JWT auth frontend flow~~ —
  resolved by usage rather than a dedicated pass: never formally
  click-through-tested or driven via `claude-in-chrome`, but roughly
  five features have shipped on top of the auth flow since the `jwt`
  branch landed, each implicitly exercising login/logout/redirect in
  normal dev use with no breakage surfacing. Accepted as sufficient
  (2026-08-14).
- ~~Admin web UI for managing grants~~ — built on the `grants` branch:
  a DB-re-checked `access.RequireRole` backend middleware,
  `access.AdminHandler` (list users + grant/revoke per feature,
  idempotent), and a `/admin` page (nav-gated to admins, a grant matrix
  of users × features with per-cell toggle buttons). See
  `docs/superpowers/specs/2026-08-14-admin-grants-ui-design.md` and
  `docs/superpowers/plans/2026-08-14-admin-grants-ui.md`. Built via
  `superpowers:subagent-driven-development`; one accepted gap, same
  shape as the `jwt` branch's — the rendered frontend was verified via
  a curl-driven server-side E2E pass plus static task review, not an
  actual browser click-through (no browser automation tool available
  in that session). Merged to `main` via PR #12; the click-through gap
  above is still open (see the browser-verification bullet above).
- ~~Changing an existing user's role~~ — built on the `access` branch,
  landing on the same `/admin` page's Access section as the grants UI
  above: `auth.Store.UpdateRole`, a `PUT /api/admin/users/{id}/role`
  handler, and a `user`/`admin` `<select>` per row (dropdown chosen over
  a toggle since the role list may grow, unlike the fixed
  Grant/Granted boolean). Self-demote is blocked both client-side
  (control disabled on the caller's own row) and server-side (403,
  re-checking the caller's id off JWT claims) — same defense-in-depth
  reasoning as `RequireRole`'s DB re-check. See `planning/history.md`
  step 11. Merged to `main` via PR #13.
- ~~Admin panel expansion, user lifecycle management~~ — built on the
  `user-lifecycle` branch: grouped, as planned, into one feature —
  user creation, deactivate/reactivate, and admin-triggered password
  reset — extending the same `/admin` Access section as the grants and
  role UI. Deactivation is soft (`users.is_active`, blocks login,
  fully reversible) — hard delete was explicitly ruled out as bad
  practice for DB management. Password reset is "admin sets a temp
  password, shown once in the UI" — no email infra added; self-service
  email reset stays a deferred idea below. Backend: `auth.Store.
  SetActive`/`SetPasswordHash`, `Handler.Login` now rejects a
  deactivated user through the same generic 401 as a bad password (no
  enumeration signal), and `access.AdminHandler` gained `CreateUser`
  (mirrors `cmd/createuser`'s validation/hashing, 409 on duplicate
  username), `SetActive` (self-deactivate blocked server-side, same
  guard shape as self-demote), `ResetPassword` (never stores/logs the
  plaintext). Frontend: a "Create user" form above the matrix, an
  Active/Deactivated toggle per row, and a per-row inline
  type-or-Generate password reset form with a one-time reveal banner.
  TDD throughout on the backend (store + handler tests, each watched
  fail first); frontend followed the existing untested convention.
  Bounded-path work (brainstormed, short in-chat design, no spec doc) —
  see `planning/history.md` step 12. Merged to `main` via PR #15; same
  accepted gap as the two admin-panel branches before it — no browser
  click-through, verified via backend tests and static review.
- ~~Admin panel expansion, audit trail + ops/system panel~~ — built on
  the `audit-ops` branch: a read-only introspection page added to
  `/admin`, admin-action audit log (grant/revoke/create/deactivate/
  reactivate/reset-password/role-change, who + when) plus basic system
  health (DB ping, deployed git SHA via `GIT_SHA`). New
  `admin_audit_log` table, `access.Store.LogAction`/`ListAuditLog`,
  `GET /api/admin/audit-log`; every mutating admin handler logs via a
  shared helper post-write. `/api/health` extended to ping the DB and
  report version. See `planning/history.md` step 13. Bounded-path work
  (brainstormed, short in-chat design, no spec doc). Not yet merged to
  `main` (branch: `audit-ops`). Two follow-ups deferred out of this
  pass, see below: audit log is capped to a 400px scrollable block
  client-side (the backend already caps `ListAuditLog` at 100 rows —
  no pagination control either way, just a visible/scrollable window
  vs. a hard invisible cutoff), and the panel doesn't live-update.

## Still open

- **Admin panel expansion, deferred: app data moderation** — the
  remaining option from the same brainstorm as the audit-trail/ops
  panel above (now resolved): read visibility into Notes/Watchlist/
  Date Night rows without SSHing in to query Postgres directly. Spans
  three unrelated domain schemas and is realistically its own
  multi-part effort, not a bolt-on.

- **Ops panel live-update** — deferred out of the audit trail + ops
  panel feature above. Today the health card and audit log only fetch
  once on page mount, same as the rest of `/admin` — an action taken in
  one tab (or by another admin) doesn't appear until the page is
  reloaded. Long-polling was raised as a lighter-weight stopgap; the
  real fix is likely the same WebSockets work already deferred below
  for Date Night/Shopping List, since it'd be one more consumer of the
  same cross-cutting real-time mechanism rather than a bespoke polling
  loop just for this page. Worth deciding at WebSockets design time
  rather than building a one-off now.

- **Self-service email password reset** — deferred out of the user
  lifecycle management feature above, where the admin-triggered reset
  is landing as "admin sets a temp password, shown once" instead (no
  email infra needed for that). This is the bigger version of the same
  problem: a user-initiated "forgot password" flow, which needs email
  sending added to the app for the first time. Noted as a real platform
  feature for later, with the concrete use case already in view.

  (Ruled out entirely: a feature-registry UI — `KnownFeatures` in
  `backend/internal/access/features.go` is a hardcoded Go slice tied to
  actual gated routes, not DB-editable data, so a UI to "manage" it
  would create dead entries with nothing to gate. The existing grants
  matrix already is the read view of known features × who has access.)
- **WebSockets** (phase 2, second half) — not started. Real-time is
  meant to be a cross-cutting platform feature (see `PLANNING.md`'s
  "Platform shell" section), needed before Shopping List's live-sync
  core loop or Date Night's deferred live-update follow-up can land.
- **Telegram bot integration** — not started, design approved and
  spec'd: `docs/superpowers/specs/2026-08-14-telegram-bot-design.md`.
  Another cross-cutting platform capability like WebSockets (see
  `PLANNING.md`'s "Platform shell" section) — two-way (commands in,
  notifications out), single-owner for v1, hand-rolled (no new
  dependency), long-polling not webhook. v1 closes Date Night's
  already-deferred "notify on propose/accept/decline" gap and adds
  `/notes` + `/newnote` commands. Motivated by a later goal — a
  financial-subscription tracker (renewal-due notifications, spend
  totals) — that's explicitly out of scope for this spec and gets its
  own design once this lands.
- **Which of Shopping List / Image Processing** gets built first once
  phase 3 resumes, or whether they're built in parallel.
- **OCR approach** (local Tesseract vs cloud API), if/when Image
  Processing gets built — not needed until that project is actually
  underway.
- **Full phased implementation plan** (SPEC-style, layer by layer like
  URL Shortener) for the shell first, then the shopping-list project —
  not written yet; next step should go through
  `superpowers:brainstorming` properly before that gets written, per
  standing skill-usage rules.
