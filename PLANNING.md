# Pet Projects — Planning Notes

Working notes from the initial scoping conversation (2026-08-07/08), kept updated as decisions land. Not a SPEC — repo structure and the VPS provider are now resolved (see "Open questions" below), but the full phased implementation plan still isn't written; this is the decision record so nothing discussed gets lost before that happens. See `../go-learning/knowledge-checks/LEARNING_PHILOSOPHY.md` and `../go-learning/knowledge-checks/GO_ROADMAP.md` for the learning context this grew out of.

## Framing

Task Tracker CLI + URL Shortener (both under `go-learning/`) were **phase 1**: get to a level of comfort actually reading and reasoning about Go. This is **phase 2**, and it's a different kind of thing — not another isolated learning exercise, but an actual small **portfolio platform**: one shell (auth, deployment, a React SPA) that different self-contained "projects" plug into as pages/routes over time. That's why this lives in its own `pet-projects/` folder, not under `go-learning/` — it's no longer purely a learning repo, even though it'll keep landing new Go concepts as a side effect.

**Build order: the shell comes first.** Auth, routing, deployment, the empty SPA shell with nothing but a nav and a "hello" page — get that real and deployed before any domain-specific project gets built on top of it. A domain idea (see below) is the *first thing plugged into* the shell, not the starting point.

**In practice this order has taken detours** — see "Actual build order so
far" below. The shell-first *intent* still holds (auth/WebSockets are
still phase 2, still not started); what's changed is that a couple of
small, non-critical features have landed on the barebones phase-1
scaffold ahead of full auth, the same way Notes did. Each detour is its
own small design doc under `docs/superpowers/specs/`, linked below rather
than duplicated here.

## Actual build order so far (deviations from the plan)

Chronological record of what's actually shipped, since it no longer
matches the shell-first order above one-for-one:

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
   see "Deployment target" below for the planned move to Timeweb once
   that runs out). Server hardened first (non-root sudo user, root SSH
   disabled, password auth disabled, SSH moved off port 22, `ufw`
   deny-by-default with only SSH/80/443 open), then Docker + Compose
   installed, repo cloned, production `infra/.env` filled in (fresh
   Postgres password, real TMDb token, Caddy basic-auth hash), DNS A
   record pointed at the VPS, `docker compose up -d --build`. Caddy
   obtained a real Let's Encrypt cert automatically, all 3 containers
   healthy on first boot. Full step-by-step in
   `infra/DEPLOYMENT_RUNBOOK.local.md` — written to make the Timeweb move a
   checklist, not a from-scratch redo. One real bug caught and fixed
   during this pass: `infra/docker-compose.yml` wasn't passing
   `TMDB_READ_ACCESS_TOKEN` into the backend container at all (backend
   exits on startup without it) — added alongside this deployment.
7. **Real JWT auth** (phase 2, first half — done, on the `jwt` branch,
   not yet merged to `main` as of this writing). Replaces the Caddy
   basic-auth stopgap from step 4: a `users` table (invite-only, no
   public registration — accounts are seeded via a new `cmd/createuser`
   CLI), bcrypt password hashes, a signed JWT held in an httpOnly
   `SameSite=Strict` session cookie, `admin`/`user` roles (fine-grained
   per-user permissions deliberately deferred — see "Open questions"
   below), and the whole SPA gated behind login. Full design/build
   record: `docs/superpowers/specs/2026-08-10-jwt-auth-design.md` and
   `docs/superpowers/plans/2026-08-10-jwt-auth.md`. Built via
   `superpowers:subagent-driven-development` — 14 tasks, a final
   whole-branch review caught one real Critical (the deploy runbook's
   admin-seeding command didn't actually work against the built Docker
   image — fixed) plus a mobile-nav logout gap, both fixed before this
   was considered done. One accepted gap: the frontend auth flow
   (login/logout/redirect) was verified via backend curl calls + code
   review, not an actual rendered browser session — no browser
   automation tool was available in that session.
8. **WebSockets** (phase 2, second half — still not started) and the
   rest of phase 3 (Family Shopping List, then Image Processing) resume
   after that, order unchanged from the sections below.

