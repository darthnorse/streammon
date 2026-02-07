import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { inputClass } from '../lib/constants'
import { useAuth } from '../context/AuthContext'
import { PlexSignInSetup } from '../components/PlexSignInSetup'
import type { User } from '../types'

type SetupMethod = 'local' | 'plex' | null

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
      setUser(user)
      clearSetupRequired()
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Setup failed')
    } finally {
      setSubmitting(false)
    }
  }

  const handlePlexSuccess = (user: User) => {
    setUser(user)
    clearSetupRequired()
    navigate('/', { replace: true })
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface dark:bg-surface-dark p-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
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

              <button
                onClick={() => setMethod('local')}
                className="w-full py-3 px-4 rounded-lg border border-border dark:border-border-dark
                         hover:bg-panel-hover dark:hover:bg-panel-hover-dark transition-colors
                         text-left"
              >
                <div className="font-medium">Create Local Account</div>
                <div className="text-sm text-muted dark:text-muted-dark">
                  Set up a username and password
                </div>
              </button>

              <button
                onClick={() => setMethod('plex')}
                className="w-full py-3 px-4 rounded-lg border border-border dark:border-border-dark
                         hover:bg-panel-hover dark:hover:bg-panel-hover-dark transition-colors
                         text-left"
              >
                <div className="font-medium flex items-center gap-2">
                  <span className="text-[#E5A00D]">Plex</span> Sign In
                </div>
                <div className="text-sm text-muted dark:text-muted-dark">
                  Use your Plex.tv account
                </div>
              </button>
            </div>
          )}

          {method === 'local' && (
            <form onSubmit={handleLocalSubmit} className="space-y-4">
              <button
                type="button"
                onClick={() => setMethod(null)}
                className="text-sm text-accent hover:underline"
              >
                &larr; Back
              </button>

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
              <button
                type="button"
                onClick={() => setMethod(null)}
                className="text-sm text-accent hover:underline"
              >
                &larr; Back
              </button>

              <PlexSignInSetup onSuccess={handlePlexSuccess} />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
