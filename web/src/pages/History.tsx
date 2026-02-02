import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { PER_PAGE } from '../lib/constants'
import { HistoryTable } from '../components/HistoryTable'
import { Pagination } from '../components/Pagination'
import type { WatchHistoryEntry, PaginatedResult } from '../types'

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

      <Pagination page={page} totalPages={totalPages} onPageChange={setPage} />
    </div>
  )
}
