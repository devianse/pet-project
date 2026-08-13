import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import {
  getMe,
  login as apiLogin,
  logout as apiLogout,
  updateMe as apiUpdateMe,
  setUnauthorizedHandler,
  type AvatarTone,
  type User,
} from './api'

type AuthState = {
  user: User | null
  loading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  updateProfile: (input: { display_name: string | null; avatar_color: AvatarTone | null }) => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

// Probes GET /api/me once on mount to discover whether a session cookie
// already exists (e.g. a page refresh), then exposes login/logout that
// keep `user` in sync with the server. Also registers as the target for
// api.ts's 401 handler, so a session that expires mid-use clears `user`
// the same way an explicit logout does.
export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setUnauthorizedHandler(() => setUser(null))
    getMe()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false))
    return () => setUnauthorizedHandler(null)
  }, [])

  async function login(username: string, password: string) {
    const me = await apiLogin(username, password)
    setUser(me)
  }

  async function logout() {
    await apiLogout()
    setUser(null)
  }

  async function updateProfile(input: { display_name: string | null; avatar_color: AvatarTone | null }) {
    const me = await apiUpdateMe(input)
    setUser(me)
  }

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, updateProfile }}>{children}</AuthContext.Provider>
  )
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return ctx
}
