// frontend/src/features/date-night/DeclineButton.tsx
import { useEffect, useRef, useState } from 'react'
import { buttonClasses } from '@/components/pouf/Button'

// Playful "make you work for it" decline mechanic — see the design
// spec's "Decline mechanics" section. Desktop dodges the cursor and
// gives up after MAX_DODGES; touch instead needs a sustained hold,
// since there's no hover to dodge from.
const MAX_DODGES = 5
const HOLD_DURATION_MS = 3000
const HOLD_COPY = ['Are you sure?', 'Really sure?', 'Okay, fine…']

function usesTouch(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(pointer: coarse)').matches
}

export function DeclineButton({ onDecline }: { onDecline: () => void }) {
  const [dodgeCount, setDodgeCount] = useState(0)
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const [holdProgress, setHoldProgress] = useState(0)
  const holdInterval = useRef<ReturnType<typeof setInterval> | null>(null)
  const containerRef = useRef<HTMLSpanElement | null>(null)
  const buttonRef = useRef<HTMLButtonElement | null>(null)
  const isTouch = useRef(usesTouch())

  // A hold in progress when the card unmounts (the other person's answer
  // lands, the tab switches) would otherwise leave an interval running and
  // eventually call onDecline on a dead component.
  useEffect(() => {
    return () => {
      if (holdInterval.current) clearInterval(holdInterval.current)
    }
  }, [])

  // The spec says the button jumps "to a new spot within its container" —
  // so the destination is measured off the container, not a fixed pixel
  // swing that could land it on top of Accept or outside the card.
  function dodge() {
    if (dodgeCount >= MAX_DODGES) return
    const container = containerRef.current
    const button = buttonRef.current
    if (!container || !button) return
    const maxX = Math.max(0, container.clientWidth - button.offsetWidth)
    const maxY = Math.max(0, container.clientHeight - button.offsetHeight)
    setDodgeCount((c) => c + 1)
    setOffset({
      x: Math.round(Math.random() * maxX),
      y: Math.round(Math.random() * maxY),
    })
  }

  function startHold() {
    if (holdInterval.current) return
    const startedAt = Date.now()
    holdInterval.current = setInterval(() => {
      const elapsed = Date.now() - startedAt
      const progress = Math.min(1, elapsed / HOLD_DURATION_MS)
      setHoldProgress(progress)
      if (progress >= 1) {
        stopHold()
        onDecline()
      }
    }, 50)
  }

  function stopHold() {
    if (holdInterval.current) {
      clearInterval(holdInterval.current)
      holdInterval.current = null
    }
    setHoldProgress(0)
  }

  if (isTouch.current) {
    const holdLabel = holdProgress < 0.34 ? HOLD_COPY[0] : holdProgress < 0.67 ? HOLD_COPY[1] : HOLD_COPY[2]
    return (
      <button
        type="button"
        className={buttonClasses({ tone: 'pink', variant: 'quiet' })}
        onPointerDown={startHold}
        onPointerUp={stopHold}
        onPointerLeave={stopHold}
        style={{
          backgroundImage: `linear-gradient(to right, var(--pink) ${holdProgress * 100}%, transparent ${holdProgress * 100}%)`,
        }}
      >
        {holdProgress > 0 ? holdLabel : 'Decline'}
      </button>
    )
  }

  // Absolute positioning inside a sized container, rather than a
  // transform: the cushion's own active:translateY press is a transform,
  // and an inline one would override it every time the button is clicked.
  return (
    <span ref={containerRef} className="relative block flex-1 min-w-0 h-24">
      <button
        ref={buttonRef}
        type="button"
        className={`${buttonClasses({ tone: 'pink', variant: 'quiet' })} absolute`}
        onMouseEnter={dodge}
        onClick={dodgeCount < MAX_DODGES ? dodge : onDecline}
        style={{ left: offset.x, top: offset.y, transition: 'left 160ms ease, top 160ms ease' }}
      >
        Decline
      </button>
    </span>
  )
}
