// frontend/src/features/date-night/format.ts
import type { DateNightTimeSlot } from '@/shared/api'

export const TIME_SLOT_LABEL: Record<DateNightTimeSlot, string> = {
  morning: 'Morning',
  afternoon: 'Afternoon',
  evening: 'Evening',
  night: 'Night',
}

/** Renders a `YYYY-MM-DD` proposal date as e.g. "Thu, Aug 20".
 *
 * Parsed field-by-field rather than via `new Date(iso)`: the string form
 * of the constructor treats a date-only value as UTC midnight, which
 * displays as the previous day for anyone west of Greenwich. This is the
 * one timezone bug this feature can actually hit, and it hits it on the
 * single most important line of the UI. */
export function formatProposalDate(iso: string): string {
  const [year, month, day] = iso.split('-').map(Number)
  return new Date(year, month - 1, day).toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
  })
}
