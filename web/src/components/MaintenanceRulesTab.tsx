import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useMultiSelect } from '../hooks/useMultiSelect'
import { useMountedRef } from '../hooks/useMountedRef'
import { useDebouncedSearch } from '../hooks/useDebouncedSearch'
import { usePersistedPerPage } from '../hooks/usePersistedPerPage'
import { useItemDetails } from '../hooks/useItemDetails'
import { api, ApiError } from '../lib/api'
import { PER_PAGE_OPTIONS, SERVER_ACCENT } from '../lib/constants'
import { readSSEStream } from '../lib/sse'
import { formatCount, formatSize } from '../lib/format'
import { Pagination } from './Pagination'
import { MediaDetailModal } from './MediaDetailModal'
import { MaintenanceRuleForm } from './MaintenanceRuleForm'
import { CrossServerDeleteDialog } from './CrossServerDeleteDialog'
import {
  SubViewHeader,
  SearchInput,
  SelectionActionBar,
  ConfirmDialog,
  OperationResult,
  DeleteProgressModal,
  type DeleteProgress,
} from './shared'
import type {
  MaintenanceRuleWithCount,
  MaintenanceCandidatesResponse,
  MaintenanceExclusionsResponse,
  MaintenanceCandidate,
  BulkDeleteResult,
  LibrariesResponse,
  Library,
  LibraryItemCache,
  CriterionType,
  SyncProgress,
} from '../types'

// --- Shared helpers ---

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

