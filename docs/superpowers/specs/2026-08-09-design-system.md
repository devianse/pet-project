# Design system — design

## Status

New branch (`designSystem`), separate from `main`, to be merged back via
PR once done. Adopts a complete third-party visual identity rather than
building tokens from scratch — see "Approach" below for why. Scope is
deliberately narrow: shell + one real feature restyled, not a full
app-wide redesign in this pass.

## Approach

Considered and rejected: hand-rolled design tokens (current `index.css`
already has an ad hoc CSS-variable system — purple accent, light/dark via
`prefers-color-scheme`), and a from-scratch distinctive visual identity
(the kind `frontend-design` skill is built for). Both are more work than
warranted for a solo project, and neither was actually wanted here — the
goal stated up front was "the less I need to think about design, the
better."

Chosen instead: **[1st-Pouf](https://1st-pouf.worksonmy.dev)**, a
complete shadcn-registry-distributed component/block kit — "puffy,
pastel, maximalist" claymorphism style, six-tone palette (purple, pink,
blue, mint, yellow, orange), 38 components / 14 blocks / 14 full
templates, MIT licensed. Distributed as a shadcn registry, not an npm
package — components are copied into the repo via the shadcn CLI
(`npx shadcn@latest add <registry-url>`), so everything ends up as
editable source under `src/components/ui/`, not an opaque dependency.

`frontend-design` skill was considered and explicitly not used — its job
(inventing a distinctive, non-templated visual identity from scratch) is
already solved by adopting 1st-Pouf's complete, opinionated system. It
remains a candidate for later, narrowly, if a page is ever built that
falls genuinely outside what 1st-Pouf's components/blocks cover.

## Stack

- **Tailwind CSS v4**, installed via `@tailwindcss/vite` — CSS-driven
  config (`@import "tailwindcss"` + `@theme`), no separate JS config file
- **shadcn/ui CLI** (`npx shadcn@latest init`) — sets up `components.json`,
  path aliases, `src/components/ui/`
- **1st-Pouf** components/blocks pulled in via its registry URL on top of
  the shadcn base
- **Icons via 1st-Pouf's own `Icon` component** (`registry/pouf/Icon.tsx`),
  which wraps `@tabler/icons-react` behind a named-role vocabulary (`home`,
  `cart`, `add`, `remove`, `photo`, `log`, etc. — not `lucide-react`, a
  correction from earlier design discussion) — needed regardless per an
  explicit requirement (icons for sidebar nav items and Notes page
  actions). Any icon-accepting prop also takes a raw React element, so a
  role not in their vocabulary (e.g. a moon icon for the dark-mode toggle)
  can still come from `@tabler/icons-react` directly, already a
  transitive dependency
- Existing `index.css` custom-property token set (`--text`, `--bg`,
  `--accent`, etc.) is **replaced**, not layered — 1st-Pouf ships its own
  token set via `pouf.css`; keeping both would be two competing sources
  of truth for color

## Scope

- **Sidebar shell**: replaces the current horizontal `<nav>` in `App.tsx`
  with a persistent left sidebar (nav links: Home, Shopping List, Image
  Processing, Notes; one icon per link via 1st-Pouf's `Icon`/`NavLink`
  components). Hand-assembled from 1st-Pouf's `layout` primitives
  (`Stack`/`Row`) and `NavLink`, rather than their prebuilt `dashboard`
  block — that block pulls in `BottomNav`, which adds a Radix-Dialog-backed
  "Menu" overflow sheet for mobile. With only four nav items and an
  explicit "no hamburger, no overflow menu, always visible" requirement,
  that block's mobile pattern doesn't fit; a small custom shell composed
  from their lower-level primitives does, without dragging in
  `@radix-ui/react-dialog` for a sheet this project doesn't want.
- **Layout container**: current `#root` (fixed `1126px` max-width,
  centered, single-column) restructured to a full-width flex shell
  (sidebar + main content area). Routing itself (`react-router-dom`)
  is unaffected — only what wraps the routes changes.
