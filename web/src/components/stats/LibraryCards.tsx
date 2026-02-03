import type { LibraryStat } from '../../types'
import { formatHours } from '../../lib/format'

interface LibraryCardsProps {
  stats: LibraryStat
  concurrentPeak: number
}

interface StatCardProps {
  label: string
  value: string | number
  icon: string
}

function StatCard({ label, value, icon }: StatCardProps) {
  return (
    <div className="card p-4">
      <div className="flex items-center gap-3">
        <div className="text-2xl opacity-50">{icon}</div>
        <div>
          <div className="text-2xl font-semibold">{value}</div>
          <div className="text-sm text-muted dark:text-muted-dark">{label}</div>
        </div>
      </div>
    </div>
  )
}

export function LibraryCards({ stats, concurrentPeak }: LibraryCardsProps) {
  return (
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
      <StatCard
        label="Total Plays"
        value={stats.total_plays.toLocaleString()}
        icon="▶"
      />
      <StatCard
        label="Watch Time"
        value={formatHours(stats.total_hours)}
        icon="◷"
      />
      <StatCard
        label="Users"
        value={stats.unique_users.toLocaleString()}
        icon="◉"
      />
      <StatCard
        label="Movies"
        value={stats.unique_movies.toLocaleString()}
        icon="▣"
      />
      <StatCard
        label="TV Shows"
        value={stats.unique_tv_shows.toLocaleString()}
        icon="▤"
      />
      <StatCard
        label="Peak Streams"
        value={concurrentPeak}
        icon="≡"
      />
    </div>
  )
}
