// frontend/src/features/date-night/confetti.ts
import confetti from 'canvas-confetti'

export function celebrateAccept() {
  confetti({
    particleCount: 120,
    spread: 70,
    origin: { y: 0.6 },
  })
}
