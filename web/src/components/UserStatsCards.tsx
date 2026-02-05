import type { UserDetailStats } from '../types'
import { formatHours } from '../lib/format'

interface StatCardProps {
  label: string
  value: string | number
  icon: string
}

export function StatCard({ label, value, icon }: StatCardProps) {
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

interface UserStatsCardsProps {
  stats: UserDetailStats
}

export function UserStatsCards({ stats }: UserStatsCardsProps) {
  return (
    <>
      <StatCard
        label="Sessions"
        value={stats.session_count.toLocaleString()}
        icon="▶"
      />
      <StatCard
        label="Watch Time"
        value={formatHours(stats.total_hours)}
        icon="◷"
      />
    </>
  )
}
