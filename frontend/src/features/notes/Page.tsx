import { useEffect, useState } from 'react'
import { createNotes, deleteNote, getNotes, type Note } from '../../shared/api'

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
    <div>
      <h1>Notes</h1>
      {error && <p>{error}</p>}

      <input
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        placeholder="New note"
      />
      <button onClick={addToStaged}>Add</button>

      {staged.length > 0 && (
        <ul>
          {staged.map((content, i) => (
            <li key={i}>
              {content} <button onClick={() => removeStaged(i)}>Remove</button>
            </li>
          ))}
        </ul>
      )}
      {staged.length > 0 && <button onClick={save}>Save</button>}

      <ul>
        {notes.map((note) => (
          <li key={note.id}>
            {note.content} <button onClick={() => remove(note.id)}>Remove</button>
          </li>
        ))}
      </ul>
    </div>
  )
}
