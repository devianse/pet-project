import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '@/shared/auth'
import { Card } from '@/components/pouf/surface'
import { Field, Input } from '@/components/pouf/Input'
import { Button } from '@/components/pouf/Button'
import { Stack } from '@/components/pouf/layout'
import { ThemeToggle } from '@/components/ThemeToggle'

type LocationState = { from?: { pathname: string } }

export default function LoginPage() {
  const { user, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  if (user) {
    return <Navigate to="/" replace />
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await login(username, password)
      const state = location.state as LocationState | null
      navigate(state?.from?.pathname ?? '/', { replace: true })
    } catch {
      setError('Invalid username or password')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center bg-bg p-4">
      <div className="absolute top-4 right-4">
        <ThemeToggle />
      </div>
      <Card>
        <form onSubmit={handleSubmit}>
          <Stack gap={4}>
            <h1 className="text-2xl font-black text-ink">Log in</h1>
            {error && (
              <p className="font-bold text-(--on-accent) bg-orange rounded-xl px-3 py-2">{error}</p>
            )}
            <Field label="Username">
              {(id, describedBy) => (
                <Input
                  id={id}
                  describedBy={describedBy}
                  value={username}
                  onChange={setUsername}
                  autoComplete="username"
                  autoFocus
                />
              )}
            </Field>
            <Field label="Password">
              {(id, describedBy) => (
                <Input
                  id={id}
                  describedBy={describedBy}
                  type="password"
                  value={password}
                  onChange={setPassword}
                  autoComplete="current-password"
                />
              )}
            </Field>
            <Button type="submit" tone="mint" block disabled={submitting}>
              {submitting ? 'Logging in…' : 'Log in'}
            </Button>
          </Stack>
        </form>
      </Card>
    </div>
  )
}
