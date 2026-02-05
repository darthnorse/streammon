import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { api } from '../lib/api'
import type { HouseholdLocation } from '../types'

interface UserHouseholdCardProps {
  userName: string
}

function CardHeader() {
  return (
    <h3 className="text-sm font-medium text-muted dark:text-muted-dark uppercase tracking-wide mb-4">
      Household Locations
    </h3>
  )
}

export function UserHouseholdCard({ userName }: UserHouseholdCardProps) {
  const { data: locations, loading, error, refetch } = useFetch<HouseholdLocation[]>(
    `/api/users/${encodeURIComponent(userName)}/household`
  )
  const [updating, setUpdating] = useState<number | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const handleToggleTrusted = async (location: HouseholdLocation) => {
    setUpdating(location.id)
    setActionError(null)
    try {
      await api.put(
        `/api/users/${encodeURIComponent(userName)}/household/${location.id}`,
        { trusted: !location.trusted }
      )
      refetch()
    } catch (err) {
      setActionError('Failed to update location')
      console.error('Failed to update location:', err)
    } finally {
      setUpdating(null)
    }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this household location?')) return
    setActionError(null)
    try {
      await api.del(`/api/users/${encodeURIComponent(userName)}/household/${id}`)
      refetch()
    } catch (err) {
      setActionError('Failed to delete location')
      console.error('Failed to delete location:', err)
    }
  }

  if (loading) {
    return (
      <div className="card p-4">
        <CardHeader />
        <div className="text-sm text-muted dark:text-muted-dark">Loading...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="card p-4">
        <CardHeader />
        <div className="text-sm text-red-500 dark:text-red-400">Failed to load</div>
      </div>
    )
  }

  return (
    <div className="card p-4">
      <CardHeader />

      {actionError && (
        <div className="text-sm text-red-500 dark:text-red-400 mb-3">{actionError}</div>
      )}

      {!locations?.length ? (
        <div className="text-sm text-muted dark:text-muted-dark">
          No household locations recorded yet.
        </div>
      ) : (
        <div className="space-y-3">
          {locations.map(loc => (
            <div key={loc.id} className="flex items-start justify-between gap-2 text-sm">
              <div className="flex-1 min-w-0">
                <div className="font-medium truncate">
                  {loc.city || loc.ip_address}{loc.country && `, ${loc.country}`}
                </div>
                <div className="text-xs text-muted dark:text-muted-dark flex items-center gap-2 mt-0.5">
                  {loc.auto_learned ? (
                    <span className="px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-400">Auto</span>
                  ) : (
                    <span className="px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-400">Manual</span>
                  )}
                  <span>{loc.session_count} sessions</span>
                </div>
              </div>
              <div className="flex items-center gap-1">
                <button
                  onClick={() => handleToggleTrusted(loc)}
                  disabled={updating === loc.id}
                  className={`px-2 py-1 text-xs font-medium rounded transition-colors
                    ${loc.trusted
                      ? 'bg-green-500/20 text-green-400 hover:bg-green-500/30'
                      : 'bg-gray-500/20 text-gray-400 hover:bg-gray-500/30'
                    } disabled:opacity-50`}
                >
                  {loc.trusted ? 'Trusted' : 'Untrusted'}
                </button>
                <button
                  onClick={() => handleDelete(loc.id)}
                  className="p-1 rounded hover:bg-red-500/20 text-red-400 transition-colors"
                  title="Delete"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
