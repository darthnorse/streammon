import { useEffect } from 'react'
import { lockBodyScroll, unlockBodyScroll } from '../../lib/bodyScroll'
import { useEpisodesChildren } from '../../hooks/useChildren'
import { useTMDBEnrichment } from '../../hooks/useTMDBEnrichment'
import { useTMDBSeasonEnrichment } from '../../hooks/useTMDBSeasonEnrichment'
import { TMDB_IMG } from '../../lib/tmdb'
import { EpisodesGrid } from './EpisodesGrid'
import { WatchHistory, serverAccent, defaultAccent } from './MediaDetailParts'
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
  const showEnrichment = useTMDBEnrichment(item?.tmdb_id, item?.media_type)
  const tmdbTV = showEnrichment.tv

  const showId = item?.parent_id || ''
  const episodes = childrenData?.episodes ?? []
  const summary = tmdbSeason?.overview || item?.summary || ''
  const backdrop = tmdbTV?.backdrop_path
  const year = tmdbSeason?.air_date?.slice(0, 4)
    || (item?.year ? String(item.year) : undefined)
  const accent = item ? (serverAccent[item.server_type] ?? defaultAccent) : defaultAccent
  const titleId = `modal-title-${item?.id ?? 'loading'}`

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center p-4 pb-20 lg:pb-4 bg-black/70 backdrop-blur-sm animate-fade-in"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
    >
      <div
        className="relative w-full max-w-6xl max-h-[90dvh] overflow-hidden rounded-xl
                   bg-panel dark:bg-panel-dark shadow-2xl animate-slide-up"
        onClick={e => e.stopPropagation()}
      >
        {!backdrop && item && <div className={`h-1 ${accent.bar}`} />}

        <button
          onClick={onClose}
          className="absolute top-3 right-3 z-10 w-8 h-8 flex items-center justify-center
                     rounded-full bg-black/40 hover:bg-black/60 text-white transition-colors"
          aria-label="Close"
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>

        {loading && (
          <div className="flex items-center justify-center py-20">
            <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {!loading && item && (
          <div className="overflow-y-auto max-h-[calc(90dvh-4px)]">
            {backdrop && (
              <div className="relative h-48 sm:h-64 overflow-hidden">
                <img src={`${TMDB_IMG}/w1280${backdrop}`} alt="" className="w-full h-full object-cover" />
                <div className="absolute inset-0 bg-gradient-to-t from-panel dark:from-panel-dark via-transparent to-transparent" />
              </div>
            )}

            <div className={`p-5 sm:p-6 space-y-4 ${backdrop ? '-mt-28 sm:-mt-36 relative' : ''}`}>
              <div className="space-y-3">
                <div>
                  {item.series_title && showId ? (
                    <button
                      id={titleId}
                      onClick={() => pushModal({ type: 'show', serverId: item.server_id, itemId: showId })}
                      className="text-2xl font-bold text-gray-900 dark:text-gray-50 hover:text-accent hover:underline text-left"
                    >
                      {item.series_title}
                    </button>
                  ) : (
                    <h2 id={titleId} className="text-2xl font-bold text-gray-900 dark:text-gray-50">
                      {item.title}
                    </h2>
                  )}
                  {item.series_title && showId && (
                    <div className="text-base font-medium text-gray-700 dark:text-gray-300 mt-1">
                      {item.title}
                    </div>
                  )}
                  <div className="flex flex-wrap items-center gap-2 mt-1.5 text-sm text-muted dark:text-muted-dark">
                    {year && <span>{year}</span>}
                    {episodes.length > 0 && (
                      <>
                        <span>·</span>
                        <span>{episodes.length} episode{episodes.length !== 1 ? 's' : ''}</span>
                      </>
                    )}
                  </div>
                </div>

                {summary && (
                  <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed">
                    {summary}
                  </p>
                )}
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

              <div className="pt-2 flex items-center justify-end text-xs text-muted dark:text-muted-dark border-t border-border dark:border-border-dark">
                <span>{item.server_name}</span>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
