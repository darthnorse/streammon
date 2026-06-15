import { useState, useMemo, useCallback } from 'react'
import { useFetch } from '../hooks/useFetch'
import { HistoryTable, SortState } from '../components/HistoryTable'
import { Pagination } from '../components/Pagination'
import { Dropdown } from '../components/Dropdown'
import { HISTORY_COLUMNS } from '../lib/historyColumns'
import { buildServerOptions } from '../lib/utils'
import type { WatchHistoryEntry, PaginatedResult, Server } from '../types'
import { SearchInput } from '../components/shared/SearchInput'
import { useDebouncedSearch } from '../hooks/useDebouncedSearch'

type PerPage = '10' | '20' | '50' | '100'

const perPageOptions: { value: PerPage; label: string }[] = [
  { value: '10', label: '10' },
  { value: '20', label: '20' },
  { value: '50', label: '50' },
  { value: '100', label: '100' },
]

export function History() {
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = useState<PerPage>('20')
  const [sort, setSort] = useState<SortState | null>(null)
  const [serverIds, setServerIds] = useState<string[]>([])
  const { searchInput, setSearchInput, search } = useDebouncedSearch(() => setPage(1))

  const { data: servers } = useFetch<Server[]>('/api/servers')

  const sortParams = useMemo(() => {
    if (!sort) return ''
    const column = HISTORY_COLUMNS.find(c => c.id === sort.columnId)
    if (!column?.sortKey) return ''
    return `&sort_by=${column.sortKey}&sort_order=${sort.direction}`
  }, [sort])

  const serverParam = useMemo(() =>
    serverIds.length > 0 ? `&server_ids=${serverIds.join(',')}` : ''
  , [serverIds])

  const searchParam = useMemo(() =>
    search ? `&search=${encodeURIComponent(search)}` : ''
  , [search])

  const handleSort = useCallback((newSort: SortState | null) => {
    setSort(newSort)
    setPage(1)
  }, [])

  const { data, loading, error } = useFetch<PaginatedResult<WatchHistoryEntry>>(
    `/api/history?page=${page}&per_page=${perPage}${sortParams}${serverParam}${searchParam}`
  )

  const totalPages = data ? Math.ceil(data.total / data.per_page) : 0

  const handlePerPageChange = useCallback((value: PerPage) => {
    setPerPage(value)
    setPage(1)
  }, [])

  const handleServerChange = useCallback((selected: string[]) => {
    setServerIds(selected)
    setPage(1)
  }, [])

  const serverOptions = buildServerOptions(servers ?? [])

  return (
    <div>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">History</h1>
          {data && (
            <p className="text-sm text-muted dark:text-muted-dark mt-1">
              {data.total} total entries
            </p>
          )}
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <SearchInput
            value={searchInput}
            onChange={setSearchInput}
            placeholder="Search title or user"
            className="w-48 sm:w-64"
          />
          <span className="text-sm text-muted dark:text-muted-dark">Show</span>
          <Dropdown
            options={perPageOptions}
            value={perPage}
            onChange={handlePerPageChange}
          />
          {servers && servers.length > 1 && (
            <Dropdown
              multi
              options={serverOptions}
              selected={serverIds}
              onChange={handleServerChange}
              allLabel="All Servers"
              noneLabel="All Servers"
            />
          )}
        </div>
      </div>

      {error && (
        <div className="card p-6 text-center text-red-500 dark:text-red-400">
          Error loading history
        </div>
      )}

      {loading && !data && (
        <div className="card p-12 text-center">
          <div className="text-muted dark:text-muted-dark animate-pulse">Loading...</div>
        </div>
      )}

      {data && data.items.length === 0 && searchInput ? (
        <div className="card p-12 text-center text-muted dark:text-muted-dark">
          No entries match &ldquo;{searchInput}&rdquo;
        </div>
      ) : data ? (
        <HistoryTable
          entries={data.items}
          sort={sort}
          onSort={handleSort}
          serverSideSorting
        />
      ) : null}

      <Pagination page={page} totalPages={totalPages} onPageChange={setPage} />
    </div>
  )
}
