// frontend/src/features/date-night/Page.tsx
import { useEffect, useState } from 'react'
import {
  DateNightForbiddenError,
  getDateNightActivities,
  getDateNightProposals,
} from '@/shared/api'
import type { DateNightActivity, DateNightProposals } from '@/shared/api'
import { Tabs } from '@/components/pouf/disclosure'
import { Stack, Row } from '@/components/pouf/layout'
import { ActivitiesSheet } from './ActivitiesSheet'
import { CurrentTab } from './CurrentTab'
import { HistoryTab } from './HistoryTab'

export default function DateNightPage() {
  const [activities, setActivities] = useState<DateNightActivity[]>([])
  const [proposals, setProposals] = useState<DateNightProposals>({ current: null, history: [] })
  const [tab, setTab] = useState('current')
  const [error, setError] = useState<string | null>(null)
  const [forbidden, setForbidden] = useState(false)

  useEffect(() => {
    Promise.all([getDateNightActivities(), getDateNightProposals()])
      .then(([a, p]) => {
        setActivities(a)
        setProposals(p)
      })
      .catch((err) => {
        if (err instanceof DateNightForbiddenError) {
          setForbidden(true)
          return
        }
        setError('failed to load date night')
      })
  }, [])

  // The nav entry is visible to every logged-in account until the
  // per-user page-visibility system lands, so a non-pair account WILL
  // reach this page. It gets a deliberate answer, not a red error box.
  if (forbidden) {
    return (
      <Stack gap={3}>
        <h1 className="text-2xl font-black text-ink">Date Night</h1>
        <p className="font-bold text-muted">
          This one's just for two 💕 — nothing to see here.
        </p>
      </Stack>
    )
  }

  return (
    <Stack gap={5}>
      <Row justify="between">
        <h1 className="text-2xl font-black text-ink">Date Night</h1>
        <ActivitiesSheet activities={activities} onActivitiesChange={setActivities} />
      </Row>
      {error && (
        <p className="font-bold text-(--on-accent) bg-orange rounded-xl px-3 py-2 self-start">{error}</p>
      )}
      <Tabs
        value={tab}
        onChange={setTab}
        tabs={[
          {
            value: 'current',
            label: 'Current',
            content: (
              <CurrentTab proposals={proposals} activities={activities} onProposalsChange={setProposals} />
            ),
          },
          {
            value: 'history',
            label: 'History',
            content: <HistoryTab history={proposals.history} activities={activities} />,
          },
        ]}
      />
    </Stack>
  )
}
