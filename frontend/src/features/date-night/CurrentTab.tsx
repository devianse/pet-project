// frontend/src/features/date-night/CurrentTab.tsx
import { useState } from 'react'
import {
  acceptDateNightProposal,
  declineDateNightProposal,
  getDateNightProposals,
  type DateNightActivity,
  type DateNightProposal,
  type DateNightProposals,
} from '@/shared/api'
import { Card } from '@/components/pouf/surface'
import { Button } from '@/components/pouf/Button'
import { Stack, Row } from '@/components/pouf/layout'
import { ProposeForm } from './ProposeForm'
import { DeclineButton } from './DeclineButton'
import { celebrateAccept } from './confetti'
import { formatProposalDate, TIME_SLOT_LABEL } from './format'

const STATUS_LABEL: Record<DateNightProposal['status'], string> = {
  pending: 'Waiting for a response',
  accepted: "It's a date! 🎉",
  declined: 'Declined',
}

export function CurrentTab({
  proposals,
  activities,
  onProposalsChange,
}: {
  proposals: DateNightProposals
  activities: DateNightActivity[]
  onProposalsChange: (proposals: DateNightProposals) => void
}) {
  const [error, setError] = useState<string | null>(null)
  const current = proposals.current

  function activityFor(id: number) {
    return activities.find((a) => a.id === id)
  }

  // Accept/decline fail with 409 when this copy is stale — the other
  // person answered, or proposed something newer, since the page loaded.
  // Re-reading turns a dead-end error into the current state; without it
  // the buttons stay on screen and can only ever fail again.
  async function refresh() {
    try {
      onProposalsChange(await getDateNightProposals())
    } catch {
      // Keep the message from the action that just failed.
    }
  }

  async function accept() {
    if (!current) return
    setError(null)
    try {
      const updated = await acceptDateNightProposal(current.id)
      onProposalsChange({ ...proposals, current: updated })
      celebrateAccept()
    } catch {
      setError("couldn't accept — this may not be the current proposal any more")
      await refresh()
    }
  }

  async function decline() {
    if (!current) return
    setError(null)
    try {
      const updated = await declineDateNightProposal(current.id)
      onProposalsChange({ ...proposals, current: updated })
    } catch {
      setError("couldn't decline — this may not be the current proposal any more")
      await refresh()
    }
  }

  function handleProposed(proposal: DateNightProposal) {
    onProposalsChange({
      current: proposal,
      history: current ? [current, ...proposals.history] : proposals.history,
    })
  }

  return (
    <Stack gap={5}>
      {error && (
        <p className="font-bold text-(--on-accent) bg-orange rounded-xl px-3 py-2 self-start">{error}</p>
      )}

      {current && (
        <Card>
          <Stack gap={3}>
            <span className="text-[13px] font-black uppercase tracking-[0.6px] text-muted">
              {STATUS_LABEL[current.status]}
            </span>
            <h2 className="text-xl font-black text-ink">
              {activityFor(current.activity_id)?.name ?? 'Unknown activity'}
            </h2>
            <span className="font-bold">
              {formatProposalDate(current.date)} · {TIME_SLOT_LABEL[current.time_slot]}
            </span>
            {current.status === 'pending' && (
              <Row gap={3}>
                <Button onClick={accept} tone="mint">
                  Accept
                </Button>
                <DeclineButton onDecline={decline} />
              </Row>
            )}
          </Stack>
        </Card>
      )}

      <ProposeForm activities={activities} onProposed={handleProposed} />
    </Stack>
  )
}
