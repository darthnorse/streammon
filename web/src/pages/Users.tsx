import { useState, useMemo, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useFetch } from '../hooks/useFetch'
import { useItemDetails } from '../hooks/useItemDetails'
import { MediaDetailModal } from '../components/MediaDetailModal'
import type { UserSummary } from '../types'
import { MS_PER_HOUR, MS_PER_DAY } from '../lib/constants'

type SortField = 'name' | 'last_streamed_at' | 'last_played' | 'last_ip' | 'total_plays' | 'total_watched_ms' | 'trust_score'
type SortDir = 'asc' | 'desc'

interface SortState {
  field: SortField
  dir: SortDir
}

const STORAGE_KEY = 'streammon-users-sort'
const VALID_FIELDS: SortField[] = ['name', 'last_streamed_at', 'last_played', 'last_ip', 'total_plays', 'total_watched_ms', 'trust_score']
const VALID_DIRS: SortDir[] = ['asc', 'desc']

function loadSortState(): SortState {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      const parsed = JSON.parse(stored)
      if (VALID_FIELDS.includes(parsed.field) && VALID_DIRS.includes(parsed.dir)) {
        return parsed
      }
    }
  } catch {}
  return { field: 'name', dir: 'asc' }
}

function saveSortState(state: SortState) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
}

function formatDuration(ms: number): string {
  const hours = ms / MS_PER_HOUR
  if (hours < 1) {
    const mins = Math.round(ms / 60000)
    return `${mins}m`
  }
  if (hours < 24) {
    return `${hours.toFixed(1)}h`
  }
  const days = hours / 24
  return `${days.toFixed(1)}d`
}

function formatDate(dateStr: string | null): string {
  if (!dateStr) return 'Never'
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / MS_PER_DAY)

  if (diffDays === 0) return 'Today'
  if (diffDays === 1) return 'Yesterday'
  if (diffDays < 7) return `${diffDays} days ago`
  if (diffDays < 30) return `${Math.floor(diffDays / 7)} weeks ago`
  return date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}

function getTrustColor(score: number): string {
  if (score >= 80) return 'text-green-400'
  if (score >= 50) return 'text-amber-400'
  return 'text-red-400'
}

