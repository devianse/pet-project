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
        <p className="font-bold text-[var(--on-accent)] bg-orange rounded-xl px-3 py-2 self-start">
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
