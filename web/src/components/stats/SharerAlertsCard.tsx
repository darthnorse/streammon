import { Link } from 'react-router-dom'
import type { SharerAlert } from '../../types'

interface SharerAlertsCardProps {
  alerts: SharerAlert[]
}

function formatLastSeen(isoDate: string): string {
  const date = new Date(isoDate)
  if (isNaN(date.getTime())) return 'Unknown'
  return date.toLocaleDateString()
}

export function SharerAlertsCard({ alerts }: SharerAlertsCardProps) {
  return (
    <div className="card p-4 border-amber-500/50 dark:border-amber-500/30">
      <h2 className="text-lg font-semibold mb-4 flex items-center gap-2 text-amber-600 dark:text-amber-400">
        <span>⚠</span>
        Potential Password Sharing
        <span className="text-sm font-normal text-muted dark:text-muted-dark">
          ({alerts.length} {alerts.length === 1 ? 'account' : 'accounts'})
        </span>
      </h2>

      <p className="text-sm text-muted dark:text-muted-dark mb-4">
        These accounts have been used from multiple IP addresses in the last 30 days.
      </p>

      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border dark:border-border-dark text-left text-muted dark:text-muted-dark">
              <th className="py-2 pr-4 font-medium">User</th>
              <th className="py-2 pr-4 font-medium text-right">Unique IPs</th>
              <th className="py-2 pr-4 font-medium">Locations</th>
              <th className="py-2 font-medium">Last Seen</th>
            </tr>
          </thead>
          <tbody>
            {alerts.map((alert) => (
              <tr
                key={alert.user_name}
                className="border-b border-border/50 dark:border-border-dark/50"
              >
                <td className="py-2 pr-4">
                  <Link
                    to={`/users/${encodeURIComponent(alert.user_name)}`}
                    className="font-medium hover:text-accent transition-colors"
                  >
                    {alert.user_name}
                  </Link>
                </td>
                <td className="py-2 pr-4 text-right tabular-nums font-semibold text-amber-600 dark:text-amber-400">
                  {alert.unique_ips}
                </td>
                <td className="py-2 pr-4">
                  {alert.locations.length > 0 ? (
                    <span className="text-muted dark:text-muted-dark">
                      {alert.locations.slice(0, 3).join(', ')}
                      {alert.locations.length > 3 && ` +${alert.locations.length - 3} more`}
                    </span>
                  ) : (
                    <span className="text-muted dark:text-muted-dark">—</span>
                  )}
                </td>
                <td className="py-2 text-muted dark:text-muted-dark">
                  {formatLastSeen(alert.last_seen)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
