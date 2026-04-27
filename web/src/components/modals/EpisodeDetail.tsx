import { useEffect, useMemo, useCallback } from 'react'
import type { ItemDetails, ModalEntry, TMDBCrew } from '../../types'
import { formatDuration, thumbUrl } from '../../lib/format'
import { useTMDBEnrichment } from '../../hooks/useTMDBEnrichment'
import { useModalStack } from '../../hooks/useModalStack'
import { lockBodyScroll, unlockBodyScroll } from '../../lib/bodyScroll'
import { TMDB_IMG } from '../../lib/tmdb'
import { CastChip } from '../CastChip'
import { ModalStackRenderer } from '../ModalStackRenderer'
import {
  StarRating,
  TechInfo,
  WatchHistory,
  serverAccent,
  defaultAccent,
  mediaTypeIcons,
} from './MediaDetailParts'

interface EpisodeDetailProps {
  item: ItemDetails | null
  loading: boolean
  onClose: () => void
  pushModal: (entry: ModalEntry) => void
  active: boolean
}

export function EpisodeDetail({ item, loading, onClose, pushModal, active }: EpisodeDetailProps) {
  const { stack, push: pushInner, pop: popInner } = useModalStack()

  const enrichment = useTMDBEnrichment(item?.tmdb_id, item?.media_type)
  const tmdbTV = enrichment.tv

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key !== 'Escape') return
    if (stack.length > 0) return
    e.stopImmediatePropagation()
    onClose()
  }, [onClose, stack.length])

  useEffect(() => {
    if (!active) return
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [handleKeyDown, active])

  useEffect(() => {
    if (!active) return
    lockBodyScroll()
    return () => unlockBodyScroll()
  }, [active])

  const handlePersonClick = useCallback((personId: number) => {
    pushInner({ type: 'person', personId })
  }, [pushInner])

  const title = item?.title || ''
  const overview = item?.summary || tmdbTV?.overview
  const backdrop = tmdbTV?.backdrop_path
  const serverThumbSrc = item?.thumb_url ? thumbUrl(item.server_id, item.thumb_url) : undefined
  const posterSrc = serverThumbSrc
  const year = item?.year ? String(item.year) : undefined
  const durationStr = item?.duration_ms ? formatDuration(item.duration_ms) : undefined
  const rating = item?.rating
  const contentRating = item?.content_rating

  const tmdbCast = tmdbTV?.credits?.cast
  const tmdbCrew = tmdbTV?.credits?.crew
  const tmdbDirectors = tmdbCrew?.filter((c: TMDBCrew) => c.job === 'Director')
  const directorNames = tmdbDirectors && tmdbDirectors.length > 0
    ? tmdbDirectors.map(d => d.name)
    : item?.directors && item.directors.length > 0
      ? item.directors
      : null
  const hasTMDBCast = tmdbCast && tmdbCast.length > 0

  const serverThumbByName = useMemo(() => {
    const map = new Map<string, string>()
    if (item?.cast) {
      for (const m of item.cast) {
        if (m.thumb_url) map.set(m.name, m.thumb_url)
      }
    }
    return map
  }, [item?.cast])

  const accent = item ? (serverAccent[item.server_type] ?? defaultAccent) : defaultAccent
  const titleId = `modal-title-${item?.id ?? 'loading'}`

  return (
    <>
      <div
        className="fixed inset-0 z-[60] flex items-center justify-center p-4 pb-20 lg:pb-4 bg-black/70 backdrop-blur-sm animate-fade-in"
        onClick={onClose}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-hidden={stack.length > 0 || undefined}
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
                <div className="flex gap-5 sm:gap-6">
                  {posterSrc && (
                    <div className="shrink-0 hidden sm:block">
                      <img src={posterSrc} alt={title} className="w-44 md:w-52 rounded-lg shadow-lg" />
                    </div>
                  )}
                  {!posterSrc && (
                    <div className="shrink-0 hidden sm:flex w-44 md:w-52 aspect-[2/3] rounded-lg bg-gray-200 dark:bg-white/10 items-center justify-center">
                      <span className="text-6xl opacity-20">
                        {mediaTypeIcons[item.media_type] ?? '📺'}
                      </span>
                    </div>
                  )}

                  <div className="flex-1 min-w-0 space-y-3">
                    <div>
                      {item.series_title && (
                        <div className="text-sm text-muted dark:text-muted-dark mb-1">
                          {item.series_id ? (
                            <button
                              onClick={() => pushModal({ type: 'show', serverId: item.server_id, itemId: item.series_id! })}
                              className="hover:text-accent hover:underline"
                            >
                              {item.series_title}
                            </button>
                          ) : (
                            <span>{item.series_title}</span>
                          )}
                          {item.season_number != null && (
                            <>
                              <span> · </span>
                              {item.parent_id ? (
                                <button
                                  onClick={() => pushModal({ type: 'season', serverId: item.server_id, itemId: item.parent_id! })}
                                  className="hover:text-accent hover:underline"
                                >
                                  S{item.season_number}
                                </button>
                              ) : (
                                <span>S{item.season_number}</span>
                              )}
                              {item.episode_number != null && <span>E{item.episode_number}</span>}
                            </>
                          )}
                        </div>
                      )}
                      <h2 id={titleId} className="text-2xl font-bold text-gray-900 dark:text-gray-50">
                        {title}
                      </h2>
                      <div className="flex flex-wrap items-center gap-2 mt-1.5 text-sm text-muted dark:text-muted-dark">
                        {year && <span>{year}</span>}
                        {durationStr && (
                          <>
                            <span>·</span>
                            <span>{durationStr}</span>
                          </>
                        )}
                        {contentRating && (
                          <>
                            <span>·</span>
                            <span className="px-1.5 py-0.5 text-xs border border-current rounded">
                              {contentRating}
                            </span>
                          </>
                        )}
                        {rating != null && rating > 0 && (
                          <>
                            <span>·</span>
                            <StarRating rating={rating} />
                          </>
                        )}
                      </div>
                    </div>

                    {overview && (
                      <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed">
                        {overview}
                      </p>
                    )}

                    {directorNames && (
                      <div className="text-sm">
                        <span className="text-muted dark:text-muted-dark">Directed by </span>
                        <span className="font-medium text-gray-900 dark:text-gray-100">
                          {directorNames.join(', ')}
                        </span>
                      </div>
                    )}
                  </div>
                </div>

                {(hasTMDBCast || (item.cast && item.cast.length > 0)) && (
                  <div className="space-y-2">
                    <div className="text-sm font-medium text-gray-900 dark:text-gray-100">Cast</div>
                    <div className="flex gap-2 overflow-x-auto pb-2 -mx-5 px-5 sm:-mx-6 sm:px-6">
                      {hasTMDBCast
                        ? tmdbCast.slice(0, 8).map(person => {
                            const serverThumb = !person.profile_path ? serverThumbByName.get(person.name) : undefined
                            return (
                              <CastChip
                                key={person.id}
                                name={person.name}
                                character={person.character}
                                profilePath={person.profile_path}
                                imgSrc={serverThumb ? thumbUrl(item.server_id, serverThumb) : undefined}
                                onClick={() => handlePersonClick(person.id)}
                              />
                            )
                          })
                        : item.cast!.slice(0, 6).map((member, idx) => (
                            <CastChip
                              key={`${member.name}-${idx}`}
                              name={member.name}
                              character={member.role}
                              imgSrc={member.thumb_url ? thumbUrl(item.server_id, member.thumb_url) : undefined}
                            />
                          ))
                      }
                    </div>
                  </div>
                )}

                <TechInfo item={item} />
                <WatchHistory item={item} />

                <div className="pt-2 flex items-center justify-between text-xs text-muted dark:text-muted-dark border-t border-border dark:border-border-dark">
                  <span>{item.studio}</span>
                  <span>{item.server_name}</span>
                </div>
              </div>
            </div>
          )}

          {!loading && !item && (
            <div className="p-8 text-center text-muted dark:text-muted-dark">
              Failed to load item details
            </div>
          )}
        </div>
      </div>

      {stack.length > 0 && (
        <ModalStackRenderer
          stack={stack}
          pushModal={pushInner}
          popModal={popInner}
          overseerrConfigured={false}
          libraryIds={new Set()}
        />
      )}
    </>
  )
}
