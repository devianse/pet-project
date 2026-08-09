import { useState } from 'react'
import { IconMoon } from '@tabler/icons-react'
import { IconButton } from '@/components/pouf/Button'
import { Icon } from '@/components/pouf/Icon'

function readIsDark(): boolean {
  return document.documentElement.classList.contains('dark')
}

// 1st-Pouf's named-icon vocabulary has no "moon" role (see Icon.tsx) — the
// sun state uses their Icon component, the moon state uses the underlying
// @tabler/icons-react glyph directly, matching the library's own
// documented escape hatch for icons outside its named set.
export function ThemeToggle() {
  const [dark, setDark] = useState(readIsDark)

  function toggle() {
    const next = !dark
    document.documentElement.classList.toggle('dark', next)
    // Storage can throw in private/restricted browsing contexts (mirrors the
    // try/catch already around the read in index.html's inline script) — the
    // toggle must keep working in-memory/DOM even when persistence fails.
    try {
      localStorage.setItem('theme', next ? 'dark' : 'light')
    } catch {
      // ignore — persistence is a nice-to-have, not required for the toggle to work
    }
    setDark(next)
  }

  return (
    <IconButton
      variant="quiet"
      size="sm"
      onClick={toggle}
      label={dark ? 'Switch to light theme' : 'Switch to dark theme'}
      icon={dark ? <IconMoon size={20} color="currentColor" stroke={2.4} /> : <Icon name="sun" />}
    />
  )
}
