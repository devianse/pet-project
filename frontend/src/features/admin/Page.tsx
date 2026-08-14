import { useEffect, useState } from 'react'
import {
  getAdminUsers,
  grantFeature,
  revokeFeature,
  KNOWN_FEATURES,
  type AdminUser,
  type FeatureKey,
} from '@/shared/api'
import { Card } from '@/components/pouf/surface'
import { Button } from '@/components/pouf/Button'
import { Stack } from '@/components/pouf/layout'

// One in-flight toggle per (user, feature) cell, keyed "userId:key" — so
// every other cell stays interactive while one grant/revoke call is out,
// and a double-click can't fire the same call twice.
function cellKey(userId: number, key: FeatureKey) {
  return `${userId}:${key}`
}

export default function AdminPage() {
  const [users, setUsers] = useState<AdminUser[]>([])
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState<Set<string>>(new Set())

  function load() {
    return getAdminUsers()
      .then(setUsers)
      .catch(() => setError('failed to load users'))
  }

  useEffect(() => {
    load()
  }, [])

  // Re-fetches after every toggle rather than patching local state: the
  // matrix is small (invite-only user base, five known features) and a
  // refetch guarantees the UI always reflects the DB's real grant rows,
  // the same "actual grants, not assumed" principle ListUsers's endpoint
  // already applies server-side. Refetching on both the success and
  // failure paths (via `finally`) matters especially on failure: a 403
  // can mean the caller's own admin role was demoted mid-session, and
  // the refetch is what surfaces that instead of leaving a stale table.
  async function toggle(user: AdminUser, key: FeatureKey, granted: boolean) {
    const cell = cellKey(user.id, key)
    setError(null)
    setPending((prev) => new Set(prev).add(cell))
    try {
      if (granted) {
        await revokeFeature(user.id, key)
      } else {
        await grantFeature(user.id, key)
      }
    } catch {
      setError(`failed to ${granted ? 'revoke' : 'grant'} ${key} for ${user.username}`)
    } finally {
      await load()
      setPending((prev) => {
        const next = new Set(prev)
        next.delete(cell)
        return next
      })
    }
  }

  return (
    <Stack gap={5}>
      <h1 className="text-2xl font-black text-ink">Admin</h1>
      {error && (
        <p className="font-bold text-(--on-accent) bg-orange rounded-xl px-3 py-2 self-start">
          {error}
        </p>
      )}

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left">
            <thead>
              <tr>
                <th className="p-2 font-black text-ink">User</th>
                {KNOWN_FEATURES.map((f) => (
                  <th key={f.key} className="p-2 font-black text-ink">
                    {f.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id}>
                  <td className="p-2 font-bold text-ink">
                    {user.display_name || user.username}
                    {user.role === 'admin' && (
                      <span className="ml-2 text-sm font-bold text-muted">admin</span>
                    )}
                  </td>
                  {KNOWN_FEATURES.map((f) => {
                    const granted = user.features.includes(f.key)
                    const cell = cellKey(user.id, f.key)
                    return (
                      <td key={f.key} className="p-2">
                        <Button
                          size="sm"
                          tone={granted ? 'mint' : 'purple'}
                          variant={granted ? 'solid' : 'quiet'}
                          loading={pending.has(cell)}
                          aria-pressed={granted}
                          onClick={() => toggle(user, f.key, granted)}
                        >
                          {granted ? 'Granted' : 'Grant'}
                        </Button>
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </Stack>
  )
}
