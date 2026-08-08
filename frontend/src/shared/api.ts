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
