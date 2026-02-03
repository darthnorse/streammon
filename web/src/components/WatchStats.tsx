import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { MediaStatCard } from './stats/MediaStatCard'
import { TopUsersCard } from './stats/TopUsersCard'
import type { StatsResponse } from '../types'

type TimePeriod = 7 | 30 | 0

const SKELETON_ITEMS = [1, 2, 3] as const

function LoadingSkeleton() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {SKELETON_ITEMS.map(i => (
        <div key={i} className="card p-4 animate-pulse">
          <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-1/3 mb-4" />
          <div className="space-y-3">
            {SKELETON_ITEMS.map(j => (
              <div key={j} className="flex gap-3">
                <div className="w-8 h-12 bg-gray-200 dark:bg-gray-700 rounded" />
                <div className="flex-1 space-y-2">
                  <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-3/4" />
                  <div className="h-2 bg-gray-200 dark:bg-gray-700 rounded w-1/2" />
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

function StatsContent({ data }: { data: StatsResponse }) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <MediaStatCard title="Most Watched Movies" items={data.top_movies} />
      <MediaStatCard title="Most Watched TV Shows" items={data.top_tv_shows} />
      <TopUsersCard users={data.top_users} compact />
    </div>
  )
}

export function WatchStats() {
  const [days, setDays] = useState<TimePeriod>(30)
  const { data, loading, error } = useFetch<StatsResponse>(`/api/stats?days=${days}`)

  const renderContent = () => {
    if (error) {
      return (
        <div className="text-sm text-red-500 dark:text-red-400">
          Failed to load statistics
        </div>
      )
    }
    if (loading) {
      return <LoadingSkeleton />
    }
    if (data) {
      return <StatsContent data={data} />
    }
    return null
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium text-muted dark:text-muted-dark uppercase tracking-wide">
          Watch Statistics
        </h2>
        <select
          value={days}
          onChange={(e) => setDays(Number(e.target.value) as TimePeriod)}
          aria-label="Time period"
          className="text-sm bg-panel dark:bg-panel-dark border border-border dark:border-border-dark rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-accent"
        >
          <option value={7}>Last 7 days</option>
          <option value={30}>Last 30 days</option>
          <option value={0}>All time</option>
        </select>
      </div>
      {renderContent()}
    </div>
  )
}
