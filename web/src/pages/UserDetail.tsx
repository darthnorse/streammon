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
import { ViolationsTable } from '../components/ViolationsTable'
import { getHistoryColumns } from '../lib/historyColumns'
import { useAuth } from '../context/AuthContext'
import type { User, WatchHistoryEntry, PaginatedResult, Role, UserDetailStats, RuleViolation } from '../types'

type Tab = 'history' | 'locations' | 'violations'

const allTabs: { key: Tab; label: string }[] = [
  { key: 'history', label: 'Watch History' },
  { key: 'locations', label: 'Locations Map' },
  { key: 'violations', label: 'Violations' },
]

interface UserDetailProps {
  userName?: string
}

const roleBadgeClass: Record<Role, string> = {
  admin: 'badge-warn',
  viewer: 'badge-muted',
}

export function UserDetail({ userName }: UserDetailProps) {
  const { name } = useParams<{ name: string }>()
  const { user: currentUser } = useAuth()
  const isAdmin = currentUser?.role === 'admin'
  const decodedName = userName ?? (name ? decodeURIComponent(name) : '')
  const encodedName = encodeURIComponent(decodedName)
  const isOwnPage = currentUser?.name === decodedName

  const { data: trustVisibility } = useFetch<{ enabled: boolean }>(
    !isAdmin && isOwnPage ? '/api/settings/trust-visibility' : null
  )
  const showTrustScore = isAdmin || (isOwnPage && trustVisibility?.enabled === true)

  const tabs = showTrustScore ? allTabs : allTabs.filter(t => t.key !== 'violations')

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

  const resetFilters = useCallback(() => {
    setPage(1)
    setViolationsPage(1)
    setSort(null)
  }, [])

  const handleTabChange = useCallback((newTab: Tab) => {
    setTab(newTab)
    resetFilters()
  }, [resetFilters])

  const handleViolationsClick = useCallback(() => {
    handleTabChange('violations')
  }, [handleTabChange])

  const userBaseUrl = decodedName ? `/api/users/${encodedName}` : null

  const { data: user, loading: userLoading, error: userError } = useFetch<User>(userBaseUrl)

  const { data: stats, loading: statsLoading, error: statsError } = useFetch<UserDetailStats>(
    userBaseUrl ? `${userBaseUrl}/stats` : null
  )

  const historyUrl = decodedName
    ? `/api/history?user=${encodedName}&page=${page}&per_page=${PER_PAGE}${sortParams}`
    : null
  const { data: history, loading: historyLoading } = useFetch<PaginatedResult<WatchHistoryEntry>>(
    tab === 'history' ? historyUrl : null
  )

  const { data: violations, loading: violationsLoading } = useFetch<PaginatedResult<RuleViolation>>(
    showTrustScore && tab === 'violations' && decodedName
      ? `/api/users/${encodedName}/violations?page=${violationsPage}&per_page=${PER_PAGE}`
      : null
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
            {showTrustScore && (
              <UserTrustScoreCard userName={decodedName} onViolationsClick={handleViolationsClick} />
            )}
          </div>
          <div className={`grid grid-cols-1 md:grid-cols-2 ${isAdmin ? 'lg:grid-cols-4' : 'lg:grid-cols-3'} gap-4`}>
            {isAdmin && <UserHouseholdCard userName={decodedName} />}
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
            onClick={() => handleTabChange(t.key)}
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

      {tab === 'locations' && (
        <LocationMap userName={decodedName} />
      )}

      {showTrustScore && tab === 'violations' && (
        <div>
          {!violations && violationsLoading ? (
            <div className="py-12 text-center text-muted dark:text-muted-dark text-sm">
              Loading violations...
            </div>
          ) : (
            <>
              <ViolationsTable
                violations={violations?.items ?? []}
                loading={violationsLoading}
              />
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
