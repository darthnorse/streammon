import { useState, useEffect, useCallback, useMemo } from 'react'
import { useFetch } from '../hooks/useFetch'
import { api } from '../lib/api'
import { PER_PAGE } from '../lib/constants'
import type {
  LibrariesResponse,
  Library,
  LibraryType,
  ServerType,
  MaintenanceDashboard,
  LibraryMaintenance,
  MaintenanceRuleWithCount,
  MaintenanceCandidatesResponse,
  CriterionTypeInfo,
  CriterionType,
} from '../types'

const serverAccent: Record<ServerType, string> = {
  plex: 'bg-warn/10 text-warn',
  emby: 'bg-emby/10 text-emby',
  jellyfin: 'bg-jellyfin/10 text-jellyfin',
}

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

type ViewState =
  | { type: 'list' }
  | { type: 'rules'; library: Library; maintenance: LibraryMaintenance | null }
  | { type: 'violations'; library: Library; maintenance: LibraryMaintenance }
  | { type: 'rule-form'; library: Library; maintenance: LibraryMaintenance | null; rule?: MaintenanceRuleWithCount }
  | { type: 'candidates'; library: Library; rule: MaintenanceRuleWithCount }

function formatCount(count: number): string {
  return count.toLocaleString()
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const value = bytes / Math.pow(1024, i)
  return `${value.toFixed(i > 1 ? 1 : 0)} ${units[i]}`
}

function getUniqueServers(libraries: Library[]): { id: number; name: string }[] {
  const seen = new Map<number, string>()
  for (const lib of libraries) {
    if (!seen.has(lib.server_id)) {
      seen.set(lib.server_id, lib.server_name)
    }
  }
  return Array.from(seen, ([id, name]) => ({ id, name }))
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

function getLibraryIcon(type: LibraryType): string {
  return libraryTypeIcon[type] || '▤'
}

function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="p-2 rounded-lg hover:bg-surface dark:hover:bg-surface-dark transition-colors"
      aria-label="Go back"
    >
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
      </svg>
    </button>
  )
}

interface SubViewHeaderProps {
  library: Library
  title: string
  subtitle: string
  onBack: () => void
}

function SubViewHeader({ library, title, subtitle, onBack }: SubViewHeaderProps) {
  return (
    <div className="flex items-center gap-4">
      <BackButton onClick={onBack} />
      <div className="flex items-center gap-3">
        <span className="text-2xl">{getLibraryIcon(library.type)}</span>
        <div>
          <h1 className="text-2xl font-semibold">{title}</h1>
          <p className="text-sm text-muted dark:text-muted-dark">{subtitle}</p>
        </div>
      </div>
    </div>
  )
}

interface LibraryRowProps {
  library: Library
  maintenance: LibraryMaintenance | null
  syncing: boolean
  onSync: () => void
  onRules: () => void
  onViolations: () => void
}

