import { useState, useEffect, useRef } from 'react'
import type { Rule, RuleType, NotificationChannel } from '../types'
import { RULE_TYPES } from '../types'
import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'
import { MultiSelectChannels } from './MultiSelectChannels'

interface RuleFormProps {
  rule?: Rule | null
  onClose: () => void
  onSaved: () => void
}

const inputClass = `w-full px-3 py-2.5 rounded-lg text-sm
  bg-surface dark:bg-surface-dark
  border border-border dark:border-border-dark
  focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20
  transition-colors`

const selectClass = `w-full px-3 py-2.5 rounded-lg text-sm
  bg-surface dark:bg-surface-dark
  border border-border dark:border-border-dark
  focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20
  transition-colors`

export function RuleForm({ rule, onClose, onSaved }: RuleFormProps) {
  const isEdit = !!rule?.id
  const modalRef = useRef<HTMLDivElement>(null)

  const [name, setName] = useState(rule?.name ?? '')
  const [ruleType, setRuleType] = useState<RuleType>(rule?.type ?? 'concurrent_streams')
  const [enabled, setEnabled] = useState(rule?.enabled ?? true)
  const [config, setConfig] = useState<Record<string, unknown>>(rule?.config ?? {})
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [selectedChannels, setSelectedChannels] = useState<number[]>([])

  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleEscape)
    modalRef.current?.querySelector<HTMLElement>('input')?.focus()
    return () => document.removeEventListener('keydown', handleEscape)
  }, [onClose])

  useEffect(() => {
    if (!rule) {
      setConfig(getDefaultConfig(ruleType))
    }
  }, [ruleType, rule])

  const { data: linkedChannels } = useFetch<NotificationChannel[]>(
    rule?.id ? `/api/rules/${rule.id}/channels` : null
  )

  useEffect(() => {
    if (linkedChannels) {
      setSelectedChannels(linkedChannels.map(c => c.id))
    }
  }, [linkedChannels])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) {
      setError('Name is required')
      return
    }

    setSaving(true)
    setError('')
    try {
      const payload = { name, type: ruleType, enabled, config }
      let ruleId: number

      if (isEdit) {
        await api.put(`/api/rules/${rule.id}`, payload)
        ruleId = rule.id
      } else {
        const created = await api.post<{ id: number }>('/api/rules', payload)
        ruleId = created.id
      }

      // Sync channel links
      const currentChannels = linkedChannels?.map(c => c.id) ?? []
      const toAdd = selectedChannels.filter(id => !currentChannels.includes(id))
      const toRemove = currentChannels.filter(id => !selectedChannels.includes(id))

      await Promise.all([
        ...toAdd.map(channelId => api.post(`/api/rules/${ruleId}/channels`, { channel_id: channelId })),
        ...toRemove.map(channelId => api.del(`/api/rules/${ruleId}/channels/${channelId}`)),
      ])

      onSaved()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        ref={modalRef}
        className="card w-full max-w-lg max-h-[90vh] overflow-y-auto p-0 animate-in"
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-border dark:border-border-dark">
          <h2 className="text-lg font-semibold">
            {isEdit ? 'Edit Rule' : 'New Rule'}
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
            <label htmlFor="rule-name" className="block text-sm font-medium mb-1.5">Name</label>
            <input
              id="rule-name"
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="My Rule"
              className={inputClass}
            />
          </div>

          <div>
            <label htmlFor="rule-type" className="block text-sm font-medium mb-1.5">Type</label>
            <select
              id="rule-type"
              value={ruleType}
              onChange={e => setRuleType(e.target.value as RuleType)}
              disabled={isEdit}
              className={selectClass}
            >
              {RULE_TYPES.map(rt => (
                <option key={rt.value} value={rt.value}>{rt.label}</option>
              ))}
            </select>
            <p className="text-xs text-muted dark:text-muted-dark mt-1">
              {RULE_TYPES.find(rt => rt.value === ruleType)?.description}
            </p>
          </div>

          <div className="flex items-center gap-2">
            <input
              id="rule-enabled"
              type="checkbox"
              checked={enabled}
              onChange={e => setEnabled(e.target.checked)}
              className="w-4 h-4 rounded border-border dark:border-border-dark"
            />
            <label htmlFor="rule-enabled" className="text-sm">Enabled</label>
          </div>

          <div className="border-t border-border dark:border-border-dark pt-4">
            <h3 className="text-sm font-semibold mb-3">Configuration</h3>
            {renderConfigFields(ruleType, config, setConfig)}
          </div>

          <div className="border-t border-border dark:border-border-dark pt-4">
            <h3 className="text-sm font-semibold mb-3">Notification Channels</h3>
            <p className="text-xs text-muted dark:text-muted-dark mb-3">
              Select channels to notify when this rule is violated.
            </p>
            <MultiSelectChannels
              selectedIds={selectedChannels}
              onChange={setSelectedChannels}
            />
          </div>

          {error && (
            <div className="text-sm text-red-500 dark:text-red-400 font-mono px-1">
              {error}
            </div>
          )}

          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2.5 text-sm font-medium rounded-lg border border-border dark:border-border-dark hover:border-accent/30 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
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

function getDefaultConfig(type: RuleType): Record<string, unknown> {
  switch (type) {
    case 'concurrent_streams':
      return { max_streams: 2, exempt_household: true }
    case 'geo_restriction':
      return { allowed_countries: [], blocked_countries: [] }
    case 'simultaneous_locations':
      return { min_distance_km: 50, exempt_household: true }
    case 'impossible_travel':
      return { max_speed_km_h: 800, min_distance_km: 100, time_window_hours: 24 }
    case 'device_velocity':
      return { max_devices_per_hour: 3, time_window_hours: 1 }
    case 'new_device':
      return { notify_on_new: true }
    case 'new_location':
      return { notify_on_new: true, min_distance_km: 50, exempt_household: true }
    default:
      return {}
  }
}

function renderConfigFields(
  type: RuleType,
  config: Record<string, unknown>,
  setConfig: (c: Record<string, unknown>) => void
) {
  const updateField = (key: string, value: unknown) => {
    setConfig({ ...config, [key]: value })
  }

  switch (type) {
    case 'concurrent_streams':
      return (
        <div className="space-y-3">
          <div>
            <label className="block text-sm mb-1">Max Streams</label>
            <input
              type="number"
              min={1}
              max={10}
              value={(config.max_streams as number) ?? 2}
              onChange={e => updateField('max_streams', parseInt(e.target.value) || 2)}
              className={inputClass}
            />
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={(config.exempt_household as boolean) ?? true}
              onChange={e => updateField('exempt_household', e.target.checked)}
              className="w-4 h-4 rounded"
            />
            <label className="text-sm">Exempt household (trusted locations)</label>
          </div>
        </div>
      )

    case 'geo_restriction':
      return (
        <div className="space-y-3">
          <div>
            <label className="block text-sm mb-1">Allowed Countries (comma-separated)</label>
            <input
              type="text"
              value={((config.allowed_countries as string[]) || []).join(', ')}
              onChange={e => updateField('allowed_countries', e.target.value.split(',').map(s => s.trim()).filter(Boolean))}
              placeholder="US, CA, GB"
              className={inputClass}
            />
            <p className="text-xs text-muted dark:text-muted-dark mt-1">Leave empty to allow all countries</p>
          </div>
          <div>
            <label className="block text-sm mb-1">Blocked Countries (comma-separated)</label>
            <input
              type="text"
              value={((config.blocked_countries as string[]) || []).join(', ')}
              onChange={e => updateField('blocked_countries', e.target.value.split(',').map(s => s.trim()).filter(Boolean))}
              placeholder="RU, CN"
              className={inputClass}
            />
          </div>
        </div>
      )

    case 'simultaneous_locations':
      return (
        <div className="space-y-3">
          <div>
            <label className="block text-sm mb-1">Minimum Distance (km)</label>
            <input
              type="number"
              min={10}
              value={(config.min_distance_km as number) ?? 50}
              onChange={e => updateField('min_distance_km', parseInt(e.target.value) || 50)}
              className={inputClass}
            />
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={(config.exempt_household as boolean) ?? true}
              onChange={e => updateField('exempt_household', e.target.checked)}
              className="w-4 h-4 rounded"
            />
            <label className="text-sm">Exempt household locations</label>
          </div>
        </div>
      )

    case 'impossible_travel':
      return (
        <div className="space-y-3">
          <div>
            <label className="block text-sm mb-1">Max Speed (km/h)</label>
            <input
              type="number"
              min={100}
              value={(config.max_speed_km_h as number) ?? 800}
              onChange={e => updateField('max_speed_km_h', parseInt(e.target.value) || 800)}
              className={inputClass}
            />
            <p className="text-xs text-muted dark:text-muted-dark mt-1">~800 km/h is typical commercial flight speed</p>
          </div>
          <div>
            <label className="block text-sm mb-1">Min Distance (km)</label>
            <input
              type="number"
              min={10}
              value={(config.min_distance_km as number) ?? 100}
              onChange={e => updateField('min_distance_km', parseInt(e.target.value) || 100)}
              className={inputClass}
            />
          </div>
          <div>
            <label className="block text-sm mb-1">Time Window (hours)</label>
            <input
              type="number"
              min={1}
              max={72}
              value={(config.time_window_hours as number) ?? 24}
              onChange={e => updateField('time_window_hours', parseInt(e.target.value) || 24)}
              className={inputClass}
            />
          </div>
        </div>
      )

    case 'device_velocity':
      return (
        <div className="space-y-3">
          <div>
            <label className="block text-sm mb-1">Max Devices</label>
            <input
              type="number"
              min={1}
              max={20}
              value={(config.max_devices_per_hour as number) ?? 3}
              onChange={e => updateField('max_devices_per_hour', parseInt(e.target.value) || 3)}
              className={inputClass}
            />
            <p className="text-xs text-muted dark:text-muted-dark mt-1">Alert if user exceeds this many devices in the time window</p>
          </div>
          <div>
            <label className="block text-sm mb-1">Time Window (hours)</label>
            <input
              type="number"
              min={1}
              max={24}
              value={(config.time_window_hours as number) ?? 1}
              onChange={e => updateField('time_window_hours', parseInt(e.target.value) || 1)}
              className={inputClass}
            />
          </div>
        </div>
      )

    case 'new_device':
      return (
        <div className="space-y-3">
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={(config.notify_on_new as boolean) ?? true}
              onChange={e => updateField('notify_on_new', e.target.checked)}
              className="w-4 h-4 rounded"
            />
            <label className="text-sm">Notify on new device</label>
          </div>
          <p className="text-xs text-muted dark:text-muted-dark">
            Alert when a user streams from a device they haven't used before.
          </p>
        </div>
      )

    case 'new_location':
      return (
        <div className="space-y-3">
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={(config.notify_on_new as boolean) ?? true}
              onChange={e => updateField('notify_on_new', e.target.checked)}
              className="w-4 h-4 rounded"
            />
            <label className="text-sm">Notify on new location</label>
          </div>
          <div>
            <label className="block text-sm mb-1">Minimum Distance (km)</label>
            <input
              type="number"
              min={10}
              value={(config.min_distance_km as number) ?? 50}
              onChange={e => updateField('min_distance_km', parseInt(e.target.value) || 50)}
              className={inputClass}
            />
            <p className="text-xs text-muted dark:text-muted-dark mt-1">Only alert if new location is at least this far from known locations</p>
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={(config.exempt_household as boolean) ?? true}
              onChange={e => updateField('exempt_household', e.target.checked)}
              className="w-4 h-4 rounded"
            />
            <label className="text-sm">Exempt household locations</label>
          </div>
        </div>
      )

    default:
      return (
        <p className="text-sm text-muted dark:text-muted-dark">
          No configuration options available for this rule type.
        </p>
      )
  }
}