const criterionNames: Record<CriterionType, string> = {
  unwatched_movie: 'Unwatched Movies',
  unwatched_tv_none: 'Unwatched TV Shows',
  low_resolution: 'Low Resolution',
  large_files: 'Large Files',
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


interface RuleOpState {
  syncKeys: string[]
  message: string
}

function makeSyncKey(serverId: number, libraryId: string): string {
  return `${serverId}-${libraryId}`
}

function parseSyncKey(key: string): { serverId: number; libraryId: string } {
  const i = key.indexOf('-')
  return { serverId: Number(key.slice(0, i)), libraryId: key.slice(i + 1) }
}

function formatSyncProgress(state: SyncProgress, libraryName?: string): string {
  const prefix = libraryName ? `${libraryName}: ` : ''
  switch (state.phase) {
    case 'items':
      return state.total ? `${prefix}Scanning ${state.current ?? 0}/${state.total}` : `${prefix}Scanning...`
    case 'history':
      return state.total ? `${prefix}History ${state.current ?? 0}/${state.total}` : `${prefix}Fetching history...`
    case 'error':
      return state.error ? `${prefix}Error: ${state.error}` : `${prefix}Sync error`
    case 'done':
      return `${prefix}Sync complete`
    default:
      return `${prefix}Syncing...`
  }
}

type SortField = 'title' | 'year' | 'resolution' | 'size' | 'reason' | 'added_at'
type SortDir = 'asc' | 'desc'

function SortHeader({
  field,
  sortField,
  sortDir,
  onSort,
  children,
}: {
  field: SortField
  sortField: SortField | null
  sortDir: SortDir
  onSort: (field: SortField) => void
  children: React.ReactNode
}) {
  const isActive = sortField === field
  const ariaSort: 'ascending' | 'descending' | undefined = isActive
    ? (sortDir === 'asc' ? 'ascending' : 'descending')
    : undefined
  return (
    <th
      onClick={() => onSort(field)}
      aria-sort={ariaSort}
      className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider cursor-pointer hover:text-accent transition-colors select-none"
    >
      <span className="inline-flex items-center gap-1">
        {children}
        {isActive && (
          <span className="text-accent">{sortDir === 'asc' ? '\u25B2' : '\u25BC'}</span>
        )}
      </span>
    </th>
  )
}

// --- ViewState ---

type ViewState =
  | { type: 'list' }
  | { type: 'candidates'; rule: MaintenanceRuleWithCount }
  | { type: 'exclusions'; rule: MaintenanceRuleWithCount }

// --- Props ---

interface MaintenanceRulesTabProps {
  filterServerID?: number
  filterLibraryID?: string
  onClearFilter?: () => void
}

// --- Library lookup ---

export interface LibraryLookup {
  getServerName: (serverId: number) => string
  getLibraryName: (serverId: number, libraryId: string) => string
  getLibrary: (serverId: number, libraryId: string) => Library | undefined
}

export function useLibraryLookup(): LibraryLookup {
  const { data } = useFetch<LibrariesResponse>('/api/libraries')

  const maps = useMemo(() => {
    const serverMap = new Map<number, string>()
    const libraryMap = new Map<string, Library>()
    if (data?.libraries) {
      for (const lib of data.libraries) {
        if (!serverMap.has(lib.server_id)) {
          serverMap.set(lib.server_id, lib.server_name)
        }
        libraryMap.set(makeSyncKey(lib.server_id, lib.id), lib)
      }
    }
    return { serverMap, libraryMap }
  }, [data?.libraries])

  const getServerName = useCallback(
    (serverId: number) => maps.serverMap.get(serverId) ?? `Server ${serverId}`,
    [maps.serverMap]
  )
  const getLibraryName = useCallback(
    (serverId: number, libraryId: string) => maps.libraryMap.get(makeSyncKey(serverId, libraryId))?.name ?? libraryId,
    [maps.libraryMap]
  )
  const getLibrary = useCallback(
    (serverId: number, libraryId: string) => maps.libraryMap.get(makeSyncKey(serverId, libraryId)),
    [maps.libraryMap]
  )

  return { getServerName, getLibraryName, getLibrary }
}

// --- Rules API response ---

interface RulesListResponse {
  rules: MaintenanceRuleWithCount[]
}

// --- CandidatesView ---

function CandidatesView({
  rule,
  onBack,
  onManageExclusions,
  lookup,
  filterServerID,
  filterLibraryID,
}: {
  rule: MaintenanceRuleWithCount
  onBack: () => void
  onManageExclusions: () => void
  lookup: LibraryLookup
  filterServerID?: number
  filterLibraryID?: string
}) {
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = usePersistedPerPage()
  const [sortField, setSortField] = useState<SortField | null>(null)
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [deleteConfirm, setDeleteConfirm] = useState<MaintenanceCandidate[] | null>(null)
  const [excludeConfirm, setExcludeConfirm] = useState<MaintenanceCandidate[] | null>(null)
  const [crossServerCandidate, setCrossServerCandidate] = useState<MaintenanceCandidate | null>(null)
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
  const sortParam = sortField ? `&sort_by=${sortField}&sort_order=${sortDir}` : ''
  const libraryFilterParam = filterServerID && filterLibraryID
    ? `&server_id=${filterServerID}&library_id=${encodeURIComponent(filterLibraryID)}`
    : ''
  const { data, loading, refetch } = useFetch<MaintenanceCandidatesResponse>(
    `/api/maintenance/rules/${rule.id}/candidates?page=${page}&per_page=${perPage}${searchParam}${sortParam}${libraryFilterParam}`
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

  function handleSort(field: SortField) {
    if (sortField === field) {
      const defaultDir = field === 'title' ? 'asc' : 'desc'
      if (sortDir !== defaultDir) {
        setSortField(null)
        setSortDir('desc')
      } else {
        setSortDir(d => d === 'asc' ? 'desc' : 'asc')
      }
    } else {
      setSortField(field)
      setSortDir(field === 'title' ? 'asc' : 'desc')
    }
    setPage(1)
  }

  useEffect(() => {
    setPage(1)
    clearSelection()
    resetSearch()
    setSortField(null)
  }, [rule.id, clearSelection, resetSearch])

  useEffect(() => {
    setRowMenuOpen(null)
    setMenuPos(null)
  }, [data])

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
    const item = candidate.item
    if (item && (item.tmdb_id || item.tvdb_id || item.imdb_id)) {
      setCrossServerCandidate(candidate)
    } else {
      setDeleteConfirm([candidate])
    }
  }

  const handleCrossServerConfirm = async (candidateId: number, sourceItemId: number, crossServerItemIds: number[]) => {
    setCrossServerCandidate(null)
    setOperating(true)
    setOperationResult(null)

    // Delete cross-server items FIRST while the source item still exists in DB.
    // handleDeleteLibraryItem needs the source to verify the cross-server match.
    // If we deleted the candidate first, its library_item CASCADE would destroy the source row.
    // Continue on error so one failure doesn't leave the operation half-done.
    let crossDeleted = 0
    let crossFailed = 0
    for (const id of crossServerItemIds) {
      try {
        await api.del(`/api/maintenance/library-items/${id}?source_item_id=${sourceItemId}`)
        crossDeleted++
      } catch (err) {
        crossFailed++
        console.error(`Cross-server delete item ${id} failed:`, err)
      }
    }

    let sourceDeleted = false
    try {
      await api.del(`/api/maintenance/candidates/${candidateId}`)
      sourceDeleted = true
    } catch (err) {
      console.error('Source candidate delete failed:', err)
    }

    if (!mountedRef.current) return

    const totalDeleted = (sourceDeleted ? 1 : 0) + crossDeleted
    const totalFailed = (sourceDeleted ? 0 : 1) + crossFailed
    const totalRequested = 1 + crossServerItemIds.length

    if (totalFailed === 0) {
      setOperationResult({ type: 'success', message: `Deleted ${totalDeleted} item${totalDeleted > 1 ? 's' : ''}` })
    } else if (totalDeleted > 0) {
      setOperationResult({ type: 'partial', message: `Deleted ${totalDeleted} of ${totalRequested} items. ${totalFailed} failed — please refresh and retry.` })
    } else {
      setOperationResult({ type: 'error', message: 'Failed to delete items. Please refresh and retry.' })
    }
    clearSelection()
    refetch()
    if (mountedRef.current) setOperating(false)
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
        <SubViewHeader
          title={rule.name}
          subtitle={`${data ? formatCount(data.total) : '0'} violations`}
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
          <label htmlFor="maint-candidates-per-page">Show</label>
          <select
            id="maint-candidates-per-page"
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
                    <SortHeader field="title" sortField={sortField} sortDir={sortDir} onSort={handleSort}>Title</SortHeader>
                    <th className="px-4 py-3 text-left text-xs font-semibold text-muted dark:text-muted-dark uppercase tracking-wider">Library</th>
                    <SortHeader field="year" sortField={sortField} sortDir={sortDir} onSort={handleSort}>Year</SortHeader>
                    <SortHeader field="resolution" sortField={sortField} sortDir={sortDir} onSort={handleSort}>Resolution</SortHeader>
                    <SortHeader field="size" sortField={sortField} sortDir={sortDir} onSort={handleSort}>Size</SortHeader>
                    <SortHeader field="reason" sortField={sortField} sortDir={sortDir} onSort={handleSort}>Reason</SortHeader>
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
                      <td className="px-4 py-3 text-sm text-muted dark:text-muted-dark">
                        {candidate.item ? (
                          <div className="space-y-0.5">
                            <div className="whitespace-nowrap">{lookup.getServerName(candidate.item.server_id)} <span className="text-muted/60 dark:text-muted-dark/60">/</span> {lookup.getLibraryName(candidate.item.server_id, candidate.item.library_id)}</div>
                            {candidate.other_copies?.map(copy => (
                              <div key={`${copy.server_id}-${copy.library_id}`} className="whitespace-nowrap text-muted/70 dark:text-muted-dark/70 text-xs">
                                {lookup.getServerName(copy.server_id)} <span className="text-muted/40 dark:text-muted-dark/40">/</span> {lookup.getLibraryName(copy.server_id, copy.library_id)}
                              </div>
                            ))}
                          </div>
                        ) : '-'}
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
          title={`Delete ${deleteConfirm.length} item${deleteConfirm.length > 1 ? 's' : ''}?`}
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
            className="text-sm hover:text-accent hover:underline mb-3 flex items-center gap-1"
          >
            {showDetails ? '\u25BC Hide details' : '\u25B6 Show details'}
          </button>
          {showDetails && (
            <div className="max-h-40 overflow-y-auto mb-4 p-2 rounded bg-surface dark:bg-surface-dark text-sm">
              {deleteConfirm.map(c => (
                <div key={c.id} className="py-1">
                  {'\u2022'} {c.item?.title} ({c.item?.year}) - {formatSize(c.item?.file_size || 0)}
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

      {crossServerCandidate && crossServerCandidate.item && (
        <CrossServerDeleteDialog
          candidateId={crossServerCandidate.id}
          item={crossServerCandidate.item}
          onConfirm={handleCrossServerConfirm}
          onCancel={() => setCrossServerCandidate(null)}
        />
      )}

      {deleteProgress && (
        <DeleteProgressModal progress={deleteProgress} />
      )}

      {detailModal}
    </div>
  )
}

// --- ExclusionsView ---

function ExclusionsView({
  rule,
  onBack,
}: {
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
      <SubViewHeader
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
          <label htmlFor="maint-exclusions-per-page">Show</label>
          <select
            id="maint-exclusions-per-page"
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

// --- Main component ---

export function MaintenanceRulesTab({ filterServerID, filterLibraryID, onClearFilter }: MaintenanceRulesTabProps) {
  const [view, setView] = useState<ViewState>({ type: 'list' })
  const [editingRule, setEditingRule] = useState<MaintenanceRuleWithCount | null>(null)
  const [showRuleForm, setShowRuleForm] = useState(false)
  const [operationError, setOperationError] = useState<string | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<MaintenanceRuleWithCount | null>(null)
  const [ruleOps, setRuleOps] = useState<Record<number, RuleOpState>>({})
  const mountedRef = useMountedRef()
  const lookup = useLibraryLookup()

  const filterParams = useMemo(() => {
    const parts: string[] = []
    if (filterServerID) parts.push(`server_id=${filterServerID}`)
    if (filterLibraryID) parts.push(`library_id=${encodeURIComponent(filterLibraryID)}`)
    return parts.length > 0 ? `?${parts.join('&')}` : ''
  }, [filterServerID, filterLibraryID])

  const { data: rulesData, refetch: refetchRules } = useFetch<RulesListResponse>(
    `/api/maintenance/rules${filterParams}`
  )

  const rules = rulesData?.rules ?? []

  const filterLibraryName = useMemo(() => {
    if (filterServerID && filterLibraryID) {
      return `${lookup.getServerName(filterServerID)} - ${lookup.getLibraryName(filterServerID, filterLibraryID)}`
    }
    return null
  }, [filterServerID, filterLibraryID, lookup])

  const hasActiveOps = Object.keys(ruleOps).length > 0

  useEffect(() => {
    if (!hasActiveOps) return
    let active = true
    const controller = new AbortController()

    const poll = async () => {
      if (!active) return
      try {
        const status = await api.get<Record<string, SyncProgress>>('/api/maintenance/sync/status', controller.signal)
        if (!active) return

        const errors: string[] = []
        setRuleOps(prev => {
          const next = { ...prev }
          let changed = false
          for (const [ruleId, op] of Object.entries(next)) {
            const activeKey = op.syncKeys.find(k => status[k] && status[k].phase !== 'done' && status[k].phase !== 'error')

            if (activeKey) {
              const { serverId, libraryId } = parseSyncKey(activeKey)
              const libName = lookup.getLibraryName(serverId, libraryId)
              next[Number(ruleId)] = { ...op, message: formatSyncProgress(status[activeKey], libName) }
              changed = true
            } else {
              const errorKeys = op.syncKeys.filter(k => status[k]?.phase === 'error')
              for (const ek of errorKeys) {
                const errMsg = status[ek]?.error
                const { serverId, libraryId } = parseSyncKey(ek)
                const libName = lookup.getLibraryName(serverId, libraryId)
                errors.push(errMsg ? `${libName}: ${errMsg}` : `${libName}: Sync error`)
              }
              delete next[Number(ruleId)]
              changed = true
            }
          }
          return changed ? next : prev
        })
        if (errors.length > 0 && mountedRef.current) {
          setOperationError(errors.join('; '))
        }
        if (active) refetchRules()
      } catch { /* ignore polling errors */ }
    }

    poll()
    const interval = setInterval(poll, 1500)
    return () => { active = false; clearInterval(interval); controller.abort() }
  }, [hasActiveOps, refetchRules, lookup.getLibraryName])

  const handleToggleRule = async (rule: MaintenanceRuleWithCount) => {
    setOperationError(null)
    try {
      await api.put(`/api/maintenance/rules/${rule.id}`, {
        name: rule.name,
        criterion_type: rule.criterion_type,
        parameters: rule.parameters,
        enabled: !rule.enabled,
        libraries: rule.libraries,
      })
      if (mountedRef.current) refetchRules()
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
        refetchRules()
      }
    } catch (err) {
      console.error('Failed to delete rule:', err)
      if (mountedRef.current) {
        setOperationError(`Failed to delete rule "${rule.name}"`)
        setDeleteConfirm(null)
      }
    }
  }

  const handleSyncAndEvaluate = async (rule: MaintenanceRuleWithCount) => {
    if (ruleOps[rule.id]) return
    setOperationError(null)

    const uniqueLibs = new Map<string, { serverID: number; libraryID: string }>()
    for (const rl of rule.libraries) {
      const key = makeSyncKey(rl.server_id, rl.library_id)
      uniqueLibs.set(key, { serverID: rl.server_id, libraryID: rl.library_id })
    }

    const syncEntries = Array.from(uniqueLibs.entries())
    const results = await Promise.allSettled(
      syncEntries.map(async ([key, lib]) => {
        await api.post('/api/maintenance/sync', { server_id: lib.serverID, library_id: lib.libraryID })
        return key
      })
    )

    if (!mountedRef.current) return

    const syncKeys: string[] = []
    for (let i = 0; i < results.length; i++) {
      const r = results[i]
      if (r.status === 'fulfilled' || (r.reason as ApiError)?.status === 409) {
        syncKeys.push(syncEntries[i][0])
      } else {
        console.error(`Failed to start sync for ${syncEntries[i][0]}:`, r.reason)
      }
    }

    if (syncKeys.length > 0) {
      setRuleOps(prev => ({
        ...prev,
        [rule.id]: { syncKeys, message: 'Syncing...' },
      }))
    } else {
      setOperationError('Failed to start sync for any library')
    }
  }

  const handleRuleSaved = () => {
    setShowRuleForm(false)
    setEditingRule(null)
    refetchRules()
  }

  if (view.type === 'candidates') {
    return (
      <CandidatesView
        rule={view.rule}
        onBack={() => setView({ type: 'list' })}
        onManageExclusions={() => setView({ type: 'exclusions', rule: view.rule })}
        lookup={lookup}
        filterServerID={filterServerID}
        filterLibraryID={filterLibraryID}
      />
    )
  }

  if (view.type === 'exclusions') {
    return (
      <ExclusionsView
        rule={view.rule}
        onBack={() => setView({ type: 'candidates', rule: view.rule })}
      />
    )
  }

  return (
    <div className="space-y-4">
      {filterLibraryName && (
        <div className="flex items-center gap-3">
          <span className="text-sm text-muted dark:text-muted-dark">
            Showing rules for <span className="font-medium text-gray-900 dark:text-gray-100">{filterLibraryName}</span>
          </span>
          {onClearFilter && (
            <button
              onClick={() => onClearFilter()}
              className="text-sm hover:text-accent hover:underline"
            >
              Clear filter
            </button>
          )}
        </div>
      )}

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

      <div className="flex items-center justify-between gap-4">
        <p className="text-sm text-muted dark:text-muted-dark">
          Libraries are automatically scanned daily at 3 AM to identify new candidates.
        </p>
        <button
          onClick={() => {
            setEditingRule(null)
            setShowRuleForm(true)
          }}
          className="px-4 py-2 text-sm font-medium rounded-lg bg-accent text-gray-900 hover:bg-accent/90 whitespace-nowrap"
        >
          Add Rule
        </button>
      </div>

      {rules.length === 0 ? (
        <div className="card p-8 text-center text-muted dark:text-muted-dark">
          No maintenance rules configured. Add your first rule to start identifying candidates for cleanup.
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
                      {criterionNames[rule.criterion_type] ?? rule.criterion_type}
                    </span>
                  </div>
                  <p className="text-sm text-muted dark:text-muted-dark mt-1">
                    {formatRuleParameters(rule)}
                  </p>
                  {rule.libraries.length > 0 && (
                    <div className="flex flex-wrap gap-1.5 mt-2">
                      {rule.libraries.map((rl) => {
                        const lib = lookup.getLibrary(rl.server_id, rl.library_id)
                        const accent = lib ? SERVER_ACCENT[lib.server_type] : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'
                        return (
                          <span
                            key={`${rl.server_id}-${rl.library_id}`}
                            className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${accent}`}
                          >
                            [{lookup.getServerName(rl.server_id)}] {lookup.getLibraryName(rl.server_id, rl.library_id)}
                          </span>
                        )
                      })}
                    </div>
                  )}
                </div>
                <div className="flex items-center gap-3 flex-wrap">
                  <div className="flex flex-col items-end text-sm">
                    {rule.candidate_count > 0 ? (
                      <button
                        onClick={() => setView({ type: 'candidates', rule })}
                        className="font-medium hover:text-accent hover:underline whitespace-nowrap"
                        aria-label={`View ${rule.candidate_count} candidates for rule ${rule.name}`}
                      >
                        {formatCount(rule.candidate_count)} candidates
                      </button>
                    ) : (
                      <span className="text-muted dark:text-muted-dark font-medium whitespace-nowrap">
                        0 candidates
                      </span>
                    )}
                    {rule.exclusion_count > 0 && (
                      <button
                        onClick={() => setView({ type: 'exclusions', rule })}
                        className="text-muted dark:text-muted-dark font-medium hover:underline whitespace-nowrap"
                        aria-label={`View ${rule.exclusion_count} exclusions for rule ${rule.name}`}
                      >
                        {formatCount(rule.exclusion_count)} excluded
                      </button>
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
                      onClick={() => {
                        setEditingRule(rule)
                        setShowRuleForm(true)
                      }}
                      className="p-1.5 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors"
                      title="Edit rule"
                      aria-label={`Edit rule ${rule.name}`}
                    >
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                      </svg>
                    </button>
                    <div className="relative">
                      <button
                        onClick={() => handleSyncAndEvaluate(rule)}
                        disabled={!!ruleOps[rule.id]}
                        className="p-1.5 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors disabled:opacity-50"
                        title="Sync & re-evaluate"
                        aria-label={`Sync and re-evaluate rule ${rule.name}`}
                      >
                        <svg className={`w-4 h-4 ${ruleOps[rule.id] ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                      </button>
                      {ruleOps[rule.id]?.message && (
                        <div className="absolute top-full right-0 mt-1 text-xs text-muted dark:text-muted-dark whitespace-nowrap">
                          {ruleOps[rule.id].message}
                        </div>
                      )}
                    </div>
                    <button
                      onClick={() => setView({ type: 'candidates', rule })}
                      className="p-1.5 rounded hover:bg-surface dark:hover:bg-surface-dark transition-colors"
                      title="View candidates"
                      aria-label={`View candidates for rule ${rule.name}`}
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

      {showRuleForm && (
        <MaintenanceRuleForm
          rule={editingRule ?? undefined}
          onClose={() => {
            setShowRuleForm(false)
            setEditingRule(null)
          }}
          onSaved={handleRuleSaved}
        />
      )}
    </div>
  )
}
