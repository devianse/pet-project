import type { ReactNode } from 'react'

export type Persona = {
  key: string
  headline: (name: string) => ReactNode
  subtext: (name: string) => string
  /** Extra className applied to the heading only — used by the handful of
   *  personas that want a skew/wiggle treatment the others don't. */
  headingClassName?: string
}

function timeOfDay(): 'morning' | 'afternoon' | 'evening' | 'night' {
  const hour = new Date().getHours()
  if (hour < 5) return 'night'
  if (hour < 12) return 'morning'
  if (hour < 18) return 'afternoon'
  return 'evening'
}

// Guilt-trip numbers are generated once per mount (not persisted anywhere,
// not a real counter) — purely a punchline, so no backend involvement.
function fakeVisitorNumber(): number {
  return 1000 + Math.floor(Math.random() * 9000)
}

function fakeBouncePercent(): number {
  return 60 + Math.floor(Math.random() * 35)
}

// Independently picked from the avatar entrance / gimmick / accent slots
// (see Page.tsx) so the combinations multiply rather than repeat. One
// (`wholesome`) is a deliberate control case so the page isn't a bit 100%
// of the time.
export const PERSONAS: Persona[] = [
  {
    key: 'corporate',
    headline: () => 'Welcome, valued Human Capital Synergy Unit.',
    subtext: () => "Let's leverage some synergistic bandwidth together.",
  },
  {
    key: 'glitch',
    headline: () => 'ERROR 418: Human detected at /home.',
    subtext: () => 'System requires you to be awake to proceed.',
  },
  {
    key: 'geocities',
    headline: () => '🚧 UNDER CONSTRUCTION SINCE 2018 🚧',
    subtext: () => 'Best viewed at 800×600. Please disable your firewall.',
    headingClassName: 'home-wiggle inline-block [text-shadow:2px_2px_0_var(--yellow)]',
  },
  {
    key: 'guilt-trip',
    headline: () => `Welcome, visitor #${fakeVisitorNumber()}.`,
    subtext: () => `${fakeBouncePercent()}% of you already left because the CSS is ugly.`,
  },
  {
    key: 'wholesome',
    headline: (name) => `Good ${timeOfDay()}, ${name}.`,
    subtext: () => "Glad you're here.",
  },
  {
    key: 'pirate',
    headline: (name) => `Ahoy, ${name}! Ye've boarded the good ship pet-projects.`,
    subtext: () => "Mind the CSS, she's a bit leaky below deck.",
  },
  {
    key: 'noir',
    headline: () => 'The name’s Home. It was a dark and stormy login.',
    subtext: () => 'Somebody had to render this page. It might as well be me.',
  },
  {
    key: 'hype',
    headline: (name) => `LADIES AND GENTLEMEN, ${name.toUpperCase()} HAS ENTERED THE BUILDING.`,
    subtext: () => 'The crowd (me) goes wild.',
  },
  {
    key: 'fortune-cookie',
    headline: () => 'The stack foresees great uptime for you today.',
    subtext: () => 'A wise deploy avoids Fridays.',
  },
]
