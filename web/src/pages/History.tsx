import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { HistoryTable } from '../components/HistoryTable'
import type { WatchHistoryEntry, PaginatedResult } from '../types'

const PER_PAGE = 20

const paginationBtnClass = `px-4 py-2 text-sm font-medium rounded-lg
  bg-panel dark:bg-panel-dark border border-border dark:border-border-dark
  disabled:opacity-40 disabled:cursor-not-allowed
  hover:border-accent/30 transition-colors`

export function History() {
  const [page, setPage] = useState(1)
  const { data, loading, error } = useFetch<PaginatedResult<WatchHistoryEntry>>(
    `/api/history?page=${page}&per_page=${PER_PAGE}`
  )

  const totalPages = data ? Math.ceil(data.total / data.per_page) : 0

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-semibold">History</h1>
        {data && (
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            {data.total} total entries
          </p>
        )}
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

      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-6">
          <button
            onClick={() => setPage(p => Math.max(1, p - 1))}
            disabled={page <= 1}
            className={paginationBtnClass}
          >
            Previous
          </button>
          <span className="text-sm text-muted dark:text-muted-dark font-mono">
            {page} / {totalPages}
          </span>
          <button
            onClick={() => setPage(p => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
            className={paginationBtnClass}
          >
            Next
          </button>
        </div>
      )}
    </div>
  )
}
