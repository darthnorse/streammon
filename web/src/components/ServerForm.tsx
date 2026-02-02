import { useState, useEffect, useRef, useCallback } from 'react'
import type { Server, ServerType } from '../types'
import { api } from '../lib/api'

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
  enabled: boolean
}

interface TestResult {
  success: boolean
  error?: string
}

const serverTypes: { value: ServerType; label: string }[] = [
  { value: 'plex', label: 'Plex' },
  { value: 'emby', label: 'Emby' },
  { value: 'jellyfin', label: 'Jellyfin' },
]

function isValidServerType(value: string): value is ServerType {
  return serverTypes.some(t => t.value === value)
}

const inputClass = `w-full px-3 py-2.5 rounded-lg text-sm font-mono
  bg-surface dark:bg-surface-dark
  border border-border dark:border-border-dark
  focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20
  transition-colors placeholder:text-muted/40 dark:placeholder:text-muted-dark/40`

export function ServerForm({ server, onClose, onSaved }: ServerFormProps) {
  const isEdit = !!server
  const modalRef = useRef<HTMLDivElement>(null)
  const [form, setForm] = useState<FormData>({
    name: server?.name ?? '',
    type: server?.type ?? 'plex',
    url: server?.url ?? '',
    api_key: '',
    enabled: server?.enabled ?? true,
  })
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [testResult, setTestResult] = useState<TestResult | null>(null)
  const [testing, setTesting] = useState(false)
  const busy = saving || testing

  const handleEscape = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') onClose()
  }, [onClose])

  useEffect(() => {
    document.addEventListener('keydown', handleEscape)
    const previouslyFocused = document.activeElement as HTMLElement | null
    modalRef.current?.querySelector<HTMLElement>('input, select, button')?.focus()
    return () => {
      document.removeEventListener('keydown', handleEscape)
      previouslyFocused?.focus()
    }
  }, [handleEscape])

  useEffect(() => {
    const modal = modalRef.current
    if (!modal) return
    function trapFocus(e: KeyboardEvent) {
      if (e.key !== 'Tab') return
      const focusable = modal!.querySelectorAll<HTMLElement>(
        'input, select, button, textarea, [tabindex]:not([tabindex="-1"])'
      )
      if (focusable.length === 0) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', trapFocus)
    return () => document.removeEventListener('keydown', trapFocus)
  }, [])

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
      enabled: form.enabled,
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
                      lg:max-w-xl animate-in"
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
              className={inputClass}
            />
          </div>

          <div>
            <label htmlFor="srv-type" className="block text-sm font-medium mb-1.5">Type</label>
            <select
              id="srv-type"
              value={form.type}
              onChange={e => handleTypeChange(e.target.value)}
              className={inputClass}
            >
              {serverTypes.map(t => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="srv-url" className="block text-sm font-medium mb-1.5">URL</label>
            <input
              id="srv-url"
              type="text"
              value={form.url}
              onChange={e => setField('url', e.target.value)}
              placeholder="http://192.168.1.100:32400"
              className={inputClass}
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
              className={inputClass}
            />
          </div>

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
