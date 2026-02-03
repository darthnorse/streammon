import { useFetch } from '../hooks/useFetch'
import type { LibraryItem } from '../types'

const serverColors: Record<string, string> = {
  plex: 'bg-amber-500',
  emby: 'bg-green-500',
  jellyfin: 'bg-purple-500',
}

export function RecentMedia() {
  const { data, loading, error } = useFetch<LibraryItem[]>('/api/dashboard/recent-media')

  if (loading) {
    return (
      <div className="text-sm text-muted dark:text-muted-dark">
        Loading recent media...
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-sm text-red-500 dark:text-red-400">
        Failed to load recent media
      </div>
    )
  }

  if (!data?.length) {
    return null
  }

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">Recently Added</h2>
      <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8 gap-3">
        {data.map((item, idx) => (
          <div key={`${item.server_id}-${item.title}-${idx}`} className="relative group">
            <div className="aspect-[2/3] rounded-lg overflow-hidden bg-panel dark:bg-panel-dark border border-border dark:border-border-dark">
              {item.thumb_url ? (
                <img
                  src={`/api/servers/${item.server_id}/thumb/${item.thumb_url}`}
                  alt={item.title}
                  className="w-full h-full object-cover"
                  loading="lazy"
                />
              ) : (
                <div className="w-full h-full flex items-center justify-center text-3xl opacity-20">
                  🎬
                </div>
              )}
            </div>
            <div
              className={`absolute top-1.5 right-1.5 w-2.5 h-2.5 rounded-full ${serverColors[item.server_type] || 'bg-gray-500'}`}
              title={item.server_name}
            />
            <div className="mt-1.5">
              <div className="text-xs font-medium truncate" title={item.title}>
                {item.title}
              </div>
              <div className="text-xs text-muted dark:text-muted-dark truncate">
                {item.year ? `${item.year} · ${item.server_name}` : item.server_name}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
