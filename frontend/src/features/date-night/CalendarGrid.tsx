// frontend/src/features/date-night/CalendarGrid.tsx
import { useState } from 'react'
import { Card } from '@/components/pouf/surface'
import { IconButton } from '@/components/pouf/Button'
import { Icon } from '@/components/pouf/Icon'
import { Row } from '@/components/pouf/layout'

const WEEKDAY_LABELS = ['S', 'M', 'T', 'W', 'T', 'F', 'S']

function daysInMonth(year: number, month: number): number {
  return new Date(year, month + 1, 0).getDate()
}

function toISODate(year: number, month: number, day: number): string {
  const mm = String(month + 1).padStart(2, '0')
  const dd = String(day).padStart(2, '0')
  return `${year}-${mm}-${dd}`
}

export function CalendarGrid({
  value,
  onChange,
}: {
  value: string | null
  onChange: (isoDate: string) => void
}) {
  const today = new Date()
  // Comparing ISO date strings is a valid chronological comparison
  // (zero-padded, most-significant-first) and sidesteps Date arithmetic.
  const todayISO = toISODate(today.getFullYear(), today.getMonth(), today.getDate())
  const [viewYear, setViewYear] = useState(today.getFullYear())
  const [viewMonth, setViewMonth] = useState(today.getMonth())

  const firstWeekday = new Date(viewYear, viewMonth, 1).getDay()
  const totalDays = daysInMonth(viewYear, viewMonth)
  const cells: (number | null)[] = [
    ...Array(firstWeekday).fill(null),
    ...Array.from({ length: totalDays }, (_, i) => i + 1),
  ]

  function goPrevMonth() {
    if (viewMonth === 0) {
      setViewMonth(11)
      setViewYear((y) => y - 1)
    } else {
      setViewMonth((m) => m - 1)
    }
  }

  function goNextMonth() {
    if (viewMonth === 11) {
      setViewMonth(0)
      setViewYear((y) => y + 1)
    } else {
      setViewMonth((m) => m + 1)
    }
  }

  const monthLabel = new Date(viewYear, viewMonth, 1).toLocaleDateString(undefined, {
    month: 'long',
    year: 'numeric',
  })

  return (
    <Card variant="tight">
      <Row justify="between">
        <IconButton variant="quiet" size="sm" onClick={goPrevMonth} label="Previous month" icon={<Icon name="prev" />} />
        <span className="font-black">{monthLabel}</span>
        <IconButton variant="quiet" size="sm" onClick={goNextMonth} label="Next month" icon={<Icon name="next" />} />
      </Row>
      <div className="grid grid-cols-7 gap-(--s1) mt-(--s3)">
        {WEEKDAY_LABELS.map((label, i) => (
          <div key={i} className="text-center text-[12px] font-black text-muted">
            {label}
          </div>
        ))}
        {cells.map((day, i) => {
          if (day === null) return <div key={i} />
          const iso = toISODate(viewYear, viewMonth, day)
          const selected = iso === value
          // You can't invite someone to last Tuesday.
          const past = iso < todayISO
          const isToday = iso === todayISO
          return (
            <button
              key={i}
              type="button"
              onClick={() => onChange(iso)}
              disabled={past}
              aria-pressed={selected}
              aria-current={isToday ? 'date' : undefined}
              className={[
                'aspect-square rounded-control font-bold text-[14px]',
                'enabled:cursor-pointer disabled:opacity-35 disabled:cursor-not-allowed',
                '[transition:transform_140ms_ease] enabled:hover:[transform:scale(1.1)] enabled:active:[transform:scale(0.95)]',
                selected ? 'bg-purple text-(--on-accent)' : 'bg-bg enabled:hover:cushion-field',
                isToday && !selected ? '[box-shadow:inset_0_0_0_2px_var(--purple)]' : '',
              ].join(' ')}
            >
              {day}
            </button>
          )
        })}
      </div>
    </Card>
  )
}
