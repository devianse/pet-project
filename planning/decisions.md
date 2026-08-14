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
- **Telegram bot integration** — v1 (commands-in only: `/notes`,
  `/newnote`) built on the `telegram` branch, not yet merged; see
  `planning/history.md` entry 14. Design/spec:
  `docs/superpowers/specs/2026-08-14-telegram-bot-design.md`. Another
  cross-cutting platform capability like WebSockets (see `PLANNING.md`'s
  "Platform shell" section) — two-way (commands in, notifications out)
  in the full design, single-owner for v1, hand-rolled (no new
  dependency), long-polling not webhook. The notifications-out half
  (Date Night's already-deferred "notify on propose/accept/decline"
  gap; a later financial-subscription tracker's renewal-due
  notifications) is still not started — v1 shipped commands-in only.
- **OpenTelemetry** — not started, no design yet. Requested want:
  observability (traces/metrics/logs) across the backend, presumably the
  reverse proxy too. Open questions to resolve before this is buildable:
  what backend/collector it ships to (self-hosted on the VPS vs. a
  hosted vendor — cost/complexity tradeoff given the "one small VPS, no
  k8s" constraint elsewhere in `PLANNING.md`), whether it replaces or
  sits alongside the existing `slog`-based logging, and where it lands
  relative to WebSockets/Telegram in priority.
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
