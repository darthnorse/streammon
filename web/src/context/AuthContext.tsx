import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { api, ApiError } from '../lib/api'
import type { User } from '../types'

interface SetupStatus {
  setup_required: boolean
  enabled_providers: string[]
}

interface AuthContextValue {
  user: User | null
  loading: boolean
  setupRequired: boolean
  setUser: (user: User) => void
  clearSetupRequired: () => void
  refreshUser: () => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  loading: true,
  setupRequired: false,
  setUser: () => {},
  clearSetupRequired: () => {},
  refreshUser: async () => {},
  logout: async () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [setupRequired, setSetupRequired] = useState(false)

  useEffect(() => {
    let mounted = true

    const fetchCurrentUser = () =>
      api.get<User>('/api/me')
        .then(u => mounted && setUser(u))
        .catch((err: unknown) => {
          if (mounted && err instanceof ApiError && err.status === 401) {
            setUser(null)
          }
        })

    api.get<SetupStatus>('/api/setup/status')
      .then(status => {
        if (!mounted) return
        if (status.setup_required) {
          setSetupRequired(true)
          setLoading(false)
          return
        }
        return fetchCurrentUser()
      })
      .catch(() => mounted && fetchCurrentUser())
      .finally(() => mounted && setLoading(false))

    return () => { mounted = false }
  }, [])

  const clearSetupRequired = () => {
    setSetupRequired(false)
  }

  const refreshUser = async () => {
    try {
      const u = await api.get<User>('/api/me')
      setUser(u)
    } catch {
      // Ignore - user may have been logged out
    }
  }

  const logout = async () => {
    try {
      await api.post('/auth/logout')
    } catch {
      // Ignore logout errors - session may already be expired
    }
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, loading, setupRequired, setUser, clearSetupRequired, refreshUser, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  return useContext(AuthContext)
}
