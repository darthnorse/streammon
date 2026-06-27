import { useState, useMemo, useCallback } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useColumnConfig } from '../hooks/useColumnConfig'
import { useDebouncedSearch } from '../hooks/useDebouncedSearch'
import { usePersistedPerPage } from '../hooks/usePersistedPerPage'
import { ColumnSettings } from './ColumnSettings'
import { LibraryItemsTable } from './LibraryItemsTable'
import { Pagination } from './Pagination'
import { SearchInput } from './shared/SearchInput'
import { getLibraryColumns, libraryColumnStorageKey } from '../lib/libraryColumns'
import type { SortState } from './HistoryTable'
import type { PaginatedResult, LibraryItemDetail, LibraryType, TitleClickHandler } from '../types'

type Filter = 'all' | 'played' | 'unplayed'
const FILTERS: Filter[] = ['all', 'played', 'unplayed']
const FILTER_LABELS: Record<Filter, string> = { all: 'All', played: 'Played', unplayed: 'Never played' }

interface LibraryItemsViewProps {
  serverId: string
  libraryId: string
  libraryType?: LibraryType
  onTitleClick: TitleClickHandler
}

// LibraryItemsView owns the items table, its controls, and the column layout.
// It is mounted keyed by library type so the column config (persisted per
// type) initializes from the correct storage key.
export function LibraryItemsView({ serverId, libraryId, libraryType, onTitleClick }: LibraryItemsViewProps) {
  const [page, setPage] = useState(1)
  const [perPage] = usePersistedPerPage()
  const [sort, setSort] = useState<SortState | null>(null)
  const [filter, setFilter] = useState<Filter>('all')
  const { searchInput, setSearchInput, search } = useDebouncedSearch(() => setPage(1))

  const allColumns = useMemo(() => getLibraryColumns(onTitleClick, libraryType), [onTitleClick, libraryType])
  const { visibleColumns, toggleColumn, moveColumn, resetToDefaults } =
    useColumnConfig(allColumns, [], libraryColumnStorageKey(libraryType))

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
  const { data, loading, error } = useFetch<PaginatedResult<LibraryItemDetail>>(
    `${base}/items?page=${page}&per_page=${perPage}${sortParam}${searchParam}${filterParam}`,
  )

  const handleSort = useCallback((s: SortState | null) => { setSort(s); setPage(1) }, [])
  const handleFilter = useCallback((f: Filter) => { setFilter(f); setPage(1) }, [])
  const totalPages = data && data.per_page > 0 ? Math.ceil(data.total / data.per_page) : 0

  return (
    <div>
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
    </div>
  )
}