export function Users() {
  const { data: users, loading, error } = useFetch<UserSummary[]>('/api/users/summary')
  const [sort, setSort] = useState<SortState>(loadSortState)
  const [selectedItem, setSelectedItem] = useState<{ serverId: number; itemId: string } | null>(null)
  const { data: itemDetails, loading: itemLoading } = useItemDetails(
    selectedItem?.serverId ?? 0,
    selectedItem?.itemId ?? null
  )

  useEffect(() => {
    saveSortState(sort)
  }, [sort])

  const sortedUsers = useMemo(() => {
    if (!users) return []
    return [...users].sort((a, b) => {
      const dir = sort.dir === 'asc' ? 1 : -1
      switch (sort.field) {
        case 'name':
          return dir * a.name.localeCompare(b.name)
        case 'last_streamed_at': {
          const aTime = a.last_streamed_at ? new Date(a.last_streamed_at).getTime() : 0
          const bTime = b.last_streamed_at ? new Date(b.last_streamed_at).getTime() : 0
          return dir * (aTime - bTime)
        }
        case 'last_played': {
          const aTitle = a.last_played_grandparent_title || a.last_played_title
          const bTitle = b.last_played_grandparent_title || b.last_played_title
          return dir * aTitle.localeCompare(bTitle)
        }
        case 'last_ip':
          return dir * a.last_ip.localeCompare(b.last_ip)
        case 'total_plays':
          return dir * (a.total_plays - b.total_plays)
        case 'total_watched_ms':
          return dir * (a.total_watched_ms - b.total_watched_ms)
        case 'trust_score':
          return dir * (a.trust_score - b.trust_score)
        default:
          return 0
      }
    })
  }, [users, sort])

  function handleSort(field: SortField) {
    setSort(prev => ({
      field,
      dir: prev.field === field && prev.dir === 'asc' ? 'desc' : 'asc'
    }))
  }

  function SortHeader({ field, children }: { field: SortField; children: React.ReactNode }) {
    const isActive = sort.field === field
    const handleKeyDown = (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault()
        handleSort(field)
      }
    }
    return (
      <th
        onClick={() => handleSort(field)}
        onKeyDown={handleKeyDown}
        tabIndex={0}
        role="button"
        aria-sort={isActive ? (sort.dir === 'asc' ? 'ascending' : 'descending') : undefined}
        className="pb-3 font-medium text-left cursor-pointer hover:text-accent transition-colors select-none focus:outline-none focus:text-accent"
      >
        <span className="inline-flex items-center gap-1">
          {children}
          {isActive && (
            <span className="text-accent">{sort.dir === 'asc' ? '▲' : '▼'}</span>
          )}
        </span>
      </th>
    )
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <span className="text-muted dark:text-muted-dark">Loading users...</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-64">
        <span className="text-red-400">Failed to load users</span>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Users</h1>
        <span className="text-sm text-muted dark:text-muted-dark">
          {users?.length || 0} users
        </span>
      </div>

      {sortedUsers.length === 0 ? (
        <div className="card p-8 text-center text-muted dark:text-muted-dark">
          No users found. Users are created when they start streaming.
        </div>
      ) : (
        <div className="card overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="text-muted dark:text-muted-dark border-b border-border dark:border-border-dark">
              <tr>
                <SortHeader field="name">User</SortHeader>
                <SortHeader field="last_streamed_at">Last Streamed</SortHeader>
                <SortHeader field="last_played">Last Played</SortHeader>
                <SortHeader field="last_ip">Last IP</SortHeader>
                <SortHeader field="total_plays">Total Plays</SortHeader>
                <SortHeader field="total_watched_ms">Watch Time</SortHeader>
                <SortHeader field="trust_score">Trust</SortHeader>
              </tr>
            </thead>
            <tbody className="divide-y divide-border dark:divide-border-dark">
              {sortedUsers.map(user => (
                <tr key={user.name} className="hover:bg-surface/50 dark:hover:bg-surface-dark/50">
                  <td className="py-3 pr-4">
                    <Link
                      to={`/users/${encodeURIComponent(user.name)}`}
                      className="flex items-center gap-3 hover:text-accent transition-colors"
                    >
                      {user.thumb_url ? (
                        <img
                          src={user.thumb_url}
                          alt=""
                          className="w-8 h-8 rounded-full object-cover bg-surface dark:bg-surface-dark"
                        />
                      ) : (
                        <div className="w-8 h-8 rounded-full bg-surface dark:bg-surface-dark flex items-center justify-center text-xs font-medium">
                          {user.name.charAt(0).toUpperCase()}
                        </div>
                      )}
                      <span className="font-medium">{user.name}</span>
                    </Link>
                  </td>
                  <td className="py-3 pr-4 text-muted dark:text-muted-dark">
                    {formatDate(user.last_streamed_at)}
                  </td>
                  <td className="py-3 pr-4 max-w-[200px]">
                    {user.last_played_title ? (
                      <div
                        className="cursor-pointer hover:text-accent transition-colors"
                        onClick={() => setSelectedItem({
                          serverId: user.last_played_server_id,
                          itemId: user.last_played_media_type === 'episode' && user.last_played_grandparent_item_id
                            ? user.last_played_grandparent_item_id
                            : user.last_played_item_id
                        })}
                      >
                        <div className="font-medium truncate">
                          {user.last_played_grandparent_title || user.last_played_title}
                        </div>
                        {user.last_played_grandparent_title && (
                          <div className="text-xs text-muted dark:text-muted-dark truncate">
                            {user.last_played_title}
                          </div>
                        )}
                      </div>
                    ) : (
                      <span className="text-muted dark:text-muted-dark">-</span>
                    )}
                  </td>
                  <td className="py-3 pr-4 font-mono text-xs text-muted dark:text-muted-dark">
                    {user.last_ip || '-'}
                  </td>
                  <td className="py-3 pr-4">
                    {user.total_plays.toLocaleString()}
                  </td>
                  <td className="py-3 pr-4">
                    {formatDuration(user.total_watched_ms)}
                  </td>
                  <td className="py-3">
                    <span className={`font-medium ${getTrustColor(user.trust_score)}`}>
                      {user.trust_score}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selectedItem && (
        <MediaDetailModal
          item={itemDetails}
          loading={itemLoading}
          onClose={() => setSelectedItem(null)}
        />
      )}
    </div>
  )
}