- **Notes page restyle** (the one real feature): staged-list and
  saved-list become 1st-Pouf `Card`/`RowCard` components (from
  `surface.tsx`) instead of raw `<ul><li>`; add-input and Save/Remove
  become 1st-Pouf `Field`/`Input`/`Button` components, with icons (`add`,
  `remove` roles) on the action buttons. Visual only — no change to
  staging/batch-save/delete behavior or to `shared/api.ts`.
- **Out of scope for this pass**: Home, Shopping List, and Image
  Processing pages stay in their current plain styling — they're
  placeholders with no real content yet, not worth restyling twice.

## Theming (dark mode)

Confirmed via 1st-Pouf's own docs (`/docs/dark-mode`, `/docs/theming`):
theming is document-based, not media-query-based — `pouf.css` provides
both light and dark token sets, switched via `class="dark"` on `<html>`
(their docs explicitly note this is "the conventional shadcn" approach).
This resolves the one open risk from design discussion: both theme
variants genuinely exist in the kit, nothing needs to be derived by hand.

Design, informed by (but diverging from) 1st-Pouf's own recommended
pattern:

- **Default theme: dark, always** — not OS/`prefers-color-scheme`-driven.
  This is a deliberate divergence from 1st-Pouf's suggested inline script
  (which falls back to OS preference when nothing is stored) — this
  project wants dark-by-default regardless of the visitor's system
  setting.
- **Persistence**: an explicit user toggle choice is saved to
  `localStorage`; absence of a stored value means dark (not "check OS").
- **Flash-of-wrong-theme prevention**: a synchronous inline `<script>` in
  `index.html`'s `<head>`, run before React mounts, reads `localStorage`
  and sets `class="dark"` (or omits it) on `<html>` before first paint.
- **Toggle UI**: 1st-Pouf ships tokens but no switcher component, so this
  is custom-built — a sun/moon icon button (`Icon name="sun"` for one
  state, a raw `IconMoon` from `@tabler/icons-react` for the other, since
  "moon" isn't in 1st-Pouf's named-role vocabulary), styled with
  1st-Pouf's existing `Button` primitive, placed in the sidebar.

## Responsiveness

Two breakpoints (Tailwind's default scale, no custom pixel values), kept
deliberately simple — this is meant to actually be used on a phone, and
"as simple as possible" was an explicit instruction:

- **Mobile** (`< md`, below 768px): the sidebar nav reflows into a plain
  horizontal bar (icons + labels), always visible — no off-canvas drawer,
  no hamburger toggle, no open/close state or animation. Same nav
  markup, just a CSS layout-direction change (`flex-col` → `flex-row`)
  at the breakpoint.
- **Desktop / large monitors** (`≥ 2xl`, 1536px+): the content column
  (not the sidebar, which stays fixed-width) gets a max-width cap and
  re-centers, so content doesn't stretch edge-to-edge on a 21"+ monitor —
  same idea as the current `#root`'s `1126px` cap, scoped to the content
  area now that a sidebar occupies the left edge.
- **In between** (`md`–`2xl`): sidebar visible at fixed width, content
  fills the remaining space fluidly — Tailwind/1st-Pouf's normal
  behavior, no special-casing.

No JS-driven responsive state anywhere in this design — both breakpoints
are pure CSS reflow.

## Testing / verification

No new backend logic or data flow — this is a pure UI/presentation
change. No TDD for this pass (matches how the frontend already works:
`tsc --noEmit` + `oxlint`, no component test suite). Verification is
manual: `make dev-frontend`, click through all four routes, confirm
sidebar nav and theme toggle work, confirm Notes add/remove/save still
functions identically to before (behavior unchanged, only markup/styling
swapped). `tsc --noEmit` and `npm run lint` must still pass.

## Workflow

Branch `designSystem`, merged back to `main` via PR (not direct commits
to `main`, unlike the Notes feature) — first feature in this repo built
this way.

## Open questions / explicitly deferred

- Exact 1st-Pouf sidebar block slug — confirmed at implementation time
  by checking their live registry/blocks listing
- Whether 1st-Pouf's sidebar block is collapsible out of the box, or
  that's added separately — confirmed at implementation time
- Home / Shopping List / Image Processing restyle — deferred until those
  pages have real content (per `PLANNING.md`'s phase order)
