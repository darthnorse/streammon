import { useState, useEffect, useRef, useCallback } from 'react'
import type { TautulliSettings, TautulliImportResult, Server } from '../types'
import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'

interface TautulliFormProps {
  settings?: TautulliSettings
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

interface ImportProgress {
  type: 'progress' | 'complete' | 'error'
  processed: number
  total: number
  inserted: number
  skipped: number
  error?: string
}

const inputClass = `w-full px-3 py-2.5 rounded-lg text-sm font-mono
  bg-surface dark:bg-surface-dark
  border border-border dark:border-border-dark
  focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20
  transition-colors placeholder:text-muted/40 dark:placeholder:text-muted-dark/40`

const selectClass = `w-full px-3 py-2.5 rounded-lg text-sm
  bg-surface dark:bg-surface-dark
  border border-border dark:border-border-dark
  focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20
  transition-colors`

export function TautulliForm({ settings, onClose, onSaved }: TautulliFormProps) {
  const isEdit = !!settings?.url
  const modalRef = useRef<HTMLDivElement>(null)
  const { data: allServers } = useFetch<Server[]>('/api/servers')
  const servers = allServers?.filter(s => s.type === 'plex')

  const [form, setForm] = useState<FormData>({
    url: settings?.url ?? '',
    api_key: isEdit ? settings?.api_key ?? '' : '',
  })
  const [selectedServer, setSelectedServer] = useState<number>(0)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [testResult, setTestResult] = useState<TestResult | null>(null)
  const [testing, setTesting] = useState(false)
  const [importing, setImporting] = useState(false)
  const [importResult, setImportResult] = useState<TautulliImportResult | null>(null)
  const [importProgress, setImportProgress] = useState<ImportProgress | null>(null)
  const [justSaved, setJustSaved] = useState(false)
  const busy = saving || testing || importing
  const showImport = isEdit || justSaved

  const handleClose = useCallback(() => {
    if (justSaved) onSaved()
    onClose()
  }, [justSaved, onSaved, onClose])

  const handleEscape = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') handleClose()
  }, [handleClose])

  useEffect(() => {
    document.addEventListener('keydown', handleEscape)
    const previouslyFocused = document.activeElement as HTMLElement | null
    modalRef.current?.querySelector<HTMLElement>('input, button')?.focus()
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

  useEffect(() => {
    if (servers && servers.length > 0 && selectedServer === 0) {
      setSelectedServer(servers[0].id)
    }
  }, [servers, selectedServer])

  function setField<K extends keyof FormData>(key: K, value: FormData[K]) {
    setForm(prev => ({ ...prev, [key]: value }))
    setError('')
    setTestResult(null)
    setImportResult(null)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.url.trim()) {
      setError('Tautulli URL is required')
      return
    }
    if (!form.api_key.trim() && !isEdit) {
      setError('API Key is required')
      return
    }

    setSaving(true)
    setError('')
    try {
      await api.put('/api/settings/tautulli', {
        url: form.url.trim(),
        api_key: form.api_key,
      })
      setJustSaved(true)
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
        setTestResult({ success: false, error: 'Tautulli URL is required' })
        return
      }
      const result = await api.post<TestResult>('/api/settings/tautulli/test', {
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

  async function handleImport() {
    if (!selectedServer) {
      setError('Please select a server')
      return
    }
    setImporting(true)
    setImportResult(null)
    setImportProgress(null)
    setError('')

    try {
      const response = await fetch('/api/settings/tautulli/import', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: selectedServer }),
      })

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      const reader = response.body?.getReader()
      if (!reader) {
        throw new Error('Streaming not supported')
      }

      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = JSON.parse(line.slice(6)) as ImportProgress
            setImportProgress(data)

            if (data.type === 'complete') {
              setImportResult({
                imported: data.inserted,
                skipped: data.skipped,
                total: data.total,
              })
            } else if (data.type === 'error') {
              setError(data.error || 'Import failed')
              setImportResult({
                imported: data.inserted,
                skipped: data.skipped,
                total: data.total,
                error: data.error,
              })
            }
          }
        }
      }
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setImporting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={e => { if (e.target === e.currentTarget) handleClose() }}
      role="dialog"
      aria-modal="true"
      aria-label={isEdit ? 'Edit Tautulli Settings' : 'Configure Tautulli'}
    >
      <div
        ref={modalRef}
        className="card w-full max-w-lg max-h-[90vh] overflow-y-auto p-0
                      lg:max-w-xl animate-in"
      >
        <div className="flex items-center justify-between px-6 py-4
                        border-b border-border dark:border-border-dark">
          <h2 className="text-lg font-semibold">
            {isEdit ? 'Edit Tautulli Settings' : 'Configure Tautulli'}
          </h2>
          <button
            onClick={handleClose}
            aria-label="Close"
            className="text-muted dark:text-muted-dark hover:text-gray-800
                       dark:hover:text-gray-100 transition-colors text-xl leading-none"
          >
            &times;
          </button>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
          <div>
            <label htmlFor="tautulli-url" className="block text-sm font-medium mb-1.5">Tautulli URL</label>
            <input
              id="tautulli-url"
              type="text"
              value={form.url}
              onChange={e => setField('url', e.target.value)}
              placeholder="http://localhost:8181"
              className={inputClass}
            />
          </div>

          <div>
            <label htmlFor="tautulli-api-key" className="block text-sm font-medium mb-1.5">API Key</label>
            <input
              id="tautulli-api-key"
              type="password"
              value={form.api_key}
              onChange={e => setField('api_key', e.target.value)}
              placeholder={isEdit ? '(unchanged)' : 'Enter API key'}
              className={inputClass}
            />
            <p className="text-xs text-muted dark:text-muted-dark mt-1">
              Found in Tautulli Settings &rarr; Web Interface &rarr; API Key
            </p>
          </div>

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
              onClick={handleClose}
              aria-label="Cancel"
              className="px-4 py-2.5 text-sm font-medium rounded-lg
                         border border-border dark:border-border-dark
                         hover:border-accent/30 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={busy || justSaved}
              className="px-5 py-2.5 text-sm font-semibold rounded-lg
                         bg-accent text-gray-900 hover:bg-accent/90
                         disabled:opacity-50 transition-colors"
            >
              {saving ? 'Saving...' : justSaved ? 'Saved' : 'Save'}
            </button>
          </div>

          {showImport && (
            <>
              <div className="border-t border-border dark:border-border-dark pt-4 mt-4">
                <h3 className="text-sm font-semibold mb-3">Import History</h3>
                {!servers?.length ? (
                  <p className="text-sm text-muted dark:text-muted-dark">
                    No Plex servers configured. Add a Plex server in the Servers tab before importing.
                  </p>
                ) : (
                  <div className="flex gap-3 items-end">
                    <div className="flex-1">
                      <label htmlFor="import-server" className="block text-sm font-medium mb-1.5">
                        Associate with Server
                      </label>
                      <select
                        id="import-server"
                        value={selectedServer}
                        onChange={e => setSelectedServer(Number(e.target.value))}
                        className={selectClass}
                      >
                        {servers.map(srv => (
                          <option key={srv.id} value={srv.id}>
                            {srv.name}
                          </option>
                        ))}
                      </select>
                    </div>
                    <button
                      type="button"
                      onClick={handleImport}
                      disabled={busy || !selectedServer}
                      className="px-4 py-2.5 text-sm font-medium rounded-lg
                                 bg-accent text-gray-900 hover:bg-accent/90
                                 disabled:opacity-50 transition-colors"
                    >
                      {importing ? 'Importing...' : 'Import Now'}
                    </button>
                  </div>
                )}

                {importing && importProgress && importProgress.total > 0 && (
                  <div className="mt-3 space-y-2">
                    <div className="flex justify-between text-xs text-muted dark:text-muted-dark">
                      <span>Enriching stream details...</span>
                      <span>{importProgress.processed} / {importProgress.total}</span>
                    </div>
                    <div className="w-full bg-surface dark:bg-surface-dark rounded-full h-2 overflow-hidden">
                      <div
                        className="bg-accent h-2 rounded-full transition-all duration-300"
                        style={{ width: `${Math.round((importProgress.processed / importProgress.total) * 100)}%` }}
                      />
                    </div>
                  </div>
                )}
              </div>

              {importResult && !importResult.error && (
                <div className="text-sm font-mono px-3 py-2 rounded-lg bg-green-500/10 text-green-600 dark:text-green-400">
                  Imported {importResult.imported.toLocaleString()} records
                  {importResult.skipped > 0 && ` (skipped ${importResult.skipped.toLocaleString()} duplicates)`}
                </div>
              )}
            </>
          )}
        </form>
      </div>
    </div>
  )
}
