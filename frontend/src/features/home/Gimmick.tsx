import { useState } from 'react'
import { Button } from '@/components/pouf/Button'
import { Text } from '@/components/pouf/text'
import { Stack, Row } from '@/components/pouf/layout'

// Interactive bits, independent of persona/entrance/accent (see Page.tsx).
// Each is entirely local state — no backend calls — and none of them block
// navigation; AppShell's nav is always right there regardless of what a
// visitor does down here.
export type GimmickKey = 'do-not-click' | 'cookie-trap' | 'terminal' | 'avatar-poke' | 'magic-8-ball' | 'rate-my-vibe' | 'loading-bar'

export const GIMMICK_KEYS: GimmickKey[] = [
  'do-not-click',
  'cookie-trap',
  'terminal',
  'avatar-poke',
  'magic-8-ball',
  'rate-my-vibe',
  'loading-bar',
]

const EIGHT_BALL_ANSWERS = [
  'It is deploying.',
  'Ask again after a cache clear.',
  'The stack traces say yes.',
  'Signs point to a 500.',
  'Reply hazy — check the logs.',
  'Without a doubt (95th percentile).',
  "Don't count on it.",
  'Outlook: eventually consistent.',
]

const VIBE_RATINGS = [
  '11/10, immaculate.',
  '7/10, needs more coffee.',
  '💯 — chef’s kiss.',
  '3/10, but the effort is there.',
  'Incalculable. The scale broke.',
  '9/10, would deploy on a Friday for this vibe.',
]

const TERMINAL_RESPONSES: Record<string, string> = {
  help: 'available commands: help, hire, whoami, sudo, ls',
  hire: "flattering, but there's a login form for that. this is just a home page.",
  whoami: 'someone procrastinating on the actual roadmap by reading terminal jokes.',
  sudo: 'nice try. permission granted: to keep scrolling.',
  ls: 'notes/  watchlist/  regrets/  ',
}

function FakeTerminal() {
  const [value, setValue] = useState('')
  const [lines, setLines] = useState<{ cmd: string; response: string }[]>([])

  function run() {
    const cmd = value.trim()
    if (!cmd) return
    const response =
      TERMINAL_RESPONSES[cmd.toLowerCase()] ?? `command not found: ${cmd} — did you mean to text me instead?`
    setLines((prev) => [...prev.slice(-3), { cmd, response }])
    setValue('')
  }

  return (
    <div className="w-full max-w-sm rounded-control bg-[#1a1a1a] px-4 py-3 font-mono text-[13px] text-[#8fe388] text-left [overflow-wrap:anywhere]">
      {lines.map((l, i) => (
        <div key={i} className="mb-1">
          <div>$ {l.cmd}</div>
          <div className="text-[#c9c9c9]">{l.response}</div>
        </div>
      ))}
      <Row gap={1} justify="start" wrap={false}>
        <span aria-hidden="true">$</span>
        <input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && run()}
          placeholder="try: help"
          aria-label="fake terminal — type a command"
          className="min-w-0 flex-1 bg-transparent border-none outline-none text-[#8fe388] placeholder:text-[#5a5a5a] [font:inherit]"
        />
      </Row>
    </div>
  )
}

function DoNotClickButton({ triggerEffect }: { triggerEffect: (effect: 'shake' | 'comic-sans') => void }) {
  const [ignored, setIgnored] = useState(0)

  function handleClick() {
    setIgnored((n) => n + 1)
    triggerEffect(ignored % 2 === 0 ? 'shake' : 'comic-sans')
  }

  return (
    <Stack gap={2}>
      <Button tone="down" onClick={handleClick}>
        Do Not Click
      </Button>
      {ignored > 0 && (
        <Text size="sm" muted>
          Times ignored basic instructions: {ignored}
        </Text>
      )}
    </Stack>
  )
}

function CookieTrap() {
  const [dread, setDread] = useState(false)
  const [sold, setSold] = useState(false)
  const [closePos, setClosePos] = useState({ x: 0, y: 0 })
  const [attempts, setAttempts] = useState(0)
  const gaveUp = attempts >= 4

  function dodge() {
    if (gaveUp) return
    setAttempts((a) => a + 1)
    setClosePos({ x: (Math.random() - 0.5) * 120, y: (Math.random() - 0.5) * 40 })
  }

  return (
    <Stack gap={2}>
      <Row gap={2} wrap>
        <Button tone="purple" onClick={() => setDread(true)}>
          Accept my existential dread
        </Button>
        <Button tone="pink" onClick={() => setSold(true)}>
          Sell my data to cats
        </Button>
        <div style={{ transform: `translate(${closePos.x}px, ${closePos.y}px)`, transition: 'transform 160ms ease' }}>
          <Button tone="idle" variant="quiet" onMouseEnter={dodge} onClick={dodge}>
            {gaveUp ? 'Fine, you win' : "Close (this won't work)"}
          </Button>
        </div>
      </Row>
      {dread && (
        <Text size="sm" muted>
          Dread accepted. Processing forever.
        </Text>
      )}
      {sold && (
        <Text size="sm" muted>
          🐱 Meow. Deal's done.
        </Text>
      )}
    </Stack>
  )
}

function MagicEightBall() {
  const [answer, setAnswer] = useState<string | null>(null)

  return (
    <Stack gap={2}>
      <Button tone="blue" onClick={() => setAnswer(EIGHT_BALL_ANSWERS[Math.floor(Math.random() * EIGHT_BALL_ANSWERS.length)])}>
        🎱 Ask the void
      </Button>
      {answer && (
        <Text size="sm" muted>
          {answer}
        </Text>
      )}
    </Stack>
  )
}

function RateMyVibe() {
  const [rating, setRating] = useState<string | null>(null)

  return (
    <Stack gap={2}>
      <Button tone="mint" onClick={() => setRating(VIBE_RATINGS[Math.floor(Math.random() * VIBE_RATINGS.length)])}>
        Rate my vibe
      </Button>
      {rating && (
        <Text size="sm" muted>
          {rating}
        </Text>
      )}
    </Stack>
  )
}

const LOADING_TAUNTS = [
  'Almost there.',
  'Just compiling some feelings.',
  'Still faster than npm install.',
  "It's not stuck, it's contemplating.",
  'Rendering vibes...',
]

function LoadingBar() {
  const [clicks, setClicks] = useState(0)
  const taunt = LOADING_TAUNTS[clicks % LOADING_TAUNTS.length]

  return (
    <Stack gap={2}>
      <button
        type="button"
        onClick={() => setClicks((c) => c + 1)}
        aria-label="hurry it up"
        className="w-full max-w-sm rounded-control bg-surface px-1 py-1 cushion-row cursor-pointer border-none"
      >
        <div className="h-3 w-[99%] rounded-control bg-[var(--yellow)]" />
      </button>
      <Text size="sm" muted>
        99% done. {taunt}
      </Text>
    </Stack>
  )
}

export function Gimmick({
  variant,
  triggerEffect,
}: {
  variant: GimmickKey
  triggerEffect: (effect: 'shake' | 'comic-sans') => void
}) {
  switch (variant) {
    case 'do-not-click':
      return <DoNotClickButton triggerEffect={triggerEffect} />
    case 'cookie-trap':
      return <CookieTrap />
    case 'terminal':
      return <FakeTerminal />
    case 'avatar-poke':
      return (
        <Text size="sm" muted>
          psst — tap the avatar
        </Text>
      )
    case 'magic-8-ball':
      return <MagicEightBall />
    case 'rate-my-vibe':
      return <RateMyVibe />
    case 'loading-bar':
      return <LoadingBar />
  }
}
