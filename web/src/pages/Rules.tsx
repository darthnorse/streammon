import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import type { Rule, RuleViolation, NotificationChannel, PaginatedResult } from '../types'
import { RULE_TYPE_LABELS } from '../types'
import { RuleForm } from '../components/RuleForm'
import { NotificationChannelForm } from '../components/NotificationChannelForm'
import { api } from '../lib/api'

type Tab = 'rules' | 'violations' | 'notifications'

const SEVERITY_COLORS: Record<string, string> = {
  info: 'bg-blue-500/20 text-blue-400',
  warning: 'bg-amber-500/20 text-amber-400',
  critical: 'bg-red-500/20 text-red-400',
}

export function Rules() {
  const [tab, setTab] = useState<Tab>('rules')
  const [editingRule, setEditingRule] = useState<Rule | null>(null)
  const [showRuleForm, setShowRuleForm] = useState(false)
  const [editingChannel, setEditingChannel] = useState<NotificationChannel | null>(null)
  const [showChannelForm, setShowChannelForm] = useState(false)
  const [page, setPage] = useState(1)

  const { data: rules, refetch: refetchRules } = useFetch<Rule[]>('/api/rules')
  const { data: violations } = useFetch<PaginatedResult<RuleViolation>>(
    tab === 'violations' ? `/api/violations?page=${page}&per_page=20` : null
  )
  const { data: channels, refetch: refetchChannels } = useFetch<NotificationChannel[]>(
    tab === 'notifications' ? '/api/notifications' : null
  )

  const handleToggleRule = async (rule: Rule) => {
    try {
      await api.put(`/api/rules/${rule.id}`, { ...rule, enabled: !rule.enabled })
      refetchRules()
    } catch (err) {
      console.error('Failed to toggle rule:', err)
    }
  }

  const handleDeleteRule = async (id: number) => {
    if (!confirm('Delete this rule?')) return
    try {
      await api.del(`/api/rules/${id}`)
      refetchRules()
    } catch (err) {
      console.error('Failed to delete rule:', err)
    }
  }

  const handleToggleChannel = async (channel: NotificationChannel) => {
    try {
      await api.put(`/api/notifications/${channel.id}`, { ...channel, enabled: !channel.enabled })
      refetchChannels()
    } catch (err) {
      console.error('Failed to toggle channel:', err)
    }
  }

  const handleDeleteChannel = async (id: number) => {
    if (!confirm('Delete this notification channel?')) return
    try {
      await api.del(`/api/notifications/${id}`)
      refetchChannels()
    } catch (err) {
      console.error('Failed to delete channel:', err)
    }
  }

  const handleRuleSaved = () => {
    setShowRuleForm(false)
    setEditingRule(null)
    refetchRules()
  }

  const handleChannelSaved = () => {
    setShowChannelForm(false)
    setEditingChannel(null)
    refetchChannels()
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Rules & Sharing Detection</h1>
      </div>

      <div className="flex gap-1 border-b border-border dark:border-border-dark">
        {(['rules', 'violations', 'notifications'] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-medium transition-colors
              ${tab === t
                ? 'border-b-2 border-accent text-accent'
                : 'text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
              }`}
          >
            {t === 'rules' && 'Rules'}
            {t === 'violations' && 'Violations'}
            {t === 'notifications' && 'Notifications'}
          </button>
        ))}
      </div>

      {tab === 'rules' && (
        <div className="space-y-4">
          <div className="flex justify-end">
            <button
              onClick={() => {
                setEditingRule(null)
                setShowRuleForm(true)
              }}
              className="px-4 py-2 text-sm font-medium rounded-lg bg-accent text-gray-900 hover:bg-accent/90"
            >
              Add Rule
            </button>
          </div>

          {!rules?.length ? (
            <div className="card p-8 text-center text-muted dark:text-muted-dark">
              No rules configured. Add your first rule to start detecting sharing violations.
            </div>
          ) : (
            <div className="space-y-3">
              {rules.map((rule) => (
                <div key={rule.id} className="card p-4">
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-3">
                        <h3 className="font-semibold truncate">{rule.name}</h3>
                        <span className="px-2 py-0.5 text-xs rounded-full bg-surface dark:bg-surface-dark">
                          {RULE_TYPE_LABELS[rule.type] || rule.type}
                        </span>
                      </div>
                      <p className="text-sm text-muted dark:text-muted-dark mt-1">
                        {formatRuleConfig(rule)}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleToggleRule(rule)}
                        className={`px-3 py-1 text-xs font-medium rounded-full transition-colors
                          ${rule.enabled
                            ? 'bg-green-500/20 text-green-400 hover:bg-green-500/30'
                            : 'bg-gray-500/20 text-gray-400 hover:bg-gray-500/30'
                          }`}
                      >
                        {rule.enabled ? 'Enabled' : 'Disabled'}
                      </button>
                      <button
                        onClick={() => {
                          setEditingRule(rule)
                          setShowRuleForm(true)
                        }}
                        className="p-1.5 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors"
                        title="Edit"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                        </svg>
                      </button>
                      <button
                        onClick={() => handleDeleteRule(rule.id)}
                        className="p-1.5 rounded hover:bg-red-500/20 text-red-400 transition-colors"
                        title="Delete"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === 'violations' && (
        <div className="space-y-4">
          {!violations?.items.length ? (
            <div className="card p-8 text-center text-muted dark:text-muted-dark">
              No violations detected yet.
            </div>
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="text-left text-sm text-muted dark:text-muted-dark border-b border-border dark:border-border-dark">
                      <th className="pb-2 font-medium">Time</th>
                      <th className="pb-2 font-medium">User</th>
                      <th className="pb-2 font-medium">Rule</th>
                      <th className="pb-2 font-medium">Severity</th>
                      <th className="pb-2 font-medium">Confidence</th>
                      <th className="pb-2 font-medium">Message</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border dark:divide-border-dark">
                    {violations.items.map((v) => (
                      <tr key={v.id} className="text-sm">
                        <td className="py-3 whitespace-nowrap">
                          {new Date(v.occurred_at).toLocaleString()}
                        </td>
                        <td className="py-3 font-medium">{v.user_name}</td>
                        <td className="py-3">{v.rule_name}</td>
                        <td className="py-3">
                          <span className={`px-2 py-0.5 text-xs rounded-full ${SEVERITY_COLORS[v.severity]}`}>
                            {v.severity}
                          </span>
                        </td>
                        <td className="py-3">{v.confidence_score.toFixed(0)}%</td>
                        <td className="py-3 max-w-xs truncate">{v.message}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {violations.total > 20 && (
                <div className="flex justify-center gap-2">
                  <button
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={page === 1}
                    className="px-3 py-1 text-sm rounded border border-border dark:border-border-dark disabled:opacity-50"
                  >
                    Previous
                  </button>
                  <span className="px-3 py-1 text-sm">
                    Page {page} of {Math.ceil(violations.total / 20)}
                  </span>
                  <button
                    onClick={() => setPage((p) => p + 1)}
                    disabled={page >= Math.ceil(violations.total / 20)}
                    className="px-3 py-1 text-sm rounded border border-border dark:border-border-dark disabled:opacity-50"
                  >
                    Next
                  </button>
                </div>
              )}
            </>
          )}
        </div>
      )}

      {tab === 'notifications' && (
        <div className="space-y-4">
          <div className="flex justify-end">
            <button
              onClick={() => {
                setEditingChannel(null)
                setShowChannelForm(true)
              }}
              className="px-4 py-2 text-sm font-medium rounded-lg bg-accent text-gray-900 hover:bg-accent/90"
            >
              Add Channel
            </button>
          </div>

          {!channels?.length ? (
            <div className="card p-8 text-center text-muted dark:text-muted-dark">
              No notification channels configured. Add a channel to receive alerts.
            </div>
          ) : (
            <div className="space-y-3">
              {channels.map((channel) => (
                <div key={channel.id} className="card p-4">
                  <div className="flex items-center justify-between gap-4">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-3">
                        <h3 className="font-semibold">{channel.name}</h3>
                        <span className="px-2 py-0.5 text-xs rounded-full bg-surface dark:bg-surface-dark capitalize">
                          {channel.channel_type}
                        </span>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleToggleChannel(channel)}
                        className={`px-3 py-1 text-xs font-medium rounded-full transition-colors
                          ${channel.enabled
                            ? 'bg-green-500/20 text-green-400 hover:bg-green-500/30'
                            : 'bg-gray-500/20 text-gray-400 hover:bg-gray-500/30'
                          }`}
                      >
                        {channel.enabled ? 'Enabled' : 'Disabled'}
                      </button>
                      <button
                        onClick={() => {
                          setEditingChannel(channel)
                          setShowChannelForm(true)
                        }}
                        className="p-1.5 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors"
                        title="Edit"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                        </svg>
                      </button>
                      <button
                        onClick={() => handleDeleteChannel(channel.id)}
                        className="p-1.5 rounded hover:bg-red-500/20 text-red-400 transition-colors"
                        title="Delete"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {showRuleForm && (
        <RuleForm
          rule={editingRule}
          onClose={() => {
            setShowRuleForm(false)
            setEditingRule(null)
          }}
          onSaved={handleRuleSaved}
        />
      )}

      {showChannelForm && (
        <NotificationChannelForm
          channel={editingChannel}
          onClose={() => {
            setShowChannelForm(false)
            setEditingChannel(null)
          }}
          onSaved={handleChannelSaved}
        />
      )}
    </div>
  )
}

function formatRuleConfig(rule: Rule): string {
  const config = rule.config
  switch (rule.type) {
    case 'concurrent_streams':
      return `Max ${config.max_streams || 2} streams${config.exempt_household ? ' (household exempt)' : ''}`
    case 'geo_restriction': {
      const allowed = config.allowed_countries as string[] | undefined
      const blocked = config.blocked_countries as string[] | undefined
      if (allowed?.length) {
        return `Allowed: ${allowed.join(', ')}`
      }
      if (blocked?.length) {
        return `Blocked: ${blocked.join(', ')}`
      }
      return 'No restrictions configured'
    }
    case 'impossible_travel':
      return `Max ${config.max_speed_km_h || 800} km/h, min ${config.min_distance_km || 100} km`
    case 'simultaneous_locations':
      return `Min distance: ${config.min_distance_km || 50} km${config.exempt_household ? ' (household exempt)' : ''}`
    case 'device_velocity':
      return `Max ${config.max_devices_per_hour || 3} devices per ${config.time_window_hours || 1}h`
    case 'new_device':
      return 'Alert on new device'
    case 'new_location':
      return `Alert on new location${config.min_distance_km ? ` (>${config.min_distance_km} km)` : ''}`
    default:
      return JSON.stringify(config)
  }
}
