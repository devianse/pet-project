# Design System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the frontend's ad hoc CSS-variable styling with Tailwind
v4 + the 1st-Pouf shadcn-registry component kit, add a persistent sidebar
(reflowing to a horizontal bar on mobile), wire a dark-by-default theme
toggle, and restyle the Notes page as proof — per
`docs/superpowers/specs/2026-08-09-design-system.md`.

**Architecture:** Tailwind v4 (CSS-driven config, no JS config file) via
`@tailwindcss/vite`, with shadcn's file-copy CLI (`npx shadcn@latest add
<registry-url>`) pulling 1st-Pouf component source directly into
`frontend/src/components/pouf/`. A hand-assembled `AppShell` composes
1st-Pouf's lower-level primitives (`Stack`/`Row`, `NavLink`, `Icon`,
`Button`) rather than their prebuilt `dashboard` block, since that block's
mobile pattern (`BottomNav` + a Radix-Dialog "Menu" overflow sheet)
contradicts this project's explicit no-hamburger, always-visible-nav
requirement.

**Tech Stack:** Tailwind CSS v4, `@tailwindcss/vite`, shadcn CLI, 1st-Pouf
registry (`https://1st-pouf.worksonmy.dev/r/*.json`), `@tabler/icons-react`
(1st-Pouf's icon dependency), `@fontsource-variable/nunito`, React 19,
`react-router-dom` v7 (all already in place).

## Global Constraints

- Tailwind v4 syntax only (`@import "tailwindcss"`, `@theme` blocks) — no
  `tailwind.config.js`, that's a v3 pattern
- Dark mode is class-based (`class="dark"` on `<html>`), default **dark**
  unless `localStorage.getItem('theme') === 'light'` — never
  OS-preference-driven
- No off-canvas drawer, no hamburger, no toggle animation for the mobile
  nav — same `NavLink` list, reflowed via Tailwind breakpoint classes only
- Responsive breakpoints: Tailwind's default `md` (768px) and `2xl`
  (1536px) — no custom pixel values
- No TDD for this pass — frontend has no component test suite; `tsc
  --noEmit` and `npm run lint` (oxlint) are the correctness gates, plus
  manual click-through via `make dev-frontend`
- All commands in this plan run from `frontend/` unless stated otherwise
- Branch `designSystem` (already checked out) — commit directly to it,
  this plan does not create or manage the PR

---

### Task 1: Tailwind v4 + shadcn CLI foundation, 1st-Pouf base tokens

**Files:**
- Modify: `frontend/vite.config.ts`
- Modify: `frontend/tsconfig.app.json`
- Create: `frontend/components.json`
- Modify: `frontend/src/index.css`
- Modify: `frontend/src/main.tsx`
- Create (via CLI): `frontend/src/components/pouf/pouf.css`

**Interfaces:**
- Produces: `@/*` path alias resolving to `frontend/src/*` (used by every
  later task's imports); `frontend/src/components/pouf/pouf.css` providing
  the `--color-*`, `--s1`..`--s8`, `--radius-*` custom properties and
  `[data-theme='dark'], .dark` overrides every later task's markup relies
  on for styling

- [ ] **Step 1: Install Tailwind v4 and its Vite plugin**

```bash
cd frontend && npm install -D tailwindcss @tailwindcss/vite
```

- [ ] **Step 2: Add the Tailwind plugin and `@` path alias to Vite config**

Edit `frontend/vite.config.ts`:

```typescript
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // loadEnv reads frontend/.env* — this runs in Node at config time, so
  // process.env isn't populated automatically the way client code's
  // import.meta.env is. '' as the third arg loads all vars, not just
  // VITE_-prefixed ones (fine here since nothing here ships to the browser).
  const env = loadEnv(mode, process.cwd(), '')

  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      port: Number(env.FRONTEND_PORT) || 3000,
      // Mirrors what Caddy will do in production: anything under /api goes
      // to the Go backend, everything else is the SPA. Keeps dev and prod
      // routing behavior consistent instead of diverging.
      proxy: {
        '/api': env.API_PROXY_TARGET || 'http://localhost:8080',
      },
    },
  }
})
```

- [ ] **Step 3: Add the same path alias to TypeScript config**

Edit `frontend/tsconfig.app.json`, adding `baseUrl`/`paths` to
`compilerOptions` (insert after `"moduleResolution": "bundler",`):

```json
    "moduleResolution": "bundler",
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    },
```

- [ ] **Step 4: Create `components.json` for the shadcn CLI**

```json
{
  "$schema": "https://ui.shadcn.com/schema.json",
  "style": "default",
  "tsx": true,
  "tailwind": {
    "config": "",
    "css": "src/index.css",
    "baseColor": "neutral",
    "cssVariables": true
  },
  "aliases": {
    "components": "@/components",
    "utils": "@/lib/utils",
    "ui": "@/components/ui"
  }
}
```

- [ ] **Step 5: Pull in 1st-Pouf's base token file**

```bash
npx shadcn@latest add https://1st-pouf.worksonmy.dev/r/base.json
```

This copies `pouf.css` to `frontend/src/components/pouf/pouf.css` and
installs its declared dependencies (`clsx`, `class-variance-authority`,
`tailwindcss`, `@fontsource-variable/nunito`) into `package.json`.

- [ ] **Step 6: Replace `index.css` to import the base tokens**

The old ad hoc CSS-variable system (`--text`, `--bg`, `--accent`, the
`#root` `1126px` cap, etc.) is fully replaced, not layered — see the
spec's "Stack" section for why.

```css
@import './components/pouf/pouf.css';
```

- [ ] **Step 7: Import the Nunito variable font once, in the app entry point**

Edit `frontend/src/main.tsx`, adding as the first import:

```typescript
import '@fontsource-variable/nunito'
```

- [ ] **Step 8: Verify the app boots and typechecks**

```bash
npm run dev &
sleep 2
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:3000
kill %1
npx tsc --noEmit
```

Expected: `200` from curl, no TypeScript errors. The page will look
unstyled/different from before (no sidebar yet — that's Task 3) but
should load without console errors; open it in a browser to confirm the
background is now 1st-Pouf's light lavender (`#f0e9ff`), not the old
white — that's the token swap taking effect.

- [ ] **Step 9: Commit**

```bash
git add vite.config.ts tsconfig.app.json components.json src/index.css src/main.tsx src/components/pouf/pouf.css package.json package-lock.json
git commit -m "feat: install Tailwind v4 + shadcn CLI, adopt 1st-Pouf base tokens"
```

---

### Task 2: Pull in 1st-Pouf primitives, wire dark-mode default + toggle

**Files:**
- Modify: `frontend/index.html`
- Create (via CLI): `frontend/src/components/pouf/layout.tsx`
- Create (via CLI): `frontend/src/components/pouf/NavLink.tsx`
- Create (via CLI): `frontend/src/components/pouf/Icon.tsx`
- Create (via CLI): `frontend/src/components/pouf/tone.ts`
- Create (via CLI): `frontend/src/components/pouf/Button.tsx`
- Create (via CLI): `frontend/src/components/pouf/Input.tsx`
- Create (via CLI): `frontend/src/components/pouf/surface.tsx`
- Create: `frontend/src/components/ThemeToggle.tsx`

**Interfaces:**
- Consumes: `frontend/src/components/pouf/pouf.css` tokens from Task 1
  (specifically the `[data-theme='dark'], .dark` selector Task 2's toggle
  targets)
- Produces: `Stack`, `Row`, `Grid`, `Spacer` (from `layout.tsx`); `NavLink`,
  `isActivePath`, `type LinkComponent` (from `NavLink.tsx`); `Icon`,
  `renderIcon`, `type IconName`, `type IconLike` (from `Icon.tsx`);
  `Tone`, `toneClass` (from `tone.ts`); `Button` (from `Button.tsx`);
  `Field`, `Input`, `Textarea` (from `Input.tsx`); `Card`, `RowCard` (from
  `surface.tsx`); `ThemeToggle` component — all consumed by Task 3 (shell)
  and Task 4 (Notes restyle)

- [ ] **Step 1: Pull in the primitives Task 3 and Task 4 need**

```bash
npx shadcn@latest add https://1st-pouf.worksonmy.dev/r/layout.json
npx shadcn@latest add https://1st-pouf.worksonmy.dev/r/nav-link.json
npx shadcn@latest add https://1st-pouf.worksonmy.dev/r/icon.json
npx shadcn@latest add https://1st-pouf.worksonmy.dev/r/tone.json
npx shadcn@latest add https://1st-pouf.worksonmy.dev/r/button.json
npx shadcn@latest add https://1st-pouf.worksonmy.dev/r/input.json
npx shadcn@latest add https://1st-pouf.worksonmy.dev/r/surface.json
```

Each command also installs that component's own declared npm
dependencies (e.g. `nav-link` installs nothing new beyond `clsx` since
`Icon`/`tone` are pulled in as registry dependencies automatically;
`icon` installs `@tabler/icons-react`) — no manual `npm install` needed
beyond what the CLI does per-command.

- [ ] **Step 2: Add the no-flash dark-mode-default script to `index.html`**

Edit `frontend/index.html`, adding the script as the **first** thing in
`<head>` (must run before any CSS paints):

```html
<!doctype html>
<html lang="en">
  <head>
    <script>
      // Runs synchronously before paint, so the correct theme is applied
      // with zero flash. Default is dark — unlike 1st-Pouf's own suggested
      // script, this deliberately ignores prefers-color-scheme: dark is
      // the default regardless of the visitor's OS setting, per this
      // project's design choice (see design-system spec, "Theming").
      ;(function () {
        try {
          var stored = localStorage.getItem('theme')
          document.documentElement.classList.toggle('dark', stored !== 'light')
        } catch (e) {
          // localStorage can throw in private/restricted contexts — fall
          // back to the default (dark) rather than losing the page.
          document.documentElement.classList.add('dark')
        }
      })()
    </script>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>frontend</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 3: Build the toggle component**

```typescript
// frontend/src/components/ThemeToggle.tsx
import { useState } from 'react'
import { IconMoon } from '@tabler/icons-react'
import { Button } from './pouf/Button'
import { Icon } from './pouf/Icon'

function readIsDark(): boolean {
  return document.documentElement.classList.contains('dark')
}

// 1st-Pouf's named-icon vocabulary has no "moon" role (see Icon.tsx) — the
// sun state uses their Icon component, the moon state uses the underlying
// @tabler/icons-react glyph directly, matching the library's own
// documented escape hatch for icons outside its named set.
export function ThemeToggle() {
  const [dark, setDark] = useState(readIsDark)

  function toggle() {
    const next = !dark
    document.documentElement.classList.toggle('dark', next)
    localStorage.setItem('theme', next ? 'dark' : 'light')
    setDark(next)
  }

  return (
    <Button
      variant="quiet"
      size="sm"
      onClick={toggle}
      label={dark ? 'Switch to light theme' : 'Switch to dark theme'}
    >
      {dark ? <IconMoon size={20} /> : <Icon name="sun" />}
    </Button>
  )
}
```

- [ ] **Step 4: Verify typecheck and lint pass**

```bash
npx tsc --noEmit
npm run lint
```

Expected: both clean.

- [ ] **Step 5: Commit**

```bash
git add index.html src/components/pouf src/components/ThemeToggle.tsx package.json package-lock.json
git commit -m "feat: pull in 1st-Pouf primitives, wire dark-by-default theme toggle"
```

---

### Task 3: Responsive AppShell (sidebar nav) replacing the top nav bar

**Files:**
- Create: `frontend/src/components/AppShell.tsx`
- Modify: `frontend/src/App.tsx`

**Interfaces:**
- Consumes: `Stack`/`Row` (`@/components/pouf/layout`), `NavLink` +
  `type LinkComponent` (`@/components/pouf/NavLink`), `type IconName`
  (`@/components/pouf/Icon`), `ThemeToggle` (`@/components/ThemeToggle`)
  — all from Task 2
- Produces: `AppShell` component (`{ children: ReactNode }` →
  `ReactElement`), wrapping page content with the sidebar/topbar nav.
  Consumed by `App.tsx` only.

- [ ] **Step 1: Write the AppShell component**

```typescript
// frontend/src/components/AppShell.tsx
import type { ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { NavLink, type LinkComponent } from './pouf/NavLink'
import type { IconName } from './pouf/Icon'
import { ThemeToggle } from './ThemeToggle'

// pouf's NavLink takes a router-agnostic `href` prop; this adapts it to
// react-router-dom's `Link`, which wants `to` instead.
const RouterLinkAdapter: LinkComponent = ({ href, children, ...rest }) => (
  <Link to={href} {...rest}>
    {children}
  </Link>
)

const NAV_ITEMS: { href: string; label: string; icon: IconName }[] = [
  { href: '/', label: 'Home', icon: 'home' },
  { href: '/shopping-list', label: 'Shopping List', icon: 'cart' },
  { href: '/image-processing', label: 'Image Processing', icon: 'photo' },
  { href: '/notes', label: 'Notes', icon: 'log' },
]

// Below `md` this reflows to a plain horizontal bar via Tailwind's
// responsive classes alone — same NavLink list, no separate mobile
// component, no drawer/hamburger/toggle state (see design-system spec,
// "Responsiveness"). Above `2xl` the <main> column gets a max-width cap
// so content doesn't stretch edge-to-edge on large monitors.
export function AppShell({ children }: { children: ReactNode }) {
  const { pathname } = useLocation()

  return (
    <div className="flex min-h-screen flex-col md:flex-row bg-bg text-ink">
      <nav
        aria-label="Primary"
        className="flex flex-row items-center gap-2 overflow-x-auto p-3 md:flex-col md:items-stretch md:gap-3 md:overflow-visible md:w-55 md:shrink-0 md:p-6 bg-surface"
      >
        <div className="hidden md:block px-2 pb-2 font-black text-lg text-ink">
          pet-projects
        </div>
        {NAV_ITEMS.map((item) => (
          <NavLink
            key={item.href}
            href={item.href}
            currentPath={pathname}
            icon={item.icon}
            link={RouterLinkAdapter}
          >
            {item.label}
          </NavLink>
        ))}
        <div className="hidden md:block flex-1" />
        <ThemeToggle />
      </nav>
      <main className="flex-1 min-w-0 p-4 md:p-8 2xl:mx-auto 2xl:w-full 2xl:max-w-350">
        {children}
      </main>
    </div>
  )
}
```

- [ ] **Step 2: Wire it into `App.tsx`, replacing the old top nav**

```typescript
// frontend/src/App.tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import HomePage from './features/home/Page'
import ShoppingListPage from './features/shopping-list/Page'
import ImageProcessingPage from './features/image-processing/Page'
import NotesPage from './features/notes/Page'
import { AppShell } from './components/AppShell'

// Phase 1 shell: nav + routing only. Auth-gating, layout polish, etc. are
// phase 2 once there's something worth protecting. Notes is a phase-2/3
// domain feature pulled forward as a deliberate detour — see
// docs/superpowers/specs/2026-08-08-notes-design.md. Nav chrome now comes
// from AppShell (Tailwind + 1st-Pouf) — see
// docs/superpowers/specs/2026-08-09-design-system.md.
export default function App() {
  return (
    <BrowserRouter>
      <AppShell>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/shopping-list" element={<ShoppingListPage />} />
          <Route path="/image-processing" element={<ImageProcessingPage />} />
          <Route path="/notes" element={<NotesPage />} />
        </Routes>
      </AppShell>
    </BrowserRouter>
  )
}
```

- [ ] **Step 3: Verify typecheck, lint, and manually click through**

```bash
npx tsc --noEmit
npm run lint
make dev-frontend
```

Open `http://localhost:3000` in a browser:
- Desktop width: left sidebar with 4 nav links + theme toggle, content to
  the right
- Narrow the window below ~768px: nav reflows to a horizontal bar at the
  top, all 4 links still visible, no hamburger/menu button anywhere
- Click the theme toggle: page switches between the pastel-light and
  dark-plum palettes instantly, no flash on reload
- Ultra-wide window (or browser zoomed out past ~1536px logical width):
  content column stops growing and re-centers

Stop the dev server (`Ctrl+C`) once confirmed.

- [ ] **Step 4: Commit**

```bash
git add src/components/AppShell.tsx src/App.tsx
git commit -m "feat: responsive sidebar AppShell replacing top nav bar"
```

---

### Task 4: Restyle the Notes page

**Files:**
- Modify: `frontend/src/features/notes/Page.tsx`

**Interfaces:**
- Consumes: `Card`, `RowCard` (`@/components/pouf/surface`); `Field`,
  `Input` (`@/components/pouf/Input`); `Button`
  (`@/components/pouf/Button`); `Icon` (`@/components/pouf/Icon`);
  `Stack`, `Row` (`@/components/pouf/layout`) — all from Task 2. `Note`
  type and `getNotes`/`createNotes`/`deleteNote` from `../../shared/api`
  (unchanged, existing)
- Produces: nothing new consumed elsewhere — this is a leaf page component

No behavior changes: staging/batch-save/delete logic is identical to the
existing implementation, only the JSX/markup changes.

- [ ] **Step 1: Rewrite the page**

```typescript
// frontend/src/features/notes/Page.tsx
import { useEffect, useState } from 'react'
import { createNotes, deleteNote, getNotes, type Note } from '../../shared/api'
import { Card, RowCard } from '@/components/pouf/surface'
import { Field, Input } from '@/components/pouf/Input'
import { Button } from '@/components/pouf/Button'
import { Icon } from '@/components/pouf/Icon'
import { Stack, Row } from '@/components/pouf/layout'

// Saved notes come from the server on mount. Staged notes are typed but
// not yet saved — they live only in this component's state until "Save"
// batches them into one POST. This mirrors the design's split between
// local-only staging edits and the network round-trip.
export default function NotesPage() {
  const [notes, setNotes] = useState<Note[]>([])
  const [staged, setStaged] = useState<string[]>([])
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getNotes()
      .then(setNotes)
      .catch(() => setError('failed to load notes'))
  }, [])

  function addToStaged() {
    if (draft.trim() === '') return
    setStaged((prev) => [...prev, draft])
    setDraft('')
  }

  function removeStaged(index: number) {
    setStaged((prev) => prev.filter((_, i) => i !== index))
  }

  async function save() {
    if (staged.length === 0) return
    setError(null)
    try {
      const updated = await createNotes(staged)
      setNotes(updated)
      setStaged([])
    } catch {
      setError('failed to save notes')
    }
  }

  async function remove(id: number) {
    setError(null)
    try {
      await deleteNote(id)
      setNotes((prev) => prev.filter((n) => n.id !== id))
    } catch {
      setError('failed to delete note')
    }
  }

  return (
    <Stack gap={5}>
      <h1 className="text-2xl font-black text-ink">Notes</h1>
      {error && (
        <p className="font-bold text-(--on-accent) bg-orange rounded-xl px-3 py-2 self-start">
          {error}
        </p>
      )}

      <Card>
        <Stack gap={3}>
          <Field label="New note">
            {(id, describedBy) => (
              <Input
                id={id}
                describedBy={describedBy}
                value={draft}
                onChange={setDraft}
                placeholder="What do you want to remember?"
              />
            )}
          </Field>
          <Row justify="end">
            <Button onClick={addToStaged} tone="purple">
              <Icon name="add" /> Add
            </Button>
          </Row>
        </Stack>
      </Card>

      {staged.length > 0 && (
        <Card variant="tight">
          <Stack gap={2}>
            {staged.map((content, i) => (
              <RowCard key={i}>
                <Row justify="between">
                  <span>{content}</span>
                  <Button
                    variant="quiet"
                    size="sm"
                    onClick={() => removeStaged(i)}
                    label="Remove"
                  >
                    <Icon name="remove" />
                  </Button>
                </Row>
              </RowCard>
            ))}
            <Button onClick={save} tone="mint" block>
              Save
            </Button>
          </Stack>
        </Card>
      )}

      <Stack gap={2}>
        {notes.map((note) => (
          <RowCard key={note.id}>
            <Row justify="between">
              <span>{note.content}</span>
              <Button
                variant="quiet"
                size="sm"
                onClick={() => remove(note.id)}
                label="Remove"
              >
                <Icon name="remove" />
              </Button>
            </Row>
          </RowCard>
        ))}
      </Stack>
    </Stack>
  )
}
```

- [ ] **Step 2: Verify typecheck and lint pass**

```bash
npx tsc --noEmit
npm run lint
```

Expected: both clean.

- [ ] **Step 3: Manually verify behavior is unchanged**

```bash
make dev-frontend
```

(Backend + Postgres must also be running — see `CHEATSHEET.local.md`'s
"Start everything from scratch" section.) Open `http://localhost:3000/notes`:
- Existing saved notes list, styled as cards
- Type a note, click Add → appears in the staged list below the input
- Click Save → staged notes batch-POST, list refreshes, staged list
  clears
- Click Remove on a saved note → it disappears, confirm it's actually
  gone via a page reload

Stop the dev server once confirmed.

- [ ] **Step 4: Commit**

```bash
git add src/features/notes/Page.tsx
git commit -m "feat: restyle Notes page with 1st-Pouf components"
```

---

### Task 5: Final verification pass

**Files:** none (verification only)

**Interfaces:** none

- [ ] **Step 1: Full typecheck and lint**

```bash
cd frontend && npx tsc --noEmit && npm run lint
```

Expected: both clean.

- [ ] **Step 2: Production build succeeds**

```bash
npm run build
```

Expected: exits 0, produces `frontend/dist/`. This is the first time this
plan exercises the real build path (`tsc -b && vite build`), not just
`tsc --noEmit` — confirms Tailwind's build-time CSS generation and the
`@/*` alias both resolve correctly outside dev-server mode too.

- [ ] **Step 3: Full manual click-through, all four routes**

```bash
make dev-frontend   # separate terminal
make dev-backend    # separate terminal, needs DATABASE_URL + Postgres running
```

Visit all of: `/`, `/shopping-list`, `/image-processing`, `/notes`.
Confirm: sidebar/topbar nav present and correctly highlights the active
route on every page; theme toggle works and persists across a reload;
narrowing the browser below ~768px reflows the nav without any console
errors; Notes page fully functional (per Task 4's checks).

Stop both dev servers once confirmed.

- [ ] **Step 4: Confirm branch is ready for review**

```bash
git status
git log --oneline main..designSystem
```

Expected: clean working tree, a small readable commit history on top of
`main`. This plan does not open the PR — that's a manual step once you're
satisfied with the result.
