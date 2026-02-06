import { useState, useEffect } from 'react'
import { useFetch } from '../hooks/useFetch'
import { api } from '../lib/api'
import type {
  MaintenanceDashboard,
  LibraryMaintenance,
  MaintenanceRuleWithCount,
  MaintenanceCandidatesResponse,
  CriterionTypeInfo,
  CriterionType,
  LibraryType,
} from '../types'

type ViewState =
  | { type: 'dashboard' }
  | { type: 'library'; library: LibraryMaintenance }
  | { type: 'candidates'; rule: MaintenanceRuleWithCount; library: LibraryMaintenance }
  | { type: 'create-rule'; library: LibraryMaintenance }

const libraryTypeIcon: Record<LibraryType, string> = {
  movie: '◉',
  show: '▦',
  music: '♫',
  other: '▤',
}

const libraryTypeLabel: Record<LibraryType, string> = {
  movie: 'Movies',
  show: 'TV Shows',
  music: 'Music',
  other: 'Other',
}

const getLibraryKey = (lib: LibraryMaintenance) => `${lib.server_id}-${lib.library_id}`

function formatDate(dateStr: string | null): string {
  if (!dateStr) return 'Never'
  return new Date(dateStr).toLocaleString()
}

function LibraryCard({
  library,
  onView,
  onSync,
  syncing,
}: {
  library: LibraryMaintenance
  onView: () => void
  onSync: () => void
  syncing: boolean
}) {
  const icon = libraryTypeIcon[library.library_type] || '▤'
  const typeLabel = libraryTypeLabel[library.library_type]
  const totalCandidates = library.rules.reduce((sum, r) => sum + r.candidate_count, 0)
  const enabledRules = library.rules.filter((r) => r.enabled).length

  return (
    <div className="card p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3 min-w-0">
          <span className="text-2xl">{icon}</span>
          <div className="min-w-0">
            <h3 className="font-semibold truncate">{library.library_name}</h3>
            <p className="text-sm text-muted dark:text-muted-dark">
              {library.server_name} - {typeLabel}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={onSync}
            disabled={syncing}
            className="px-3 py-1.5 text-sm font-medium rounded-lg border border-border dark:border-border-dark
                       hover:bg-surface dark:hover:bg-surface-dark transition-colors disabled:opacity-50"
          >
            {syncing ? 'Syncing...' : 'Sync'}
          </button>
          <button
            onClick={onView}
            className="px-3 py-1.5 text-sm font-medium rounded-lg bg-accent text-gray-900 hover:bg-accent/90"
          >
            View
          </button>
        </div>
      </div>

      <div className="mt-4 grid grid-cols-3 gap-4 text-center">
        <div>
          <div className="text-2xl font-bold">{library.total_items.toLocaleString()}</div>
          <div className="text-xs text-muted dark:text-muted-dark">Items</div>
        </div>
        <div>
          <div className="text-2xl font-bold">{enabledRules}</div>
          <div className="text-xs text-muted dark:text-muted-dark">Rules</div>
        </div>
        <div>
          <div className="text-2xl font-bold text-amber-500">{totalCandidates.toLocaleString()}</div>
          <div className="text-xs text-muted dark:text-muted-dark">Candidates</div>
        </div>
      </div>

      <div className="mt-3 pt-3 border-t border-border dark:border-border-dark text-xs text-muted dark:text-muted-dark">
        Last synced: {formatDate(library.last_synced_at)}
      </div>
    </div>
  )
}

