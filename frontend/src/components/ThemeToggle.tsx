import { useState } from 'react'
import { IconMoon } from '@tabler/icons-react'
import { Button } from './pouf/Button'
import { Icon } from './pouf/Icon'

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
    localStorage.setItem('theme', next ? 'dark' : 'light')
    setDark(next)
  }

  return (
    <Button
      variant="quiet"
      size="sm"
      onClick={toggle}
      label={dark ? 'Switch to light theme' : 'Switch to dark theme'}
    >
      {dark ? <IconMoon size={20} /> : <Icon name="sun" />}
    </Button>
  )
}
