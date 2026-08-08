import { BrowserRouter, Routes, Route, NavLink } from 'react-router-dom'
import HomePage from './features/home/Page'
import ShoppingListPage from './features/shopping-list/Page'
import ImageProcessingPage from './features/image-processing/Page'
import NotesPage from './features/notes/Page'

// Phase 1 shell: nav + routing only. Auth-gating, layout polish, etc. are
// phase 2 once there's something worth protecting. Notes is a phase-2/3
// domain feature pulled forward as a deliberate detour — see
// docs/superpowers/specs/2026-08-08-notes-design.md.
export default function App() {
  return (
    <BrowserRouter>
      <nav>
        <NavLink to="/">Home</NavLink>{' | '}
        <NavLink to="/shopping-list">Shopping List</NavLink>{' | '}
        <NavLink to="/image-processing">Image Processing</NavLink>{' | '}
        <NavLink to="/notes">Notes</NavLink>
      </nav>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/shopping-list" element={<ShoppingListPage />} />
        <Route path="/image-processing" element={<ImageProcessingPage />} />
        <Route path="/notes" element={<NotesPage />} />
      </Routes>
    </BrowserRouter>
  )
}
