import type { ReactNode } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useLocation } from 'react-router-dom'
import HomePage from './features/home/Page'
import ShoppingListPage from './features/shopping-list/Page'
import ImageProcessingPage from './features/image-processing/Page'
import NotesPage from './features/notes/Page'
import WatchlistPage from './features/watchlist/Page'
import DateNightPage from './features/date-night/Page'
import LoginPage from './features/auth/Page'
import { AppShell } from './components/AppShell'
import { AuthProvider, useAuth } from './shared/auth'

// Whole app is gated behind login (per the auth design spec — "everything").
// /login is the one route outside RequireAuth; everything else redirects
// there, remembering where the visitor was headed via router state.
function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()
  const location = useLocation()

  if (loading) {
    return null
  }
  if (!user) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }
  return <>{children}</>
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            path="/*"
            element={
              <RequireAuth>
                <AppShell>
                  <Routes>
                    <Route path="/" element={<HomePage />} />
                    <Route path="/shopping-list" element={<ShoppingListPage />} />
                    <Route path="/image-processing" element={<ImageProcessingPage />} />
                    <Route path="/notes" element={<NotesPage />} />
                    <Route path="/watchlist" element={<WatchlistPage />} />
                    <Route path="/date-night" element={<DateNightPage />} />
                  </Routes>
                </AppShell>
              </RequireAuth>
            }
          />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
