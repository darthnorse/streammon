import { useState, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'
import { inputClass } from '../lib/constants'
import { errorMessage } from '../lib/utils'
import { useAuth } from '../context/AuthContext'
import { LoadingScreen } from '../components/LoadingScreen'
import { PlexSignInLogin } from '../components/PlexSignInLogin'
import type { User } from '../types'

interface Provider {
  name: string
  enabled: boolean
}

export function Login() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { setUser } = useAuth()
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState(() => {
    const urlError = searchParams.get('error')
    if (urlError === 'guest_access_disabled') return 'Guest access is disabled. Only admins can sign in.'
    return ''
  })
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (searchParams.has('error')) {
      setSearchParams({}, { replace: true })
    }
    api.get<Provider[]>('/auth/providers')
      .then(setProviders)
      .catch(() => setProviders([]))
      .finally(() => setLoading(false))
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const handleLocalSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      const user = await api.post<User>('/auth/local/login', { username, password })
      setUser(user)
      navigate('/', { replace: true })
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  const handlePlexSuccess = (user: User) => {
    setUser(user)
    navigate('/', { replace: true })
  }

  const handleOIDCLogin = () => {
    window.location.href = '/auth/oidc/login'
  }

  const hasLocal = providers.some(p => p.name === 'local' && p.enabled)
  const hasPlex = providers.some(p => p.name === 'plex' && p.enabled)
  const hasOIDC = providers.some(p => p.name === 'oidc' && p.enabled)

  if (loading) {
    return <LoadingScreen />
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface dark:bg-surface-dark p-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <img src="/android-chrome-192x192.png" alt="StreamMon" className="w-16 h-16 mx-auto mb-4" />
          <h1 className="text-3xl font-bold mb-2">StreamMon</h1>
          <p className="text-muted dark:text-muted-dark">Sign in to continue</p>
        </div>

        <div className="card p-6 space-y-6">
          {error && (
            <p className="text-sm text-red-500 dark:text-red-400 text-center">{error}</p>
          )}

          {hasLocal && (
            <form onSubmit={handleLocalSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-1">Username</label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  autoComplete="username"
                  className={inputClass}
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-1">Password</label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoComplete="current-password"
                  className={inputClass}
                />
              </div>

              <button
                type="submit"
                disabled={submitting}
                className="w-full py-2 px-4 rounded-lg bg-accent text-gray-900 font-semibold
                         hover:bg-accent/90 disabled:opacity-50 transition-colors"
              >
                {submitting ? 'Signing in...' : 'Sign In'}
              </button>
            </form>
          )}

          {(hasPlex || hasOIDC) && hasLocal && (
            <div className="relative">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-border dark:border-border-dark" />
              </div>
              <div className="relative flex justify-center text-sm">
                <span className="px-2 bg-panel dark:bg-panel-dark text-muted dark:text-muted-dark">
                  or continue with
                </span>
              </div>
            </div>
          )}

          <div className="space-y-3">
            {hasPlex && (
              <PlexSignInLogin onSuccess={handlePlexSuccess} />
            )}

            {hasOIDC && (
              <button
                onClick={handleOIDCLogin}
                className="w-full py-2 px-4 rounded-lg border border-border dark:border-border-dark
                         hover:bg-panel-hover dark:hover:bg-panel-hover-dark transition-colors
                         font-medium"
              >
                Sign in with SSO
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
