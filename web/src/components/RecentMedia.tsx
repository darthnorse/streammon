import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useItemDetails } from '../hooks/useItemDetails'
import { MediaDetailModal } from './MediaDetailModal'
import type { LibraryItem } from '../types'

const serverColors: Record<string, string> = {
  plex: 'bg-amber-500',
  emby: 'bg-green-500',
  jellyfin: 'bg-purple-500',
}

interface SelectedItem {
  serverId: number
  itemId: string
}

export function RecentMedia() {
  const { data, loading, error } = useFetch<LibraryItem[]>('/api/dashboard/recent-media')
  const [selectedItem, setSelectedItem] = useState<SelectedItem | null>(null)

  const { data: itemDetails, loading: detailsLoading } = useItemDetails(
    selectedItem?.serverId ?? 0,
    selectedItem?.itemId ?? null
  )

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

  const handleItemClick = (item: LibraryItem) => {
    if (item.item_id) {
      setSelectedItem({ serverId: item.server_id, itemId: item.item_id })
    }
  }

  return (
    <div className="space-y-3">
      <h2 className="text-sm font-medium text-muted dark:text-muted-dark uppercase tracking-wide">Recently Added</h2>
      <div className="flex gap-3 overflow-x-auto pb-2 -mx-4 px-4 scrollbar-thin scrollbar-thumb-gray-300 dark:scrollbar-thumb-gray-600">
        {data.slice(0, 25).map(item => (
          <div
            key={`${item.server_id}-${item.item_id || item.title}`}
            className={`relative group shrink-0 w-24 ${item.item_id ? 'cursor-pointer' : ''}`}
            onClick={() => handleItemClick(item)}
          >
            <div className="aspect-[2/3] rounded overflow-hidden bg-panel dark:bg-panel-dark border border-border dark:border-border-dark transition-transform duration-200 group-hover:scale-105 group-hover:shadow-lg">
              {item.thumb_url ? (
                <img
                  src={`/api/servers/${item.server_id}/thumb/${item.thumb_url}`}
                  alt={item.title}
                  className="w-full h-full object-cover"
                  loading="lazy"
                />
              ) : (
                <div className="w-full h-full flex items-center justify-center text-2xl opacity-20">
                  🎬
                </div>
              )}
            </div>
            <div
              className={`absolute top-1 right-1 w-2 h-2 rounded-full ${serverColors[item.server_type] || 'bg-gray-500'}`}
              title={item.server_name}
            />
            <div className="mt-1">
              <div className="text-[11px] font-medium truncate" title={item.title}>
                {item.title}
              </div>
              <div className="text-[10px] text-muted dark:text-muted-dark truncate">
                {item.media_type === 'episode' && item.season_number != null && item.episode_number != null
                  ? `S${item.season_number} · E${item.episode_number}`
                  : item.year && item.year > 0 ? item.year : ''}
              </div>
            </div>
          </div>
        ))}
      </div>

      {selectedItem && (
        <MediaDetailModal
          item={itemDetails}
          loading={detailsLoading}
          onClose={() => setSelectedItem(null)}
        />
      )}
    </div>
  )
}
