import { useState, useRef, useCallback, useEffect } from 'react'
import { getClientId, requestPin, checkPin, getAuthUrl, fetchResources, type PlexResource } from '../lib/plexOAuth'
import { api } from '../lib/api'

interface PlexSignInProps {
  onServersAdded: () => void
}

type Phase = 'idle' | 'polling' | 'selecting' | 'adding'

function pickConnection(resource: PlexResource): string {
  const conns = resource.connections
  const httpsNonRelay = conns.find(c => c.protocol === 'https' && !c.relay)
  if (httpsNonRelay) return httpsNonRelay.uri
  const httpNonRelay = conns.find(c => !c.relay)
  if (httpNonRelay) return httpNonRelay.uri
  return conns[0]?.uri ?? ''
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message
  return String(err)
}

export function PlexSignIn({ onServersAdded }: PlexSignInProps) {
  const [phase, setPhase] = useState<Phase>('idle')
  const [error, setError] = useState('')
  const [resources, setResources] = useState<PlexResource[]>([])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [token, setToken] = useState('')
  const abortRef = useRef<AbortController | null>(null)
  const popupRef = useRef<Window | null>(null)

  const cleanup = useCallback(() => {
    if (abortRef.current) {
      abortRef.current.abort()
      abortRef.current = null
    }
    if (popupRef.current && !popupRef.current.closed) {
      popupRef.current.close()
    }
    popupRef.current = null
  }, [])

  useEffect(() => cleanup, [cleanup])

  async function startAuth() {
    setError('')
    setPhase('polling')
    try {
      const pin = await requestPin()
      const clientId = getClientId()
      const authUrl = getAuthUrl(clientId, pin.code)
      popupRef.current = window.open(authUrl, 'plexAuth', 'width=800,height=700')

      abortRef.current = new AbortController()
      pollForToken(pin.id, abortRef.current.signal)
    } catch {
      setError('Failed to start Plex sign-in. Please try again.')
      setPhase('idle')
    }
  }

  async function pollForToken(pinId: number, signal: AbortSignal) {
    const timeout = Date.now() + 5 * 60 * 1000
    while (!signal.aborted && Date.now() < timeout) {
      await new Promise(r => setTimeout(r, 1500))
      if (signal.aborted) return

      if (popupRef.current?.closed) {
        setError('Sign-in window was closed. Please try again.')
        setPhase('idle')
        return
      }

      try {
        const result = await checkPin(pinId)
        if (result.authToken) {
          if (signal.aborted) return
          if (popupRef.current && !popupRef.current.closed) {
            popupRef.current.close()
          }
          setToken(result.authToken)
          await loadResources(result.authToken)
          return
        }
      } catch {
        // continue polling
      }
    }

    if (!signal.aborted) {
      setError('Sign-in timed out. Please try again.')
      setPhase('idle')
    }
  }

  async function loadResources(authToken: string) {
    try {
      const servers = await fetchResources(authToken)
      setResources(servers)
      setSelected(new Set(servers.map(s => s.clientIdentifier)))
      setPhase('selecting')
    } catch {
      setError('Failed to fetch server list.')
      setPhase('idle')
    }
  }

  function toggleServer(id: string) {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function addSelected() {
    setPhase('adding')
    setError('')
    const toAdd = resources.filter(r => selected.has(r.clientIdentifier))
    try {
      for (const r of toAdd) {
        const url = pickConnection(r)
        await api.post('/api/servers', {
          name: r.name,
          type: 'plex',
          url,
          api_key: token || r.accessToken,
          enabled: true,
        })
      }
      onServersAdded()
      setPhase('idle')
      setResources([])
      setSelected(new Set())
    } catch (err) {
      setError(errorMessage(err))
      setPhase('selecting')
    }
  }

  function cancel() {
    cleanup()
    setPhase('idle')
    setResources([])
    setSelected(new Set())
    setError('')
  }

  if (phase === 'idle' && resources.length === 0) {
    return (
      <div>
        <button
          onClick={startAuth}
          className="px-4 py-2.5 text-sm font-semibold rounded-lg
                     bg-[#e5a00d] text-gray-900 hover:bg-[#cc8e0b] transition-colors"
        >
          Sign in to Plex
        </button>
        {error && (
          <p className="text-sm text-red-500 dark:text-red-400 mt-2">{error}</p>
        )}
      </div>
    )
  }

  if (phase === 'polling') {
    return (
      <div className="card p-5">
        <p className="text-sm">Waiting for Plex authorization...</p>
        <p className="text-xs text-muted dark:text-muted-dark mt-1">
          Complete sign-in in the popup window.
        </p>
        <button onClick={cancel} className="text-sm text-accent hover:underline mt-3">
          Cancel
        </button>
        {error && (
          <p className="text-sm text-red-500 dark:text-red-400 mt-2">{error}</p>
        )}
      </div>
    )
  }

  return (
    <div className="card p-5">
      <h3 className="font-semibold text-base mb-3">Select Plex Servers</h3>

      {resources.length === 0 && (
        <p className="text-sm text-muted dark:text-muted-dark">No servers found on this account.</p>
      )}

      <div className="space-y-2 mb-4">
        {resources.map(r => {
          const url = pickConnection(r)
          return (
            <label key={r.clientIdentifier} className="flex items-start gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={selected.has(r.clientIdentifier)}
                onChange={() => toggleServer(r.clientIdentifier)}
                className="w-4 h-4 mt-0.5 rounded border-border dark:border-border-dark accent-accent cursor-pointer"
              />
              <div className="min-w-0">
                <span className="text-sm font-medium">{r.name}</span>
                <span className="block text-xs text-muted dark:text-muted-dark font-mono truncate">{url}</span>
              </div>
            </label>
          )
        })}
      </div>

      {error && (
        <p className="text-sm text-red-500 dark:text-red-400 mb-3">{error}</p>
      )}

      <div className="flex gap-2">
        <button
          onClick={addSelected}
          disabled={selected.size === 0 || phase === 'adding'}
          className="px-4 py-2 text-sm font-semibold rounded-lg
                     bg-accent text-gray-900 hover:bg-accent/90 disabled:opacity-50 transition-colors"
        >
          {phase === 'adding' ? 'Adding...' : 'Add Selected'}
        </button>
        <button
          onClick={cancel}
          className="px-4 py-2 text-sm font-medium rounded-lg
                     border border-border dark:border-border-dark hover:border-accent/30 transition-colors"
        >
          Cancel
        </button>
      </div>
    </div>
  )
}
