import { BrowserRouter, Routes, Route } from 'react-router-dom'
import HomePage from './features/home/Page'
import ShoppingListPage from './features/shopping-list/Page'
import ImageProcessingPage from './features/image-processing/Page'
import NotesPage from './features/notes/Page'
import WatchlistPage from './features/watchlist/Page'
import { AppShell } from './components/AppShell'

// Phase 1 shell: nav + routing only. Auth-gating, layout polish, etc. are
// phase 2 once there's something worth protecting. Notes and Watchlist
// are phase-2/3 domain features pulled forward as deliberate detours —
// see docs/superpowers/specs/2026-08-08-notes-design.md and
// docs/superpowers/specs/2026-08-09-movie-tv-sharing-list-design.md. Nav
// chrome comes from AppShell (Tailwind + 1st-Pouf) — see
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
          <Route path="/watchlist" element={<WatchlistPage />} />
        </Routes>
      </AppShell>
    </BrowserRouter>
  )
}
