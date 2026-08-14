# Open questions / not yet decided

Decision log split out of `PLANNING.md` for size — see that file for
the framing and roadmap these decisions feed into.

## Resolved

Full resolution narrative lives in `planning/history.md` (chronological,
one numbered entry per shipped feature) — these are pointers, not a
second copy.

- ~~Exact repo/folder structure inside `pet-projects/`~~ — see
  `README.md`'s "Layout" section.
- ~~VPS provider~~ — see `PLANNING.md`'s "Deployment target" section.
- ~~Movie Sharing's metadata source~~ — TMDb, see
  `docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md`.
- ~~`jwt` branch merge~~ — merged to `main` via PR #4.
- ~~Fine-grained per-user permissions~~ — see `history.md` step 9. Two
  follow-ups deferred; one resolved (same step), the other still open
  below (admin web UI for grants → resolved next).
- ~~Browser-level verification of the JWT auth frontend flow~~ —
  accepted as sufficient by usage (2026-08-14), not a dedicated pass.
- ~~Admin web UI for managing grants~~ — see `history.md` step 10.
- ~~Changing an existing user's role~~ — see `history.md` step 11.
- ~~Admin panel expansion, user lifecycle management~~ — see
  `history.md` step 12.
- ~~Admin panel expansion, audit trail + ops/system panel~~ — see
  `history.md` step 13. Two follow-ups deferred, see "Still open" below.
- ~~Telegram bot dev+prod deployment (two-bot setup, `/help` command +
  native command menu)~~ — see `history.md` step 16.
- ~~Redeploy's `GIT_SHA` export is manual~~ — replaced with a `make
  redeploy` target (root `Makefile`) that computes `GIT_SHA` inline as
  part of the same command that runs `docker compose up -d --build`,
  so it can't linger stale from an `export` left over in an earlier
  shell session. `infra/deployment-runbook/03-redeploy.md` updated to
  match (2026-08-15).

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
  Design in progress (2026-08-15), scoped to the shell only (connection
  lifecycle, auth handshake, message protocol, hub/broadcast) — Ops
  panel is the first consumer but is being split into its own follow-up
  task rather than co-designed with the shell.
- **Auth rewrite to an OAuth library** — raised while brainstorming
  WebSockets auth (2026-08-15) as a "more secure" alternative to the
  current hand-rolled JWT/httpOnly-cookie auth (`backend/internal/auth`).
  Not decided or scoped — assessed as its own architectural effort (own
  library choice, own questions about existing sessions/roles), not a
  prerequisite for WebSockets. WS auth is being designed against
  today's cookie behind a swappable interface so this can land later
  without a WS redesign. Needs its own brainstorm when picked up.
- **Telegram bot integration** — v1 (commands-in only: `/notes`,
  `/newnote`, `/help`) deployed to both dev and prod, two separate
  `@BotFather` bots per the routing constraint below; see
  `planning/history.md` entries 14 and 16. Design/spec:
  `docs/superpowers/specs/2026-08-14-telegram-bot-design.md`. Another
  cross-cutting platform capability like WebSockets (see `PLANNING.md`'s
  "Platform shell" section) — two-way (commands in, notifications out)
  in the full design, single-owner for v1, hand-rolled (no new
  dependency), long-polling not webhook. The notifications-out half
  (Date Night's already-deferred "notify on propose/accept/decline"
  gap; a later financial-subscription tracker's renewal-due
  notifications) is still not started — v1 shipped commands-in only.

  **Two bots, one chat ID** (decided 2026-08-14): dev and prod need
  **two separate bots** (two `@BotFather`-issued tokens), not one bot
  shared across both. `Poller`'s `GetUpdates` long-poll only allows one
  active consumer per bot token — two envs polling the same token race
  and get `409 Conflict`ed against each other, and there's no way to
  route "this update is for dev vs. prod" anyway (`chat_id` identifies
  the Telegram *account* messaging the bot — same value across all
  devices/clients of that account and across every bot it DMs — not
  which backend should handle it). So: `TELEGRAM_CHAT_ID` stays
  identical in `backend/.env` and `infra/.env` (still your own account
  either way); `TELEGRAM_BOT_TOKEN` differs (one bot for dev, one for
  prod).
- **OpenTelemetry** — not started, no design yet. Requested want:
  observability (traces/metrics/logs) across the backend, presumably the
  reverse proxy too. Open questions to resolve before this is buildable:
  what backend/collector it ships to (self-hosted on the VPS vs. a
  hosted vendor — cost/complexity tradeoff given the "one small VPS, no
  k8s" constraint elsewhere in `PLANNING.md`), whether it replaces or
  sits alongside the existing `slog`-based logging, and where it lands
  relative to WebSockets/Telegram in priority.
- **Redis (cache) or RabbitMQ (queue)** — not started, no design yet,
  no concrete driving need identified yet either. Two distinct options,
  not a package: Redis as a cache layer (session/query caching) vs.
  RabbitMQ as a message queue (candidate fit: Image Processing's worker
  pool, per `PLANNING.md`'s "Second project to plug in" section, though
  that section currently assumes an in-process queue, not a broker).
  Either adds a 4th container to the one-VPS Compose stack — worth
  weighing against that "no k8s / keep it simple" constraint before
  committing to either.
- **`notes`/feature multi-tenancy** — `notes` (and most other feature
  tables) currently have no owner column: single flat table, one
  implicit shared owner. Once a feature gets a real `user_id` column,
  its tests should adopt the scoping convention `internal/access`/
  `internal/auth` already use (distinct fixture usernames per package,
  `DELETE` scoped to that owner's rows only) instead of an unscoped
  `DELETE FROM <table>` at setup. That scoping is also the permanent
  fix for a concurrency gap found while closing out the `telegram`
  branch: `internal/notes` and `cmd/api`'s telegram tests both run
  unscoped `DELETE FROM notes` against the same shared table, and
  `go test ./...`'s default parallel-package execution lets one
  package's setup race another's mid-test (reproduced 1-in-3 runs:
  content collisions and a row deleted out from under a test between
  its own insert and delete). Current stopgap: `make test-backend`
  runs `go test -p 1 ./...`, serializing package execution so the race
  can't happen — intentionally temporary, not meant to be built out
  further (no advisory locks, no bespoke isolation abstraction).
  Once `notes` is user-scoped, the race disappears as a side effect of
  the ownership-scoped test convention above, and `-p 1` can go away.
- **`cmd/api/main.go` split** — raised while writing the WebSockets shell
  plan (2026-08-15): `main()` does everything in one ~200-line pass —
  reads env vars, constructs every store/handler, registers every route
  on `mux`, starts the server — with no `buildMux(...)`/`newApp(...)`
  function that returns the wired router as a value. Noticed because it
  meant Task 7 of that plan couldn't hit `/api/ws` through a real router
  in a test without a refactor bigger than that task needed, so it
  didn't add one (relied on `internal/realtime`'s own handler tests
  instead — see `docs/superpowers/plans/2026-08-15-websockets-shell.md`
  Task 7). Not a current bug — every route's behavior is already tested
  at the package level (`internal/auth`, `internal/access`, etc.), and
  `main.go` wiring is trusted by inspection like the rest of that file.
  Worth a real look once it grows past this point (each new feature adds
  ~10-15 more lines of route registration) or the next time something
  wants to test routing/wiring directly — no design yet.
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
