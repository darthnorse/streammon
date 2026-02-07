import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { DailyChart } from '../components/DailyChart'
import { LibraryCards } from '../components/stats/LibraryCards'
import { TopMediaCard } from '../components/stats/TopMediaCard'
import { TopUsersCard } from '../components/stats/TopUsersCard'
import { LocationsCard } from '../components/stats/LocationsCard'
import { SharerAlertsCard } from '../components/stats/SharerAlertsCard'
import { ActivityByDayChart } from '../components/stats/ActivityByDayChart'
import { ActivityByHourChart } from '../components/stats/ActivityByHourChart'
import { DistributionDonut } from '../components/stats/DistributionDonut'
import { ConcurrentStreamsChart } from '../components/stats/ConcurrentStreamsChart'
import type { StatsResponse } from '../types'

type DaysFilter = 0 | 7 | 30

const STORAGE_KEY = 'streammon:stats-days'

const filterOptions: { value: DaysFilter; label: string }[] = [
  { value: 0, label: 'All time' },
  { value: 7, label: 'Last 7 days' },
  { value: 30, label: 'Last 30 days' },
]

function getStoredDays(): DaysFilter {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === '7') return 7
  if (stored === '30') return 30
  return 0
}

export function Statistics() {
  const [days, setDays] = useState<DaysFilter>(getStoredDays)
  const url = days === 0 ? '/api/stats' : `/api/stats?days=${days}`
  const { data, loading, error } = useFetch<StatsResponse>(url)

  if (loading) {
    return (
      <div className="card p-12 text-center">
        <div className="text-muted dark:text-muted-dark animate-pulse">Loading statistics...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="card p-6 text-center text-red-500 dark:text-red-400">
        Error loading statistics
      </div>
    )
  }

  if (!data) return null

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">Statistics</h1>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            Viewing trends and insights
          </p>
        </div>
        <div className="flex gap-1">
          {filterOptions.map(opt => (
            <button
              key={opt.value}
              onClick={() => {
                setDays(opt.value)
                localStorage.setItem(STORAGE_KEY, String(opt.value))
              }}
              className={`px-3 py-1.5 rounded text-xs font-medium transition-colors
                ${days === opt.value
                  ? 'bg-accent/15 text-accent-dim dark:text-accent'
                  : 'text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
                }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      <LibraryCards stats={data.library} concurrentPeak={data.concurrent_peak} />

      <DailyChart days={days} />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ActivityByDayChart data={data.activity_by_day_of_week} />
        <ActivityByHourChart data={data.activity_by_hour} />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        <DistributionDonut title="Platforms" data={data.platform_distribution} />
        <DistributionDonut title="Players" data={data.player_distribution} />
        <DistributionDonut title="Stream Quality" data={data.quality_distribution} />
      </div>

      <ConcurrentStreamsChart data={data.concurrent_time_series} />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <TopMediaCard title="Most Popular Movies" items={data.top_movies} icon="▶" />
        <TopMediaCard title="Most Popular TV Shows" items={data.top_tv_shows} icon="▷" />
      </div>

      <TopUsersCard users={data.top_users} />

      <LocationsCard locations={data.locations} />

      {data.potential_sharers.length > 0 && (
        <SharerAlertsCard alerts={data.potential_sharers} />
      )}
    </div>
  )
}
