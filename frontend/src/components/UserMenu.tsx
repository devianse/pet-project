import * as RPopover from '@radix-ui/react-popover'
import { useState } from 'react'
import clsx from 'clsx'
import { Avatar } from '@/components/pouf/avatar'
import { Button } from '@/components/pouf/Button'
import { Input } from '@/components/pouf/Input'
import { Row, Stack } from '@/components/pouf/layout'
import { Eyebrow, Text } from '@/components/pouf/text'
import { toneClass, type Tone } from '@/components/pouf/tone'
import { useAuth } from '@/shared/auth'
import type { AvatarTone } from '@/shared/api'

// Kept to the same six "brand" tones the backend's allowedAvatarColors
// accepts (backend/internal/auth/handlers.go) — the other five pouf
// Tones (up/down/warn/info/idle) are reserved for status, not identity.
const AVATAR_TONES: AvatarTone[] = ['pink', 'purple', 'blue', 'mint', 'yellow', 'orange']

// No avatar_color set yet: pick a tone deterministically from the
// username so the same account always lands on the same color instead
// of jumping around on every reload.
function deterministicTone(seed: string): AvatarTone {
  let hash = 0
  for (const ch of seed) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0
  return AVATAR_TONES[hash % AVATAR_TONES.length]
}

function initialsOf(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) return '?'
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase()
  return (words[0][0] + words[1][0]).toUpperCase()
}

const dateFormatter = new Intl.DateTimeFormat(undefined, { year: 'numeric', month: 'long' })

/** Avatar-triggered popover with the account's basic info: an editable
 * display name, the (immutable) username, when the account was created,
 * an avatar-color picker, and logout — replacing AppShell's old plain
 * username-and-logout block. */
export function UserMenu() {
  const { user, logout, updateProfile } = useAuth()
  const [open, setOpen] = useState(false)
  const [nameDraft, setNameDraft] = useState('')

  if (!user) return null

  const displayName = user.display_name || user.username
  const tone: Tone = user.avatar_color ?? deterministicTone(user.username)
  const memberSince = dateFormatter.format(new Date(user.created_at))

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) setNameDraft(user!.display_name ?? '')
  }

  async function saveName() {
    const trimmed = nameDraft.trim()
    if (trimmed === (user!.display_name ?? '')) return
    await updateProfile({ display_name: trimmed || null, avatar_color: user!.avatar_color })
  }

  async function saveColor(next: AvatarTone) {
    if (next === user!.avatar_color) return
    await updateProfile({ display_name: user!.display_name, avatar_color: next })
  }

  async function handleLogout() {
    setOpen(false)
    await logout()
  }

  return (
    <RPopover.Root open={open} onOpenChange={handleOpenChange}>
      <RPopover.Trigger asChild>
        <button type="button" aria-label="Account menu" className="pouf-menu__anchor">
          <Avatar fallback={initialsOf(displayName)} tone={tone} size="sm" />
        </button>
      </RPopover.Trigger>
      <RPopover.Portal>
        <RPopover.Content
          className="pouf-menu user-menu-popover w-72 max-w-[calc(100vw-2rem)]"
          sideOffset={8}
          align="end"
          collisionPadding={16}
        >
          <Stack gap={4}>
            <Row gap={3} align="top">
              <Avatar fallback={initialsOf(displayName)} tone={tone} size="md" />
              <Stack gap={1}>
                <Input
                  value={nameDraft}
                  onChange={setNameDraft}
                  onBlur={saveName}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') e.currentTarget.blur()
                  }}
                  placeholder={user.username}
                  label="Display name"
                />
                <Text size="sm" muted truncate>
                  @{user.username}
                </Text>
              </Stack>
            </Row>
            <Text size="sm" muted>
              Member since {memberSince}
            </Text>
            <Stack gap={2}>
              <Eyebrow>Avatar color</Eyebrow>
              <Row gap={2}>
                {AVATAR_TONES.map((t) => (
                  <button
                    key={t}
                    type="button"
                    aria-label={t}
                    aria-pressed={t === tone}
                    onClick={() => saveColor(t)}
                    className={clsx(
                      'w-7 h-7 rounded-full bg-(--tone) cursor-pointer transition-transform',
                      toneClass(t),
                      t === tone && 'ring-2 ring-offset-2 ring-ink scale-110',
                    )}
                  />
                ))}
              </Row>
            </Stack>
            <Button onClick={handleLogout} variant="quiet" size="sm" block>
              Log out
            </Button>
          </Stack>
        </RPopover.Content>
      </RPopover.Portal>
    </RPopover.Root>
  )
}
