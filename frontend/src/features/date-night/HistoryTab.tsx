import type { DateNightActivity, DateNightProposal } from '@/shared/api'
import { RowCard } from '@/components/pouf/surface'
import { Stack, Row } from '@/components/pouf/layout'
import { formatProposalDate, TIME_SLOT_LABEL } from './format'

export function HistoryTab({
  history,
  activities,
}: {
  history: DateNightProposal[]
  activities: DateNightActivity[]
}) {
  function activityFor(id: number) {
    return activities.find((a) => a.id === id)
  }

  if (history.length === 0) {
    return <p className="font-bold text-muted">No past proposals yet.</p>
  }

  return (
    <Stack gap={2}>
      {history.map((proposal) => (
        <RowCard key={proposal.id}>
          <Row justify="between">
            <span>
              {activityFor(proposal.activity_id)?.name ?? 'Unknown activity'} —{' '}
              {formatProposalDate(proposal.date)} · {TIME_SLOT_LABEL[proposal.time_slot]}
            </span>
            <span className="font-black uppercase text-[12px]">{proposal.status}</span>
          </Row>
        </RowCard>
      ))}
    </Stack>
  )
}
