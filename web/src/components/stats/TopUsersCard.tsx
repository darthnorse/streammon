import { Link } from 'react-router-dom'
import type { UserStat } from '../../types'
import { formatHours } from '../../lib/format'

interface TopUsersCardProps {
  users: UserStat[]
  compact?: boolean
}

export function TopUsersCard({ users, compact = false }: TopUsersCardProps) {
  if (compact) {
    return (
      <div className="card p-4">
        <h3 className="text-sm font-medium text-muted dark:text-muted-dark mb-3">
          Top Users
        </h3>

        {users.length === 0 ? (
          <div className="text-sm text-muted dark:text-muted-dark py-4 text-center">
            No data available
          </div>
        ) : (
          <div className="space-y-2">
            {users.slice(0, 5).map((user, idx) => (
              <div key={user.user_name} className="flex items-center gap-3">
                <div className="w-6 h-6 rounded-full bg-accent/20 dark:bg-accent/10 flex items-center justify-center text-xs font-medium text-accent shrink-0">
                  {idx + 1}
                </div>
                <div className="flex-1 min-w-0">
                  <Link
                    to={`/users/${encodeURIComponent(user.user_name)}`}
                    className="text-sm font-medium truncate block hover:text-accent transition-colors"
                  >
                    {user.user_name}
                  </Link>
                  <div className="text-xs text-muted dark:text-muted-dark">
                    {user.play_count} plays · {formatHours(user.total_hours)}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="card p-4">
      <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
        <span className="opacity-50">◉</span>
        Top Users
      </h2>

      {users.length === 0 ? (
        <div className="text-center py-8 text-muted dark:text-muted-dark">
          No user data available
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border dark:border-border-dark text-left text-muted dark:text-muted-dark">
                <th className="py-2 pr-4 font-medium w-12">#</th>
                <th className="py-2 pr-4 font-medium">User</th>
                <th className="py-2 pr-4 font-medium text-right">Plays</th>
                <th className="py-2 font-medium text-right">Watch Time</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user, idx) => (
                <tr
                  key={user.user_name}
                  className="border-b border-border/50 dark:border-border-dark/50 hover:bg-panel-hover dark:hover:bg-panel-hover-dark transition-colors"
                >
                  <td className="py-2 pr-4 text-muted dark:text-muted-dark">
                    {idx + 1}
                  </td>
                  <td className="py-2 pr-4">
                    <Link
                      to={`/users/${encodeURIComponent(user.user_name)}`}
                      className="font-medium hover:text-accent transition-colors"
                    >
                      {user.user_name}
                    </Link>
                  </td>
                  <td className="py-2 pr-4 text-right tabular-nums">
                    {user.play_count.toLocaleString()}
                  </td>
                  <td className="py-2 text-right tabular-nums">
                    {formatHours(user.total_hours)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
