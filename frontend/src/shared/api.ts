// Phase 1: just enough to prove frontend -> backend wiring works end to end.
// Phase 2: grows into a real API client — auth-aware requests plus a
// session probe (getMe) and login/logout calls.

let onUnauthorized: (() => void) | null = null

// Set by AuthProvider so any authenticated call that gets a 401 (session
// expired mid-use, not just at app load) can clear client-side auth state
// and let RequireAuth redirect to /login — without every page's fetch
// call needing to know about auth itself.
export function setUnauthorizedHandler(handler: (() => void) | null) {
  onUnauthorized = handler
}

// Wraps fetch for calls that require an existing session. Login/logout/
// getMe deliberately use plain fetch instead — a 401 from getMe is how a
// logged-out session gets discovered in the first place, not a session
// that expired mid-use.
async function request(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const res = await fetch(input, init)
  if (res.status === 401) {
    onUnauthorized?.()
  }
  return res
}

export async function getHealth(): Promise<{ status: string }> {
  const res = await fetch('/api/health')
  if (!res.ok) {
    throw new Error(`health check failed: ${res.status}`)
  }
  return res.json()
}

export type User = {
  username: string
  display_name: string | null
  role: 'admin' | 'user'
}

export async function getMe(): Promise<User | null> {
  const res = await fetch('/api/me')
  if (res.status === 401) {
    return null
  }
  if (!res.ok) {
    throw new Error(`failed to load session: ${res.status}`)
  }
  return res.json()
}

export async function login(username: string, password: string): Promise<User> {
  const res = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) {
    throw new Error('invalid credentials')
  }
  return res.json()
}

export async function logout(): Promise<void> {
  await fetch('/api/auth/logout', { method: 'POST' })
}

export type Note = {
  id: number
  content: string
  created_at: string
}

export async function getNotes(): Promise<Note[]> {
  const res = await request('/api/notes')
  if (!res.ok) {
    throw new Error(`failed to load notes: ${res.status}`)
  }
  return res.json()
}

export async function createNotes(items: string[]): Promise<Note[]> {
  const res = await request('/api/notes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items }),
  })
  if (!res.ok) {
    throw new Error(`failed to save notes: ${res.status}`)
  }
  return res.json()
}

export async function deleteNote(id: number): Promise<void> {
  const res = await request(`/api/notes/${id}`, { method: 'DELETE' })
  if (!res.ok) {
    throw new Error(`failed to delete note: ${res.status}`)
  }
}

export type WatchlistItem = {
  id: number
  imdb_id: string
  media_type: 'movie' | 'tv'
  tmdb_id: number
  title: string
  original_title: string
  original_language: string
  release_year: string | null
  poster_path: string | null
  overview: string
  vote_average: number
  vote_count: number
  genres: string
  viewed: boolean
  created_at: string
}

export async function getWatchlist(): Promise<WatchlistItem[]> {
  const res = await request('/api/watchlist')
  if (!res.ok) {
    throw new Error(`failed to load watchlist: ${res.status}`)
  }
  return res.json()
}

export async function addToWatchlist(link: string): Promise<WatchlistItem> {
  const res = await request('/api/watchlist', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ link }),
  })
  if (!res.ok) {
    const message = await res.text()
    throw new Error(message || `failed to add link: ${res.status}`)
  }
  return res.json()
}

export async function setWatchlistItemViewed(id: number, viewed: boolean): Promise<void> {
  const res = await request(`/api/watchlist/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ viewed }),
  })
  if (!res.ok) {
    throw new Error(`failed to update watchlist item: ${res.status}`)
  }
}

export async function removeFromWatchlist(id: number): Promise<void> {
  const res = await request(`/api/watchlist/${id}`, { method: 'DELETE' })
  if (!res.ok) {
    throw new Error(`failed to remove watchlist item: ${res.status}`)
  }
}
