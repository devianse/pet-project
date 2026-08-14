import type { Target, Transition } from 'framer-motion'

type Entrance = { initial: Target; animate: Target; transition: Transition }

// Avatar entrance choreographies. Each is transform/opacity only (cheap
// on phones, no layout-triggering properties) and every one collapses to a
// quick fade under reduced motion — see avatarEntrance() below, which is the
// only thing Page.tsx calls.
const ENTRANCES: Entrance[] = [
  {
    // bounce-in
    initial: { scale: 0, y: -40, opacity: 0 },
    animate: { scale: 1, y: 0, opacity: 1 },
    transition: { type: 'spring', bounce: 0.6, duration: 0.8 },
  },
  {
    // spin-in
    initial: { rotate: -180, scale: 0, opacity: 0 },
    animate: { rotate: 0, scale: 1, opacity: 1 },
    transition: { type: 'spring', bounce: 0.35, duration: 0.7 },
  },
  {
    // slide-up-pop
    initial: { y: 60, scale: 0.7, opacity: 0 },
    animate: { y: 0, scale: 1, opacity: 1 },
    transition: { type: 'spring', bounce: 0.55, duration: 0.6 },
  },
  {
    // wobble-settle
    initial: { rotate: 20, scale: 0.8, opacity: 0 },
    animate: { rotate: [20, -10, 5, -3, 0], scale: 1, opacity: 1 },
    transition: { duration: 0.7, ease: 'easeOut' },
  },
  {
    // fade-scale-flip
    initial: { opacity: 0, scale: 0.5, rotateY: 90 },
    animate: { opacity: 1, scale: 1, rotateY: 0 },
    transition: { duration: 0.6, ease: 'easeOut' },
  },
  {
    // drop-and-squash — falls from above then squishes flat before settling
    initial: { y: -120, scaleY: 1, scaleX: 1, opacity: 0 },
    animate: { y: [-120, 0, 0, 0], scaleY: [1, 0.6, 1.08, 1], scaleX: [1, 1.2, 0.95, 1], opacity: 1 },
    transition: { duration: 0.65, ease: 'easeIn', times: [0, 0.55, 0.8, 1] },
  },
  {
    // heartbeat-pulse-in
    initial: { scale: 0, opacity: 0 },
    animate: { scale: [0, 1.15, 0.95, 1.05, 1], opacity: 1 },
    transition: { duration: 0.7, ease: 'easeOut' },
  },
  {
    // teleport-glitch — a couple of quick opacity/position flickers before landing
    initial: { opacity: 0, x: -30, scale: 0.9 },
    animate: { opacity: [0, 1, 0, 1, 0, 1], x: [-30, 10, -6, 3, 0, 0], scale: [0.9, 1, 0.9, 1, 0.97, 1] },
    transition: { duration: 0.5, ease: 'linear' },
  },
]

export function avatarEntrance(index: number, reduceMotion: boolean): Entrance {
  const entrance = ENTRANCES[index % ENTRANCES.length]
  if (!reduceMotion) return entrance
  return { initial: { opacity: 0 }, animate: { opacity: 1 }, transition: { duration: 0.01 } }
}

export const ENTRANCE_COUNT = ENTRANCES.length
