// frontend/src/features/date-night/categories.ts
import type { DateNightCategory } from '@/shared/api'
import type { Tone } from '@/components/pouf/tone'

export const CATEGORY_TONE: Record<DateNightCategory, Tone> = {
  food: 'orange',
  outdoor: 'mint',
  cozy: 'purple',
  adventure: 'pink',
  culture: 'blue',
}

export const CATEGORY_LABEL: Record<DateNightCategory, string> = {
  food: 'Food',
  outdoor: 'Outdoor',
  cozy: 'Cozy',
  adventure: 'Adventure',
  culture: 'Culture',
}

export const CATEGORIES = Object.keys(CATEGORY_LABEL) as DateNightCategory[]
