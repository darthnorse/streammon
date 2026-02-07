import { useState, useMemo, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useFetch } from '../hooks/useFetch'
import { ApiError } from '../lib/api'
import { PER_PAGE } from '../lib/constants'
import { HistoryTable, SortState } from '../components/HistoryTable'
import { Pagination } from '../components/Pagination'
import { LocationMap } from '../components/LocationMap'
import { UserStatsCards } from '../components/UserStatsCards'
import { UserLocationsCard } from '../components/UserLocationsCard'
import { UserDevicesCard } from '../components/UserDevicesCard'
import { UserISPCard } from '../components/UserISPCard'
import { UserTrustScoreCard } from '../components/UserTrustScoreCard'
import { UserHouseholdCard } from '../components/UserHouseholdCard'
import { getHistoryColumns } from '../lib/historyColumns'
import type { User, WatchHistoryEntry, PaginatedResult, Role, UserDetailStats, RuleViolation } from '../types'

type Tab = 'history' | 'locations' | 'violations'

const tabs: { key: Tab; label: string }[] = [
  { key: 'history', label: 'Watch History' },
  { key: 'locations', label: 'Locations Map' },
  { key: 'violations', label: 'Violations' },
]

const SEVERITY_COLORS: Record<string, string> = {
  info: 'bg-blue-500/20 text-blue-400',
  warning: 'bg-amber-500/20 text-amber-400',
  critical: 'bg-red-500/20 text-red-400',
}

const roleBadgeClass: Record<Role, string> = {
  admin: 'badge-warn',
  viewer: 'badge-muted',
}

export function UserDetail() {
  const { name } = useParams<{ name: string }>()
  const decodedName = name ? decodeURIComponent(name) : ''

  const [tab, setTab] = useState<Tab>('history')
  const [page, setPage] = useState(1)
  const [violationsPage, setViolationsPage] = useState(1)
  const [sort, setSort] = useState<SortState | null>(null)

  const columns = useMemo(() => getHistoryColumns(), [])

  const sortParams = useMemo(() => {
    if (!sort) return ''
    const column = columns.find(c => c.id === sort.columnId)
    if (!column?.sortKey) return ''
    return `&sort_by=${column.sortKey}&sort_order=${sort.direction}`
  }, [sort, columns])

  const handleSort = useCallback((newSort: SortState | null) => {
    setSort(newSort)
    setPage(1)
  }, [])

  const { data: user, loading: userLoading, error: userError } = useFetch<User>(
    decodedName ? `/api/users/${encodeURIComponent(decodedName)}` : null
  )

  const { data: stats, loading: statsLoading, error: statsError } = useFetch<UserDetailStats>(
    decodedName ? `/api/users/${encodeURIComponent(decodedName)}/stats` : null
  )

  const historyUrl = decodedName
    ? `/api/history?user=${encodeURIComponent(decodedName)}&page=${page}&per_page=${PER_PAGE}${sortParams}`
    : null
  const { data: history, loading: historyLoading } = useFetch<PaginatedResult<WatchHistoryEntry>>(
    tab === 'history' ? historyUrl : null
  )

  const violationsUrl = decodedName
    ? `/api/violations?user=${encodeURIComponent(decodedName)}&page=${violationsPage}&per_page=${PER_PAGE}`
    : null
  const { data: violations, loading: violationsLoading } = useFetch<PaginatedResult<RuleViolation>>(
    tab === 'violations' ? violationsUrl : null
  )

  if (userLoading || statsLoading) {
    return (
      <div className="flex items-center justify-center py-20 text-muted dark:text-muted-dark text-sm">
        Loading user...
      </div>
    )
  }

  const userNotFound = userError instanceof ApiError && userError.status === 404
  if (userError && !userNotFound) {
    return (
      <div className="card p-12 text-center">
        <div className="text-4xl mb-3 opacity-30">!</div>
        <p className="text-muted dark:text-muted-dark">Failed to load user</p>
        <Link to="/history" className="text-sm text-accent-dim dark:text-accent hover:underline mt-2 inline-block">
          Back to History
        </Link>
      </div>
    )
  }

  const totalPages = history ? Math.ceil(history.total / history.per_page) : 0
  const violationsTotalPages = violations ? Math.ceil(violations.total / violations.per_page) : 0

  const handleViolationsClick = useCallback(() => {
    setTab('violations')
    setViolationsPage(1)
  }, [])

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
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
          {user?.email && (
            <p className="text-sm text-muted dark:text-muted-dark mt-0.5">
              {user.email}
            </p>
          )}
        </div>
      </div>

      {statsError && !statsLoading && (
        <div className="text-sm text-red-500 dark:text-red-400">
          Failed to load user statistics
        </div>
      )}

      {stats && (
        <>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <UserStatsCards stats={stats} />
            <UserTrustScoreCard userName={decodedName} onViolationsClick={handleViolationsClick} />
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <UserHouseholdCard userName={decodedName} />
            <UserLocationsCard locations={stats.locations} />
            <UserISPCard isps={stats.isps} />
            <UserDevicesCard devices={stats.devices} />
          </div>
        </>
      )}

      <div className="flex gap-1 border-b border-border dark:border-border-dark">
        {tabs.map(t => (
          <button
            key={t.key}
            onClick={() => { setTab(t.key); setPage(1); setSort(null) }}
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
              <HistoryTable
                entries={history.items}
                hideUser
                sort={sort}
                onSort={handleSort}
                serverSideSorting
              />
              <Pagination page={page} totalPages={totalPages} onPageChange={setPage} />
            </>
          ) : null}
        </div>
      )}

      {tab === 'locations' && decodedName && (
        <LocationMap userName={decodedName} />
      )}

      {tab === 'violations' && (
        <div>
          {violationsLoading ? (
            <div className="py-12 text-center text-muted dark:text-muted-dark text-sm">
              Loading violations...
            </div>
          ) : !violations?.items.length ? (
            <div className="card p-8 text-center text-muted dark:text-muted-dark">
              No violations detected for this user.
            </div>
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="text-left text-sm text-muted dark:text-muted-dark border-b border-border dark:border-border-dark">
                      <th className="pb-2 font-medium">Time</th>
                      <th className="pb-2 font-medium">Rule</th>
                      <th className="pb-2 font-medium">Severity</th>
                      <th className="pb-2 font-medium">Confidence</th>
                      <th className="pb-2 font-medium">Message</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border dark:divide-border-dark">
                    {violations.items.map((v) => (
                      <tr key={v.id} className="text-sm">
                        <td className="py-3 whitespace-nowrap">
                          {new Date(v.occurred_at).toLocaleString()}
                        </td>
                        <td className="py-3">{v.rule_name}</td>
                        <td className="py-3">
                          <span className={`px-2 py-0.5 text-xs rounded-full ${SEVERITY_COLORS[v.severity] || 'bg-gray-500/20 text-gray-400'}`}>
                            {v.severity}
                          </span>
                        </td>
                        <td className="py-3">{v.confidence_score.toFixed(0)}%</td>
                        <td className="py-3 max-w-md">{v.message}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <Pagination
                page={violationsPage}
                totalPages={violationsTotalPages}
                onPageChange={setViolationsPage}
              />
            </>
          )}
        </div>
      )}
    </div>
  )
}
