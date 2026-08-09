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
4. **Pre-deployment security hardening** (in progress) — Caddy/Postgres
   infra config, backend timeout/body-size hardening, basic-auth stopgap
   ahead of real JWT auth, manual dependency/secret scans. Needed now
   because step 6 below deploys without phase-2 auth existing yet. See
   `docs/superpowers/specs/2026-08-09-pre-deployment-security-design.md`.
5. **Movie/TV Sharing List** (in progress) — another pre-phase-3 detour,
   same pattern as Notes: a shareable watchlist, paste an IMDb link, get
   a preview card (title, description, poster image) resolved via TMDb,
   mark items as viewed/expand for detail. See
   `docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md`
   and `docs/superpowers/plans/2026-08-09-movie-tv-sharing-list.md`.
6. **First VPS deployment attempt** — see "Deployment target" below.
   Happens once step 5 lands, still without phase-2 auth (the
   security-hardening pass in step 4 is what makes that acceptable).
7. **Phase 2** (auth, WebSockets) and the rest of phase 3 (Family
   Shopping List, then Image Processing) resume after that, order
   unchanged from the sections below.

## Platform shell — what gets built first

- **Backend**: one Go module, real package structure (`cmd/`, `internal/...`) — unlike Task Tracker/URL Shortener's deliberate flat layout, the surface area here justifies it. Revisit `golang-standards/project-layout` for real.
- **Frontend**: React SPA (Vite), client-side routing via `react-router`. User already knows React — this is "wire it to something real," not a React-learning exercise.
- **Auth**: JWT, not sessions — user is comfortable with JWT from frontend work already, so the new ground is entirely the Go-side signing/verification/expiry logic. (Not currently a `GO_ROADMAP.md` concept — should be added there.)
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

Both are viable "barebones" options at this scale (2GB RAM comfortably runs all 3 containers). **Decided: Cloudzy** — crypto/card/PayPal payment path preferred over Timeweb's domestic-card route; table above kept as the comparison record. A domain name (needed for Caddy's automatic HTTPS) is a separate purchase either way, from neither of these.

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

## Security TODO (deferred out of the pre-deployment hardening pass)

Recorded during the pre-deployment security brainstorm
(`docs/superpowers/specs/2026-08-09-pre-deployment-security-design.md`)
as genuinely dependent on later phases, not skipped by oversight:

- **Real JWT auth** (phase 2) replacing the Caddy basic-auth stopgap
- **CI/CD pipeline** automating what's manual for now: `npm audit`,
  `govulncheck ./...`, `gitleaks detect` on every push/PR, plus
  Dependabot/Renovate for ongoing dependency updates
- **Rate limiting** beyond the request body-size cap, once real traffic
  patterns exist
- **CORS policy** — not needed yet (Caddy makes frontend/API
  same-origin), revisit if that changes
- **CSP tightening** as the frontend grows past its current shape
- **WebSocket auth** (phase 2 feature, doesn't exist yet)
- **Postgres backup strategy** / secrets rotation
