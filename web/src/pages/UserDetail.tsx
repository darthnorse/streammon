import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useFetch } from '../hooks/useFetch'
import { ApiError } from '../lib/api'
import { PER_PAGE } from '../lib/constants'
import { HistoryTable } from '../components/HistoryTable'
import { Pagination } from '../components/Pagination'
import { LocationMap } from '../components/LocationMap'
import type { User, WatchHistoryEntry, PaginatedResult, Role } from '../types'

type Tab = 'history' | 'locations'

const tabs: { key: Tab; label: string }[] = [
  { key: 'history', label: 'Watch History' },
  { key: 'locations', label: 'Locations' },
]

const roleBadgeClass: Record<Role, string> = {
  admin: 'badge-warn',
  viewer: 'badge-muted',
}

export function UserDetail() {
  const { name } = useParams<{ name: string }>()
  const decodedName = name ? decodeURIComponent(name) : ''

  const [tab, setTab] = useState<Tab>('history')
  const [page, setPage] = useState(1)

  const { data: user, loading: userLoading, error: userError } = useFetch<User>(
    decodedName ? `/api/users/${encodeURIComponent(decodedName)}` : null
  )

  const historyUrl = decodedName
    ? `/api/history?user=${encodeURIComponent(decodedName)}&page=${page}&per_page=${PER_PAGE}`
    : null
  const { data: history, loading: historyLoading } = useFetch<PaginatedResult<WatchHistoryEntry>>(
    tab === 'history' ? historyUrl : null
  )

  if (userLoading) {
    return (
      <div className="flex items-center justify-center py-20 text-muted dark:text-muted-dark text-sm">
        Loading user...
      </div>
    )
  }

  if (userError) {
    const notFound = userError instanceof ApiError && userError.status === 404
    return (
      <div className="card p-12 text-center">
        <div className="text-4xl mb-3 opacity-30">?</div>
        <p className="text-muted dark:text-muted-dark">
          {notFound ? 'User not found' : 'Failed to load user'}
        </p>
        <Link to="/history" className="text-sm text-accent-dim dark:text-accent hover:underline mt-2 inline-block">
          Back to History
        </Link>
      </div>
    )
  }

  const totalPages = history ? Math.ceil(history.total / history.per_page) : 0

  return (
    <div>
      <div className="flex items-center gap-4 mb-6">
        {user?.thumb_url && (
          <img
            src={user.thumb_url}
            alt=""
            className="w-12 h-12 rounded-full border-2 border-border dark:border-border-dark object-cover"
          />
        )}
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-semibold">{decodedName}</h1>
            {user && (
              <span className={`badge ${roleBadgeClass[user.role]}`}>
                {user.role}
              </span>
            )}
          </div>
          {user && (
            <p className="text-sm text-muted dark:text-muted-dark mt-0.5">
              Joined {new Date(user.created_at).toLocaleDateString(undefined, {
                month: 'long', year: 'numeric',
              })}
            </p>
          )}
        </div>
      </div>

      <div className="flex gap-1 mb-6 border-b border-border dark:border-border-dark">
        {tabs.map(t => (
          <button
            key={t.key}
            onClick={() => { setTab(t.key); setPage(1) }}
            className={`px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors
              ${tab === t.key
                ? 'border-accent text-accent-dim dark:text-accent'
                : 'border-transparent text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
              }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'history' && (
        <div>
          {historyLoading ? (
            <div className="py-12 text-center text-muted dark:text-muted-dark text-sm">
              Loading history...
            </div>
          ) : history ? (
            <>
              <div className="text-sm text-muted dark:text-muted-dark mb-3">
                {history.total} entr{history.total === 1 ? 'y' : 'ies'}
              </div>
              <HistoryTable entries={history.items} hideUser />
              <Pagination page={page} totalPages={totalPages} onPageChange={setPage} />
            </>
          ) : null}
        </div>
      )}

      {tab === 'locations' && decodedName && (
        <LocationMap userName={decodedName} />
      )}
    </div>
  )
}
