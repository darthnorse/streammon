import { useState, useCallback } from 'react'
import { useFetch } from '../hooks/useFetch'
import { api } from '../lib/api'
import { errorMessage } from '../lib/utils'
import { EmptyState } from './EmptyState'

interface GuestSettings {
  access_enabled: boolean
  store_plex_tokens: boolean
  show_discover: boolean
  visible_trust_score: boolean
  visible_violations: boolean
  visible_watch_history: boolean
  visible_household: boolean
  visible_devices: boolean
  visible_isps: boolean
  plex_tokens_available: boolean
}

type SettingKey = Exclude<keyof GuestSettings, 'plex_tokens_available'>

interface ToggleRowProps {
  title: string
  description: string
  enabled: boolean
  saving: boolean
  settingKey: SettingKey
  onToggle: (key: SettingKey) => void
}

function ToggleRow({ title, description, enabled, saving, settingKey, onToggle }: ToggleRowProps) {
  return (
    <div className="p-4 flex items-center justify-between">
      <div>
        <h4 className="font-medium text-sm">{title}</h4>
        <p className="text-sm text-muted dark:text-muted-dark mt-0.5">{description}</p>
      </div>
      <button
        onClick={() => onToggle(settingKey)}
        disabled={saving}
        className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ${
          enabled ? 'bg-accent' : 'bg-gray-300 dark:bg-white/20'
        } ${saving ? 'opacity-50 cursor-not-allowed' : ''}`}
        role="switch"
        aria-checked={enabled}
      >
        <span
          className={`pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform transition-transform duration-200 ${
            enabled ? 'translate-x-5' : 'translate-x-0'
          }`}
        />
      </button>
    </div>
  )
}

const visibilityToggles: { key: SettingKey; title: string; description: string }[] = [
  { key: 'visible_trust_score', title: 'Trust Score', description: 'Allow viewers to see their trust score on their profile.' },
  { key: 'visible_violations', title: 'Violations', description: 'Allow viewers to see their rule violations on their profile.' },
  { key: 'visible_watch_history', title: 'Watch History', description: 'Allow viewers to see their watch history and location map on their profile.' },
  { key: 'visible_household', title: 'Household Locations', description: 'Allow viewers to see their household locations on their profile.' },
  { key: 'visible_devices', title: 'Devices', description: 'Allow viewers to see their devices on their profile.' },
  { key: 'visible_isps', title: 'ISPs', description: 'Allow viewers to see their ISP information on their profile.' },
]

export function GuestAccessSettings() {
  const { data, loading, error, refetch } = useFetch<GuestSettings>('/api/settings/guest')
  const [optimistic, setOptimistic] = useState<Partial<Record<SettingKey, boolean>>>({})
  const [saving, setSaving] = useState<SettingKey | null>(null)
  const [saveError, setSaveError] = useState('')

  const get = useCallback((key: SettingKey): boolean => {
    if (key in optimistic) return optimistic[key]!
    return data ? data[key] : false
  }, [data, optimistic])

  const toggle = useCallback(async (key: SettingKey) => {
    if (!data) return
    const newValue = !get(key)
    setOptimistic(prev => ({ ...prev, [key]: newValue }))
    setSaving(key)
    setSaveError('')
    try {
      await api.put('/api/settings/guest', { [key]: newValue })
      refetch()
    } catch (err) {
      setSaveError(errorMessage(err))
    } finally {
      setOptimistic(prev => {
        const next = { ...prev }
        delete next[key]
        return next
      })
      setSaving(null)
    }
  }, [data, get, refetch])

  if (loading) {
    return <EmptyState icon="&#8635;" title="Loading..." />
  }

  if (error) {
    return (
      <EmptyState icon="!" title="Failed to load guest settings">
        <button onClick={refetch} className="text-sm text-accent hover:underline">Retry</button>
      </EmptyState>
    )
  }

  if (!data) return null

  return (
    <div>
      {saveError && (
        <div className="card p-4 mb-4 text-center text-red-500 dark:text-red-400">
          {saveError}
        </div>
      )}

      <div className="card">
        <div className="divide-y divide-border dark:divide-border-dark">
          <ToggleRow
            settingKey="access_enabled"
            title="Guest Access"
            description="Allow non-admin users to sign in. When disabled, only admins can log in."
            enabled={get('access_enabled')}
            saving={saving === 'access_enabled'}
            onToggle={toggle}
          />
          {data.plex_tokens_available && (
            <ToggleRow
              settingKey="store_plex_tokens"
              title="Store Plex Tokens"
              description="Store encrypted Plex tokens to attribute Overseerr / Seerr requests to the correct user. Requires TOKEN_ENCRYPTION_KEY."
              enabled={get('store_plex_tokens')}
              saving={saving === 'store_plex_tokens'}
              onToggle={toggle}
            />
          )}
          <ToggleRow
            settingKey="show_discover"
            title="Show Discover / Requests Link"
            description="Display the Discover (or Requests, if Overseerr is configured) page in the navigation for all users."
            enabled={get('show_discover')}
            saving={saving === 'show_discover'}
            onToggle={toggle}
          />
        </div>
      </div>

      <div className="card mt-4">
        <div className="p-4 border-b border-border dark:border-border-dark">
          <h3 className="font-semibold">Profile Visibility</h3>
          <p className="text-sm text-muted dark:text-muted-dark mt-0.5">
            Control which sections viewers can see on their own profile page.
          </p>
        </div>
        <div className="divide-y divide-border dark:divide-border-dark">
          {visibilityToggles.map(t => (
            <ToggleRow
              key={t.key}
              settingKey={t.key}
              title={t.title}
              description={t.description}
              enabled={get(t.key)}
              saving={saving === t.key}
              onToggle={toggle}
            />
          ))}
        </div>
      </div>
    </div>
  )
}
