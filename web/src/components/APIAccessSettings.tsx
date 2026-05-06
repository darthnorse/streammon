import { useState } from 'react'
import { api } from '../lib/api'
import { btnOutline, btnDanger } from '../lib/constants'
import { useFetch } from '../hooks/useFetch'
import { ConfirmDialog } from './shared/ConfirmDialog'

interface APIKeyStatus {
  configured: boolean
  key?: string
  created_at?: string
}

interface RotateResponse {
  key: string
  created_at: string
}

type ConfirmAction = 'rotate' | 'revoke'

const MASK = '••••••••••••••••••••••••••'

function EyeIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.8}
        d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.8}
        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  )
}

function EyeSlashIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.8}
        d="M3.98 8.223A10.477 10.477 0 001.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.45 10.45 0 0112 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 01-4.293 5.774M6.228 6.228L3 3m3.228 3.228l3.65 3.65m7.894 7.894L21 21m-3.228-3.228l-3.65-3.65m0 0a3 3 0 10-4.243-4.243m4.242 4.242L9.88 9.88" />
    </svg>
  )
}

export function APIAccessSettings() {
  const { data: status, loading, error: fetchError, refetch } = useFetch<APIKeyStatus>('/api/admin/api-key')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [revealed, setRevealed] = useState(false)
  const [copied, setCopied] = useState(false)
  const [pending, setPending] = useState<ConfirmAction | null>(null)

  const verb = status?.configured ? 'Rotate' : 'Generate'

  async function doRotate() {
    setPending(null)
    setBusy(true)
    setError('')
    try {
      await api.post<RotateResponse>('/api/admin/api-key/rotate')
      setRevealed(true)
      setCopied(false)
      refetch()
    } catch (err) {
      setError(`${verb} failed: ${(err as Error).message}`)
    } finally {
      setBusy(false)
    }
  }

  async function doRevoke() {
    setPending(null)
    setBusy(true)
    setError('')
    try {
      await api.del('/api/admin/api-key')
      setRevealed(false)
      refetch()
    } catch (err) {
      setError(`Revoke failed: ${(err as Error).message}`)
    } finally {
      setBusy(false)
    }
  }

  async function copyKey() {
    if (!status?.key) return
    try {
      await navigator.clipboard.writeText(status.key)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard may be unavailable in non-HTTPS contexts; user can still copy manually.
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

      {loading && <div className="text-sm text-muted dark:text-muted-dark">Loading…</div>}
      {!loading && fetchError && (
        <div className="text-sm text-red-600 dark:text-red-400">
          Failed to load API key status: {fetchError.message}
        </div>
      )}

      {!loading && !fetchError && status && status.configured && status.key && (
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <code className="flex-1 font-mono text-sm break-all px-3 py-2 rounded bg-gray-50 dark:bg-white/5 border border-border dark:border-border-dark">
              {revealed ? status.key : MASK}
            </code>
            <button
              onClick={() => setRevealed(v => !v)}
              className={btnOutline}
              type="button"
              aria-label={revealed ? 'Hide API key' : 'Show API key'}
              title={revealed ? 'Hide' : 'Show'}
            >
              {revealed ? <EyeSlashIcon /> : <EyeIcon />}
            </button>
            <button onClick={copyKey} className={btnOutline} type="button">
              {copied ? 'Copied' : 'Copy'}
            </button>
          </div>
          {status.created_at && (
            <div className="text-xs text-muted dark:text-muted-dark">
              Created {new Date(status.created_at).toLocaleString()}
            </div>
          )}
        </div>
      )}

      {!loading && !fetchError && status && !status.configured && (
        <div className="text-sm text-muted dark:text-muted-dark">No key configured.</div>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={() => setPending('rotate')}
          disabled={busy || loading || !!fetchError || !status}
          className={btnOutline}
          type="button"
        >
          {verb} Key
        </button>
        {status?.configured && (
          <button onClick={() => setPending('revoke')} disabled={busy} className={btnDanger} type="button">
            Revoke
          </button>
        )}
      </div>

      {error && <div className="text-sm text-red-600 dark:text-red-400">{error}</div>}

      <div className="text-xs text-muted dark:text-muted-dark">
        Example: <code className="font-mono">curl -H 'X-API-Key: sm_…' https://your-host/api/me</code>
      </div>

      {pending === 'rotate' && (
        <ConfirmDialog
          title={status?.configured ? 'Rotate API key?' : 'Generate API key?'}
          message={status?.configured
            ? 'Rotating will immediately invalidate the existing key. Any clients using it will stop working until you give them the new one.'
            : 'A new API key grants full admin-level access. Store it securely.'}
          confirmLabel={verb}
          onConfirm={doRotate}
          onCancel={() => setPending(null)}
          isDestructive={status?.configured}
          disabled={busy}
        />
      )}

      {pending === 'revoke' && (
        <ConfirmDialog
          title="Revoke API key?"
          message="Any clients using this key will immediately stop working. This cannot be undone."
          confirmLabel="Revoke"
          onConfirm={doRevoke}
          onCancel={() => setPending(null)}
          isDestructive
          disabled={busy}
        />
      )}
    </div>
  )
}
