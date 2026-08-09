// Phase 1: just enough to prove frontend -> backend wiring works end to end.
// Grows into a real API client (auth headers, error handling) in phase 2.
export async function getHealth(): Promise<{ status: string }> {
  const res = await fetch('/api/health')
  if (!res.ok) {
    throw new Error(`health check failed: ${res.status}`)
  }
  return res.json()
}

export type Note = {
  id: number
  content: string
  created_at: string
}

export async function getNotes(): Promise<Note[]> {
  const res = await fetch('/api/notes')
  if (!res.ok) {
    throw new Error(`failed to load notes: ${res.status}`)
  }
  return res.json()
}

export async function createNotes(items: string[]): Promise<Note[]> {
  const res = await fetch('/api/notes', {
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
  const res = await fetch(`/api/notes/${id}`, { method: 'DELETE' })
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
  const res = await fetch('/api/watchlist')
  if (!res.ok) {
    throw new Error(`failed to load watchlist: ${res.status}`)
  }
  return res.json()
}

export async function addToWatchlist(link: string): Promise<WatchlistItem> {
  const res = await fetch('/api/watchlist', {
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
  const res = await fetch(`/api/watchlist/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ viewed }),
  })
  if (!res.ok) {
    throw new Error(`failed to update watchlist item: ${res.status}`)
  }
}

export async function removeFromWatchlist(id: number): Promise<void> {
  const res = await fetch(`/api/watchlist/${id}`, { method: 'DELETE' })
  if (!res.ok) {
    throw new Error(`failed to remove watchlist item: ${res.status}`)
  }
}
