// frontend/src/features/date-night/ActivitiesSheet.tsx
import { useState } from 'react'
import {
  createDateNightActivity,
  deleteDateNightActivity,
  type DateNightActivity,
  type DateNightCategory,
} from '@/shared/api'
import { Sheet } from '@/components/pouf/sheet'
import { Card, RowCard } from '@/components/pouf/surface'
import { Field, Input, Textarea } from '@/components/pouf/Input'
import { Button, IconButton } from '@/components/pouf/Button'
import { Icon } from '@/components/pouf/Icon'
import { Segmented } from '@/components/pouf/Segmented'
import { Stack, Row } from '@/components/pouf/layout'
import { CATEGORIES, CATEGORY_LABEL } from './categories'

export function ActivitiesSheet({
  activities,
  onActivitiesChange,
}: {
  activities: DateNightActivity[]
  onActivitiesChange: (activities: DateNightActivity[]) => void
}) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState<DateNightCategory>('food')
  const [error, setError] = useState<string | null>(null)

  async function add() {
    if (name.trim() === '') return
    setError(null)
    try {
      const created = await createDateNightActivity(name, description || null, category)
      onActivitiesChange([created, ...activities])
      setName('')
      setDescription('')
    } catch {
      setError('failed to add activity')
    }
  }

  async function remove(id: number) {
    setError(null)
    try {
      await deleteDateNightActivity(id)
      onActivitiesChange(activities.filter((a) => a.id !== id))
    } catch (err) {
      // An activity attached to a proposal can't be deleted; the server
      // explains why, so show that rather than a generic failure — same
      // pattern as the Watchlist page's duplicate-link message.
      setError(err instanceof Error ? err.message : 'failed to remove activity')
    }
  }

  return (
    <Sheet
      title="Manage activities"
      description="Add or remove ideas for future dates."
      trigger={
        <Button variant="quiet">
          <Icon name="tag" /> Manage activities
        </Button>
      }
      onOpenChange={(open) => {
        // A failed add/remove otherwise sits in state and reappears stale
        // the next time the sheet opens, even for an unrelated action.
        if (!open) setError(null)
      }}
    >
      <Stack gap={4}>
        {error && (
          <p className="font-bold text-(--on-accent) bg-orange rounded-xl px-3 py-2">{error}</p>
        )}
        <Card variant="tight">
          <Stack gap={3}>
            <Field label="Name">
              {(id, describedBy) => (
                <Input
                  id={id}
                  describedBy={describedBy}
                  value={name}
                  onChange={setName}
                  placeholder="Sushi & a movie"
                />
              )}
            </Field>
            <Field label="Description (optional)">
              {(id, describedBy) => (
                <Textarea id={id} describedBy={describedBy} value={description} onChange={setDescription} rows={2} />
              )}
            </Field>
            <Segmented
              label="Category"
              value={category}
              onChange={setCategory}
              options={CATEGORIES.map((c) => ({ value: c, label: CATEGORY_LABEL[c] }))}
            />
            <Row justify="end">
              <Button onClick={add} tone="purple">
                <Icon name="add" /> Add
              </Button>
            </Row>
          </Stack>
        </Card>

        <Stack gap={2}>
          {activities.map((activity) => (
            <RowCard key={activity.id}>
              <Row justify="between">
                <span>{activity.name}</span>
                <IconButton
                  variant="quiet"
                  size="sm"
                  onClick={() => remove(activity.id)}
                  label={`Remove activity: ${activity.name}`}
                  icon={<Icon name="remove" />}
                />
              </Row>
            </RowCard>
          ))}
        </Stack>
      </Stack>
    </Sheet>
  )
}
