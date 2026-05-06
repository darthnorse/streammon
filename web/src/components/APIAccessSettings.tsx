import { useState } from 'react'
import { api } from '../lib/api'
import { btnOutline, btnDanger } from '../lib/constants'
import { useFetch } from '../hooks/useFetch'

interface APIKeyStatus {
  configured: boolean
  created_at?: string
}

interface RotateResponse {
  key: string
  created_at: string
}

export function APIAccessSettings() {
  const { data: status, loading, error: fetchError, refetch } = useFetch<APIKeyStatus>('/api/admin/api-key')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [revealed, setRevealed] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const verb = status?.configured ? 'Rotate' : 'Generate'

  async function rotate() {
    if (!window.confirm(
      status?.configured
        ? 'Rotating will immediately invalidate the existing API key. Any clients using it will stop working until you give them the new one. Continue?'
        : 'Generate a new API key? It grants full admin-level API access — store it securely.',
    )) return
    setBusy(true)
    setError('')
    try {
      const res = await api.post<RotateResponse>('/api/admin/api-key/rotate')
      setRevealed(res.key)
      setCopied(false)
      refetch()
    } catch (err) {
      setError(`${verb} failed: ${(err as Error).message}`)
    } finally {
      setBusy(false)
    }
  }

  async function revoke() {
    if (!window.confirm('Revoke the API key? Any clients using it will immediately stop working.')) return
    setBusy(true)
    setError('')
    try {
      await api.del('/api/admin/api-key')
      setRevealed(null)
      refetch()
    } catch (err) {
      setError(`Revoke failed: ${(err as Error).message}`)
    } finally {
      setBusy(false)
    }
  }

  async function copyKey(value: string) {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
    } catch {
      // Clipboard may be unavailable in non-HTTPS contexts; the user can still copy manually.
    }
  }

  return (
    <div className="card p-5 space-y-4">
      <div>
        <h3 className="font-semibold text-base">API Key</h3>
        <p className="text-sm text-muted dark:text-muted-dark mt-1">
          Grants full admin-level access to all StreamMon APIs. Use the <code className="font-mono text-xs px-1 py-0.5 rounded bg-gray-100 dark:bg-white/10">X-API-Key</code> request header.
        </p>
      </div>

      <div className="text-sm">
        {loading && <div className="text-muted dark:text-muted-dark">Loading…</div>}
        {!loading && fetchError && (
          <div className="text-red-600 dark:text-red-400">
            Failed to load API key status: {fetchError.message}
          </div>
        )}
        {!loading && !fetchError && status?.configured && (
          <div className="flex items-center gap-2 text-green-700 dark:text-green-400">
            <span aria-hidden="true">●</span>
            <span>
              Active{status.created_at ? ` (created ${new Date(status.created_at).toLocaleString()})` : ''}
            </span>
          </div>
        )}
        {!loading && !fetchError && status && !status.configured && (
          <div className="text-muted dark:text-muted-dark">No key configured.</div>
        )}
      </div>

      {revealed && (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 space-y-2">
          <div className="text-sm font-medium text-amber-700 dark:text-amber-300">
            Copy this key now — it will not be shown again.
          </div>
          <div className="flex items-center gap-2">
            <code className="flex-1 font-mono text-xs break-all px-2 py-1.5 rounded bg-white/60 dark:bg-black/40">
              {revealed}
            </code>
            <button onClick={() => copyKey(revealed)} disabled={copied} className={btnOutline} type="button">
              {copied ? 'Copied' : 'Copy'}
            </button>
          </div>
          <button
            onClick={() => { setRevealed(null); setCopied(false) }}
            className="text-xs text-muted dark:text-muted-dark hover:text-accent hover:underline"
            type="button"
          >
            Done
          </button>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={rotate}
          disabled={busy || loading || !!fetchError || !status}
          className={btnOutline}
          type="button"
        >
          {verb} Key
        </button>
        {status?.configured && (
          <button onClick={revoke} disabled={busy} className={btnDanger} type="button">
            Revoke
          </button>
        )}
      </div>

      {error && <div className="text-sm text-red-600 dark:text-red-400">{error}</div>}

      <div className="text-xs text-muted dark:text-muted-dark">
        Example: <code className="font-mono">curl -H 'X-API-Key: sm_…' https://your-host/api/me</code>
      </div>
    </div>
  )
}
