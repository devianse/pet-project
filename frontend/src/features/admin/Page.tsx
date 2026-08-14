import { useEffect, useState } from 'react'
import {
  getAdminUsers,
  grantFeature,
  revokeFeature,
  updateUserRole,
  createUser,
  setUserActive,
  resetUserPassword,
  KNOWN_FEATURES,
  type AdminUser,
  type FeatureKey,
} from '@/shared/api'
import { Card } from '@/components/pouf/surface'
import { Button } from '@/components/pouf/Button'
import { Field, Input } from '@/components/pouf/Input'
import { Stack } from '@/components/pouf/layout'
import { useAuth } from '@/shared/auth'

// One in-flight toggle per (user, feature) cell, keyed "userId:key" — so
// every other cell stays interactive while one grant/revoke call is out,
// and a double-click can't fire the same call twice.
function cellKey(userId: number, key: FeatureKey) {
  return `${userId}:${key}`
}

// generateTempPassword is a client-side convenience for the "Reset
// password" form's Generate button — not a security boundary (the
// server never sees or trusts this generator, only the final string).
function generateTempPassword() {
  const bytes = new Uint8Array(12)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => b.toString(36)).join('').slice(0, 16)
}

export default function AdminPage() {
  const { user: currentUser } = useAuth()
  const [users, setUsers] = useState<AdminUser[]>([])
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState<Set<string>>(new Set())

  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newRole, setNewRole] = useState<'admin' | 'user'>('user')
  const [creating, setCreating] = useState(false)

  const [resetTarget, setResetTarget] = useState<number | null>(null)
  const [resetPassword, setResetPassword] = useState('')
  // Shown once right after a successful reset, then discarded — the
  // server never round-trips a plaintext password, so this is the only
  // place it's ever visible again after the admin typed/generated it.
  const [revealedReset, setRevealedReset] = useState<{ username: string; password: string } | null>(null)

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

  // Same cellKey/pending/refetch pattern as toggle() above — keyed
  // "userId:role" so a role change and a feature toggle on the same row
  // never contend for one pending flag.
  async function changeRole(user: AdminUser, role: 'admin' | 'user') {
    const cell = `${user.id}:role`
    setError(null)
    setPending((prev) => new Set(prev).add(cell))
    try {
      await updateUserRole(user.id, role)
    } catch {
      setError(`failed to update role for ${user.username}`)
    } finally {
      await load()
      setPending((prev) => {
        const next = new Set(prev)
        next.delete(cell)
        return next
      })
    }
  }

  async function toggleActive(user: AdminUser) {
    const cell = `${user.id}:active`
    setError(null)
    setPending((prev) => new Set(prev).add(cell))
    try {
      await setUserActive(user.id, !user.is_active)
    } catch {
      setError(`failed to ${user.is_active ? 'deactivate' : 'reactivate'} ${user.username}`)
    } finally {
      await load()
      setPending((prev) => {
        const next = new Set(prev)
        next.delete(cell)
        return next
      })
    }
  }

  async function handleCreateUser() {
    if (!newUsername.trim() || !newPassword) return
    setError(null)
    setCreating(true)
    try {
      await createUser(newUsername.trim(), newPassword, newRole)
      setNewUsername('')
      setNewPassword('')
      setNewRole('user')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to create user')
    } finally {
      setCreating(false)
    }
  }

  function openResetPassword(user: AdminUser) {
    setError(null)
    setRevealedReset(null)
    setResetPassword('')
    setResetTarget(user.id)
  }

  async function submitResetPassword(user: AdminUser) {
    if (!resetPassword) return
    const cell = `${user.id}:reset`
    setError(null)
    setPending((prev) => new Set(prev).add(cell))
    try {
      await resetUserPassword(user.id, resetPassword)
      setRevealedReset({ username: user.username, password: resetPassword })
      setResetTarget(null)
      setResetPassword('')
    } catch {
      setError(`failed to reset password for ${user.username}`)
    } finally {
      setPending((prev) => {
        const next = new Set(prev)
        next.delete(cell)
        return next
      })
    }
  }

  return (
    <Stack gap={5}>
      <div className="space-y-2">
        <h1 className="text-2xl font-black text-ink">Admin</h1>
        <p className="text-lg font-bold text-muted">Access</p>
      </div>
      {error && (
        <p className="font-bold text-(--on-accent) bg-orange rounded-xl px-3 py-2 self-start">
          {error}
        </p>
      )}
      {revealedReset && (
        <div className="font-bold text-(--on-accent) bg-mint rounded-xl px-3 py-2 self-start flex items-center gap-3">
          <span>
            New password for <span className="font-mono">{revealedReset.username}</span>:{' '}
            <span className="font-mono">{revealedReset.password}</span> — share it now, it won't be shown again.
          </span>
          <Button size="sm" variant="quiet" onClick={() => setRevealedReset(null)}>
            Dismiss
          </Button>
        </div>
      )}

      <Card>
        <Stack gap={4}>
          <h2 className="text-lg font-black text-ink">Create user</h2>
          <div className="grid grid-cols-1 sm:grid-cols-4 gap-3 items-end">
            <Field label="Username">
              {(id) => <Input id={id} value={newUsername} onChange={setNewUsername} autoComplete="off" />}
            </Field>
            <Field label="Password">
              {(id) => (
                <Input id={id} type="password" value={newPassword} onChange={setNewPassword} autoComplete="new-password" />
              )}
            </Field>
            <Field label="Role">
              {(id) => (
                <select
                  id={id}
                  className="rounded-lg bg-bg px-2 py-1 font-bold text-ink min-h-[52px] w-full"
                  value={newRole}
                  onChange={(e) => setNewRole(e.target.value as 'admin' | 'user')}
                >
                  <option value="user">user</option>
                  <option value="admin">admin</option>
                </select>
              )}
            </Field>
            <Button
              tone="mint"
              loading={creating}
              disabled={!newUsername.trim() || !newPassword}
              onClick={handleCreateUser}
            >
              Create
            </Button>
          </div>
        </Stack>
      </Card>

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left">
            <thead>
              <tr>
                <th className="p-2 font-black text-ink">User</th>
                <th className="p-2 font-black text-ink">Role</th>
                <th className="p-2 font-black text-ink">Active</th>
                <th className="p-2 font-black text-ink">Password</th>
                {KNOWN_FEATURES.map((f) => (
                  <th key={f.key} className="p-2 font-black text-ink">
                    {f.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {users.map((user) => {
                const isSelf = user.id === currentUser?.id
                const roleCell = `${user.id}:role`
                const activeCell = `${user.id}:active`
                const resetCell = `${user.id}:reset`
                return (
                  <tr key={user.id}>
                    <td className={`p-2 font-bold text-ink ${user.is_active ? '' : 'opacity-50'}`}>
                      {user.display_name || user.username}
                    </td>
                    <td className="p-2">
                      <select
                        className="rounded-lg bg-bg px-2 py-1 font-bold text-ink disabled:opacity-50 disabled:cursor-not-allowed"
                        value={user.role}
                        disabled={isSelf || pending.has(roleCell)}
                        title={isSelf ? "you can't change your own role" : undefined}
                        onChange={(e) => changeRole(user, e.target.value as 'admin' | 'user')}
                      >
                        <option value="user">user</option>
                        <option value="admin">admin</option>
                      </select>
                    </td>
                    <td className="p-2">
                      <div className="w-28">
                        <Button
                          size="sm"
                          block
                          tone={user.is_active ? 'up' : 'down'}
                          variant={user.is_active ? 'solid' : 'quiet'}
                          loading={pending.has(activeCell)}
                          disabled={isSelf}
                          title={isSelf ? "you can't deactivate your own account" : undefined}
                          aria-pressed={user.is_active}
                          onClick={() => toggleActive(user)}
                        >
                          {user.is_active ? 'Active' : 'Deactivated'}
                        </Button>
                      </div>
                    </td>
                    <td className="p-2">
                      {resetTarget === user.id ? (
                        <div className="flex flex-col gap-2 w-56">
                          <Input
                            value={resetPassword}
                            onChange={setResetPassword}
                            type="text"
                            label={`new password for ${user.username}`}
                            autoComplete="off"
                          />
                          <div className="flex gap-2">
                            <Button size="sm" variant="quiet" onClick={() => setResetPassword(generateTempPassword())}>
                              Generate
                            </Button>
                            <Button
                              size="sm"
                              tone="mint"
                              loading={pending.has(resetCell)}
                              disabled={!resetPassword}
                              onClick={() => submitResetPassword(user)}
                            >
                              Set
                            </Button>
                            <Button size="sm" variant="quiet" onClick={() => setResetTarget(null)}>
                              Cancel
                            </Button>
                          </div>
                        </div>
                      ) : (
                        <Button size="sm" variant="quiet" onClick={() => openResetPassword(user)}>
                          Reset
                        </Button>
                      )}
                    </td>
                    {KNOWN_FEATURES.map((f) => {
                      const granted = user.features.includes(f.key)
                      const cell = cellKey(user.id, f.key)
                      return (
                        <td key={f.key} className="p-2">
                          <div className="w-24">
                            <Button
                              size="sm"
                              block
                              tone={granted ? 'mint' : 'purple'}
                              variant={granted ? 'solid' : 'quiet'}
                              loading={pending.has(cell)}
                              aria-pressed={granted}
                              onClick={() => toggle(user, f.key, granted)}
                            >
                              {granted ? 'Granted' : 'Grant'}
                            </Button>
                          </div>
                        </td>
                      )
                    })}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </Card>
    </Stack>
  )
}