function LibraryRow({ library, maintenance, syncing, onSync, onRules, onViolations }: LibraryRowProps) {
  const accent = serverAccent[library.server_type] || 'bg-gray-100 text-gray-600'
  const icon = getLibraryIcon(library.type)
  const rules = maintenance?.rules || []
  const ruleCount = rules.length
  const violationCount = rules.reduce((sum, r) => sum + r.candidate_count, 0)
  const isMaintenanceSupported = library.type === 'movie' || library.type === 'show'

  return (
    <tr className="border-b border-border dark:border-border-dark hover:bg-gray-50 dark:hover:bg-white/5 transition-colors">
      <td className="px-4 py-3">
        <div className="flex items-center gap-3">
          <span className="text-xl">{icon}</span>
          <span className="font-medium text-gray-900 dark:text-gray-100">{library.name}</span>
        </div>
      </td>
      <td className="px-4 py-3">
        <span className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${accent}`}>
          {library.server_name}
        </span>
      </td>
      <td className="px-4 py-3 text-muted dark:text-muted-dark">
        {libraryTypeLabel[library.type]}
      </td>
      <td className="px-4 py-3 text-right font-medium text-gray-900 dark:text-gray-100">
        {formatCount(library.item_count)}
      </td>
      <td className="hidden md:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
        {library.child_count > 0 ? formatCount(library.child_count) : '-'}
      </td>
      <td className="hidden lg:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
        {library.grandchild_count > 0 ? formatCount(library.grandchild_count) : '-'}
      </td>
      <td className="hidden xl:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
        {formatSize(library.total_size)}
      </td>
      <td className="px-4 py-3 text-center">
        {isMaintenanceSupported ? (
          <button
            onClick={onRules}
            className="text-sm text-accent hover:underline"
          >
            {ruleCount}
          </button>
        ) : (
          <span className="text-muted dark:text-muted-dark">-</span>
        )}
      </td>
      <td className="px-4 py-3 text-center">
        {isMaintenanceSupported && violationCount > 0 ? (
          <button
            onClick={onViolations}
            className="text-sm text-amber-500 hover:underline font-medium"
          >
            {formatCount(violationCount)}
          </button>
        ) : (
          <span className="text-muted dark:text-muted-dark">-</span>
        )}
      </td>
      <td className="px-4 py-3">
        {isMaintenanceSupported && (
          <div className="flex items-center justify-end gap-2">
            <button
              onClick={onSync}
              disabled={syncing}
              className="px-2 py-1 text-xs font-medium rounded border border-border dark:border-border-dark
                       hover:bg-surface dark:hover:bg-surface-dark transition-colors disabled:opacity-50"
            >
              {syncing ? 'Syncing...' : 'Sync'}
            </button>
            <button
              onClick={onRules}
              className="px-2 py-1 text-xs font-medium rounded border border-border dark:border-border-dark
                       hover:bg-surface dark:hover:bg-surface-dark transition-colors"
            >
              Rules
            </button>
          </div>
        )}
      </td>
    </tr>
  )
}

function RulesView({
  library,
  maintenance,
  onBack,
  onEditRule,
  onCreateRule,
  onViewCandidates,
  onRefresh,
}: {
  library: Library
  maintenance: LibraryMaintenance | null
  onBack: () => void
  onEditRule: (rule: MaintenanceRuleWithCount) => void
  onCreateRule: () => void
  onViewCandidates: (rule: MaintenanceRuleWithCount) => void
  onRefresh: () => void
}) {
  const rules = maintenance?.rules || []
  const [operationError, setOperationError] = useState<string | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<MaintenanceRuleWithCount | null>(null)

  const handleToggleRule = async (rule: MaintenanceRuleWithCount) => {
    setOperationError(null)
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
      setOperationError(`Failed to ${rule.enabled ? 'disable' : 'enable'} rule "${rule.name}"`)
    }
  }

  const handleDeleteRule = async (rule: MaintenanceRuleWithCount) => {
    setOperationError(null)
    try {
      await api.del(`/api/maintenance/rules/${rule.id}`)
      setDeleteConfirm(null)
      onRefresh()
    } catch (err) {
      console.error('Failed to delete rule:', err)
      setOperationError(`Failed to delete rule "${rule.name}"`)
      setDeleteConfirm(null)
    }
  }

  const handleEvaluateRule = async (rule: MaintenanceRuleWithCount) => {
    setOperationError(null)
    try {
      await api.post(`/api/maintenance/rules/${rule.id}/evaluate`)
      onRefresh()
    } catch (err) {
      console.error('Failed to evaluate rule:', err)
      setOperationError(`Failed to evaluate rule "${rule.name}"`)
    }
  }

  return (
    <div className="space-y-6">
      <SubViewHeader
        library={library}
        title={library.name}
        subtitle={`${library.server_name} - Maintenance Rules`}
        onBack={onBack}
      />

      {operationError && (
        <div className="p-3 rounded-lg bg-red-500/10 text-red-500 text-sm flex items-center justify-between">
          <span>{operationError}</span>
          <button
            onClick={() => setOperationError(null)}
            className="ml-4 text-red-400 hover:text-red-300"
            aria-label="Dismiss error"
          >
            ✕
          </button>
        </div>
      )}

      {deleteConfirm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="card p-6 max-w-sm mx-4">
            <h3 className="text-lg font-semibold mb-2">Delete Rule</h3>
            <p className="text-muted dark:text-muted-dark mb-4">
              Are you sure you want to delete "{deleteConfirm.name}"? This action cannot be undone.
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setDeleteConfirm(null)}
                className="px-4 py-2 text-sm font-medium rounded-lg border border-border dark:border-border-dark
                         hover:bg-surface dark:hover:bg-surface-dark transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => handleDeleteRule(deleteConfirm)}
                className="px-4 py-2 text-sm font-medium rounded-lg bg-red-500 text-white hover:bg-red-600 transition-colors"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="flex items-center justify-between">
        <h2 className="text-lg font-medium">Rules</h2>
        <button
          onClick={onCreateRule}
          className="px-4 py-2 text-sm font-medium rounded-lg bg-accent text-gray-900 hover:bg-accent/90"
        >
          Add Rule
        </button>
      </div>

      {rules.length === 0 ? (
        <div className="card p-8 text-center text-muted dark:text-muted-dark">
          No maintenance rules configured for this library.
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
                      {rule.criterion_type.replace(/_/g, ' ')}
                    </span>
                  </div>
                  <p className="text-sm text-muted dark:text-muted-dark mt-1">
                    {formatRuleParameters(rule)}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-amber-500 font-medium text-sm">
                    {formatCount(rule.candidate_count)} violations
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
                    onClick={() => onEditRule(rule)}
                    className="p-1.5 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors"
                    title="Edit rule"
                    aria-label={`Edit rule ${rule.name}`}
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                    </svg>
                  </button>
                  <button
                    onClick={() => handleEvaluateRule(rule)}
                    className="p-1.5 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors"
                    title="Re-evaluate"
                    aria-label={`Re-evaluate rule ${rule.name}`}
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                    </svg>
                  </button>
                  <button
                    onClick={() => onViewCandidates(rule)}
                    className="p-1.5 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors"
                    title="View violations"
                    aria-label={`View violations for rule ${rule.name}`}
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                    </svg>
                  </button>
                  <button
                    onClick={() => setDeleteConfirm(rule)}
                    className="p-1.5 rounded hover:bg-red-500/20 text-red-400 transition-colors"
                    title="Delete"
                    aria-label={`Delete rule ${rule.name}`}
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

function ViolationsView({
  library,
  maintenance,
  onBack,
  onViewRule,
}: {
  library: Library
  maintenance: LibraryMaintenance
  onBack: () => void
  onViewRule: (rule: MaintenanceRuleWithCount) => void
}) {
  const rules = maintenance.rules || []
  const rulesWithViolations = useMemo(
    () => rules.filter(r => r.candidate_count > 0),
    [rules]
  )
  const totalViolations = useMemo(
    () => rules.reduce((sum, r) => sum + r.candidate_count, 0),
    [rules]
  )

  return (
    <div className="space-y-6">
      <SubViewHeader
        library={library}
        title={library.name}
        subtitle={`${library.server_name} - ${formatCount(totalViolations)} violations`}
        onBack={onBack}
      />

      {rulesWithViolations.length === 0 ? (
        <div className="card p-8 text-center text-muted dark:text-muted-dark">
          No violations found for this library.
        </div>
      ) : (
        <div className="space-y-3">
          {rulesWithViolations.map((rule) => (
            <button
              key={rule.id}
              onClick={() => onViewRule(rule)}
              className="card p-4 w-full text-left hover:bg-surface dark:hover:bg-surface-dark transition-colors"
            >
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-semibold">{rule.name}</h3>
                  <p className="text-sm text-muted dark:text-muted-dark mt-1">
                    {formatRuleParameters(rule)}
                  </p>
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-amber-500 font-medium">
                    {formatCount(rule.candidate_count)} violations
                  </span>
                  <svg className="w-5 h-5 text-muted dark:text-muted-dark" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                  </svg>
                </div>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

function CandidatesView({
  library,
  rule,
  onBack,
}: {
  library: Library
  rule: MaintenanceRuleWithCount
  onBack: () => void
}) {
  const [page, setPage] = useState(1)

  useEffect(() => {
    setPage(1)
  }, [rule.id])

  const { data, loading } = useFetch<MaintenanceCandidatesResponse>(
    `/api/maintenance/rules/${rule.id}/candidates?page=${page}&per_page=${PER_PAGE}`
  )

  const totalPages = data ? Math.ceil(data.total / PER_PAGE) : 0

  return (
    <div className="space-y-6">
      <SubViewHeader
        library={library}
        title={rule.name}
        subtitle={`${library.name} - ${data ? formatCount(data.total) : '0'} violations`}
        onBack={onBack}
      />

      {loading && !data ? (
        <div className="card p-12 text-center">
          <div className="text-muted dark:text-muted-dark animate-pulse">Loading...</div>
        </div>
      ) : !data?.items.length ? (
        <div className="card p-8 text-center text-muted dark:text-muted-dark">
          No violations found for this rule.
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
                        {candidate.item?.added_at ? new Date(candidate.item.added_at).toLocaleString() : '-'}
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

function RuleFormView({
  library,
  rule,
  onBack,
  onSaved,
}: {
  library: Library
  rule?: MaintenanceRuleWithCount
  onBack: () => void
  onSaved: () => void
}) {
  const isEdit = !!rule
  const { data: criterionTypes, loading: typesLoading, error: typesError } = useFetch<{ types: CriterionTypeInfo[] }>('/api/maintenance/criterion-types')
  const [name, setName] = useState(rule?.name || '')
  const [criterionType, setCriterionType] = useState<CriterionType | ''>(rule?.criterion_type || '')
  const [parameters, setParameters] = useState<Record<string, string | number>>(
    (rule?.parameters as Record<string, string | number>) || {}
  )
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const availableTypes = useMemo(() => {
    if (!criterionTypes?.types) return []
    return criterionTypes.types.filter((ct) => {
      if (library.type === 'movie') {
        return ct.media_types.includes('movie')
      }
      if (library.type === 'show') {
        return ct.media_types.includes('episode')
      }
      return false
    })
  }, [criterionTypes?.types, library.type])

  const selectedType = useMemo(
    () => availableTypes.find((ct) => ct.type === criterionType),
    [availableTypes, criterionType]
  )

  // Update parameters when criterion type changes
  useEffect(() => {
    if (!isEdit || (isEdit && criterionType !== rule?.criterion_type)) {
      const currentSelectedType = availableTypes.find((ct) => ct.type === criterionType)
      if (currentSelectedType) {
        const defaults: Record<string, string | number> = {}
        for (const param of currentSelectedType.parameters) {
          defaults[param.name] = param.default
        }
        setParameters(defaults)
      } else {
        setParameters({})
      }
    }
  }, [criterionType, isEdit, rule?.criterion_type, availableTypes])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!criterionType) return

    setSaving(true)
    setError(null)

    try {
      if (isEdit && rule) {
        await api.put(`/api/maintenance/rules/${rule.id}`, {
          name,
          criterion_type: criterionType,
          parameters,
          enabled: rule.enabled,
        })
      } else {
        await api.post('/api/maintenance/rules', {
          server_id: library.server_id,
          library_id: library.id,
          name,
          criterion_type: criterionType,
          parameters,
          enabled: true,
        })
      }
      onSaved()
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${isEdit ? 'update' : 'create'} rule`)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      <SubViewHeader
        library={library}
        title={`${isEdit ? 'Edit' : 'Create'} Rule`}
        subtitle={library.name}
        onBack={onBack}
      />

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
          {typesLoading ? (
            <div className="w-full px-3 py-2 rounded-lg border border-border dark:border-border-dark
                           bg-panel dark:bg-panel-dark text-muted dark:text-muted-dark">
              Loading criterion types...
            </div>
          ) : typesError ? (
            <div className="p-3 rounded-lg bg-red-500/10 text-red-500 text-sm">
              Failed to load criterion types. Please try again.
            </div>
          ) : (
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
          )}
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
            {saving ? (isEdit ? 'Saving...' : 'Creating...') : (isEdit ? 'Save Changes' : 'Create Rule')}
          </button>
        </div>
      </form>
    </div>
  )
}

