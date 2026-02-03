import { useFetch } from '../hooks/useFetch'
import { DailyChart } from '../components/DailyChart'
import { LibraryCards } from '../components/stats/LibraryCards'
import { TopMediaCard } from '../components/stats/TopMediaCard'
import { TopUsersCard } from '../components/stats/TopUsersCard'
import { LocationsCard } from '../components/stats/LocationsCard'
import { SharerAlertsCard } from '../components/stats/SharerAlertsCard'
import type { StatsResponse } from '../types'

export function Statistics() {
  const { data, loading, error } = useFetch<StatsResponse>('/api/stats')

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
      <div className="mb-6">
        <h1 className="text-2xl font-semibold">Statistics</h1>
        <p className="text-sm text-muted dark:text-muted-dark mt-1">
          Viewing trends and insights
        </p>
      </div>

      <LibraryCards stats={data.library} concurrentPeak={data.concurrent_peak} />

      <DailyChart />

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