function LibraryView({
  library,
  onBack,
  onViewCandidates,
  onCreateRule,
  onRefresh,
}: {
  library: LibraryMaintenance
  onBack: () => void
  onViewCandidates: (rule: MaintenanceRuleWithCount) => void
  onCreateRule: () => void
  onRefresh: () => void
}) {
  const handleToggleRule = async (rule: MaintenanceRuleWithCount) => {
    try {
      await api.put(`/api/maintenance/rules/${rule.id}`, {
        name: rule.name,
        criterion_type: rule.criterion_type,
        parameters: rule.parameters,
        enabled: !rule.enabled,
      })
      onRefresh()
    } catch (err) {
      console.error('Failed to toggle rule:', err)
    }
  }

  const handleDeleteRule = async (rule: MaintenanceRuleWithCount) => {
    if (!confirm(`Delete rule "${rule.name}"?`)) return
    try {
      await api.del(`/api/maintenance/rules/${rule.id}`)
      onRefresh()
    } catch (err) {
      console.error('Failed to delete rule:', err)
    }
  }

  const handleEvaluateRule = async (rule: MaintenanceRuleWithCount) => {
    try {
      await api.post(`/api/maintenance/rules/${rule.id}/evaluate`)
      onRefresh()
    } catch (err) {
      console.error('Failed to evaluate rule:', err)
    }
  }

  const icon = libraryTypeIcon[library.library_type] || '▤'

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <button
          onClick={onBack}
          className="p-2 rounded-lg hover:bg-surface dark:hover:bg-surface-dark transition-colors"
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        <div className="flex items-center gap-3">
          <span className="text-2xl">{icon}</span>
          <div>
            <h1 className="text-2xl font-semibold">{library.library_name}</h1>
            <p className="text-sm text-muted dark:text-muted-dark">{library.server_name}</p>
          </div>
        </div>
      </div>

      <div className="flex items-center justify-between">
        <h2 className="text-lg font-medium">Maintenance Rules</h2>
        <button
          onClick={onCreateRule}
          className="px-4 py-2 text-sm font-medium rounded-lg bg-accent text-gray-900 hover:bg-accent/90"
        >
          Add Rule
        </button>
      </div>

      {library.rules.length === 0 ? (
        <div className="card p-8 text-center text-muted dark:text-muted-dark">
          No maintenance rules configured for this library.
        </div>
      ) : (
        <div className="space-y-3">
          {library.rules.map((rule) => (
            <div key={rule.id} className="card p-4">
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-3">
                    <h3 className="font-semibold truncate">{rule.name}</h3>
                    <span className="px-2 py-0.5 text-xs rounded-full bg-surface dark:bg-surface-dark">
                      {rule.criterion_type.replace(/_/g, ' ')}
                    </span>
                  </div>
                  <p className="text-sm text-muted dark:text-muted-dark mt-1">
                    {formatRuleParameters(rule)}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-amber-500 font-medium text-sm">
                    {rule.candidate_count.toLocaleString()} candidates
                  </span>
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
                    onClick={() => handleEvaluateRule(rule)}
                    className="p-1.5 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors"
                    title="Re-evaluate"
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                    </svg>
                  </button>
                  <button
                    onClick={() => onViewCandidates(rule)}
                    className="p-1.5 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors"
                    title="View candidates"
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                    </svg>
                  </button>
                  <button
                    onClick={() => handleDeleteRule(rule)}
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
  )
}

function CandidatesView({
  rule,
  library,
  onBack,
}: {
  rule: MaintenanceRuleWithCount
  library: LibraryMaintenance
  onBack: () => void
}) {
  const [page, setPage] = useState(1)
  const perPage = 20

  // Reset to page 1 when viewing a different rule
  useEffect(() => {
    setPage(1)
  }, [rule.id])

  const { data, loading } = useFetch<MaintenanceCandidatesResponse>(
    `/api/maintenance/rules/${rule.id}/candidates?page=${page}&per_page=${perPage}`
  )

  const totalPages = data ? Math.ceil(data.total / perPage) : 0

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <button
          onClick={onBack}
          className="p-2 rounded-lg hover:bg-surface dark:hover:bg-surface-dark transition-colors"
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        <div>
          <h1 className="text-2xl font-semibold">{rule.name}</h1>
          <p className="text-sm text-muted dark:text-muted-dark">
            {library.library_name} - {data?.total.toLocaleString() || 0} candidates
          </p>
        </div>
      </div>

      {loading && !data ? (
        <div className="card p-12 text-center">
          <div className="text-muted dark:text-muted-dark animate-pulse">Loading...</div>
        </div>
      ) : !data?.items.length ? (
        <div className="card p-8 text-center text-muted dark:text-muted-dark">
          No candidates found for this rule.
        </div>
      ) : (
        <>
          <div className="card overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="bg-gray-50 dark:bg-white/5 border-b border-border dark:border-border-dark">
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                      Title
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                      Year
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                      Resolution
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                      Added
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                      Reason
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {data.items.map((candidate) => (
                    <tr
                      key={candidate.id}
                      className="border-b border-border dark:border-border-dark hover:bg-gray-50 dark:hover:bg-white/5 transition-colors"
                    >
                      <td className="px-4 py-3 font-medium">
                        {candidate.item?.title || 'Unknown'}
                      </td>
                      <td className="px-4 py-3 text-muted dark:text-muted-dark">
                        {candidate.item?.year || '-'}
                      </td>
                      <td className="px-4 py-3 text-muted dark:text-muted-dark">
                        {candidate.item?.video_resolution || '-'}
                      </td>
                      <td className="px-4 py-3 text-muted dark:text-muted-dark">
                        {candidate.item?.added_at ? formatDate(candidate.item.added_at) : '-'}
                      </td>
                      <td className="px-4 py-3 text-sm text-amber-500">
                        {candidate.reason}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {totalPages > 1 && (
            <div className="flex justify-center gap-2">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
                className="px-3 py-1 text-sm rounded border border-border dark:border-border-dark disabled:opacity-50"
              >
                Previous
              </button>
              <span className="px-3 py-1 text-sm">
                Page {page} of {totalPages}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
                className="px-3 py-1 text-sm rounded border border-border dark:border-border-dark disabled:opacity-50"
              >
                Next
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}

function CreateRuleView({
  library,
  onBack,
  onCreated,
}: {
  library: LibraryMaintenance
  onBack: () => void
  onCreated: () => void
}) {
  const { data: criterionTypes } = useFetch<{ types: CriterionTypeInfo[] }>('/api/maintenance/criterion-types')
  const [name, setName] = useState('')
  const [criterionType, setCriterionType] = useState<CriterionType | ''>('')
  const [parameters, setParameters] = useState<Record<string, string | number>>({})
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // TV shows use 'episode' media type in the API
  const availableTypes = criterionTypes?.types.filter((ct) => {
    if (library.library_type === 'movie') {
      return ct.media_types.includes('movie')
    }
    if (library.library_type === 'show') {
      return ct.media_types.includes('episode')
    }
    return false
  }) || []

  const selectedType = availableTypes.find((ct) => ct.type === criterionType)

  useEffect(() => {
    if (selectedType) {
      const defaults: Record<string, string | number> = {}
      for (const param of selectedType.parameters) {
        defaults[param.name] = param.default
      }
      setParameters(defaults)
    } else {
      setParameters({})
    }
  }, [criterionType]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!criterionType) return

    setSaving(true)
    setError(null)

    try {
      await api.post('/api/maintenance/rules', {
        server_id: library.server_id,
        library_id: library.library_id,
        name,
        criterion_type: criterionType,
        parameters,
        enabled: true,
      })
      onCreated()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create rule')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <button
          onClick={onBack}
          className="p-2 rounded-lg hover:bg-surface dark:hover:bg-surface-dark transition-colors"
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        <div>
          <h1 className="text-2xl font-semibold">Create Maintenance Rule</h1>
          <p className="text-sm text-muted dark:text-muted-dark">{library.library_name}</p>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="card p-6 space-y-6 max-w-xl">
        {error && (
          <div className="p-3 rounded-lg bg-red-500/10 text-red-500 text-sm">
            {error}
          </div>
        )}

        <div>
          <label className="block text-sm font-medium mb-2">Rule Name</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            className="w-full px-3 py-2 rounded-lg border border-border dark:border-border-dark
                       bg-panel dark:bg-panel-dark focus:outline-none focus:ring-2 focus:ring-accent"
            placeholder="e.g., Unwatched Movies > 90 days"
          />
        </div>

        <div>
          <label className="block text-sm font-medium mb-2">Criterion Type</label>
          <select
            value={criterionType}
            onChange={(e) => setCriterionType(e.target.value as CriterionType)}
            required
            className="w-full px-3 py-2 rounded-lg border border-border dark:border-border-dark
                       bg-panel dark:bg-panel-dark focus:outline-none focus:ring-2 focus:ring-accent"
          >
            <option value="">Select a criterion...</option>
            {availableTypes.map((ct) => (
              <option key={ct.type} value={ct.type}>
                {ct.name}
              </option>
            ))}
          </select>
          {selectedType && (
            <p className="mt-1 text-sm text-muted dark:text-muted-dark">
              {selectedType.description}
            </p>
          )}
        </div>

        {selectedType && selectedType.parameters.length > 0 && (
          <div className="space-y-4">
            <h3 className="text-sm font-medium">Parameters</h3>
            {selectedType.parameters.map((param) => (
              <div key={param.name}>
                <label className="block text-sm text-muted dark:text-muted-dark mb-1">
                  {param.label}
                </label>
                <input
                  type={param.type === 'int' ? 'number' : 'text'}
                  value={parameters[param.name] ?? param.default}
                  onChange={(e) =>
                    setParameters((prev) => {
                      let value: string | number = e.target.value
                      if (param.type === 'int') {
                        const parsed = parseInt(e.target.value, 10)
                        value = isNaN(parsed) ? param.default : parsed
                      }
                      return { ...prev, [param.name]: value }
                    })
                  }
                  min={param.min}
                  max={param.max}
                  className="w-full px-3 py-2 rounded-lg border border-border dark:border-border-dark
                             bg-panel dark:bg-panel-dark focus:outline-none focus:ring-2 focus:ring-accent"
                />
              </div>
            ))}
          </div>
        )}

        <div className="flex justify-end gap-3 pt-4">
          <button
            type="button"
            onClick={onBack}
            className="px-4 py-2 text-sm font-medium rounded-lg border border-border dark:border-border-dark
                       hover:bg-surface dark:hover:bg-surface-dark transition-colors"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={saving || !name || !criterionType}
            className="px-4 py-2 text-sm font-medium rounded-lg bg-accent text-gray-900 hover:bg-accent/90
                       disabled:opacity-50 transition-colors"
          >
            {saving ? 'Creating...' : 'Create Rule'}
          </button>
        </div>
      </form>
    </div>
  )
}

const criterionFormatters: Record<CriterionType, (params: Record<string, unknown>) => string> = {
  unwatched_movie: (p) => `Movies unwatched for ${p.days || 365} days`,
  unwatched_tv_none: (p) => `TV shows with no plays in ${p.days || 365} days`,
  unwatched_tv_low: (p) => `TV shows with <${p.max_percent || 10}% watched in ${p.days || 365} days`,
  low_resolution: (p) => `Resolution at or below ${p.max_height || 720}p`,
  large_files: (p) => `Files larger than ${p.min_size_gb || 10} GB`,
}

function formatRuleParameters(rule: MaintenanceRuleWithCount): string {
  const params = rule.parameters as Record<string, unknown>
  const formatter = criterionFormatters[rule.criterion_type]
  return formatter ? formatter(params) : JSON.stringify(params)
}

export function Maintenance() {
  const [view, setView] = useState<ViewState>({ type: 'dashboard' })
  const [syncingLibrary, setSyncingLibrary] = useState<string | null>(null)

  const { data, loading, error, refetch } = useFetch<MaintenanceDashboard>('/api/maintenance/dashboard')

  const handleSync = async (library: LibraryMaintenance) => {
    const key = getLibraryKey(library)
    setSyncingLibrary(key)
    try {
      await api.post('/api/maintenance/sync', {
        server_id: library.server_id,
        library_id: library.library_id,
      })
      refetch()
    } catch (err) {
      console.error('Sync failed:', err)
    } finally {
      setSyncingLibrary(null)
    }
  }

  if (view.type === 'library') {
    const freshLibrary = data?.libraries.find(
      (l) => l.server_id === view.library.server_id && l.library_id === view.library.library_id
    ) || view.library

    return (
      <LibraryView
        library={freshLibrary}
        onBack={() => setView({ type: 'dashboard' })}
        onViewCandidates={(rule) => setView({ type: 'candidates', rule, library: freshLibrary })}
        onCreateRule={() => setView({ type: 'create-rule', library: freshLibrary })}
        onRefresh={refetch}
      />
    )
  }

  if (view.type === 'candidates') {
    return (
      <CandidatesView
        rule={view.rule}
        library={view.library}
        onBack={() => setView({ type: 'library', library: view.library })}
      />
    )
  }

  if (view.type === 'create-rule') {
    return (
      <CreateRuleView
        library={view.library}
        onBack={() => setView({ type: 'library', library: view.library })}
        onCreated={() => {
          refetch()
          setView({ type: 'library', library: view.library })
        }}
      />
    )
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-semibold">Library Maintenance</h1>
        <p className="text-sm text-muted dark:text-muted-dark mt-1">
          Identify unwatched and low-quality content across your libraries
        </p>
      </div>

      {error && (
        <div className="card p-6 text-center text-red-500 dark:text-red-400">
          Error loading maintenance dashboard
        </div>
      )}

      {loading && !data && (
        <div className="card p-12 text-center">
          <div className="text-muted dark:text-muted-dark animate-pulse">Loading...</div>
        </div>
      )}

      {data && (
        <>
          {data.libraries.length === 0 ? (
            <div className="card p-8 text-center text-muted dark:text-muted-dark">
              No movie or TV libraries found. Configure a media server in Settings to get started.
            </div>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {data.libraries.map((library) => {
                const key = getLibraryKey(library)
                return (
                  <LibraryCard
                    key={key}
                    library={library}
                    onView={() => setView({ type: 'library', library })}
                    onSync={() => handleSync(library)}
                    syncing={syncingLibrary === key}
                  />
                )
              })}
            </div>
          )}
        </>
      )}
    </div>
  )
}
