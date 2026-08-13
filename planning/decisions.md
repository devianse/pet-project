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

## Still open

- **WebSockets** (phase 2, second half) — not started. Real-time is
  meant to be a cross-cutting platform feature (see `PLANNING.md`'s
  "Platform shell" section), needed before Shopping List's live-sync
  core loop or Date Night's deferred live-update follow-up can land.
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
- **Admin web UI for managing grants** (deferred by the same design) —
  CLI-only (`cmd/grantaccess`) for now. In progress as of 2026-08-14;
  scoped to feature grants only — role promote/demote (see below) is a
  separate, sequential follow-up task, not bundled into this one, though
  it's expected to land on the same admin page once designed.
- **Changing an existing user's role** — `cmd/createuser` only creates
  new accounts; there's no way yet to promote/demote an existing one
  (e.g. `user` → `admin`) short of editing the DB by hand. Noted while
  seeding the first production admin account (2026-08-10 redeploy).
  Originally filed as "not urgent, `role` isn't enforced anywhere yet" —
  that's no longer true (`role` now gates the admin bypass in
  `internal/access`, see the resolved permissions item above), so this
  is worth revisiting sooner than the original note implied. Sequenced
  as a follow-up to the admin grants UI above (separate task/PR, not
  bundled), expected to land on the same admin page once designed.
