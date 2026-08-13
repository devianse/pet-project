import type { DateNightActivity, DateNightProposal } from '@/shared/api'
import { Card } from '@/components/pouf/surface'
import { Stack, Row } from '@/components/pouf/layout'
import { toneClass } from '@/components/pouf/tone'
import { CATEGORY_TONE, CATEGORY_LABEL } from './categories'
import { formatProposalDate, TIME_SLOT_LABEL, ENERGY_LABEL, MOOD_LABEL } from './format'

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
      {history.map((proposal) => {
        const activity = activityFor(proposal.activity_id)
        return (
          <Card key={proposal.id} variant="tight">
            <Stack gap={2}>
              <Row justify="between" wrap>
                {activity ? (
                  <span
                    className={[
                      toneClass(CATEGORY_TONE[activity.category]),
                      'font-black uppercase text-[12px] bg-[var(--tone)]/10 text-[var(--tone)] rounded-full px-2 py-1',
                    ].join(' ')}
                  >
                    {CATEGORY_LABEL[activity.category]}
                  </span>
                ) : (
                  <span />
                )}
                <span className="font-black uppercase text-[12px]">{proposal.status}</span>
              </Row>

              <span className="font-black text-ink">{activity?.name ?? 'Unknown activity'}</span>
              {activity?.description && <span className="text-[13px]">{activity.description}</span>}

              <span className="font-bold text-[13px]">
                {formatProposalDate(proposal.date)} · {TIME_SLOT_LABEL[proposal.time_slot]}
              </span>

              <span className="text-[13px]">{ENERGY_LABEL[proposal.energy_level]}</span>

              <Row wrap gap={1}>
                {proposal.moods.map((mood) => (
                  <span
                    key={mood}
                    className="font-bold text-[12px] bg-pink/10 text-pink rounded-full px-2 py-1"
                  >
                    {MOOD_LABEL[mood]}
                  </span>
                ))}
              </Row>

              <span className="text-[13px] text-muted">{proposal.proposed_by_username} proposed</span>
            </Stack>
          </Card>
        )
      })}
    </Stack>
  )
}
