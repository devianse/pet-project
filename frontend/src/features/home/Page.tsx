import { useRef, useState } from 'react'
import { motion, useReducedMotion } from 'framer-motion'
import { useAuth } from '@/shared/auth'
import { Avatar } from '@/components/pouf/avatar'
import { Eyebrow, Heading, Text } from '@/components/pouf/text'
import { Stack } from '@/components/pouf/layout'
import clsx from 'clsx'
import { PERSONAS } from './personas'
import { avatarEntrance, ENTRANCE_COUNT } from './entrances'
import { Gimmick, GIMMICK_KEYS } from './Gimmick'
import { Accent, ACCENT_KEYS } from './Accent'

// Four independently-randomized slots (persona, gimmick, avatar entrance,
// background accent) combine into a lot of different landings without any
// one of them needing to know about the others — see the brainstorm in
// planning/history.md for why. Picked once per mount via lazy useState
// initializers, so it re-rolls on every visit but stays stable across
// re-renders triggered by a gimmick's own local state.
export default function HomePage() {
  const { user } = useAuth()
  const reduceMotion = useReducedMotion() ?? false
  const [personaIndex] = useState(() => Math.floor(Math.random() * PERSONAS.length))
  const [gimmickIndex] = useState(() => Math.floor(Math.random() * GIMMICK_KEYS.length))
  const [entranceIndex] = useState(() => Math.floor(Math.random() * ENTRANCE_COUNT))
  const [accentIndex] = useState(() => Math.floor(Math.random() * ACCENT_KEYS.length))
  const [pageEffect, setPageEffect] = useState<'shake' | 'comic-sans' | null>(null)
  const [avatarPoked, setAvatarPoked] = useState(false)
  const effectTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)

  const persona = PERSONAS[personaIndex]
  const gimmick = GIMMICK_KEYS[gimmickIndex]
  const accent = ACCENT_KEYS[accentIndex]
  const name = user?.display_name || user?.username || 'friend'
  const entrance = avatarEntrance(entranceIndex, reduceMotion)

  function triggerPageEffect(effect: 'shake' | 'comic-sans') {
    if (effectTimeout.current) clearTimeout(effectTimeout.current)
    setPageEffect(effect)
    effectTimeout.current = setTimeout(() => setPageEffect(null), effect === 'shake' ? 500 : 2500)
  }

  return (
    <div
      className={clsx(
        'relative flex min-h-[70dvh] flex-col items-center justify-center overflow-hidden text-center px-4',
        pageEffect === 'shake' && !reduceMotion && 'home-shake',
        pageEffect === 'comic-sans' && 'home-comic-sans',
      )}
    >
      <Accent variant={accent} />
      <div className="relative flex flex-col items-center gap-(--s5)">
        <motion.div
          initial={entrance.initial}
          animate={avatarPoked ? { rotate: [0, -12, 12, -8, 0] } : entrance.animate}
          transition={entrance.transition}
          onClick={() => setAvatarPoked(true)}
          onAnimationComplete={() => avatarPoked && setAvatarPoked(false)}
          className="cursor-pointer"
        >
          <Avatar size="lg" tone={user?.avatar_color ?? 'purple'} fallback={name} />
        </motion.div>
        <Stack gap={2}>
          <Eyebrow>pet-projects</Eyebrow>
          <Heading level={1}>
            <span className={persona.headingClassName}>{persona.headline(name)}</span>
          </Heading>
          <Text muted>{persona.subtext(name)}</Text>
        </Stack>
        <div className="mt-(--s2)">
          <Gimmick variant={gimmick} triggerEffect={triggerPageEffect} />
        </div>
      </div>
    </div>
  )
}