export function Libraries() {
  const [selectedServer, setSelectedServer] = useState<number | 'all'>('all')
  const [view, setView] = useState<ViewState>({ type: 'list' })
  const [syncingLibrary, setSyncingLibrary] = useState<string | null>(null)
  const [syncError, setSyncError] = useState<string | null>(null)

  const { data, loading, error, refetch } = useFetch<LibrariesResponse>('/api/libraries')
  const { data: maintenanceData, refetch: refetchMaintenance } = useFetch<MaintenanceDashboard>('/api/maintenance/dashboard')

  const libraries = data?.libraries || []
  const servers = useMemo(() => getUniqueServers(libraries), [libraries])

  const displayedLibraries = useMemo(
    () => selectedServer === 'all'
      ? libraries
      : libraries.filter(l => l.server_id === selectedServer),
    [libraries, selectedServer]
  )

  const getMaintenanceForLibrary = useCallback((lib: Library): LibraryMaintenance | null => {
    return maintenanceData?.libraries.find(
      m => m.server_id === lib.server_id && m.library_id === lib.id
    ) || null
  }, [maintenanceData])

  // Handle violations view navigation when maintenance data becomes unavailable
  useEffect(() => {
    if (view.type === 'violations') {
      const freshMaintenance = getMaintenanceForLibrary(view.library)
      if (!freshMaintenance) {
        setView({ type: 'list' })
      }
    }
  }, [view, getMaintenanceForLibrary])

  const handleSync = async (library: Library) => {
    const key = `${library.server_id}-${library.id}`
    setSyncingLibrary(key)
    setSyncError(null)
    try {
      await api.post('/api/maintenance/sync', {
        server_id: library.server_id,
        library_id: library.id,
      })
      refetchMaintenance()
    } catch (err) {
      console.error('Sync failed:', err)
      setSyncError(`Failed to sync "${library.name}". Please try again.`)
    } finally {
      setSyncingLibrary(null)
    }
  }

  const handleRefresh = useCallback(() => {
    refetch()
    refetchMaintenance()
  }, [refetch, refetchMaintenance])

  const totals = useMemo(
    () => displayedLibraries.reduce(
      (acc, lib) => ({
        items: acc.items + lib.item_count,
        children: acc.children + lib.child_count,
        grandchildren: acc.grandchildren + lib.grandchild_count,
        totalSize: acc.totalSize + lib.total_size,
      }),
      { items: 0, children: 0, grandchildren: 0, totalSize: 0 }
    ),
    [displayedLibraries]
  )

  // Handle sub-views
  if (view.type === 'rules') {
    const freshMaintenance = getMaintenanceForLibrary(view.library)
    return (
      <RulesView
        library={view.library}
        maintenance={freshMaintenance}
        onBack={() => setView({ type: 'list' })}
        onEditRule={(rule) => setView({ type: 'rule-form', library: view.library, maintenance: freshMaintenance, rule })}
        onCreateRule={() => setView({ type: 'rule-form', library: view.library, maintenance: freshMaintenance })}
        onViewCandidates={(rule) => setView({ type: 'candidates', library: view.library, rule })}
        onRefresh={handleRefresh}
      />
    )
  }

  if (view.type === 'violations') {
    const freshMaintenance = getMaintenanceForLibrary(view.library)
    if (!freshMaintenance) {
      // Will be handled by useEffect above, return loading state
      return (
        <div className="card p-12 text-center">
          <div className="text-muted dark:text-muted-dark animate-pulse">Loading...</div>
        </div>
      )
    }
    return (
      <ViolationsView
        library={view.library}
        maintenance={freshMaintenance}
        onBack={() => setView({ type: 'list' })}
        onViewRule={(rule) => setView({ type: 'candidates', library: view.library, rule })}
      />
    )
  }

  if (view.type === 'candidates') {
    return (
      <CandidatesView
        library={view.library}
        rule={view.rule}
        onBack={() => {
          const maintenance = getMaintenanceForLibrary(view.library)
          if (maintenance) {
            setView({ type: 'violations', library: view.library, maintenance })
          } else {
            setView({ type: 'list' })
          }
        }}
      />
    )
  }

  if (view.type === 'rule-form') {
    return (
      <RuleFormView
        library={view.library}
        rule={view.rule}
        onBack={() => setView({ type: 'rules', library: view.library, maintenance: view.maintenance })}
        onSaved={() => {
          handleRefresh()
          setView({ type: 'rules', library: view.library, maintenance: view.maintenance })
        }}
      />
    )
  }

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <div>
          <h1 className="text-2xl font-semibold">Libraries</h1>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            Content and maintenance across all media servers
          </p>
        </div>

        {servers.length > 1 && (
          <div className="flex items-center gap-2">
            <label htmlFor="server-filter" className="text-sm text-muted dark:text-muted-dark">
              Server
            </label>
            <select
              id="server-filter"
              value={selectedServer}
              onChange={(e) => setSelectedServer(e.target.value === 'all' ? 'all' : Number(e.target.value))}
              className="px-3 py-1.5 text-sm rounded border border-border dark:border-border-dark bg-panel dark:bg-panel-dark text-gray-900 dark:text-gray-100"
            >
              <option value="all">All Servers</option>
              {servers.map(server => (
                <option key={server.id} value={server.id}>
                  {server.name}
                </option>
              ))}
            </select>
          </div>
        )}
      </div>

      {syncError && (
        <div className="mb-4 p-3 rounded-lg bg-red-500/10 text-red-500 text-sm flex items-center justify-between">
          <span>{syncError}</span>
          <button
            onClick={() => setSyncError(null)}
            className="ml-4 text-red-400 hover:text-red-300"
            aria-label="Dismiss error"
          >
            ✕
          </button>
        </div>
      )}

      {error && (
        <div className="card p-6 text-center text-red-500 dark:text-red-400">
          Error loading libraries
        </div>
      )}

      {loading && !data && (
        <div className="card p-12 text-center">
          <div className="text-muted dark:text-muted-dark animate-pulse">Loading...</div>
        </div>
      )}

      {data && (
        <div className="card overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50 dark:bg-white/5 border-b border-border dark:border-border-dark">
                  <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Library
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Server
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Type
                  </th>
                  <th className="px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Items
                  </th>
                  <th className="hidden md:table-cell px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Seasons / Albums
                  </th>
                  <th className="hidden lg:table-cell px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Episodes / Tracks
                  </th>
                  <th className="hidden xl:table-cell px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Total Size
                  </th>
                  <th className="px-4 py-3 text-center text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Rules
                  </th>
                  <th className="px-4 py-3 text-center text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Violations
                  </th>
                  <th className="px-4 py-3 text-right text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {displayedLibraries.map(library => {
                  const maintenance = getMaintenanceForLibrary(library)
                  const key = `${library.server_id}-${library.id}`
                  return (
                    <LibraryRow
                      key={key}
                      library={library}
                      maintenance={maintenance}
                      syncing={syncingLibrary === key}
                      onSync={() => handleSync(library)}
                      onRules={() => setView({ type: 'rules', library, maintenance })}
                      onViolations={() => maintenance && setView({ type: 'violations', library, maintenance })}
                    />
                  )
                })}
                {displayedLibraries.length === 0 && (
                  <tr>
                    <td colSpan={10} className="px-4 py-12 text-center">
                      <div className="text-4xl mb-3 opacity-30">▤</div>
                      <p className="text-muted dark:text-muted-dark">No libraries found</p>
                    </td>
                  </tr>
                )}
              </tbody>
              {displayedLibraries.length > 0 && (
                <tfoot>
                  <tr className="bg-gray-50 dark:bg-white/5 border-t border-border dark:border-border-dark font-semibold">
                    <td className="px-4 py-3 text-gray-900 dark:text-gray-100" colSpan={3}>
                      Total ({displayedLibraries.length} {displayedLibraries.length === 1 ? 'library' : 'libraries'})
                    </td>
                    <td className="px-4 py-3 text-right text-gray-900 dark:text-gray-100">
                      {formatCount(totals.items)}
                    </td>
                    <td className="hidden md:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
                      {totals.children > 0 ? formatCount(totals.children) : '-'}
                    </td>
                    <td className="hidden lg:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
                      {totals.grandchildren > 0 ? formatCount(totals.grandchildren) : '-'}
                    </td>
                    <td className="hidden xl:table-cell px-4 py-3 text-right text-muted dark:text-muted-dark">
                      {formatSize(totals.totalSize)}
                    </td>
                    <td className="px-4 py-3" colSpan={3}></td>
                  </tr>
                </tfoot>
              )}
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
