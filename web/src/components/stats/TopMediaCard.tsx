import type { MediaStat } from '../../types'
import { formatHours } from '../../lib/format'

interface TopMediaCardProps {
  title: string
  items: MediaStat[]
  icon: string
}

export function TopMediaCard({ title, items, icon }: TopMediaCardProps) {
  return (
    <div className="card p-4">
      <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
        <span className="opacity-50">{icon}</span>
        {title}
      </h2>

      {items.length === 0 ? (
        <div className="text-center py-8 text-muted dark:text-muted-dark">
          No data available
        </div>
      ) : (
        <div className="space-y-3">
          {items.map((item, idx) => (
            <div
              key={idx}
              className="flex items-center gap-3"
            >
              <div className="w-6 h-6 rounded-full bg-accent/10 dark:bg-accent/20 flex items-center justify-center text-xs font-semibold text-accent">
                {idx + 1}
              </div>
              <div className="flex-1 min-w-0">
                <div className="font-medium truncate">
                  {item.title}
                  {item.year ? <span className="text-muted dark:text-muted-dark ml-1">({item.year})</span> : null}
                </div>
                <div className="text-sm text-muted dark:text-muted-dark">
                  {item.play_count} plays · {formatHours(item.total_hours)}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
