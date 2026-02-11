import { useState, useEffect } from 'react'
import type { NotificationChannel, ChannelType } from '../types'
import { api } from '../lib/api'
import { formInputClass } from '../lib/constants'
import { useModal } from '../hooks/useModal'

interface NotificationChannelFormProps {
  channel?: NotificationChannel | null
  onClose: () => void
  onSaved: () => void
}

const CHANNEL_TYPES: { value: ChannelType; label: string }[] = [
  { value: 'discord', label: 'Discord Webhook' },
  { value: 'webhook', label: 'HTTP Webhook' },
  { value: 'pushover', label: 'Pushover' },
  { value: 'ntfy', label: 'Ntfy' },
]

const selectClass = `w-full px-3 py-2.5 rounded-lg text-sm
  bg-surface dark:bg-surface-dark
  border border-border dark:border-border-dark
  focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20
  transition-colors`

export function NotificationChannelForm({ channel, onClose, onSaved }: NotificationChannelFormProps) {
  const isEdit = !!channel?.id
  const modalRef = useModal(onClose)

  const [name, setName] = useState(channel?.name ?? '')
  const [channelType, setChannelType] = useState<ChannelType>(channel?.channel_type ?? 'discord')
  const [enabled, setEnabled] = useState(channel?.enabled ?? true)
  const [config, setConfig] = useState<Record<string, unknown>>(channel?.config ?? {})
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; error?: string } | null>(null)

  useEffect(() => {
    if (!channel) {
      setConfig(getDefaultConfig(channelType))
    }
  }, [channelType, channel])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) {
      setError('Name is required')
      return
    }

    const validationError = validateConfig(channelType, config)
    if (validationError) {
      setError(validationError)
      return
    }

    setSaving(true)
    setError('')
    try {
      const payload = { name, channel_type: channelType, enabled, config }
      if (isEdit) {
        await api.put(`/api/notifications/${channel.id}`, payload)
      } else {
        await api.post('/api/notifications', payload)
      }
      onSaved()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSaving(false)
    }
  }

  async function handleTest() {
    const validationError = validateConfig(channelType, config)
    if (validationError) {
      setTestResult({ success: false, error: validationError })
      return
    }

    setTesting(true)
    setTestResult(null)
    try {
      // For testing, we need to save first if it's a new channel
      // or use the existing channel ID
      if (isEdit && channel) {
        await api.post(`/api/notifications/${channel.id}/test`, {})
        setTestResult({ success: true })
      } else {
        setTestResult({ success: false, error: 'Please save the channel first before testing' })
      }
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
    >
      <div
        ref={modalRef}
        className="card w-full max-w-lg max-h-[90vh] overflow-y-auto p-0 animate-slide-up"
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-border dark:border-border-dark">
          <h2 className="text-lg font-semibold">
            {isEdit ? 'Edit Notification Channel' : 'New Notification Channel'}
          </h2>
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-100 transition-colors text-xl leading-none"
          >
            &times;
          </button>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
          <div>
            <label htmlFor="channel-name" className="block text-sm font-medium mb-1.5">Name</label>
            <input
              id="channel-name"
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="My Discord"
              className={formInputClass}
            />
          </div>

          <div>
            <label htmlFor="channel-type" className="block text-sm font-medium mb-1.5">Type</label>
            <select
              id="channel-type"
              value={channelType}
              onChange={e => setChannelType(e.target.value as ChannelType)}
              disabled={isEdit}
              className={selectClass}
            >
              {CHANNEL_TYPES.map(ct => (
                <option key={ct.value} value={ct.value}>{ct.label}</option>
              ))}
            </select>
          </div>

          <div className="flex items-center gap-2">
            <input
              id="channel-enabled"
              type="checkbox"
              checked={enabled}
              onChange={e => setEnabled(e.target.checked)}
              className="w-4 h-4 rounded border-border dark:border-border-dark"
            />
            <label htmlFor="channel-enabled" className="text-sm">Enabled</label>
          </div>

          <div className="border-t border-border dark:border-border-dark pt-4">
            <h3 className="text-sm font-semibold mb-3">Configuration</h3>
            {renderConfigFields(channelType, config, setConfig)}
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
              {testResult.success ? 'Test notification sent successfully' : testResult.error}
            </div>
          )}

          <div className="flex flex-col-reverse sm:flex-row items-stretch sm:items-center gap-3 pt-2">
            {isEdit && (
              <button
                type="button"
                onClick={handleTest}
                disabled={testing || saving}
                className="px-4 py-2.5 text-sm font-medium rounded-lg border border-border dark:border-border-dark hover:border-accent/30 transition-colors disabled:opacity-50"
              >
                {testing ? 'Testing...' : 'Test'}
              </button>
            )}
            <div className="flex-1" />
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2.5 text-sm font-medium rounded-lg border border-border dark:border-border-dark hover:border-accent/30 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving || testing}
              className="px-5 py-2.5 text-sm font-semibold rounded-lg bg-accent text-gray-900 hover:bg-accent/90 disabled:opacity-50 transition-colors"
            >
              {saving ? 'Saving...' : isEdit ? 'Update' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function getDefaultConfig(type: ChannelType): Record<string, unknown> {
  switch (type) {
    case 'discord':
      return { webhook_url: '' }
    case 'webhook':
      return { url: '', method: 'POST', headers: {} }
    case 'pushover':
      return { user_key: '', api_token: '' }
    case 'ntfy':
      return { server_url: 'https://ntfy.sh', topic: '', token: '' }
    default:
      return {}
  }
}

function validateConfig(type: ChannelType, config: Record<string, unknown>): string | null {
  switch (type) {
    case 'discord':
      if (!config.webhook_url) return 'Webhook URL is required'
      break
    case 'webhook':
      if (!config.url) return 'URL is required'
      break
    case 'pushover':
      if (!config.user_key) return 'User Key is required'
      if (!config.api_token) return 'API Token is required'
      break
    case 'ntfy':
      if (!config.topic) return 'Topic is required'
      break
  }
  return null
}

function renderConfigFields(
  type: ChannelType,
  config: Record<string, unknown>,
  setConfig: (c: Record<string, unknown>) => void
) {
  const updateField = (key: string, value: unknown) => {
    setConfig({ ...config, [key]: value })
  }

  switch (type) {
    case 'discord':
      return (
        <div>
          <label className="block text-sm mb-1">Webhook URL</label>
          <input
            type="url"
            value={(config.webhook_url as string) ?? ''}
            onChange={e => updateField('webhook_url', e.target.value)}
            placeholder="https://discord.com/api/webhooks/..."
            className={formInputClass}
          />
          <p className="text-xs text-muted dark:text-muted-dark mt-1">
            Server Settings &rarr; Integrations &rarr; Webhooks
          </p>
        </div>
      )

    case 'webhook':
      return (
        <div className="space-y-3">
          <div>
            <label className="block text-sm mb-1">URL</label>
            <input
              type="url"
              value={(config.url as string) ?? ''}
              onChange={e => updateField('url', e.target.value)}
              placeholder="https://example.com/webhook"
              className={formInputClass}
            />
          </div>
          <div>
            <label className="block text-sm mb-1">Method</label>
            <select
              value={(config.method as string) ?? 'POST'}
              onChange={e => updateField('method', e.target.value)}
              className={selectClass}
            >
              <option value="POST">POST</option>
              <option value="PUT">PUT</option>
            </select>
          </div>
        </div>
      )

    case 'pushover':
      return (
        <div className="space-y-3">
          <div>
            <label className="block text-sm mb-1">User Key</label>
            <input
              type="text"
              value={(config.user_key as string) ?? ''}
              onChange={e => updateField('user_key', e.target.value)}
              placeholder="Your Pushover user key"
              className={formInputClass}
            />
          </div>
          <div>
            <label className="block text-sm mb-1">API Token</label>
            <input
              type="password"
              value={(config.api_token as string) ?? ''}
              onChange={e => updateField('api_token', e.target.value)}
              placeholder="Your Pushover API token"
              className={formInputClass}
            />
          </div>
        </div>
      )

    case 'ntfy':
      return (
        <div className="space-y-3">
          <div>
            <label className="block text-sm mb-1">Server URL</label>
            <input
              type="url"
              value={(config.server_url as string) ?? 'https://ntfy.sh'}
              onChange={e => updateField('server_url', e.target.value)}
              placeholder="https://ntfy.sh"
              className={formInputClass}
            />
          </div>
          <div>
            <label className="block text-sm mb-1">Topic</label>
            <input
              type="text"
              value={(config.topic as string) ?? ''}
              onChange={e => updateField('topic', e.target.value)}
              placeholder="my-streammon-alerts"
              className={formInputClass}
            />
          </div>
          <div>
            <label className="block text-sm mb-1">Token (optional)</label>
            <input
              type="password"
              value={(config.token as string) ?? ''}
              onChange={e => updateField('token', e.target.value)}
              placeholder="For authenticated topics"
              className={formInputClass}
            />
          </div>
        </div>
      )

    default:
      return null
  }
}
