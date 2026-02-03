import type { MediaStat } from '../../types'
import { formatHours } from '../../lib/format'

interface MediaStatCardProps {
  title: string
  items: MediaStat[]
}

export function MediaStatCard({ title, items }: MediaStatCardProps) {
  return (
    <div className="card p-4">
      <h3 className="text-sm font-medium text-muted dark:text-muted-dark mb-3">
        {title}
      </h3>

      {items.length === 0 ? (
        <div className="text-sm text-muted dark:text-muted-dark py-4 text-center">
          No data available
        </div>
      ) : (
        <div className="space-y-2">
          {items.slice(0, 5).map((item, idx) => (
            <div key={`${item.title}-${item.year ?? idx}`} className="flex items-center gap-3">
              {/* Poster thumbnail */}
              <div className="w-8 h-12 rounded bg-gray-100 dark:bg-white/5 overflow-hidden shrink-0">
                {item.thumb_url && item.server_id ? (
                  <img
                    src={`/api/servers/${item.server_id}/thumb/${item.thumb_url}`}
                    alt=""
                    className="w-full h-full object-cover"
                    loading="lazy"
                  />
                ) : (
                  <div className="w-full h-full flex items-center justify-center text-xs font-medium text-muted dark:text-muted-dark">
                    {idx + 1}
                  </div>
                )}
              </div>

              {/* Title and stats */}
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium truncate" title={item.title}>
                  {item.title}
                  {item.year ? (
                    <span className="text-muted dark:text-muted-dark ml-1">({item.year})</span>
                  ) : null}
                </div>
                <div className="text-xs text-muted dark:text-muted-dark">
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
