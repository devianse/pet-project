// frontend/src/features/date-night/ActivityCard.tsx
import type { DateNightActivity } from '@/shared/api'
import { toneClass } from '@/components/pouf/tone'
import { CATEGORY_TONE, CATEGORY_LABEL } from './categories'

export function ActivityCard({
  activity,
  selected,
  onClick,
}: {
  activity: DateNightActivity
  selected: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={[
        toneClass(CATEGORY_TONE[activity.category]),
        'text-left rounded-card px-(--s4) py-(--s3) border-2 cursor-pointer max-w-70',
        '[transition:transform_140ms_ease] border-[var(--tone)]',
        selected
          ? 'bg-[var(--tone)] text-(--on-accent) [transform:translateY(-2px)]'
          : 'bg-[var(--tone)]/10 hover:[transform:translateY(-2px)]',
      ].join(' ')}
    >
      <div className="font-black">{activity.name}</div>
      <div className="text-[13px] font-bold opacity-80">{CATEGORY_LABEL[activity.category]}</div>
      {activity.description && <div className="text-[13px] mt-1">{activity.description}</div>}
    </button>
  )
}
