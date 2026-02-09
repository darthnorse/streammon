import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import { inputClass } from '../lib/constants'
import { errorMessage } from '../lib/utils'
import type { User } from '../types'

interface ServerOption {
  id: number
  name: string
}

interface MediaServerSignInProps {
  serverType: 'emby' | 'jellyfin'
  loginEndpoint: string
  serversEndpoint: string
  onSuccess: (user: User) => void
}

export function MediaServerSignIn({
  serverType,
  loginEndpoint,
  serversEndpoint,
  onSuccess,
}: MediaServerSignInProps) {
  const [servers, setServers] = useState<ServerOption[]>([])
  const [selectedServer, setSelectedServer] = useState<number | null>(null)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [loadingServers, setLoadingServers] = useState(true)

  const label = serverType === 'emby' ? 'Emby' : 'Jellyfin'

  useEffect(() => {
    api.get<ServerOption[]>(serversEndpoint)
      .then(list => {
        setServers(list)
        if (list.length === 1) {
          setSelectedServer(list[0].id)
        }
      })
      .catch(() => setServers([]))
      .finally(() => setLoadingServers(false))
  }, [serversEndpoint])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedServer) return
    setError('')
    setSubmitting(true)
    try {
      const user = await api.post<User>(loginEndpoint, {
        server_id: selectedServer,
        username,
        password,
      })
      onSuccess(user)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  if (loadingServers) {
    return (
      <div className="text-sm text-muted dark:text-muted-dark text-center py-2">
        Loading {label} servers...
      </div>
    )
  }

  if (servers.length === 0) {
    return (
      <p className="text-sm text-muted dark:text-muted-dark text-center py-2">
        No {label} servers configured. Add a server in Settings first.
      </p>
    )
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      {servers.length > 1 && (
        <div>
          <label className="block text-sm font-medium mb-1">{label} Server</label>
          <select
            value={selectedServer ?? ''}
            onChange={e => setSelectedServer(e.target.value ? Number(e.target.value) : null)}
            className={inputClass}
            required
          >
            <option value="">Select a server...</option>
            {servers.map(s => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>
        </div>
      )}

      <div>
        <label className="block text-sm font-medium mb-1">{label} Username</label>
        <input
          type="text"
          value={username}
          onChange={e => setUsername(e.target.value)}
          required
          autoComplete="username"
          className={inputClass}
        />
      </div>

      <div>
        <label className="block text-sm font-medium mb-1">{label} Password</label>
        <input
          type="password"
          value={password}
          onChange={e => setPassword(e.target.value)}
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
        disabled={submitting || !selectedServer}
        className="w-full py-2 px-4 rounded-lg border border-border dark:border-border-dark
                 hover:bg-panel-hover dark:hover:bg-panel-hover-dark disabled:opacity-50
                 transition-colors font-medium"
      >
        {submitting ? 'Signing in...' : `Sign in with ${label}`}
      </button>
    </form>
  )
}
