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

// Hand-kept in sync with backend/internal/access/features.go's
// KnownFeatures — same duplication route paths already have between
// cmd/api/main.go and App.tsx, no shared source of truth across the
// language boundary.
export type FeatureKey = 'notes' | 'watchlist' | 'date-night' | 'shopping-list' | 'image-processing'

export type User = {
  id: number
  username: string
  display_name: string | null
  role: 'admin' | 'user'
  features: FeatureKey[]
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

export type DateNightCategory = 'food' | 'outdoor' | 'cozy' | 'adventure' | 'culture'

export type DateNightActivity = {
  id: number
  name: string
  description: string | null
  category: DateNightCategory
  created_at: string
}

export type DateNightTimeSlot = 'morning' | 'afternoon' | 'evening' | 'night'
export type DateNightEnergyLevel = 'couch_potato' | 'casual' | 'adventurous' | 'unstoppable'
export type DateNightMood =
  | 'romantic'
  | 'playful'
  | 'nostalgic'
  | 'cozy'
  | 'excited'
  | 'chill'
  | 'sentimental'
  | 'silly'
export type DateNightStatus = 'pending' | 'accepted' | 'declined'

export type DateNightProposal = {
  id: number
  activity_id: number
  date: string
  time_slot: DateNightTimeSlot
  energy_level: DateNightEnergyLevel
  moods: DateNightMood[]
  status: DateNightStatus
  proposed_by_user_id: number
  proposed_by_username: string
  created_at: string
}

export type DateNightProposals = {
  current: DateNightProposal | null
  history: DateNightProposal[]
}

// The datenight routes 403 any account not granted the "date-night"
// feature (backend/internal/access) — RequireFeature in App.tsx and the
// nav filtering in AppShell.tsx keep this from being reachable in the
// UI at all for most accounts, but this class still exists as the
// server-side signal a direct/stale request can hit.
export class DateNightForbiddenError extends Error {
  constructor() {
    super('this account is not part of the pair')
  }
}

export async function getDateNightActivities(): Promise<DateNightActivity[]> {
  const res = await request('/api/datenight/activities')
  if (res.status === 403) throw new DateNightForbiddenError()
  if (!res.ok) throw new Error(`failed to load activities: ${res.status}`)
  return res.json()
}

export async function createDateNightActivity(
  name: string,
  description: string | null,
  category: DateNightCategory,
): Promise<DateNightActivity> {
  const res = await request('/api/datenight/activities', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, description, category }),
  })
  if (!res.ok) throw new Error(`failed to create activity: ${res.status}`)
  return res.json()
}

export async function deleteDateNightActivity(id: number): Promise<void> {
  const res = await request(`/api/datenight/activities/${id}`, { method: 'DELETE' })
  // 409 means a proposal still references it — the server's message says
  // so in words worth showing, unlike a bare status code.
  if (res.status === 409) throw new Error((await res.text()).trim())
  if (!res.ok) throw new Error(`failed to delete activity: ${res.status}`)
}

export async function getDateNightProposals(): Promise<DateNightProposals> {
  const res = await request('/api/datenight/proposals')
  if (res.status === 403) throw new DateNightForbiddenError()
  if (!res.ok) throw new Error(`failed to load proposals: ${res.status}`)
  return res.json()
}

export async function createDateNightProposal(input: {
  activity_id: number
  date: string
  time_slot: DateNightTimeSlot
  energy_level: DateNightEnergyLevel
  moods: DateNightMood[]
}): Promise<DateNightProposal> {
  const res = await request('/api/datenight/proposals', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
  if (!res.ok) throw new Error(`failed to create proposal: ${res.status}`)
  return res.json()
}

export async function acceptDateNightProposal(id: number): Promise<DateNightProposal> {
  const res = await request(`/api/datenight/proposals/${id}/accept`, { method: 'POST' })
  if (!res.ok) throw new Error(`failed to accept proposal: ${res.status}`)
  return res.json()
}

export async function declineDateNightProposal(id: number): Promise<DateNightProposal> {
  const res = await request(`/api/datenight/proposals/${id}/decline`, { method: 'POST' })
  if (!res.ok) throw new Error(`failed to decline proposal: ${res.status}`)
  return res.json()
}
