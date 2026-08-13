// frontend/src/features/date-night/ProposeForm.tsx
import { useState } from 'react'
import {
  createDateNightProposal,
  type DateNightActivity,
  type DateNightEnergyLevel,
  type DateNightMood,
  type DateNightProposal,
  type DateNightTimeSlot,
} from '@/shared/api'
import { Card } from '@/components/pouf/surface'
import { Button } from '@/components/pouf/Button'
import { Segmented } from '@/components/pouf/Segmented'
import { Stack, Row } from '@/components/pouf/layout'
import { CalendarGrid } from './CalendarGrid'
import { ActivityCard } from './ActivityCard'
import { ENERGY_LABEL, MOOD_LABEL, TIME_SLOT_LABEL } from './format'

// Built from the same map the proposal card reads, so a slot can never be
// labelled one way in the picker and another way in the result.
const TIME_SLOT_OPTIONS: { value: DateNightTimeSlot; label: string }[] = (
  Object.keys(TIME_SLOT_LABEL) as DateNightTimeSlot[]
).map((value) => ({ value, label: TIME_SLOT_LABEL[value] }))

const ENERGY_OPTIONS: { value: DateNightEnergyLevel; label: string }[] = (
  Object.keys(ENERGY_LABEL) as DateNightEnergyLevel[]
).map((value) => ({ value, label: ENERGY_LABEL[value] }))

const MOOD_OPTIONS: { value: DateNightMood; label: string }[] = (
  Object.keys(MOOD_LABEL) as DateNightMood[]
).map((value) => ({ value, label: MOOD_LABEL[value] }))

export function ProposeForm({
  activities,
  onProposed,
}: {
  activities: DateNightActivity[]
  onProposed: (proposal: DateNightProposal) => void
}) {
  const [date, setDate] = useState<string | null>(null)
  const [timeSlot, setTimeSlot] = useState<DateNightTimeSlot>('evening')
  const [energyLevel, setEnergyLevel] = useState<DateNightEnergyLevel>('casual')
  const [moods, setMoods] = useState<DateNightMood[]>([])
  const [activityId, setActivityId] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function toggleMood(mood: DateNightMood) {
    setMoods((prev) => (prev.includes(mood) ? prev.filter((m) => m !== mood) : [...prev, mood]))
  }

  const canSubmit = date !== null && activityId !== null && moods.length > 0

  async function submit() {
    if (!canSubmit || date === null || activityId === null) return
    setSubmitting(true)
    setError(null)
    try {
      const proposal = await createDateNightProposal({
        activity_id: activityId,
        date,
        time_slot: timeSlot,
        energy_level: energyLevel,
        moods,
      })
      onProposed(proposal)
      setDate(null)
      setActivityId(null)
      setMoods([])
    } catch {
      setError('failed to send proposal')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card>
      <Stack gap={5}>
        <h2 className="text-xl font-black text-ink">Propose a new date</h2>
        {error && (
          <p className="font-bold text-(--on-accent) bg-orange rounded-xl px-3 py-2 self-start">{error}</p>
        )}

        <CalendarGrid value={date} onChange={setDate} />

        <Segmented label="Time of day" value={timeSlot} onChange={setTimeSlot} options={TIME_SLOT_OPTIONS} tone="blue" />
        <Segmented label="Energy level" value={energyLevel} onChange={setEnergyLevel} options={ENERGY_OPTIONS} tone="orange" />

        <Stack gap={2}>
          <span className="pouf-label text-[13px] font-black tracking-[0.6px] uppercase text-ink">
            Vibe (pick any)
          </span>
          <Row wrap gap={2}>
            {MOOD_OPTIONS.map((option) => (
              <Button
                key={option.value}
                size="sm"
                variant={moods.includes(option.value) ? 'solid' : 'quiet'}
                tone="pink"
                onClick={() => toggleMood(option.value)}
              >
                {option.label}
              </Button>
            ))}
          </Row>
        </Stack>

        <Stack gap={2}>
          <span className="pouf-label text-[13px] font-black tracking-[0.6px] uppercase text-ink">Activity</span>
          {activities.length === 0 ? (
            <p className="font-bold text-muted">
              No activities yet — add one via "Manage activities" first.
            </p>
          ) : (
            <Row wrap gap={2}>
              {activities.map((activity) => (
                <ActivityCard
                  key={activity.id}
                  activity={activity}
                  selected={activity.id === activityId}
                  onClick={() => setActivityId(activity.id)}
                />
              ))}
            </Row>
          )}
        </Stack>

        <Button onClick={submit} tone="mint" block disabled={!canSubmit} loading={submitting}>
          Send proposal
        </Button>
      </Stack>
    </Card>
  )
}