## Platform shell — what gets built first

- **Backend**: one Go module, real package structure (`cmd/`, `internal/...`) — unlike Task Tracker/URL Shortener's deliberate flat layout, the surface area here justifies it. Revisit `golang-standards/project-layout` for real.
- **Frontend**: React SPA (Vite), client-side routing via `react-router`. User already knows React — this is "wire it to something real," not a React-learning exercise.
- **Auth**: JWT, not sessions — user is comfortable with JWT from frontend work already, so the new ground is entirely the Go-side signing/verification/expiry logic. (Not currently a `GO_ROADMAP.md` concept — should be added there.) **Done** (see build-order step 7 above) — landed as invite-only accounts (no self-registration), a single long-lived token (no refresh flow) in an httpOnly cookie, not `localStorage`.
- **Real-time**: WebSockets as a cross-cutting platform feature (available to any project plugged in later), not a dedicated page. (Also not currently on `GO_ROADMAP.md` — should be added.)
- **Reverse proxy**: Caddy, not nginx — automatic Let's Encrypt TLS with a ~3-line Caddyfile, no separate certbot container/cron needed. Caddy also serves the built React static files directly and reverse-proxies `/api/*` to the Go backend — no separate frontend container.
  - Known gotcha to avoid: nothing under `public/api/` in the frontend — the `/api/*` proxy rule is checked before the static-file fallback, so anything there would be permanently unreachable as a static asset.
  - SPA routing needs `try_files {path} /index.html` in the Caddyfile so client-side routes survive a hard refresh.
- **Database**: self-hosted Postgres as a container alongside the app, not a managed DB service — managed Postgres roughly doubles monthly cost for backup/replica features not needed at this scale. Backups via `pg_dump` cron or the VPS provider's snapshot backups instead.
- **Storage**: no S3/object storage for now — a local Docker volume is enough at this scale; revisit only if a later project's file storage needs actually grow past that.
- **No Kubernetes / load balancer** — one VPS, one instance of the stack is the whole architecture. LB/orchestration solves a scaling problem this doesn't have.

Deployment shape once it's time: **Caddy (TLS + static frontend + reverse proxy) + Go API + Postgres** — 3 containers via one `docker-compose.yml`.

## Deployment target — later decision, not the first step

Provisioning an actual VPS isn't step one — the shell gets built and run locally first. Capturing the comparison here so it's not lost by the time it's actually relevant:

Russia-based payment friction with most Western PaaS (Fly.io/Railway/Render all require a foreign-issued card) is why this is "pick a VPS + Docker Compose," not "push to a PaaS."

| Provider | Relevant tier | Specs | Price | Notes |
|---|---|---|---|---|
| Timeweb Cloud | Entry | 2 vCPU @ 3.3GHz, 2GB RAM, 40GB NVMe, 1Gbps | 800₽/mo (~$8-9) | Russian provider, domestic card/SBP payment, Moscow/SPb regions — low latency, simplest payment path from Russia |
| Cloudzy | Advanced | 1 vCPU @ 4.2GHz, 2GB RAM, 60GB NVMe, 3TB transfer | $7.48/mo (50%-off intro rate; $14.95 regular) | Accepts crypto (BTC/ETH/USDT/LTC) + card + PayPal — sidesteps the card-restriction problem differently than Timeweb; 13 global regions, latency from Russia unverified; intro pricing won't last at renewal |

Both are viable "barebones" options at this scale (2GB RAM comfortably runs all 3 containers). **Originally decided: Cloudzy** — crypto/card/PayPal payment path preferred over Timeweb's domestic-card route; table above kept as the comparison record. A domain name (needed for Caddy's automatic HTTPS) is a separate purchase either way, from neither of these.

**Revised during actual provisioning (2026-08-09)**: crypto top-up into Cloudzy carries a real cost the original comparison missed — ~12% conversion + a flat ~$3.5 fee per payment — which erodes Cloudzy's price edge once that's counted. Timeweb also turned out to offer a Netherlands region (not just Moscow/SPb), which gets the same low-latency-from-Russia benefit as Cloudzy's Frankfurt option while keeping Timeweb's fee-free domestic card/SBP payment — collapsing the tradeoff the original table was built around.

