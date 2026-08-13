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
import type { FeatureKey } from './shared/api'

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

// Feature-gated routes redirect to / (not /login) — the page simply
// isn't there for this person, not an auth failure. This is convenience
// on top of the real server-side gate (access.RequireFeature 403s the
// underlying API calls regardless of what the UI shows).
function RequireFeature({ feature, children }: { feature: FeatureKey; children: ReactNode }) {
  const { user } = useAuth()
  if (!user || !user.features.includes(feature)) {
    return <Navigate to="/" replace />
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
                    <Route
                      path="/shopping-list"
                      element={
                        <RequireFeature feature="shopping-list">
                          <ShoppingListPage />
                        </RequireFeature>
                      }
                    />
                    <Route
                      path="/image-processing"
                      element={
                        <RequireFeature feature="image-processing">
                          <ImageProcessingPage />
                        </RequireFeature>
                      }
                    />
                    <Route
                      path="/notes"
                      element={
                        <RequireFeature feature="notes">
                          <NotesPage />
                        </RequireFeature>
                      }
                    />
                    <Route
                      path="/watchlist"
                      element={
                        <RequireFeature feature="watchlist">
                          <WatchlistPage />
                        </RequireFeature>
                      }
                    />
                    <Route
                      path="/date-night"
                      element={
                        <RequireFeature feature="date-night">
                          <DateNightPage />
                        </RequireFeature>
                      }
                    />
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
