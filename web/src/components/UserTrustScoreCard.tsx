import { useFetch } from '../hooks/useFetch'
import { formatRelativeTime } from '../lib/format'
import type { UserTrustScore } from '../types'

interface UserTrustScoreCardProps {
  userName: string
}

function getScoreColor(score: number): string {
  if (score >= 80) return 'text-green-400'
  if (score >= 50) return 'text-amber-400'
  return 'text-red-400'
}

function getScoreBarColor(score: number): string {
  if (score >= 80) return 'bg-green-500'
  if (score >= 50) return 'bg-amber-500'
  return 'bg-red-500'
}

function CardHeader() {
  return (
    <h3 className="text-sm font-medium text-muted dark:text-muted-dark uppercase tracking-wide mb-4">
      Trust Score
    </h3>
  )
}

export function UserTrustScoreCard({ userName }: UserTrustScoreCardProps) {
  const { data: trustScore, loading, error } = useFetch<UserTrustScore>(
    `/api/users/${encodeURIComponent(userName)}/trust`
  )

  if (loading) {
    return (
      <div className="card p-4">
        <CardHeader />
        <div className="flex items-center gap-4 animate-pulse">
          <div className="h-10 w-12 rounded bg-gray-200 dark:bg-white/10" />
          <div className="flex-1">
            <div className="h-2 rounded-full bg-gray-200 dark:bg-white/10" />
          </div>
        </div>
        <div className="mt-4 space-y-1 animate-pulse">
          <div className="h-4 w-24 rounded bg-gray-200 dark:bg-white/10" />
        </div>
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

  const score = trustScore?.score ?? 100

  return (
    <div className="card p-4">
      <CardHeader />

      <div className="flex items-center gap-4">
        <div className={`text-4xl font-bold ${getScoreColor(score)}`}>
          {score}
        </div>
        <div className="flex-1">
          <div className="h-2 rounded-full bg-gray-200 dark:bg-white/10 overflow-hidden">
            <div
              className={`h-full rounded-full ${getScoreBarColor(score)}`}
              style={{ width: `${score}%` }}
            />
          </div>
        </div>
      </div>

      <div className="mt-4 space-y-1 text-sm">
        <div className="flex justify-between">
          <span className="text-muted dark:text-muted-dark">Violations</span>
          <span className={trustScore?.violation_count ? 'text-amber-400' : ''}>
            {trustScore?.violation_count ?? 0}
          </span>
        </div>
        {trustScore?.last_violation_at && (
          <div className="flex justify-between">
            <span className="text-muted dark:text-muted-dark">Last violation</span>
            <span>{formatRelativeTime(trustScore.last_violation_at)}</span>
          </div>
        )}
      </div>
    </div>
  )
}
