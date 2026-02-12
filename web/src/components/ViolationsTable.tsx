import { Link } from 'react-router-dom'
import type { RuleViolation, Severity } from '../types'
import { formatDate } from '../lib/format'
import { SEVERITY_COLORS } from '../lib/constants'

interface ViolationsTableProps {
  violations: RuleViolation[]
  loading?: boolean
}

function SeverityBadge({ severity, className = '' }: { severity: Severity; className?: string }) {
  return (
    <span className={`px-2 py-0.5 text-xs rounded-full ${SEVERITY_COLORS[severity]} ${className}`}>
      {severity}
    </span>
  )
}

function ViolationCard({ violation }: { violation: RuleViolation }) {
  return (
    <div className="card p-4" data-testid="violation-card">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="font-medium">{violation.rule_name ?? 'Unknown rule'}</div>
          <p className="text-sm text-muted dark:text-muted-dark mt-1 line-clamp-2">
            {violation.message}
          </p>
        </div>
        <SeverityBadge severity={violation.severity} className="shrink-0" />
      </div>
      <div className="flex items-center gap-3 mt-2 text-xs text-muted dark:text-muted-dark">
        <Link to={`/users/${encodeURIComponent(violation.user_name)}`} className="text-accent-dim dark:text-accent hover:underline">
          {violation.user_name}
        </Link>
        <span>&middot;</span>
        <span>{formatDate(violation.occurred_at)}</span>
        <span>&middot;</span>
        <span>{Math.round(violation.confidence_score)}% confidence</span>
      </div>
    </div>
  )
}

export function ViolationsTable({ violations, loading }: ViolationsTableProps) {
  const loadingClass = loading ? 'opacity-50 pointer-events-none' : ''

  if (loading && violations.length === 0) {
    return (
      <div className={`card p-12 text-center ${loadingClass}`}>
        <p className="text-muted dark:text-muted-dark">Loading violations&hellip;</p>
      </div>
    )
  }

  if (violations.length === 0) {
    return (
      <div className="card p-12 text-center">
        <div className="text-4xl mb-3 opacity-30">✓</div>
        <p className="text-muted dark:text-muted-dark">No violations detected</p>
      </div>
    )
  }

  return (
    <>
      <div className={`md:hidden space-y-3 ${loadingClass}`}>
        {violations.map(violation => (
          <ViolationCard key={violation.id} violation={violation} />
        ))}
      </div>

      <div className={`hidden md:block card overflow-hidden ${loadingClass}`}>
        <div className="flex items-center justify-between px-4 py-2 border-b border-border dark:border-border-dark">
          <span className="text-xs text-muted dark:text-muted-dark uppercase tracking-wider">
            {violations.length} {violations.length === 1 ? 'violation' : 'violations'}
          </span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border dark:border-border-dark text-left text-xs
                            text-muted dark:text-muted-dark uppercase tracking-wider">
                <th className="px-4 py-3 font-medium">Time</th>
                <th className="px-4 py-3 font-medium">User</th>
                <th className="px-4 py-3 font-medium">Rule</th>
                <th className="px-4 py-3 font-medium">Severity</th>
                <th className="px-4 py-3 font-medium">Confidence</th>
                <th className="px-4 py-3 font-medium">Message</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border dark:divide-border-dark">
              {violations.map((v) => (
                <tr key={v.id} data-testid="violation-row" className="hover:bg-gray-50 dark:hover:bg-white/[0.02] transition-colors">
                  <td className="px-4 py-3 whitespace-nowrap">
                    {formatDate(v.occurred_at)}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <Link to={`/users/${encodeURIComponent(v.user_name)}`} className="text-accent-dim dark:text-accent hover:underline">
                      {v.user_name}
                    </Link>
                  </td>
                  <td className="px-4 py-3">{v.rule_name ?? 'Unknown rule'}</td>
                  <td className="px-4 py-3">
                    <SeverityBadge severity={v.severity} />
                  </td>
                  <td className="px-4 py-3">{Math.round(v.confidence_score)}%</td>
                  <td className="px-4 py-3 max-w-md truncate" title={v.message}>{v.message}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </>
  )
}
