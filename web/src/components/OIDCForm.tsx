import { useState } from 'react'
import type { OIDCSettings } from '../types'
import { api } from '../lib/api'
import { formInputClass } from '../lib/constants'
import { useModal } from '../hooks/useModal'

interface OIDCFormProps {
  settings?: OIDCSettings
  onClose: () => void
  onSaved: () => void
}

interface FormData {
  issuer: string
  client_id: string
  client_secret: string
  redirect_url: string
}

interface TestResult {
  success: boolean
  error?: string
}

export function OIDCForm({ settings, onClose, onSaved }: OIDCFormProps) {
  const isEdit = !!settings?.issuer
  const modalRef = useModal(onClose)
  const [form, setForm] = useState<FormData>({
    issuer: settings?.issuer ?? '',
    client_id: settings?.client_id ?? '',
    client_secret: isEdit ? settings?.client_secret ?? '' : '',
    redirect_url: settings?.redirect_url ?? '',
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
    if (!form.issuer.trim()) {
      setError('Issuer URL is required')
      return
    }
    if (!form.client_id.trim()) {
      setError('Client ID is required')
      return
    }
    if (!form.client_secret.trim() && !isEdit) {
      setError('Client Secret is required')
      return
    }
    if (!form.redirect_url.trim()) {
      setError('Redirect URL is required')
      return
    }

    setSaving(true)
    setError('')
    try {
      const result = await api.put<{ warning?: string }>('/api/settings/oidc', {
        issuer: form.issuer.trim(),
        client_id: form.client_id.trim(),
        client_secret: form.client_secret,
        redirect_url: form.redirect_url.trim(),
      })
      if (result?.warning) {
        setError(result.warning)
        return
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
      if (!form.issuer.trim()) {
        setTestResult({ success: false, error: 'Issuer URL is required' })
        return
      }
      const result = await api.post<TestResult>('/api/settings/oidc/test', {
        issuer: form.issuer.trim(),
      })
      setTestResult(result)
    } catch (err) {
      setTestResult({ success: false, error: (err as Error).message })
    } finally {
      setTesting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
      role="dialog"
      aria-modal="true"
      aria-label={isEdit ? 'Edit OIDC Settings' : 'Configure OIDC'}
    >
      <div
        ref={modalRef}
        className="card w-full max-w-lg max-h-[90vh] overflow-y-auto p-0
                      lg:max-w-xl animate-slide-up"
      >
        <div className="flex items-center justify-between px-6 py-4
                        border-b border-border dark:border-border-dark">
          <h2 className="text-lg font-semibold">
            {isEdit ? 'Edit OIDC Settings' : 'Configure OIDC'}
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
            <label htmlFor="oidc-issuer" className="block text-sm font-medium mb-1.5">Issuer URL</label>
            <input
              id="oidc-issuer"
              type="text"
              value={form.issuer}
              onChange={e => setField('issuer', e.target.value)}
              placeholder="https://accounts.google.com"
              className={formInputClass}
            />
          </div>

          <div>
            <label htmlFor="oidc-client-id" className="block text-sm font-medium mb-1.5">Client ID</label>
            <input
              id="oidc-client-id"
              type="text"
              value={form.client_id}
              onChange={e => setField('client_id', e.target.value)}
              placeholder="your-client-id"
              className={formInputClass}
            />
          </div>

          <div>
            <label htmlFor="oidc-client-secret" className="block text-sm font-medium mb-1.5">Client Secret</label>
            <input
              id="oidc-client-secret"
              type="password"
              autoComplete="off"
              value={form.client_secret}
              onChange={e => setField('client_secret', e.target.value)}
              placeholder={isEdit ? '(unchanged)' : 'Enter client secret'}
              className={formInputClass}
            />
          </div>

          <div>
            <label htmlFor="oidc-redirect" className="block text-sm font-medium mb-1.5">Redirect URL</label>
            <input
              id="oidc-redirect"
              type="text"
              value={form.redirect_url}
              onChange={e => setField('redirect_url', e.target.value)}
              placeholder="https://streammon.example.com/auth/callback"
              className={formInputClass}
            />
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
