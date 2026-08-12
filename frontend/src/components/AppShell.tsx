import type { ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { NavLink, type LinkComponent } from '@/components/pouf/NavLink'
import type { IconName } from '@/components/pouf/Icon'
import { Button } from '@/components/pouf/Button'
import { ThemeToggle } from './ThemeToggle'
import { useAuth } from '@/shared/auth'

// pouf's NavLink takes a router-agnostic `href` prop; this adapts it to
// react-router-dom's `Link`, which wants `to` instead.
const RouterLinkAdapter: LinkComponent = ({ href, children, ...rest }) => (
  <Link to={href} {...rest}>
    {children}
  </Link>
)

const NAV_ITEMS: { href: string; label: string; icon: IconName }[] = [
  { href: '/', label: 'Home', icon: 'home' },
  { href: '/shopping-list', label: 'Shopping List', icon: 'cart' },
  { href: '/image-processing', label: 'Image Processing', icon: 'photo' },
  { href: '/notes', label: 'Notes', icon: 'log' },
  { href: '/watchlist', label: 'Watchlist', icon: 'play' },
  { href: '/date-night', label: 'Date Night', icon: 'heart' },
]

// Below `md` this reflows to a plain horizontal bar via Tailwind's
// responsive classes alone — same NavLink list, no separate mobile
// component, no drawer/hamburger/toggle state (see design-system spec,
// "Responsiveness"). Above `2xl` the <main> column gets a max-width cap
// so content doesn't stretch edge-to-edge on large monitors.
export function AppShell({ children }: { children: ReactNode }) {
  const { pathname } = useLocation()
  const { user, logout } = useAuth()

  return (
    <div className="flex min-h-screen flex-col md:flex-row bg-bg text-ink">
      <nav
        aria-label="Primary"
        className="flex flex-row items-center gap-2 overflow-x-auto p-3 md:flex-col md:items-stretch md:gap-3 md:overflow-visible md:w-55hrink-0 md:p-6 bg-surface [&>a]:shrink-0 [&>a]:whitespace-nowrap"
      >
        <div className="hidden md:block px-2 pb-2 font-black text-lg text-ink">
          pet-projects
        </div>
        {NAV_ITEMS.map((item) => (
          <NavLink
            key={item.href}
            href={item.href}
            currentPath={pathname}
            icon={item.icon}
            link={RouterLinkAdapter}
          >
            {item.label}
          </NavLink>
        ))}
        <div className="hidden md:block flex-1" />
        {user && (
          <div className="flex flex-row items-center gap-2 md:flex-col md:items-stretch md:gap-2 px-2 md:pb-2">
            <span className="hidden md:block text-sm font-bold text-muted truncate">
              {user.display_name || user.username}
            </span>
            <Button onClick={logout} variant="quiet" size="sm">
              Log out
            </Button>
          </div>
        )}
        <ThemeToggle />
      </nav>
      <main className="flex-1 min-w-0 p-4 md:p-8 2xl:mx-auto 2xl:w-full 2xl:max-w-350">
        {children}
      </main>
    </div>
  )
}
