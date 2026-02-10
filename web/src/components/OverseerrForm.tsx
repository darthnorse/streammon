import { useState } from 'react'
import type { OverseerrSettings } from '../types'
import { api } from '../lib/api'
import { formInputClass } from '../lib/constants'
import { useModal } from '../hooks/useModal'

interface OverseerrFormProps {
  settings?: OverseerrSettings
  onClose: () => void
  onSaved: () => void
}

interface FormData {
  url: string
  api_key: string
}

interface TestResult {
  success: boolean
  error?: string
}

export function OverseerrForm({ settings, onClose, onSaved }: OverseerrFormProps) {
  const isEdit = !!settings?.url
  const modalRef = useModal(onClose)

  const [form, setForm] = useState<FormData>({
    url: settings?.url ?? '',
    api_key: isEdit ? settings?.api_key ?? '' : '',
  })
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [testResult, setTestResult] = useState<TestResult | null>(null)
  const [testing, setTesting] = useState(false)
  const busy = saving || testing

  function setField<K extends keyof FormData>(key: K, value: FormData[K]) {
    setForm(prev => ({ ...prev, [key]: value }))
    setError('')
    setTestResult(null)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.url.trim()) {
      setError('Overseerr URL is required')
      return
    }
    if (!form.api_key.trim() && !isEdit) {
      setError('API Key is required')
      return
    }

    setSaving(true)
    setError('')
    try {
      await api.put('/api/settings/overseerr', {
        url: form.url.trim(),
        api_key: form.api_key,
      })
      onClose()
      onSaved()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSaving(false)
    }
  }

  async function handleTest() {
    setTesting(true)
    setTestResult(null)
    try {
      if (!form.url.trim()) {
        setTestResult({ success: false, error: 'Overseerr URL is required' })
        return
      }
      const result = await api.post<TestResult>('/api/settings/overseerr/test', {
        url: form.url.trim(),
        api_key: form.api_key,
      })
      setTestResult(result)
    } catch (err) {
      setTestResult({ success: false, error: (err as Error).message })
    } finally {
      setTesting(false)
    }
  }

  const saveLabel = saving ? 'Saving...' : 'Save'

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
      role="dialog"
      aria-modal="true"
      aria-label={isEdit ? 'Edit Overseerr Settings' : 'Configure Overseerr'}
    >
      <div
        ref={modalRef}
        className="card w-full max-w-lg max-h-[90vh] overflow-y-auto p-0
                      lg:max-w-xl animate-slide-up"
      >
        <div className="flex items-center justify-between px-6 py-4
                        border-b border-border dark:border-border-dark">
          <h2 className="text-lg font-semibold">
            {isEdit ? 'Edit Overseerr Settings' : 'Configure Overseerr'}
          </h2>
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-muted dark:text-muted-dark hover:text-gray-800
                       dark:hover:text-gray-100 transition-colors text-xl leading-none"
          >
            &times;
          </button>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
          <div>
            <label htmlFor="overseerr-url" className="block text-sm font-medium mb-1.5">Overseerr URL</label>
            <input
              id="overseerr-url"
              type="text"
              value={form.url}
              onChange={e => setField('url', e.target.value)}
              placeholder="http://localhost:5055"
              className={formInputClass}
            />
          </div>

          {form.url.startsWith('http://') &&
            !form.url.startsWith('http://localhost') &&
            !form.url.startsWith('http://127.0.0.1') && (
            <p className="text-xs text-amber-600 dark:text-amber-400 bg-amber-500/10 rounded-lg px-3 py-2">
              Plex token attribution requires HTTPS (or localhost). With a plain HTTP URL, Overseerr requests will fall back to email matching to avoid sending tokens over an unencrypted connection. If email match is not possible, requests will fall back to the Overseerr admin account.
            </p>
          )}

          <div>
            <label htmlFor="overseerr-api-key" className="block text-sm font-medium mb-1.5">API Key</label>
            <input
              id="overseerr-api-key"
              type="password"
              value={form.api_key}
              onChange={e => setField('api_key', e.target.value)}
              placeholder={isEdit ? '(unchanged)' : 'Enter API key'}
              className={formInputClass}
            />
            <p className="text-xs text-muted dark:text-muted-dark mt-1">
              Found in Overseerr Settings &rarr; General &rarr; API Key
            </p>
          </div>

          <p className="text-xs text-amber-600 dark:text-amber-400 bg-amber-500/10 rounded-lg px-3 py-2">
            If you use per-user tagging in Radarr/Sonarr, disable &ldquo;Tag Requests&rdquo; in Overseerr Settings &rarr; Services &rarr; Radarr/Sonarr.
            A <a href="https://github.com/sct/overseerr/issues/4306" target="_blank" rel="noopener noreferrer" className="underline hover:text-amber-800 dark:hover:text-amber-300">known Overseerr bug</a> causes
            tag creation to fail with newer Radarr/Sonarr versions.
          </p>

          {error && (
            <div className="text-sm text-red-500 dark:text-red-400 font-mono px-1">
              {error}
            </div>
          )}

          {testResult && (
            <div className={`text-sm font-mono px-3 py-2 rounded-lg ${
              testResult.success
                ? 'bg-green-500/10 text-green-600 dark:text-green-400'
                : 'bg-red-500/10 text-red-500 dark:text-red-400'
            }`}>
              {testResult.success ? 'Connection successful' : testResult.error}
            </div>
          )}

          <div className="flex flex-col-reverse sm:flex-row items-stretch sm:items-center
                          gap-3 pt-2">
            <button
              type="button"
              onClick={handleTest}
              disabled={busy}
              className="px-4 py-2.5 text-sm font-medium rounded-lg
                         border border-border dark:border-border-dark
                         hover:border-accent/30 transition-colors
                         disabled:opacity-50"
            >
              {testing ? 'Testing...' : 'Test Connection'}
            </button>
            <div className="flex-1" />
            <button
              type="button"
              onClick={onClose}
              aria-label="Cancel"
              className="px-4 py-2.5 text-sm font-medium rounded-lg
                         border border-border dark:border-border-dark
                         hover:border-accent/30 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={busy}
              className="px-5 py-2.5 text-sm font-semibold rounded-lg
                         bg-accent text-gray-900 hover:bg-accent/90
                         disabled:opacity-50 transition-colors"
            >
              {saveLabel}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
