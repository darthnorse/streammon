import { useFetch } from '../hooks/useFetch'
import type { UserTrustScore } from '../types'

interface UserTrustScoreCardProps {
  userName: string
  onViolationsClick?: () => void
}

function getScoreColor(score: number): string {
  if (score >= 80) return 'text-green-400'
  if (score >= 50) return 'text-amber-400'
  return 'text-red-400'
}

function getScoreIcon(score: number): string {
  if (score >= 80) return '✓'
  if (score >= 50) return '!'
  return '✗'
}

export function UserTrustScoreCard({ userName, onViolationsClick }: UserTrustScoreCardProps) {
  const { data: trustScore, loading, error } = useFetch<UserTrustScore>(
    `/api/users/${encodeURIComponent(userName)}/trust`
  )

  if (loading) {
    return (
      <div className="card p-4">
        <div className="flex items-center gap-3 animate-pulse">
          <div className="text-2xl opacity-50 w-6" />
          <div>
            <div className="h-7 w-12 rounded bg-gray-200 dark:bg-white/10" />
            <div className="h-4 w-20 rounded bg-gray-200 dark:bg-white/10 mt-1" />
          </div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="card p-4">
        <div className="flex items-center gap-3">
          <div className="text-2xl opacity-50">⚠</div>
          <div>
            <div className="text-2xl font-semibold">—</div>
            <div className="text-sm text-muted dark:text-muted-dark">Trust Score</div>
          </div>
        </div>
      </div>
    )
  }

  const score = trustScore?.score ?? 100
  const violations = trustScore?.violation_count ?? 0

  return (
    <div className="card p-4">
      <div className="flex items-center gap-3">
        <div className={`text-2xl ${getScoreColor(score)}`}>{getScoreIcon(score)}</div>
        <div>
          <div className="flex items-baseline gap-2">
            <span className={`text-2xl font-semibold ${getScoreColor(score)}`}>{score}</span>
            {violations > 0 && (
              <button
                onClick={onViolationsClick}
                className="text-xs text-amber-400 hover:text-amber-300 hover:underline transition-colors"
              >
                ({violations} violation{violations !== 1 ? 's' : ''})
              </button>
            )}
          </div>
          <div className="text-sm text-muted dark:text-muted-dark">Trust Score</div>
        </div>
      </div>
    </div>
  )
}
