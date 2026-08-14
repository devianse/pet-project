# Pet Projects — Planning Notes

Working notes from the initial scoping conversation (2026-08-07/08), kept updated as decisions land. Not a SPEC — repo structure and the VPS provider are now resolved (see `planning/decisions.md`), but the full phased implementation plan still isn't written; this is the decision record so nothing discussed gets lost before that happens. See `../go-learning/knowledge-checks/LEARNING_PHILOSOPHY.md` and `../go-learning/knowledge-checks/GO_ROADMAP.md` for the learning context this grew out of.

This file holds the framing and the architecture/roadmap. Related notes
live in `planning/`, split out for size:

| File | What's there |
|---|---|
| [`planning/history.md`](./planning/history.md) | Chronological build log — what's actually shipped, in order |
| [`planning/decisions.md`](./planning/decisions.md) | Open questions: resolved decisions and what's still undecided |
| [`planning/security-todo.md`](./planning/security-todo.md) | Security follow-ups deferred out of the pre-deployment hardening pass |

## Framing

Task Tracker CLI + URL Shortener (both under `go-learning/`) were **phase 1**: get to a level of comfort actually reading and reasoning about Go. This is **phase 2**, and it's a different kind of thing — not another isolated learning exercise, but an actual small **portfolio platform**: one shell (auth, deployment, a React SPA) that different self-contained "projects" plug into as pages/routes over time. That's why this lives in its own `pet-projects/` folder, not under `go-learning/` — it's no longer purely a learning repo, even though it'll keep landing new Go concepts as a side effect.

**Build order: the shell comes first.** Auth, routing, deployment, the empty SPA shell with nothing but a nav and a "hello" page — get that real and deployed before any domain-specific project gets built on top of it. A domain idea (see below) is the *first thing plugged into* the shell, not the starting point.

**In practice this order has taken detours** — see
[`planning/history.md`](./planning/history.md) for the full chronological
record. The shell-first *intent* still holds (WebSockets, phase 2's
second half, is still not started); what's changed is that a couple of small,
non-critical features have landed on the barebones phase-1 scaffold
ahead of full auth, the same way Notes did. Each detour is its own small
design doc under `docs/superpowers/specs/`, linked from the history file
rather than duplicated here.

## Platform shell — standing constraints and what's left

Backend (Go, `cmd/`/`internal/...`), frontend (React SPA via Vite), auth
(JWT), and the reverse proxy/deployment shape (Caddy + Go API + Postgres,
3 containers via one `docker-compose.yml`) are all built — see
`planning/history.md` for how each landed and `README.md` for the
current layout, not repeated here since it's just describing what
already exists in the repo. One gotcha worth keeping since it's not
visible from the code either (it's about a directory that must *not*
exist): nothing under `frontend/public/api/` — the `/api/*` proxy rule
in `infra/Caddyfile` is checked before the static-file fallback, so
anything placed there would be permanently unreachable as a static
asset.

Still open:

- **Real-time (WebSockets)** — meant to be a cross-cutting platform
  feature (available to any project plugged in later), not a dedicated
  page. Not started — see `planning/decisions.md`.
- **Telegram bot integration** — same "cross-cutting capability, not a
  page" relationship as WebSockets. Design approved and spec'd, not
  built yet — see `planning/decisions.md`.

Standing decisions (not "todo," deliberate choices not to build
something — worth keeping since the absence isn't visible from the code
either way):

- **Storage**: no S3/object storage — a local Docker volume is enough at
  this scale; revisit only if a later project's file storage needs
  actually grow past that.
- **No Kubernetes / load balancer** — one VPS, one instance of the stack
  is the whole architecture. LB/orchestration solves a scaling problem
  this doesn't have.

## Deployment target — later decision, not the first step

Provisioning an actual VPS isn't step one — the shell gets built and run locally first. Capturing the comparison here so it's not lost by the time it's actually relevant:

Russia-based payment friction with most Western PaaS (Fly.io/Railway/Render all require a foreign-issued card) is why this is "pick a VPS + Docker Compose," not "push to a PaaS."

| Provider | Relevant tier | Specs | Price | Notes |
|---|---|---|---|---|
| Timeweb Cloud | Entry | 2 vCPU @ 3.3GHz, 2GB RAM, 40GB NVMe, 1Gbps | 800₽/mo (~$8-9) | Russian provider, domestic card/SBP payment, Moscow/SPb regions — low latency, simplest payment path from Russia |
| Cloudzy | Advanced | 1 vCPU @ 4.2GHz, 2GB RAM, 60GB NVMe, 3TB transfer | $7.48/mo (50%-off intro rate; $14.95 regular) | Accepts crypto (BTC/ETH/USDT/LTC) + card + PayPal — sidesteps the card-restriction problem differently than Timeweb; 13 global regions, latency from Russia unverified; intro pricing won't last at renewal |

Both are viable "barebones" options at this scale (2GB RAM comfortably runs all 3 containers). **Originally decided: Cloudzy** — crypto/card/PayPal payment path preferred over Timeweb's domestic-card route; table above kept as the comparison record. A domain name (needed for Caddy's automatic HTTPS) is a separate purchase either way, from neither of these.

**Revised during actual provisioning (2026-08-09)**: crypto top-up into Cloudzy carries a real cost the original comparison missed — ~12% conversion + a flat ~$3.5 fee per payment — which erodes Cloudzy's price edge once that's counted. Timeweb also turned out to offer a Netherlands region (not just Moscow/SPb), which gets the same low-latency-from-Russia benefit as Cloudzy's Frankfurt option while keeping Timeweb's fee-free domestic card/SBP payment — collapsing the tradeoff the original table was built around.

**Current plan**: the first deployment (see `planning/history.md` step 6) runs on **Cloudzy, Frankfurt, hourly billing**, deliberately treated as a ~3-week paid trial funded by an already-deposited $10 balance — not the permanent home. Once that runs out, **move to Timeweb (Netherlands)** for the ongoing recurring hosting, using `infra/deployment-runbook/` as the checklist (same Docker Compose stack, so it's a redo of the OS-level steps — hardening, Docker install — plus a deliberate Postgres `pg_dump`/restore and a DNS cutover, not a from-scratch rebuild).

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
