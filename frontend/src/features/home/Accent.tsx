import { motion, useReducedMotion } from 'framer-motion'
import { useMemo } from 'react'

// Background flavors for the Home cover page, picked independently of
// persona/gimmick/entrance (see Page.tsx). All transform/opacity-only or
// plain CSS gradients — no canvas, no JS animation loop — so they stay
// cheap on a phone. `calm` is the deliberate no-op control case.
export type AccentKey = 'confetti' | 'blob' | 'scanlines' | 'sparkles' | 'bubbles' | 'grid' | 'calm'

export const ACCENT_KEYS: AccentKey[] = ['confetti', 'blob', 'scanlines', 'sparkles', 'bubbles', 'grid', 'calm']

const CONFETTI_TONES = ['var(--pink)', 'var(--purple)', 'var(--blue)', 'var(--mint)', 'var(--yellow)', 'var(--orange)']

function Confetti() {
  const reduceMotion = useReducedMotion()
  // Fixed seed per mount, not per render — Math.random() in render would
  // reshuffle every re-render (e.g. a gimmick's setState) and the burst
  // would look like it's replaying.
  const pieces = useMemo(
    () =>
      Array.from({ length: 24 }, (_, i) => ({
        left: Math.random() * 100,
        delay: Math.random() * 0.4,
        duration: 1.6 + Math.random() * 1.2,
        color: CONFETTI_TONES[i % CONFETTI_TONES.length],
        drift: (Math.random() - 0.5) * 60,
      })),
    [],
  )
  if (reduceMotion) return null
  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      {pieces.map((p, i) => (
        <motion.span
          key={i}
          className="absolute top-[-24px] w-2 h-3 rounded-[2px]"
          style={{ left: `${p.left}%`, backgroundColor: p.color }}
          initial={{ y: -24, x: 0, opacity: 1, rotate: 0 }}
          animate={{ y: '110dvh', x: p.drift, opacity: [1, 1, 0], rotate: 360 }}
          transition={{ duration: p.duration, delay: p.delay, ease: 'easeIn' }}
        />
      ))}
    </div>
  )
}

function GradientBlob() {
  return <div className="pointer-events-none absolute inset-0 overflow-hidden home-blob" aria-hidden="true" />
}

function Scanlines() {
  return <div className="pointer-events-none absolute inset-0 overflow-hidden home-scanlines" aria-hidden="true" />
}

function Sparkles() {
  const reduceMotion = useReducedMotion()
  const pieces = useMemo(
    () =>
      Array.from({ length: 16 }, () => ({
        left: Math.random() * 100,
        top: Math.random() * 100,
        delay: Math.random() * 2,
        duration: 1.4 + Math.random() * 1.6,
        size: 3 + Math.random() * 4,
      })),
    [],
  )
  if (reduceMotion) return null
  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      {pieces.map((p, i) => (
        <motion.span
          key={i}
          className="absolute rounded-[50%] bg-[var(--yellow)]"
          style={{ left: `${p.left}%`, top: `${p.top}%`, width: p.size, height: p.size }}
          initial={{ opacity: 0, scale: 0 }}
          animate={{ opacity: [0, 1, 0], scale: [0, 1, 0] }}
          transition={{ duration: p.duration, delay: p.delay, repeat: Infinity, repeatDelay: 1 }}
        />
      ))}
    </div>
  )
}

const BUBBLE_TONES = ['var(--pink)', 'var(--blue)', 'var(--mint)']

function Bubbles() {
  const reduceMotion = useReducedMotion()
  const pieces = useMemo(
    () =>
      Array.from({ length: 12 }, (_, i) => ({
        left: Math.random() * 100,
        delay: Math.random() * 4,
        duration: 6 + Math.random() * 5,
        size: 16 + Math.random() * 28,
        color: BUBBLE_TONES[i % BUBBLE_TONES.length],
      })),
    [],
  )
  if (reduceMotion) return null
  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      {pieces.map((p, i) => (
        <motion.span
          key={i}
          className="absolute bottom-[-10%] rounded-[50%] opacity-30"
          style={{ left: `${p.left}%`, width: p.size, height: p.size, backgroundColor: p.color }}
          initial={{ y: 0, opacity: 0 }}
          animate={{ y: '-120dvh', opacity: [0, 0.3, 0.3, 0] }}
          transition={{ duration: p.duration, delay: p.delay, repeat: Infinity, ease: 'linear' }}
        />
      ))}
    </div>
  )
}

function Grid() {
  return <div className="pointer-events-none absolute inset-0 overflow-hidden home-grid" aria-hidden="true" />
}

export function Accent({ variant }: { variant: AccentKey }) {
  switch (variant) {
    case 'confetti':
      return <Confetti />
    case 'blob':
      return <GradientBlob />
    case 'scanlines':
      return <Scanlines />
    case 'sparkles':
      return <Sparkles />
    case 'bubbles':
      return <Bubbles />
    case 'grid':
      return <Grid />
    case 'calm':
      return null
  }
}
