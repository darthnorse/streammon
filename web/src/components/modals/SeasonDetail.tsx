import { useEffect } from 'react'
import { lockBodyScroll, unlockBodyScroll } from '../../lib/bodyScroll'
import { useEpisodesChildren } from '../../hooks/useChildren'
import { useTMDBSeasonEnrichment } from '../../hooks/useTMDBSeasonEnrichment'
import { EpisodesGrid } from './EpisodesGrid'
import { WatchHistory } from './MediaDetailParts'
import type { ItemDetails, ModalEntry } from '../../types'

interface SeasonDetailProps {
  item: ItemDetails | null
  loading: boolean
  onClose: () => void
  pushModal: (entry: ModalEntry) => void
  active: boolean
}

export function SeasonDetail({ item, loading, onClose, pushModal, active }: SeasonDetailProps) {
  useEffect(() => {
    if (!active) return
    lockBodyScroll()
    return () => unlockBodyScroll()
  }, [active])

  useEffect(() => {
    if (!active) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [active, onClose])

  const { data: childrenData, loading: childrenLoading } = useEpisodesChildren(
    item?.server_id ?? 0,
    item?.id ?? null,
  )
  const { data: tmdbSeason } = useTMDBSeasonEnrichment(item?.tmdb_id, item?.season_number)

  const showId = item?.parent_id || ''
  const episodes = childrenData?.episodes ?? []
  const summary = tmdbSeason?.overview || item?.summary || ''

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center p-4 pb-20 lg:pb-4 bg-black/70 backdrop-blur-sm animate-fade-in"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
    >
      <div className="relative w-full max-w-4xl max-h-[90dvh] overflow-hidden rounded-xl bg-panel dark:bg-panel-dark shadow-2xl animate-slide-up" onClick={e => e.stopPropagation()}>
        <button onClick={onClose} aria-label="Close" className="absolute top-3 right-3 z-10 w-8 h-8 flex items-center justify-center rounded-full bg-black/40 hover:bg-black/60 text-white">
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
        {loading && (
          <div className="flex items-center justify-center py-20">
            <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin" />
          </div>
        )}
        {!loading && item && (
          <div className="overflow-y-auto max-h-[90dvh] p-5 sm:p-6 space-y-4">
            <div>
              {item.series_title && showId && (
                <button
                  onClick={() => pushModal({ type: 'show', serverId: item.server_id, itemId: showId })}
                  className="text-sm text-muted dark:text-muted-dark hover:text-accent hover:underline"
                >
                  {item.series_title}
                </button>
              )}
              <h2 className="text-2xl font-bold text-gray-900 dark:text-gray-50 mt-1">{item.title}</h2>
              {summary && <p className="mt-3 text-sm text-gray-700 dark:text-gray-300 leading-relaxed">{summary}</p>}
            </div>
            {childrenLoading ? (
              <div className="text-sm text-muted dark:text-muted-dark">Loading episodes…</div>
            ) : (
              <EpisodesGrid
                serverId={item.server_id}
                episodes={episodes}
                tmdbEpisodes={tmdbSeason?.episodes}
                pushModal={pushModal}
              />
            )}
            <WatchHistory item={item} />
          </div>
        )}
      </div>
    </div>
  )
}
