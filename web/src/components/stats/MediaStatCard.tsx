import { useState } from 'react'
import type { MediaStat } from '../../types'
import { formatHours } from '../../lib/format'
import { useItemDetails } from '../../hooks/useItemDetails'
import { MediaDetailModal } from '../MediaDetailModal'

interface SelectedItem {
  serverId: number
  itemId: string
}

interface MediaStatCardProps {
  title: string
  items: MediaStat[]
}

export function MediaStatCard({ title, items }: MediaStatCardProps) {
  const [hoveredIdx, setHoveredIdx] = useState<number | null>(null)
  const [selectedItem, setSelectedItem] = useState<SelectedItem | null>(null)

  const { data: itemDetails, loading: detailsLoading } = useItemDetails(
    selectedItem?.serverId ?? 0,
    selectedItem?.itemId ?? null
  )

  const displayedItem = hoveredIdx !== null ? items[hoveredIdx] : items[0]

  const handleItemClick = (item: MediaStat) => {
    if (item.item_id && item.server_id) {
      setSelectedItem({ serverId: item.server_id, itemId: item.item_id })
    }
  }

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
        <div className="flex gap-4">
          <div
            className={`w-20 h-28 rounded bg-gray-100 dark:bg-white/5 overflow-hidden shrink-0 ${
              displayedItem?.item_id && displayedItem?.server_id ? 'cursor-pointer' : ''
            }`}
            onClick={() => displayedItem && handleItemClick(displayedItem)}
          >
            {displayedItem?.thumb_url && displayedItem?.server_id ? (
              <img
                src={`/api/servers/${displayedItem.server_id}/thumb/${displayedItem.thumb_url}`}
                alt=""
                className="w-full h-full object-cover transition-opacity duration-200"
                loading="lazy"
              />
            ) : (
              <div className="w-full h-full flex items-center justify-center text-2xl opacity-20">
                🎬
              </div>
            )}
          </div>

          <div className="flex-1 min-w-0 space-y-1">
            {items.slice(0, 5).map((item, idx) => (
              <div
                key={`${idx}-${item.title}-${item.year ?? 0}`}
                className={`flex items-center gap-2 py-0.5 px-1 -mx-1 rounded transition-colors hover:bg-panel-hover dark:hover:bg-panel-hover-dark ${
                  item.item_id && item.server_id ? 'cursor-pointer' : ''
                }`}
                onMouseEnter={() => setHoveredIdx(idx)}
                onMouseLeave={() => setHoveredIdx(null)}
                onClick={() => handleItemClick(item)}
              >
                <div className="w-5 h-5 rounded-full bg-accent/20 dark:bg-accent/10 flex items-center justify-center text-[10px] font-medium text-accent shrink-0">
                  {idx + 1}
                </div>
                <div className="flex-1 min-w-0 text-xs font-medium truncate" title={item.title}>
                  {item.title}
                  {item.year ? (
                    <span className="text-muted dark:text-muted-dark ml-1">({item.year})</span>
                  ) : null}
                </div>
                <div className="text-[10px] text-muted dark:text-muted-dark whitespace-nowrap">
                  {item.play_count} plays · {formatHours(item.total_hours)}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

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