**Current plan**: the first deployment (see build-order step 6 above) runs on **Cloudzy, Frankfurt, hourly billing**, deliberately treated as a ~3-week paid trial funded by an already-deposited $10 balance — not the permanent home. Once that runs out, **move to Timeweb (Netherlands)** for the ongoing recurring hosting, using `infra/DEPLOYMENT_RUNBOOK.local.md` as the checklist (same Docker Compose stack, so it's a redo of the OS-level steps — hardening, Docker install — plus a deliberate Postgres `pg_dump`/restore and a DNS cutover, not a from-scratch rebuild).

Domain: **`mikelab.dev`**, bought on reg.ru, 1-year registration (chose 1-year over 2-year prepay since renewal is low-friction and 2-year prepay only insures against a price hike that isn't guaranteed).

## First project to plug in: Family Shopping List — comes later, after the shell exists

A shared, real-time shopping list app for family use. This is the first candidate for "a project living behind the shell," not something to build in parallel with the shell itself.

- **Core loop**: add items, check off items bought, list stays in sync live across everyone's devices (using the platform's WebSocket support).
- **Meal-plan expansion**: plan meals from a recipe collection → generate a shopping list from the plan by aggregating ingredient quantities across all planned meals (e.g. 3 recipes each needing onions → one line, "6 onions"). This is the concrete payoff for relational/aggregation work — an actually-used feature, not a demo query.
- Not scoped into the MVP: OCR/photo parsing (see the separate Image Processing project below) — shopping list can *consume* that capability later once it exists, but it isn't shopping list's own feature to build.

### Concepts this lands (from `GO_ROADMAP.md`'s unassigned items)

| Concept | Where it lands |
|---|---|
| Generics | TBD utility code — revisit once real code exists to justify it, not forced in |
| `slices` package | Filtering/sorting item or ingredient collections |
| Transactions | Meal-plan → shopping-list generation as one atomic multi-row insert |
| Joins / relational schema | recipes → ingredients → meal-plan entries → generated list items |
| Aggregation queries | `SUM(quantity) GROUP BY ingredient` across a meal plan |

## Second project to plug in: Image Processing — its own domain, not a shopping-list feature

Corrected framing from earlier in planning: OCR/photo parsing isn't a shopping-list stretch goal, it's a **domain in its own right** — upload an image, a pipeline of workers processes it (OCR text extraction, maybe resizing/thumbnailing, format conversion), results come back. Shopping list happens to be one *consumer* of that capability later (cross-referencing extracted receipt text against a list), the same way it consumes the platform's auth and WebSocket support — but the image-processing pipeline itself is a standalone page/project behind the shell, matching the original "Image Processing Service" idea from `GO_ROADMAP.md`'s project-3 options.

- **Core loop**: upload an image → job goes into a queue → a fixed pool of worker goroutines pulls jobs and processes them concurrently → status/result available (polling or, once it exists, the platform's WebSocket support for live progress).
- **OCR specifically**, when this gets built — two approaches, decided when actually relevant:
  - **Run it yourself**: the Tesseract engine (free, open-source) does the text extraction locally, either via a Go wrapper library calling it directly or by shelling out to the `tesseract` command. Runs entirely on your own VPS, no per-image cost, nothing leaves the server — but out-of-the-box accuracy on messy real-world photos (crumpled receipts, handwriting) is mediocre.
  - **Send it out**: upload the photo to a third-party API (Google Cloud Vision, AWS Textract, Yandex Vision, etc.), it reads it remotely, hands back text over HTTP. Usually far more accurate, but costs money per image and means a photo leaves your server.

### Concepts this lands

| Concept | Where it lands |
|---|---|
| Worker pool pattern | N worker goroutines pulling jobs off one queue — genuinely different shape from URL Shortener's single long-lived writer goroutine |
| `slices` package | Filtering/sorting a batch of processed results |
| Generics | Possible candidate here too (a generic job-queue/result type) — same as shopping list, revisit once real code justifies it |

## Open questions / not yet decided

- ~~Exact repo/folder structure inside `pet-projects/`~~ — resolved by
  what actually exists: `backend/`, `frontend/`, `docs/`, `infra/`
  (added in step 4 above) at the root, each app managing its own deps.
  See `README.md`'s "Layout" section for the tree; not revisited here
  since it's just describing reality at this point, not a pending
  decision.
- ~~VPS provider~~ — resolved, Cloudzy (see "Deployment target" above).
- ~~Movie Sharing's metadata source~~ — resolved, TMDb (see
  `docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md`).
  Link parsing is IMDb-URL-only; TMDb's `/find` resolves the id. TMDb's
  Russia/Belarus IP block doesn't reach a Cloudzy-hosted backend, since
  the block is IP-based and Cloudzy has no Russia region.
- Which of Shopping List / Image Processing gets built first once phase
  3 resumes, or whether they're built in parallel
- OCR approach (local Tesseract vs cloud API), if/when Image Processing gets built — not needed until that project is actually underway
- Full phased implementation plan (SPEC-style, layer by layer like URL Shortener) for the shell first, then the shopping-list project — not written yet; next step should go through `superpowers:brainstorming` properly before that gets written, per standing skill-usage rules
- **Fine-grained per-user permissions**, deferred during the JWT auth
  brainstorm — the current `admin`/`user` role split is enough for "I
  want to see some things other users can't at all," but the eventual
  want is explicit feature-to-user gating (e.g. "user X can see feature
  Y, user Z can't") rather than a role bucket. Not designed yet; a
  likely later addition (a permissions table) that shouldn't require
  touching the auth core when it happens.
- **Browser-level verification of the JWT auth frontend flow**
  (login/logout/redirect actually rendering and working, not just
  passing code review + a curl-proven backend) — not done as of the
  `jwt` branch landing; no browser automation tool was available in that
  session and it was explicitly accepted as an open gap rather than
  chased down. Worth a manual `make dev-backend` + `make dev-frontend`
  click-through, or picking this up once `/chrome` is connected in a
  future session, before or shortly after this branch merges.
- **`jwt` branch merge** — implementation, task-level review, and a
  final whole-branch review (with one fix wave) are all done and clean;
  the branch itself hasn't been merged to `main` or opened as a PR yet,
  by deliberate choice ("keep as-is" for now).

## Security TODO (deferred out of the pre-deployment hardening pass)

Recorded during the pre-deployment security brainstorm
(`docs/superpowers/specs/2026-08-09-pre-deployment-security-design.md`)
as genuinely dependent on later phases, not skipped by oversight:

- ~~Real JWT auth (phase 2) replacing the Caddy basic-auth stopgap~~ —
  done, see `docs/superpowers/specs/2026-08-10-jwt-auth-design.md`
- **CI/CD pipeline** automating what's manual for now: `npm audit`,
  `govulncheck ./...`, `gitleaks detect` on every push/PR, plus
  Dependabot/Renovate for ongoing dependency updates
- ~~Rate limiting on `POST /api/auth/login`~~ — done: an in-process
  per-IP token-bucket middleware (`backend/cmd/api/ratelimit.go`, 5
  requests/min, burst 5), chosen over a Caddy-plugin or fail2ban layer
  because it needs no custom Caddy build and stays testable in the same
  suite as the rest of `internal/auth`. Trusts `X-Forwarded-For` from
  Caddy, which is safe only because the backend has no host port mapping
  in `infra/docker-compose.yml` — revisit if that ever changes. State is
  per-process/per-replica (fine for the current single-VPS, single-
  instance topology per this doc's "No Kubernetes / load balancer" note
  above; wouldn't be if that changes). Broader request-volume limiting
  beyond this one endpoint is still deferred until real traffic patterns
  exist.
- **CORS policy** — not needed yet (Caddy makes frontend/API
  same-origin), revisit if that changes
- **CSP tightening** as the frontend grows past its current shape
- **WebSocket auth** (phase 2 feature, doesn't exist yet)
- **Postgres backup strategy** / secrets rotation
