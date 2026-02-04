import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { HistoryTable } from '../components/HistoryTable'
import { Pagination } from '../components/Pagination'
import type { WatchHistoryEntry, PaginatedResult } from '../types'

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100]

export function History() {
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = useState(20)
  const { data, loading, error } = useFetch<PaginatedResult<WatchHistoryEntry>>(
    `/api/history?page=${page}&per_page=${perPage}`
  )

  const totalPages = data ? Math.ceil(data.total / data.per_page) : 0

  function handlePerPageChange(newPerPage: number) {
    setPerPage(newPerPage)
    setPage(1)
  }

  return (
    <div>
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">History</h1>
          {data && (
            <p className="text-sm text-muted dark:text-muted-dark mt-1">
              {data.total} total entries
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <label htmlFor="per-page" className="text-sm text-muted dark:text-muted-dark">
            Show
          </label>
          <select
            id="per-page"
            value={perPage}
            onChange={(e) => handlePerPageChange(Number(e.target.value))}
            className="px-2 py-1 text-sm rounded border border-border dark:border-border-dark bg-panel dark:bg-panel-dark text-gray-900 dark:text-gray-100"
          >
            {PAGE_SIZE_OPTIONS.map(size => (
              <option key={size} value={size}>{size}</option>
            ))}
          </select>
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

      {data && <HistoryTable entries={data.items} />}

      <Pagination page={page} totalPages={totalPages} onPageChange={setPage} />
    </div>
  )
}
