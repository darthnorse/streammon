import { useState } from 'react'
import type { Server, ServerType } from '../types'
import { api } from '../lib/api'
import { formInputClass } from '../lib/constants'
import { useModal } from '../hooks/useModal'
import { PlexSignIn } from './PlexSignIn'

interface ServerFormProps {
  server?: Server
  onClose: () => void
  onSaved: () => void
}

interface FormData {
  name: string
  type: ServerType
  url: string
  api_key: string
  machine_id: string
  enabled: boolean
  show_recent_media: boolean
}

interface TestResult {
  success: boolean
  error?: string
  machine_id?: string
}

const serverTypes: { value: ServerType; label: string }[] = [
  { value: 'plex', label: 'Plex' },
  { value: 'emby', label: 'Emby' },
  { value: 'jellyfin', label: 'Jellyfin' },
]

function isValidServerType(value: string): value is ServerType {
  return serverTypes.some(t => t.value === value)
}

export function ServerForm({ server, onClose, onSaved }: ServerFormProps) {
  const isEdit = !!server
  const modalRef = useModal(onClose)
  const [form, setForm] = useState<FormData>({
    name: server?.name ?? '',
    type: server?.type ?? 'plex',
    url: server?.url ?? '',
    api_key: '',
    machine_id: server?.machine_id ?? '',
    enabled: server?.enabled ?? true,
    show_recent_media: server?.show_recent_media ?? false,
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

  function handleTypeChange(value: string) {
    if (isValidServerType(value)) {
      setField('type', value)
    }
  }

  function buildPayload() {
    return {
      name: form.name.trim(),
      type: form.type,
      url: form.url.trim(),
      api_key: form.api_key,
      machine_id: form.machine_id,
      enabled: form.enabled,
      show_recent_media: form.show_recent_media,
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) {
      setError('Name is required')
      return
    }
    if (!form.url.trim()) {
      setError('URL is required')
      return
    }
    if (!form.api_key.trim() && !isEdit) {
      setError('API key is required')
      return
    }
    // Require machine_id for all Plex servers (security: prevents name-spoofing auth bypass)
    if (form.type === 'plex' && !form.machine_id) {
      setError('Machine ID is required — click "Test Connection" to populate it')
      return
    }

    setSaving(true)
    setError('')
    try {
      if (isEdit) {
        await api.put(`/api/servers/${server.id}`, buildPayload())
      } else {
        await api.post('/api/servers', buildPayload())
      }
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
      let result: TestResult
      if (isEdit) {
        result = await api.post<TestResult>(`/api/servers/${server.id}/test`)
      } else {
        if (!form.name.trim() || !form.url.trim() || !form.api_key.trim()) {
          setTestResult({ success: false, error: 'Fill in all fields before testing' })
          return
        }
        result = await api.post<TestResult>('/api/servers/test', buildPayload())
      }
      setTestResult(result)
      // Auto-populate machine_id from successful Plex test
      // Update form directly to avoid setField clearing testResult
      if (result.success && result.machine_id && form.type === 'plex') {
        setForm(prev => ({ ...prev, machine_id: result.machine_id! }))
      }
    } catch (err) {
      setTestResult({ success: false, error: (err as Error).message })
    } finally {
      setTesting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
      role="dialog"
      aria-modal="true"
      aria-label={isEdit ? 'Edit Server' : 'New Server'}
    >
      <div
        ref={modalRef}
        className="card w-full max-w-lg max-h-[90vh] overflow-y-auto p-0
                      lg:max-w-xl animate-slide-up"
      >
        <div className="flex items-center justify-between px-6 py-4
                        border-b border-border dark:border-border-dark">
          <h2 className="text-lg font-semibold">
            {isEdit ? 'Edit Server' : 'New Server'}
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
            <label htmlFor="srv-name" className="block text-sm font-medium mb-1.5">Name</label>
            <input
              id="srv-name"
              type="text"
              value={form.name}
              onChange={e => setField('name', e.target.value)}
              placeholder="My Plex Server"
              className={formInputClass}
            />
          </div>

          <div>
            <label htmlFor="srv-type" className="block text-sm font-medium mb-1.5">Type</label>
            <select
              id="srv-type"
              value={form.type}
              onChange={e => handleTypeChange(e.target.value)}
              className={formInputClass}
            >
              {serverTypes.map(t => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
          </div>

          {!isEdit && form.type === 'plex' && (
            <div className="border border-border dark:border-border-dark rounded-lg p-4">
              <PlexSignIn onServersAdded={onSaved} />
              <p className="text-xs text-muted dark:text-muted-dark mt-2">
                Or fill in the fields below manually.
              </p>
            </div>
          )}

          <div>
            <label htmlFor="srv-url" className="block text-sm font-medium mb-1.5">URL</label>
            <input
              id="srv-url"
              type="text"
              value={form.url}
              onChange={e => setField('url', e.target.value)}
              placeholder="http://192.168.1.100:32400"
              className={formInputClass}
            />
          </div>

          <div>
            <label htmlFor="srv-key" className="block text-sm font-medium mb-1.5">API Key</label>
            <input
              id="srv-key"
              type="password"
              value={form.api_key}
              onChange={e => setField('api_key', e.target.value)}
              placeholder={isEdit ? '(unchanged)' : 'Enter API key'}
              className={formInputClass}
            />
          </div>

          {form.type === 'plex' && (
            <div>
              <label htmlFor="srv-machine-id" className="block text-sm font-medium mb-1.5">
                Machine ID
                <span className="text-red-500 ml-0.5">*</span>
                {!form.machine_id && isEdit && (
                  <span className="ml-2 text-xs text-amber-500 dark:text-amber-400">
                    (missing — test connection to populate)
                  </span>
                )}
              </label>
              <input
                id="srv-machine-id"
                type="text"
                value={form.machine_id}
                readOnly
                placeholder="Click 'Test Connection' to populate"
                className={`${formInputClass} ${!form.machine_id ? 'border-amber-400 dark:border-amber-500' : ''} bg-gray-100 dark:bg-gray-800 cursor-not-allowed`}
              />
              {!form.machine_id && (
                <p className="text-xs text-amber-600 dark:text-amber-400 mt-1">
                  Required for secure authentication. Click "Test Connection" below to populate.
                </p>
              )}
            </div>
          )}

          <label className="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              checked={form.enabled}
              onChange={e => setField('enabled', e.target.checked)}
              className="w-4 h-4 rounded border-border dark:border-border-dark
                         accent-accent cursor-pointer"
            />
            <span className="text-sm font-medium">Enabled</span>
          </label>

          <label className="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              checked={form.show_recent_media}
              onChange={e => setField('show_recent_media', e.target.checked)}
              className="w-4 h-4 rounded border-border dark:border-border-dark
                         accent-accent cursor-pointer"
            />
            <span className="text-sm font-medium">Show recent media on dashboard</span>
          </label>

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
              {saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
