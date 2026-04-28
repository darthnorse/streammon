import { useSeasonsChildren } from '../../hooks/useChildren'
import { thumbUrl } from '../../lib/format'
import type { ModalEntry } from '../../types'

interface SeasonsGridProps {
  serverId: number
  showId: string
  pushModal: (entry: ModalEntry) => void
}

export function SeasonsGrid({ serverId, showId, pushModal }: SeasonsGridProps) {
  const { data, loading, error } = useSeasonsChildren(serverId, showId)

  if (loading) return <div className="text-sm text-muted dark:text-muted-dark">Loading seasons…</div>
  if (error) return <div className="text-sm text-red-500 dark:text-red-400">Failed to load seasons</div>
  const seasons = data?.seasons ?? []
  if (seasons.length === 0) return null

  return (
    <div className="space-y-2">
      <div className="text-sm font-medium text-gray-900 dark:text-gray-100">Seasons</div>
      <div className="flex gap-3 overflow-x-auto pb-2 -mx-5 px-5 sm:-mx-6 sm:px-6">
        {seasons.map(s => (
          <button
            key={s.id}
            onClick={() => pushModal({ type: 'season', serverId, itemId: s.id })}
            className="shrink-0 w-28 group text-left"
          >
            <div className="aspect-[2/3] rounded overflow-hidden bg-panel dark:bg-panel-dark border border-border dark:border-border-dark transition-transform duration-200 group-hover:scale-105">
              {s.thumb_url ? (
                <img src={thumbUrl(serverId, s.thumb_url)} alt={s.title} className="w-full h-full object-cover" loading="lazy" />
              ) : (
                <div className="w-full h-full flex items-center justify-center text-2xl opacity-20">📺</div>
              )}
            </div>
            <div className="mt-1 text-xs font-medium truncate group-hover:text-accent group-hover:underline" title={s.title}>{s.title}</div>
            {s.episode_count != null && (
              <div className="text-[11px] text-muted dark:text-muted-dark">{s.episode_count} episode{s.episode_count !== 1 ? 's' : ''}</div>
            )}
          </button>
        ))}
      </div>
    </div>
  )
}
