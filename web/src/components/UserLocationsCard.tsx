import type { LocationStat } from '../types'
import { formatLocation, formatRelativeTime } from '../lib/format'

interface UserLocationsCardProps {
  locations: LocationStat[]
}

export function UserLocationsCard({ locations }: UserLocationsCardProps) {
  if (locations.length === 0) {
    return (
      <div className="card p-4">
        <h3 className="text-sm font-medium text-muted dark:text-muted-dark uppercase tracking-wide mb-4">
          Locations
        </h3>
        <p className="text-sm text-muted dark:text-muted-dark">No location data</p>
      </div>
    )
  }

  return (
    <div className="card p-4">
      <h3 className="text-sm font-medium text-muted dark:text-muted-dark uppercase tracking-wide mb-4">
        Locations
      </h3>
      <div className="space-y-3">
        {locations.map((loc) => (
          <div key={`${loc.city}-${loc.country}`} className="flex items-center gap-3">
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between text-sm mb-1">
                <span className="truncate">{formatLocation(loc.city, loc.country)}</span>
                <span className="text-muted dark:text-muted-dark ml-2 shrink-0">
                  {loc.session_count}
                </span>
              </div>
              <div className="flex items-center gap-2">
                <div className="flex-1 h-1.5 rounded-full bg-gray-200 dark:bg-white/10 overflow-hidden">
                  <div
                    className="h-full rounded-full bg-accent"
                    style={{ width: `${loc.percentage}%` }}
                  />
                </div>
                <span className="text-xs text-muted dark:text-muted-dark w-12 text-right shrink-0">
                  {loc.percentage.toFixed(0)}%
                </span>
              </div>
              {loc.last_seen && (
                <div className="text-xs text-muted dark:text-muted-dark mt-1">
                  Last seen: {formatRelativeTime(loc.last_seen)}
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
