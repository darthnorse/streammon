import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { inputClass } from '../lib/constants'
import { errorMessage } from '../lib/utils'
import { useAuth } from '../context/AuthContext'
import { PlexSignInSetup } from '../components/PlexSignInSetup'
import { MediaServerSignIn } from '../components/MediaServerSignIn'
import type { User } from '../types'

type SetupMethod = 'local' | 'plex' | 'emby' | 'jellyfin' | null

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

export function Setup() {
  const navigate = useNavigate()
  const { setUser, clearSetupRequired } = useAuth()
  const [method, setMethod] = useState<SetupMethod>(null)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSuccess = (user: User) => {
    setUser(user)
    clearSetupRequired()
    navigate('/', { replace: true })
  }

  const handleLocalSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    setSubmitting(true)
    try {
      const user = await api.post<User>('/api/setup/local', { username, password, email })
      handleSuccess(user)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  const goBack = () => setMethod(null)

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface dark:bg-surface-dark p-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <img src="/android-chrome-192x192.png" alt="StreamMon" className="w-16 h-16 mx-auto mb-4" />
          <h1 className="text-3xl font-bold mb-2">Welcome to StreamMon</h1>
          <p className="text-muted dark:text-muted-dark">
            Let's set up your admin account to get started.
          </p>
        </div>

        <div className="card p-6">
          {!method && (
            <div className="space-y-4">
              <h2 className="text-lg font-semibold text-center mb-4">
                Choose how to create your admin account
              </h2>

              <button onClick={() => setMethod('local')} className={methodBtnClass}>
                <div className="font-medium">Create Local Account</div>
                <div className="text-sm text-muted dark:text-muted-dark">
                  Set up a username and password
                </div>
              </button>

              <button
                onClick={() => setMethod('plex')}
                className="w-full py-3 px-4 rounded-lg bg-[#E5A00D] hover:bg-[#cc8e0b] text-gray-900 font-semibold text-center transition-colors"
              >
                Sign in with Plex
              </button>

              <button
                onClick={() => setMethod('emby')}
                className="w-full py-3 px-4 rounded-lg bg-[#52B54B] hover:bg-[#47a040] text-white font-semibold text-center transition-colors"
              >
                Sign in with Emby
              </button>

              <button
                onClick={() => setMethod('jellyfin')}
                className="w-full py-3 px-4 rounded-lg bg-[#00A4DC] hover:bg-[#0090c4] text-white font-semibold text-center transition-colors"
              >
                Sign in with Jellyfin
              </button>
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
                <label className="block text-sm font-medium mb-1">Email (optional)</label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  autoComplete="email"
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
                  minLength={8}
                  autoComplete="new-password"
                  className={inputClass}
                />
                <p className="text-xs text-muted dark:text-muted-dark mt-1">
                  Minimum 8 characters
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium mb-1">Confirm Password</label>
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  required
                  autoComplete="new-password"
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
                {submitting ? 'Creating...' : 'Create Admin Account'}
              </button>
            </form>
          )}

          {method === 'plex' && (
            <div className="space-y-4">
              <BackButton onClick={goBack} />
              <PlexSignInSetup onSuccess={handleSuccess} />
            </div>
          )}

          {method === 'emby' && (
            <div className="space-y-4">
              <BackButton onClick={goBack} />
              <p className="text-sm text-muted dark:text-muted-dark">
                Sign in with an Emby admin account to create the StreamMon admin.
              </p>
              <MediaServerSignIn
                serverType="emby"
                loginEndpoint="/api/setup/emby"
                serversEndpoint="/auth/emby/servers"
                onSuccess={handleSuccess}
              />
            </div>
          )}

          {method === 'jellyfin' && (
            <div className="space-y-4">
              <BackButton onClick={goBack} />
              <p className="text-sm text-muted dark:text-muted-dark">
                Sign in with a Jellyfin admin account to create the StreamMon admin.
              </p>
              <MediaServerSignIn
                serverType="jellyfin"
                loginEndpoint="/api/setup/jellyfin"
                serversEndpoint="/auth/jellyfin/servers"
                onSuccess={handleSuccess}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
