import { useState, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'
import { inputClass } from '../lib/constants'
import { errorMessage } from '../lib/utils'
import { useAuth } from '../context/AuthContext'
import { LoadingScreen } from '../components/LoadingScreen'
import { PlexSignInLogin } from '../components/PlexSignInLogin'
import { MediaServerSignIn } from '../components/MediaServerSignIn'
import type { User } from '../types'

interface Provider {
  name: string
  enabled: boolean
}

type LoginMethod = 'local' | 'plex' | 'emby' | 'jellyfin' | null

const methodBtnClass =
  'w-full py-3 px-4 rounded-lg border border-border dark:border-border-dark hover:bg-panel-hover dark:hover:bg-panel-hover-dark transition-colors text-left'

function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="text-sm text-accent hover:underline"
    >
      &larr; Back
    </button>
  )
}

export function Login() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { setUser } = useAuth()
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [method, setMethod] = useState<LoginMethod>(null)
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

  const handleProviderSuccess = (user: User) => {
    setUser(user)
    navigate('/', { replace: true })
  }

  const handleOIDCLogin = () => {
    window.location.href = '/auth/oidc/login'
  }

  const goBack = () => {
    setMethod(null)
    setError('')
  }

  const hasLocal = providers.some(p => p.name === 'local' && p.enabled)
  const hasPlex = providers.some(p => p.name === 'plex' && p.enabled)
  const hasOIDC = providers.some(p => p.name === 'oidc' && p.enabled)
  const hasEmby = providers.some(p => p.name === 'emby' && p.enabled)
  const hasJellyfin = providers.some(p => p.name === 'jellyfin' && p.enabled)

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

        <div className="card p-6">
          {error && !method && (
            <p className="text-sm text-red-500 dark:text-red-400 text-center mb-4">{error}</p>
          )}

          {!method && (
            <div className="space-y-3">
              {hasLocal && (
                <button onClick={() => setMethod('local')} className={methodBtnClass}>
                  <div className="font-medium">Local Account</div>
                  <div className="text-sm text-muted dark:text-muted-dark">
                    Sign in with username and password
                  </div>
                </button>
              )}

              {hasPlex && (
                <button
                  onClick={() => setMethod('plex')}
                  className="w-full py-3 px-4 rounded-lg bg-[#E5A00D] hover:bg-[#cc8e0b] text-gray-900 font-semibold text-center transition-colors"
                >
                  Sign in with Plex
                </button>
              )}

              {hasEmby && (
                <button
                  onClick={() => setMethod('emby')}
                  className="w-full py-3 px-4 rounded-lg bg-[#52B54B] hover:bg-[#47a040] text-white font-semibold text-center transition-colors"
                >
                  Sign in with Emby
                </button>
              )}

              {hasJellyfin && (
                <button
                  onClick={() => setMethod('jellyfin')}
                  className="w-full py-3 px-4 rounded-lg bg-[#00A4DC] hover:bg-[#0090c4] text-white font-semibold text-center transition-colors"
                >
                  Sign in with Jellyfin
                </button>
              )}

              {hasOIDC && (
                <button
                  onClick={handleOIDCLogin}
                  className="w-full py-3 px-4 rounded-lg border border-border dark:border-border-dark hover:bg-panel-hover dark:hover:bg-panel-hover-dark font-semibold text-center transition-colors"
                >
                  Sign in with SSO
                </button>
              )}
            </div>
          )}

          {method === 'local' && (
            <form onSubmit={handleLocalSubmit} className="space-y-4">
              <BackButton onClick={goBack} />

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

              {error && (
                <p className="text-sm text-red-500 dark:text-red-400">{error}</p>
              )}

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

          {method === 'plex' && (
            <div className="space-y-4">
              <BackButton onClick={goBack} />
              <PlexSignInLogin onSuccess={handleProviderSuccess} />
            </div>
          )}

          {method === 'emby' && (
            <div className="space-y-4">
              <BackButton onClick={goBack} />
              <MediaServerSignIn
                serverType="emby"
                loginEndpoint="/auth/emby/login"
                serversEndpoint="/auth/emby/servers"
                onSuccess={handleProviderSuccess}
              />
            </div>
          )}

          {method === 'jellyfin' && (
            <div className="space-y-4">
              <BackButton onClick={goBack} />
              <MediaServerSignIn
                serverType="jellyfin"
                loginEndpoint="/auth/jellyfin/login"
                serversEndpoint="/auth/jellyfin/servers"
                onSuccess={handleProviderSuccess}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
