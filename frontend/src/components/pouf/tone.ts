export type Tone =
  | 'pink'
  | 'purple'
  | 'blue'
  | 'mint'
  | 'yellow'
  | 'orange'
  | 'up'
  | 'down'
  | 'warn'
  | 'info'
  | 'idle'

export const toneClass = (tone?: Tone): string | undefined => (tone ? `tone-${tone}` : undefined)
