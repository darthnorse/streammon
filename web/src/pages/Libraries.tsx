import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useMultiSelect } from '../hooks/useMultiSelect'
import { useMountedRef } from '../hooks/useMountedRef'
import { useDebouncedSearch } from '../hooks/useDebouncedSearch'
import { usePersistedPerPage } from '../hooks/usePersistedPerPage'
import { useItemDetails } from '../hooks/useItemDetails'
import { api, ApiError } from '../lib/api'
import { PER_PAGE_OPTIONS } from '../lib/constants'
import { readSSEStream } from '../lib/sse'
import { errorMessage } from '../lib/utils'
import { formatCount, formatSize } from '../lib/format'
import { Pagination } from '../components/Pagination'
import { MediaDetailModal } from '../components/MediaDetailModal'
import {
  SubViewHeader,
  SearchInput,
  SelectionActionBar,
  ConfirmDialog,
  OperationResult,
  DeleteProgressModal,
  type DeleteProgress,
} from '../components/shared'
import type {
  LibrariesResponse,
  Library,
  LibraryType,
  LibraryItemCache,
  ServerType,
  MaintenanceDashboard,
  LibraryMaintenance,
  MaintenanceRuleWithCount,
  MaintenanceCandidatesResponse,
  MaintenanceExclusionsResponse,
  MaintenanceCandidate,
  BulkDeleteResult,
  CriterionTypeInfo,
  CriterionType,
  SyncProgress,
} from '../types'

function useMediaDetailModal() {
  const [selectedItem, setSelectedItem] = useState<{ serverId: number; itemId: string } | null>(null)
  const { data: itemDetails, loading: detailsLoading } = useItemDetails(
    selectedItem?.serverId ?? 0,
    selectedItem?.itemId ?? null
  )
  const modal = selectedItem ? (
    <MediaDetailModal
      item={itemDetails}
      loading={detailsLoading}
      onClose={() => setSelectedItem(null)}
    />
  ) : null
  return { setSelectedItem, modal }
}

function ItemTitleButton({ item, onSelect }: { item?: LibraryItemCache; onSelect: (serverId: number, itemId: string) => void }) {
  if (!item) return <>Unknown</>
  return (
    <button
      onClick={() => onSelect(item.server_id, item.item_id)}
      className="text-left hover:text-accent hover:underline"
      aria-label={`View details for ${item.title}`}
    >
      {item.title}
    </button>
  )
}

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
  | { type: 'exclusions'; library: Library; rule: MaintenanceRuleWithCount }

function syncKey(lib: Library): string {
  return `${lib.server_id}-${lib.id}`
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
  unwatched_movie: (p) => `Movies not watched in ${p.days || 365} days`,
  unwatched_tv_none: (p) => `TV shows with no watch activity for ${p.days || 365}+ days`,
  low_resolution: (p) => `Resolution at or below ${p.max_height || 720}p`,
  large_files: (p) => `Files larger than ${p.min_size_gb || 10} GB`,
}

function formatRuleParameters(rule: MaintenanceRuleWithCount): string {
  const params = rule.parameters
  return criterionFormatters[rule.criterion_type]?.(params) ?? JSON.stringify(params)
}

function LibrarySubViewHeader({
  library,
  title,
  subtitle,
  onBack,
}: {
  library: Library
  title: string
  subtitle: string
  onBack: () => void
}) {
  return (
    <SubViewHeader
      icon={libraryTypeIcon[library.type]}
      title={title}
      subtitle={subtitle}
      onBack={onBack}
    />
  )
}

interface LibraryRowProps {
  library: Library
  maintenance: LibraryMaintenance | null
  syncState: SyncProgress | null
  onSync: () => void
  onRules: () => void
  onViolations: () => void
}

function formatSyncStatus(state: SyncProgress): string {
  if (state.phase === 'items') {
    if (state.total) {
      return `Scanning ${state.current ?? 0}/${state.total}`
    }
    return 'Scanning...'
  }
  if (state.phase === 'history') {
    if (state.total) {
      return `History ${state.current ?? 0}/${state.total}`
    }
    return 'Fetching history...'
  }
  return 'Syncing...'
}

