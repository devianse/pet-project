import { Row } from '@/components/pouf/layout'
import { MOOD_LABEL } from './format'
import type { DateNightMood } from '@/shared/api'

export function MoodChips({ moods }: { moods: DateNightMood[] }) {
  return (
    <Row wrap gap={1}>
      {moods.map((mood) => (
        <span key={mood} className="font-bold text-[12px] bg-pink/10 rounded-full px-2 py-1">
          {MOOD_LABEL[mood]}
        </span>
      ))}
    </Row>
  )
}
