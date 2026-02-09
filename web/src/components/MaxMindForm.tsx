import { useState } from 'react'
import { api } from '../lib/api'
import { formInputClass } from '../lib/constants'

export interface MaxMindSettings {
  license_key: string
  last_updated: string
  db_available: boolean
}

interface MaxMindFormProps {
  settings: MaxMindSettings | null
  onSaved: () => void
}

export function MaxMindForm({ settings, onSaved }: MaxMindFormProps) {
  const [key, setKey] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!key.trim()) {
      setError('License key is required')
      return
    }
    setSaving(true)
    setError('')
    try {
      await api.put('/api/settings/maxmind', { license_key: key.trim() })
      setKey('')
      onSaved()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    if (!window.confirm('Remove MaxMind license key?')) return
    try {
      await api.del('/api/settings/maxmind')
      onSaved()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <div className="card p-5">
      <h3 className="font-semibold text-base mb-4">MaxMind GeoIP</h3>

      {settings?.license_key && (
        <div className="space-y-2 text-sm mb-4">
          <div>
            <span className="text-muted dark:text-muted-dark">License Key: </span>
            <span className="font-mono">{settings.license_key}</span>
          </div>
          {settings.last_updated && (
            <div>
              <span className="text-muted dark:text-muted-dark">Last Updated: </span>
              <span className="font-mono">{new Date(settings.last_updated).toLocaleString()}</span>
            </div>
          )}
          <div>
            <span className="text-muted dark:text-muted-dark">Database: </span>
            <span className={settings.db_available ? 'text-green-600 dark:text-green-400' : 'text-red-500'}>
              {settings.db_available ? 'Available' : 'Not available'}
            </span>
          </div>
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label htmlFor="maxmind-key" className="block text-sm font-medium mb-1.5">
            {settings?.license_key ? 'Update License Key' : 'License Key'}
          </label>
          <input
            id="maxmind-key"
            type="password"
            value={key}
            onChange={e => { setKey(e.target.value); setError('') }}
            placeholder="Enter MaxMind license key"
            className={formInputClass}
          />
        </div>

        {error && (
          <div className="text-sm text-red-500 dark:text-red-400 font-mono">{error}</div>
        )}

        <div className="flex items-center gap-2">
          <button
            type="submit"
            disabled={saving}
            className="px-4 py-2.5 text-sm font-semibold rounded-lg
                       bg-accent text-gray-900 hover:bg-accent/90
                       disabled:opacity-50 transition-colors"
          >
            {saving ? 'Saving...' : 'Save & Download'}
          </button>
          {settings?.license_key && (
            <button
              type="button"
              onClick={handleDelete}
              className="px-3 py-1.5 text-xs font-medium rounded-md border border-red-300 dark:border-red-500/30 text-red-600 dark:text-red-400 hover:bg-red-500/10 transition-colors"
            >
              Remove
            </button>
          )}
        </div>
      </form>
    </div>
  )
}
