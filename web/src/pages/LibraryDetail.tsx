import { useState, useMemo, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useFetch } from '../hooks/useFetch'
import { useColumnConfig } from '../hooks/useColumnConfig'
import { useMediaDetailModal } from '../hooks/useMediaDetailModal'
import { useDebouncedSearch } from '../hooks/useDebouncedSearch'
import { usePersistedPerPage } from '../hooks/usePersistedPerPage'
import { ColumnSettings } from '../components/ColumnSettings'
import { LibraryItemsTable } from '../components/LibraryItemsTable'
import { Pagination } from '../components/Pagination'
import { SearchInput } from '../components/shared/SearchInput'
import { getLibraryColumns, LIBRARY_COLUMN_STORAGE_KEY } from '../lib/libraryColumns'
import { formatSize } from '../lib/format'
import type { SortState } from '../components/HistoryTable'
import type { PaginatedResult, LibraryItemDetail, LibrarySummary, Library } from '../types'

type Filter = 'all' | 'played' | 'unplayed'
const FILTERS: Filter[] = ['all', 'played', 'unplayed']
const FILTER_LABELS: Record<Filter, string> = { all: 'All', played: 'Played', unplayed: 'Never played' }

export function LibraryDetail() {
  const { serverId = '', libraryId = '' } = useParams()
  const [page, setPage] = useState(1)
  const [perPage] = usePersistedPerPage()
  const [sort, setSort] = useState<SortState | null>(null)
  const [filter, setFilter] = useState<Filter>('all')
  const { searchInput, setSearchInput, search } = useDebouncedSearch(() => setPage(1))
  const { handleTitleClick, modal } = useMediaDetailModal()

  // Library name/type/server aren't stored per-item; resolve them from the
  // (cached, admin-only) libraries list to drive the header + Episodes column.
  const { data: libsData } = useFetch<{ libraries: Library[] }>('/api/libraries')
  const lib = useMemo(
    () => libsData?.libraries.find(l => String(l.server_id) === serverId && l.id === libraryId),
    [libsData, serverId, libraryId],
  )

  const allColumns = useMemo(() => getLibraryColumns(handleTitleClick, lib?.type), [handleTitleClick, lib?.type])
  const { visibleColumns, toggleColumn, moveColumn, resetToDefaults } =
    useColumnConfig(allColumns, [], LIBRARY_COLUMN_STORAGE_KEY)

  const orderedVisible = useMemo(
    () => visibleColumns.map(id => allColumns.find(c => c.id === id)).filter((c): c is NonNullable<typeof c> => !!c),
    [visibleColumns, allColumns],
  )

  const sortParam = sort
    ? `&sort_by=${allColumns.find(c => c.id === sort.columnId)?.sortKey ?? ''}&sort_order=${sort.direction}`
    : ''
  const searchParam = search ? `&search=${encodeURIComponent(search)}` : ''
  const filterParam = filter !== 'all' ? `&filter=${filter}` : ''

  const base = `/api/libraries/${serverId}/${encodeURIComponent(libraryId)}`
  const { data: summary } = useFetch<LibrarySummary>(`${base}/summary`)
  const { data, loading, error } = useFetch<PaginatedResult<LibraryItemDetail>>(
    `${base}/items?page=${page}&per_page=${perPage}${sortParam}${searchParam}${filterParam}`,
  )

  const handleSort = useCallback((s: SortState | null) => { setSort(s); setPage(1) }, [])
  const handleFilter = useCallback((f: Filter) => { setFilter(f); setPage(1) }, [])
  const totalPages = data ? Math.ceil(data.total / data.per_page) : 0
  const watchedPct = summary && summary.total_titles > 0
    ? Math.round((summary.watched_titles / summary.total_titles) * 100) : 0

  return (
    <div>
      <div className="mb-4">
        <Link to="/library" className="text-sm hover:text-accent hover:underline">← Libraries</Link>
        <h1 className="text-2xl font-semibold mt-1">{lib?.name ?? 'Library'}</h1>
        {lib && <p className="text-sm text-muted dark:text-muted-dark">{lib.server_name}</p>}
      </div>

      {summary && (
        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-6">
          <Stat label="Total titles" value={String(summary.total_titles)} />
          <Stat label="Total size" value={formatSize(summary.total_size)} />
          <Stat label="Ever watched" value={`${summary.watched_titles} (${watchedPct}%)`} />
          <Stat label="Never played" value={String(summary.never_played)} />
          <Stat label="Reclaimable" value={formatSize(summary.reclaimable_size)} />
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2 mb-4">
        <SearchInput value={searchInput} onChange={setSearchInput} placeholder="Search title" className="w-48 sm:w-64" />
        <div className="flex gap-1">
          {FILTERS.map(f => (
            <button
              key={f}
              onClick={() => handleFilter(f)}
              className={`px-2.5 py-1 rounded text-sm ${filter === f ? 'bg-accent text-white' : 'hover:bg-gray-100 dark:hover:bg-white/5'}`}
            >
              {FILTER_LABELS[f]}
            </button>
          ))}
        </div>
        <a
          href={`${base}/items?format=csv${searchParam}${filterParam}`}
          className="px-2.5 py-1 rounded text-sm hover:bg-gray-100 dark:hover:bg-white/5"
        >
          Download CSV
        </a>
        <ColumnSettings
          columns={allColumns}
          visibleColumns={visibleColumns}
          onToggle={toggleColumn}
          onMove={moveColumn}
          onReset={resetToDefaults}
        />
      </div>

      {error && <div className="card p-6 text-center text-red-500 dark:text-red-400">Error loading library</div>}
      {loading && !data && <div className="card p-12 text-center text-muted dark:text-muted-dark animate-pulse">Loading...</div>}
      {data && data.items.length === 0 && !loading && (
        <div className="card p-12 text-center text-muted dark:text-muted-dark">No titles match the current filters</div>
      )}
      {data && data.items.length > 0 && (
        <LibraryItemsTable items={data.items} columns={orderedVisible} sort={sort} onSort={handleSort} />
      )}
      <Pagination page={page} totalPages={totalPages} onPageChange={setPage} />
      {modal}
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="card p-3">
      <div className="text-xs text-muted dark:text-muted-dark">{label}</div>
      <div className="text-lg font-semibold mt-0.5">{value}</div>
    </div>
  )
}