function LibraryRow({ library, maintenance, syncState, onSync, onRules, onViolations }: LibraryRowProps) {
  const accent = serverAccent[library.server_type] || 'bg-gray-100 text-gray-600'
  const icon = libraryTypeIcon[library.type]
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
            className="text-sm hover:text-accent hover:underline"
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
            <span className="relative group/sync">
              <button
                onClick={onSync}
                disabled={!!syncState}
                className="px-2 py-1 text-xs font-medium rounded border border-border dark:border-border-dark
                         hover:bg-surface dark:hover:bg-surface-dark transition-colors disabled:opacity-50"
              >
                {syncState ? formatSyncStatus(syncState) : 'Sync'}
                {syncState?.phase === 'history' && syncState.total && (
                  <span className="ml-0.5 opacity-60">&#9432;</span>
                )}
              </button>
              {syncState?.phase === 'history' && syncState.total && (
                <div className="absolute bottom-full right-0 mb-1 px-2 py-1 text-xs rounded w-48
                              bg-gray-900 text-white dark:bg-gray-700
                              opacity-0 group-hover/sync:opacity-100 pointer-events-none transition-opacity z-50">
                  This is total watch history, not episode count — includes rewatches
                </div>
              )}
            </span>
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
  onViewExclusions,
  onRefresh,
}: {
  library: Library
  maintenance: LibraryMaintenance | null
  onBack: () => void
  onEditRule: (rule: MaintenanceRuleWithCount) => void
  onCreateRule: () => void
  onViewCandidates: (rule: MaintenanceRuleWithCount) => void
  onViewExclusions: (rule: MaintenanceRuleWithCount) => void
  onRefresh: () => void
}) {
  const rules = maintenance?.rules || []
  const [operationError, setOperationError] = useState<string | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<MaintenanceRuleWithCount | null>(null)
  const mountedRef = useMountedRef()

  const handleToggleRule = async (rule: MaintenanceRuleWithCount) => {
    setOperationError(null)
    try {
      await api.put(`/api/maintenance/rules/${rule.id}`, {
        name: rule.name,
        criterion_type: rule.criterion_type,
        parameters: rule.parameters,
        enabled: !rule.enabled,
      })
      if (mountedRef.current) onRefresh()
    } catch (err) {
      console.error('Failed to toggle rule:', err)
      if (mountedRef.current) setOperationError(`Failed to ${rule.enabled ? 'disable' : 'enable'} rule "${rule.name}"`)
    }
  }

  const handleDeleteRule = async (rule: MaintenanceRuleWithCount) => {
    setOperationError(null)
    try {
      await api.del(`/api/maintenance/rules/${rule.id}`)
      if (mountedRef.current) {
        setDeleteConfirm(null)
        onRefresh()
      }
    } catch (err) {
      console.error('Failed to delete rule:', err)
      if (mountedRef.current) {
        setOperationError(`Failed to delete rule "${rule.name}"`)
        setDeleteConfirm(null)
      }
    }
  }

  const handleEvaluateRule = async (rule: MaintenanceRuleWithCount) => {
    setOperationError(null)
    try {
      await api.post(`/api/maintenance/rules/${rule.id}/evaluate`)
      if (mountedRef.current) onRefresh()
    } catch (err) {
      console.error('Failed to evaluate rule:', err)
      if (mountedRef.current) setOperationError(`Failed to evaluate rule "${rule.name}"`)
    }
  }

  return (
    <div className="space-y-6">
      <LibrarySubViewHeader
        library={library}
        title={library.name}
        subtitle={`${library.server_name} - Maintenance Rules`}
        onBack={onBack}
      />

      {operationError && (
        <OperationResult
          type="error"
          message={operationError}
          onDismiss={() => setOperationError(null)}
        />
      )}

      {deleteConfirm && (
        <ConfirmDialog
          title="Delete Rule"
          message={`Are you sure you want to delete "${deleteConfirm.name}"? This action cannot be undone.`}
          confirmLabel="Delete"
          onConfirm={() => handleDeleteRule(deleteConfirm)}
          onCancel={() => setDeleteConfirm(null)}
          isDestructive
        />
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
              <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3 sm:gap-4">
                <div className="min-w-0">
                  <div className="flex items-center gap-3 flex-wrap">
                    <h3 className="font-semibold truncate">{rule.name}</h3>
                    <span className="px-2 py-0.5 text-xs rounded-full bg-surface dark:bg-surface-dark whitespace-nowrap">
                      {rule.criterion_type.replace(/_/g, ' ')}
                    </span>
                  </div>
                  <p className="text-sm text-muted dark:text-muted-dark mt-1">
                    {formatRuleParameters(rule)}
                  </p>
                </div>
                <div className="flex items-center gap-2 flex-wrap">
                  <div className="flex items-center gap-2">
                    {rule.candidate_count > 0 ? (
                      <button
                        onClick={() => onViewCandidates(rule)}
                        className="text-amber-500 font-medium text-sm hover:underline whitespace-nowrap"
                        aria-label={`View ${rule.candidate_count} violations for rule ${rule.name}`}
                      >
                        {formatCount(rule.candidate_count)} violations
                      </button>
                    ) : (
                      <span className="text-amber-500 font-medium text-sm whitespace-nowrap">
                        0 violations
                      </span>
                    )}
                    {rule.exclusion_count > 0 && (
                      <>
                        <span className="text-muted dark:text-muted-dark text-sm" aria-hidden="true">·</span>
                        <button
                          onClick={() => onViewExclusions(rule)}
                          className="text-muted dark:text-muted-dark font-medium text-sm hover:underline whitespace-nowrap"
                          aria-label={`View ${rule.exclusion_count} exclusions for rule ${rule.name}`}
                        >
                          {formatCount(rule.exclusion_count)} excluded
                        </button>
                      </>
                    )}
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
      <LibrarySubViewHeader
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
  onManageExclusions,
}: {
  library: Library
  rule: MaintenanceRuleWithCount
  onBack: () => void
  onManageExclusions: () => void
}) {
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = usePersistedPerPage()
  const [deleteConfirm, setDeleteConfirm] = useState<MaintenanceCandidate[] | null>(null)
  const [excludeConfirm, setExcludeConfirm] = useState<MaintenanceCandidate[] | null>(null)
  const [showDetails, setShowDetails] = useState(false)
  const [operating, setOperating] = useState(false)
  const [deleteProgress, setDeleteProgress] = useState<DeleteProgress | null>(null)
  const [operationResult, setOperationResult] = useState<{ type: 'success' | 'partial' | 'error'; message: string; errors?: Array<{ title: string; error: string }> } | null>(null)
  const [rowMenuOpen, setRowMenuOpen] = useState<number | null>(null)
  const [menuPos, setMenuPos] = useState<{ top: number; right: number } | null>(null)
  const { setSelectedItem, modal: detailModal } = useMediaDetailModal()
  const mountedRef = useMountedRef()
  const deleteAbortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    return () => { deleteAbortRef.current?.abort() }
  }, [])

  const { searchInput, setSearchInput, search, resetSearch } = useDebouncedSearch(() => setPage(1))

  const searchParam = search ? `&search=${encodeURIComponent(search)}` : ''
  const { data, loading, refetch } = useFetch<MaintenanceCandidatesResponse>(
    `/api/maintenance/rules/${rule.id}/candidates?page=${page}&per_page=${perPage}${searchParam}`
  )

  const totalPages = data ? Math.ceil(data.total / perPage) : 0
  const filteredItems = data?.items || []

  const {
    selected,
    toggleSelect,
    toggleSelectAll,
    clearSelection,
    allVisibleSelected,
    selectedItems,
  } = useMultiSelect(
    filteredItems,
    (c) => c.id
  )

  const hasSelection = selected.size > 0
  const selectedSize = hasSelection
    ? selectedItems.reduce((sum, c) => sum + (c.item?.file_size || 0), 0)
    : 0
  const menuCandidate = rowMenuOpen !== null ? filteredItems.find(c => c.id === rowMenuOpen) ?? null : null

  useEffect(() => {
    setPage(1)
    clearSelection()
    resetSearch()
  }, [rule.id, clearSelection, resetSearch])

  // Close row menu when data changes (page/search navigations)
  useEffect(() => {
    setRowMenuOpen(null)
    setMenuPos(null)
  }, [data])

  // Clamp page to valid range after data changes (e.g., after delete)
  useEffect(() => {
    if (totalPages > 0 && page > totalPages) {
      setPage(totalPages)
    }
  }, [totalPages, page])

  const handleBulkDelete = async () => {
    if (!deleteConfirm) return
    setOperating(true)
    setDeleteConfirm(null)
    setOperationResult(null)

    const candidateIds = deleteConfirm.map(c => c.id)

    setDeleteProgress({
      current: 0,
      total: candidateIds.length,
      title: 'Starting...',
      status: 'deleting',
      deleted: 0,
      failed: 0,
      skipped: 0,
      total_size: 0,
    })

    deleteAbortRef.current?.abort()
    const abortController = new AbortController()
    deleteAbortRef.current = abortController

    try {
      const response = await fetch('/api/maintenance/candidates/bulk-delete', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'text/event-stream',
        },
        body: JSON.stringify({ candidate_ids: candidateIds }),
        signal: abortController.signal,
      })

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      const ref: { result: BulkDeleteResult | null } = { result: null }

      await readSSEStream(response, {
        onData(raw) {
          try {
            if (mountedRef.current) setDeleteProgress(JSON.parse(raw) as DeleteProgress)
          } catch {
            // Skip malformed SSE data
          }
        },
        onEvent(event, raw) {
          if (event === 'complete') {
            try {
              ref.result = JSON.parse(raw) as BulkDeleteResult
            } catch {
              // Skip malformed SSE data
            }
          }
        },
      })

      if (!mountedRef.current) return

      const finalResult = ref.result
      if (finalResult) {
        const errorDetails = finalResult.errors?.map(e => ({ title: e.title, error: e.error })) ?? []
        const totalRequested = finalResult.deleted + finalResult.failed + finalResult.skipped

        if (finalResult.failed === 0 && finalResult.skipped === 0) {
          setOperationResult({ type: 'success', message: `Deleted ${finalResult.deleted} items (${formatSize(finalResult.total_size)} reclaimed)` })
        } else if (finalResult.deleted > 0) {
          let msg = `Deleted ${finalResult.deleted} of ${totalRequested} items.`
          if (finalResult.failed > 0) msg += ` ${finalResult.failed} failed.`
          if (finalResult.skipped > 0) msg += ` ${finalResult.skipped} skipped (excluded).`
          setOperationResult({ type: 'partial', message: msg, errors: errorDetails })
        } else if (finalResult.skipped > 0 && finalResult.failed === 0) {
          setOperationResult({ type: 'partial', message: `All ${finalResult.skipped} items were skipped (excluded since page load)`, errors: errorDetails })
        } else {
          setOperationResult({ type: 'error', message: `Failed to delete items`, errors: errorDetails })
        }
      }

      clearSelection()
      refetch()
    } catch (err) {
      if (abortController.signal.aborted) return
      console.error('Bulk delete failed:', err)
      if (mountedRef.current) setOperationResult({ type: 'error', message: 'Failed to delete items' })
    } finally {
      if (mountedRef.current) {
        setOperating(false)
        setDeleteProgress(null)
      }
    }
  }

  const handleBulkExclude = async () => {
    if (!excludeConfirm) return
    setOperating(true)
    setOperationResult(null)
    try {
      await api.post(`/api/maintenance/rules/${rule.id}/exclusions`, {
        library_item_ids: excludeConfirm.map(c => c.library_item_id)
      })
      if (!mountedRef.current) return
      setOperationResult({ type: 'success', message: `Excluded ${excludeConfirm.length} items from this rule` })
      clearSelection()
      refetch()
    } catch (err) {
      console.error('Bulk exclude failed:', err)
      if (mountedRef.current) setOperationResult({ type: 'error', message: 'Failed to exclude items' })
    } finally {
      if (mountedRef.current) {
        setOperating(false)
        setExcludeConfirm(null)
      }
    }
  }

  const closeRowMenu = () => {
    setRowMenuOpen(null)
    setMenuPos(null)
  }

  const handleSingleDelete = (candidate: MaintenanceCandidate) => {
    closeRowMenu()
    setShowDetails(false)
    setDeleteConfirm([candidate])
  }

  const handleSingleExclude = (candidate: MaintenanceCandidate) => {
    closeRowMenu()
    setExcludeConfirm([candidate])
  }

  const handleBulkDeleteClick = () => {
    setShowDetails(false)
    setDeleteConfirm(selectedItems)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <LibrarySubViewHeader
          library={library}
          title={rule.name}
          subtitle={`${library.name} - ${data ? formatCount(data.total) : '0'} violations`}
          onBack={onBack}
        />
        <button
          onClick={onManageExclusions}
          className="px-3 py-1.5 text-sm font-medium rounded border border-border dark:border-border-dark
                     hover:bg-surface dark:hover:bg-surface-dark transition-colors"
        >
          Manage Exclusions
        </button>
      </div>

      {operationResult && (
        <OperationResult
          type={operationResult.type}
          message={operationResult.message}
          onDismiss={() => setOperationResult(null)}
          errors={operationResult.errors}
        />
      )}

      <div className="grid grid-cols-3 gap-4">
        <div className="card p-4">
          <div className="text-sm text-muted dark:text-muted-dark mb-1">
            {hasSelection ? 'Selected' : 'Items'}
          </div>
          <div className="text-2xl font-semibold">
            {formatCount(hasSelection ? selected.size : (data?.total ?? 0))}
          </div>
        </div>
        <div className="card p-4">
          <div className="text-sm text-muted dark:text-muted-dark mb-1">
            {hasSelection ? 'Selected Size' : 'Total Size'}
          </div>
          <div className="text-2xl font-semibold">
            {formatSize(hasSelection ? selectedSize : (data?.total_size ?? 0))}
          </div>
        </div>
        <div className="card p-4">
          <div className="text-sm text-muted dark:text-muted-dark mb-1">Exclusions</div>
          <div className="text-2xl font-semibold">
            {formatCount(data?.exclusion_count ?? 0)}
          </div>
        </div>
      </div>

      <div className="flex gap-4 items-center">
        <SearchInput
          value={searchInput}
          onChange={setSearchInput}
          placeholder="Search title, year, resolution..."
        />
        <div className="flex items-center gap-2 text-sm text-muted dark:text-muted-dark">
          <label htmlFor="candidates-per-page">Show</label>
          <select
            id="candidates-per-page"
            value={perPage}
            onChange={(e) => { setPerPage(Number(e.target.value)); setPage(1); clearSelection() }}
            className="px-2 py-1 rounded border border-border dark:border-border-dark bg-panel dark:bg-panel-dark"
          >
            {PER_PAGE_OPTIONS.map(n => <option key={n} value={n}>{n}</option>)}
          </select>
        </div>
      </div>

      <SelectionActionBar selectedCount={selected.size}>
        <button
          onClick={() => setExcludeConfirm(selectedItems)}
          className="px-3 py-1.5 text-sm font-medium rounded border border-border dark:border-border-dark
                     hover:bg-panel dark:hover:bg-panel-dark transition-colors"
        >
          Exclude Selected
        </button>
        <button
          onClick={handleBulkDeleteClick}
          className="px-3 py-1.5 text-sm font-medium rounded bg-red-500 text-white hover:bg-red-600 transition-colors"
        >
          Delete Selected
        </button>
      </SelectionActionBar>

      {loading && !data ? (
        <div className="card p-12 text-center">
          <div className="text-muted dark:text-muted-dark animate-pulse">Loading...</div>
        </div>
      ) : !filteredItems.length ? (
        <div className="card p-8 text-center text-muted dark:text-muted-dark">
          {search ? 'No matching items found.' : 'No violations found for this rule.'}
        </div>
      ) : (
        <>
          <div className="card">
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="bg-gray-50 dark:bg-white/5 border-b border-border dark:border-border-dark">
                    <th className="px-4 py-3 w-10">
                      <input
                        type="checkbox"
                        checked={allVisibleSelected}
                        onChange={toggleSelectAll}
                        className="rounded border-border dark:border-border-dark"
                      />
                    </th>
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
                      Size
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                      Reason
                    </th>
                    <th className="px-4 py-3 w-10"></th>
                  </tr>
                </thead>
                <tbody>
                  {filteredItems.map((candidate) => (
                    <tr
                      key={candidate.id}
                      className={`border-b border-border dark:border-border-dark hover:bg-gray-50 dark:hover:bg-white/5 transition-colors
                        ${selected.has(candidate.id) ? 'bg-accent/5' : ''}`}
                    >
                      <td className="px-4 py-3">
                        <input
                          type="checkbox"
                          checked={selected.has(candidate.id)}
                          onChange={() => toggleSelect(candidate.id)}
                          className="rounded border-border dark:border-border-dark"
                        />
                      </td>
                      <td className="px-4 py-3 font-medium">
                        <ItemTitleButton item={candidate.item} onSelect={(s, i) => setSelectedItem({ serverId: s, itemId: i })} />
                      </td>
                      <td className="px-4 py-3 text-muted dark:text-muted-dark">
                        {candidate.item?.year || '-'}
                      </td>
                      <td className="px-4 py-3 text-muted dark:text-muted-dark">
                        {candidate.item?.video_resolution || '-'}
                      </td>
                      <td className="px-4 py-3 text-muted dark:text-muted-dark">
                        {candidate.item?.file_size ? formatSize(candidate.item.file_size) : '-'}
                      </td>
                      <td className="px-4 py-3 text-sm text-amber-500">
                        {candidate.reason}
                      </td>
                      <td className="px-4 py-3">
                        <button
                          onClick={(e) => {
                            if (rowMenuOpen === candidate.id) {
                              closeRowMenu()
                            } else {
                              const rect = e.currentTarget.getBoundingClientRect()
                              setMenuPos({ top: rect.bottom + 4, right: window.innerWidth - rect.right })
                              setRowMenuOpen(candidate.id)
                            }
                          }}
                          className="p-1 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors"
                        >
                          <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                            <circle cx="12" cy="5" r="2" />
                            <circle cx="12" cy="12" r="2" />
                            <circle cx="12" cy="19" r="2" />
                          </svg>
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={(p) => { setPage(p); clearSelection() }}
          />
        </>
      )}

      {menuCandidate && menuPos && (
        <>
          <div className="fixed inset-0 z-10" onClick={closeRowMenu} />
          <div
            className="fixed z-20 bg-panel dark:bg-panel-dark border border-border dark:border-border-dark rounded-lg shadow-lg min-w-[140px]"
            style={{ top: menuPos.top, right: menuPos.right }}
          >
            <button
              onClick={() => handleSingleExclude(menuCandidate)}
              className="w-full px-4 py-2 text-left text-sm hover:bg-surface dark:hover:bg-surface-dark transition-colors rounded-t-lg"
            >
              Exclude from Rule
            </button>
            <button
              onClick={() => handleSingleDelete(menuCandidate)}
              className="w-full px-4 py-2 text-left text-sm text-red-500 hover:bg-surface dark:hover:bg-surface-dark transition-colors rounded-b-lg"
            >
              Delete
            </button>
          </div>
        </>
      )}

      {deleteConfirm && (
        <ConfirmDialog
          title={`Delete ${deleteConfirm.length} item${deleteConfirm.length > 1 ? 's' : ''} from ${library.server_name}?`}
          message={
            <>
              <p className="mb-2">
                This will permanently delete these files from your media server. Cannot be undone.
              </p>
              <p className="text-sm font-medium">
                Total size: {formatSize(deleteConfirm.reduce((sum, c) => sum + (c.item?.file_size || 0), 0))}
              </p>
            </>
          }
          confirmLabel={operating ? 'Deleting...' : `Delete ${deleteConfirm.length} Item${deleteConfirm.length > 1 ? 's' : ''}`}
          onConfirm={handleBulkDelete}
          onCancel={() => { setDeleteConfirm(null); setShowDetails(false) }}
          isDestructive
          disabled={operating}
        >
          <button
            onClick={() => setShowDetails(!showDetails)}
            className="text-sm text-accent hover:underline mb-3 flex items-center gap-1"
          >
            {showDetails ? '▼ Hide details' : '▶ Show details'}
          </button>
          {showDetails && (
            <div className="max-h-40 overflow-y-auto mb-4 p-2 rounded bg-surface dark:bg-surface-dark text-sm">
              {deleteConfirm.map(c => (
                <div key={c.id} className="py-1">
                  • {c.item?.title} ({c.item?.year}) - {formatSize(c.item?.file_size || 0)}
                </div>
              ))}
            </div>
          )}
        </ConfirmDialog>
      )}

      {excludeConfirm && (
        <ConfirmDialog
          title={`Exclude ${excludeConfirm.length} item${excludeConfirm.length > 1 ? 's' : ''} from this rule?`}
          message="They won't appear in future evaluations of this rule. You can manage exclusions later."
          confirmLabel={operating ? 'Excluding...' : 'Exclude'}
          onConfirm={handleBulkExclude}
          onCancel={() => setExcludeConfirm(null)}
          disabled={operating}
        />
      )}

      {deleteProgress && (
        <DeleteProgressModal progress={deleteProgress} />
      )}

      {detailModal}
    </div>
  )
}

function ExclusionsView({
  library,
  rule,
  onBack,
}: {
  library: Library
  rule: MaintenanceRuleWithCount
  onBack: () => void
}) {
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = usePersistedPerPage()
  const [operating, setOperating] = useState(false)
  const [operationResult, setOperationResult] = useState<{ type: 'success' | 'error'; message: string } | null>(null)
  const { setSelectedItem, modal: detailModal } = useMediaDetailModal()
  const mountedRef = useMountedRef()

  const { searchInput, setSearchInput, search } = useDebouncedSearch(() => setPage(1))

  const searchParam = search ? `&search=${encodeURIComponent(search)}` : ''
  const { data, loading, refetch } = useFetch<MaintenanceExclusionsResponse>(
    `/api/maintenance/rules/${rule.id}/exclusions?page=${page}&per_page=${perPage}${searchParam}`
  )

  const totalPages = data ? Math.ceil(data.total / perPage) : 0
  const filteredItems = data?.items || []

  const {
    selected,
    toggleSelect,
    toggleSelectAll,
    clearSelection,
    allVisibleSelected,
  } = useMultiSelect(
    filteredItems,
    (e) => e.library_item_id
  )

  // Clamp page to valid range after data changes (e.g., after removal)
  useEffect(() => {
    if (totalPages > 0 && page > totalPages) {
      setPage(totalPages)
    }
  }, [totalPages, page])

  const handleRemoveExclusions = async () => {
    if (selected.size === 0) return
    setOperating(true)
    setOperationResult(null)
    try {
      await api.post(`/api/maintenance/rules/${rule.id}/exclusions/bulk-remove`, {
        library_item_ids: Array.from(selected)
      })
      if (!mountedRef.current) return
      setOperationResult({ type: 'success', message: `Removed ${selected.size} exclusions` })
      clearSelection()
      refetch()
    } catch (err) {
      console.error('Remove exclusions failed:', err)
      if (mountedRef.current) setOperationResult({ type: 'error', message: 'Failed to remove exclusions' })
    } finally {
      if (mountedRef.current) setOperating(false)
    }
  }

  return (
    <div className="space-y-6">
      <LibrarySubViewHeader
        library={library}
        title="Excluded Items"
        subtitle={`Rule: ${rule.name} - ${data ? formatCount(data.total) : '0'} excluded`}
        onBack={onBack}
      />

      {operationResult && (
        <OperationResult
          type={operationResult.type}
          message={operationResult.message}
          onDismiss={() => setOperationResult(null)}
        />
      )}

      <div className="flex gap-4 items-center">
        <SearchInput
          value={searchInput}
          onChange={setSearchInput}
          placeholder="Search title, year, resolution..."
        />
        <div className="flex items-center gap-2 text-sm text-muted dark:text-muted-dark">
          <label htmlFor="exclusions-per-page">Show</label>
          <select
            id="exclusions-per-page"
            value={perPage}
            onChange={(e) => { setPerPage(Number(e.target.value)); setPage(1); clearSelection() }}
            className="px-2 py-1 rounded border border-border dark:border-border-dark bg-panel dark:bg-panel-dark"
          >
            {PER_PAGE_OPTIONS.map(n => <option key={n} value={n}>{n}</option>)}
          </select>
        </div>
      </div>

      <SelectionActionBar selectedCount={selected.size}>
        <button
          onClick={handleRemoveExclusions}
          disabled={operating}
          className="px-3 py-1.5 text-sm font-medium rounded bg-accent text-gray-900 hover:bg-accent/90 transition-colors disabled:opacity-50"
        >
          {operating ? 'Removing...' : 'Remove Exclusion'}
        </button>
      </SelectionActionBar>

      {loading && !data ? (
        <div className="card p-12 text-center">
          <div className="text-muted dark:text-muted-dark animate-pulse">Loading...</div>
        </div>
      ) : !filteredItems.length ? (
        <div className="card p-8 text-center text-muted dark:text-muted-dark">
          {search ? 'No matching excluded items found.' : 'No excluded items for this rule.'}
        </div>
      ) : (
        <>
          <div className="card">
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="bg-gray-50 dark:bg-white/5 border-b border-border dark:border-border-dark">
                    <th className="px-4 py-3 w-10">
                      <input
                        type="checkbox"
                        checked={allVisibleSelected}
                        onChange={toggleSelectAll}
                        className="rounded border-border dark:border-border-dark"
                      />
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                      Title
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                      Year
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                      Excluded At
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">
                      Excluded By
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {filteredItems.map((exclusion) => (
                    <tr
                      key={exclusion.id}
                      className={`border-b border-border dark:border-border-dark hover:bg-gray-50 dark:hover:bg-white/5 transition-colors
                        ${selected.has(exclusion.library_item_id) ? 'bg-accent/5' : ''}`}
                    >
                      <td className="px-4 py-3">
                        <input
                          type="checkbox"
                          checked={selected.has(exclusion.library_item_id)}
                          onChange={() => toggleSelect(exclusion.library_item_id)}
                          className="rounded border-border dark:border-border-dark"
                        />
                      </td>
                      <td className="px-4 py-3 font-medium">
                        <ItemTitleButton item={exclusion.item} onSelect={(s, i) => setSelectedItem({ serverId: s, itemId: i })} />
                      </td>
                      <td className="px-4 py-3 text-muted dark:text-muted-dark">
                        {exclusion.item?.year || '-'}
                      </td>
                      <td className="px-4 py-3 text-muted dark:text-muted-dark">
                        {new Date(exclusion.excluded_at).toLocaleString()}
                      </td>
                      <td className="px-4 py-3 text-muted dark:text-muted-dark">
                        {exclusion.excluded_by}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={(p) => { setPage(p); clearSelection() }}
          />
        </>
      )}

      {detailModal}
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
      setError(errorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      <LibrarySubViewHeader
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
  const [syncStates, setSyncStates] = useState<Record<string, SyncProgress>>({})
  const handledKeysRef = useRef(new Set<string>())
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

  // Poll for sync progress (only on list view where progress is displayed)
  const hasSyncsRunning = Object.keys(syncStates).length > 0
  useEffect(() => {
    if (view.type !== 'list' && !hasSyncsRunning) return

    let active = true

    const poll = async () => {
      if (!active) return
      try {
        const status = await api.get<Record<string, SyncProgress>>('/api/maintenance/sync/status')
        if (!active) return

        let needRefresh = false
        const activeStates: Record<string, SyncProgress> = {}

        for (const [key, progress] of Object.entries(status)) {
          if (progress.phase === 'done') {
            if (!handledKeysRef.current.has(key)) {
              handledKeysRef.current.add(key)
              needRefresh = true
            }
          } else if (progress.phase === 'error') {
            if (!handledKeysRef.current.has(key)) {
              handledKeysRef.current.add(key)
              setSyncError(`Sync failed: ${progress.error}`)
              needRefresh = true
            }
          } else {
            activeStates[key] = progress
            handledKeysRef.current.delete(key)
          }
        }

        setSyncStates(prev => {
          const next: Record<string, SyncProgress> = { ...activeStates }
          // Preserve optimistic entries not yet reported by backend
          for (const key of Object.keys(prev)) {
            if (!(key in next) && !(key in status)) {
              next[key] = prev[key]
            }
          }
          return next
        })
        if (needRefresh) refetchMaintenance()
      } catch { /* ignore polling errors */ }
    }

    poll()
    const interval = setInterval(poll, 1500)
    return () => { active = false; clearInterval(interval) }
  }, [view.type, hasSyncsRunning, refetchMaintenance])

  const handleSync = async (library: Library) => {
    setSyncError(null)
    try {
      await api.post('/api/maintenance/sync', {
        server_id: library.server_id,
        library_id: library.id,
      })
      setSyncStates(prev => ({
        ...prev,
        [syncKey(library)]: { phase: 'items', library: library.id },
      }))
    } catch (err) {
      if ((err as ApiError).status !== 409) {
        setSyncError(`Failed to start sync for "${library.name}"`)
      }
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
        onViewExclusions={(rule) => setView({ type: 'exclusions', library: view.library, rule })}
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
        onManageExclusions={() => setView({ type: 'exclusions', library: view.library, rule: view.rule })}
      />
    )
  }

  if (view.type === 'exclusions') {
    return (
      <ExclusionsView
        library={view.library}
        rule={view.rule}
        onBack={() => setView({ type: 'candidates', library: view.library, rule: view.rule })}
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
        <div className="card">
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
                  const key = syncKey(library)
                  return (
                    <LibraryRow
                      key={key}
                      library={library}
                      maintenance={maintenance}
                      syncState={syncStates[key] || null}
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
