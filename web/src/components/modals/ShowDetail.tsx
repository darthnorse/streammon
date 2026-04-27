import { useEffect, useMemo, useCallback } from 'react'
import type { ItemDetails, ModalEntry, TMDBCrew } from '../../types'
import { formatDuration, thumbUrl } from '../../lib/format'
import { useTMDBEnrichment } from '../../hooks/useTMDBEnrichment'
import { useOverseerrRequest } from '../../hooks/useOverseerrRequest'
import { useModalStack } from '../../hooks/useModalStack'
import { lockBodyScroll, unlockBodyScroll } from '../../lib/bodyScroll'
import { TMDB_IMG } from '../../lib/tmdb'
import { MEDIA_STATUS, mediaStatusBadge } from '../../lib/overseerr'
import { CastChip } from '../CastChip'
import { ModalStackRenderer } from '../ModalStackRenderer'
import {
  StarRating,
  TechInfo,
  WatchHistory,
  TVStatusBadge,
  NetworkLogos,
  serverAccent,
  defaultAccent,
  mediaTypeIcons,
  regularSeasonNumbers,
  requestButtonLabel,
  resolveMediaType,
} from './MediaDetailParts'

interface ShowDetailProps {
  item: ItemDetails | null
  loading: boolean
  onClose: () => void
  pushModal: (entry: ModalEntry) => void
  active: boolean
  overseerrConfigured: boolean
}

export function ShowDetail({
  item,
  loading,
  onClose,
  active,
  overseerrConfigured,
}: ShowDetailProps) {
  const { stack, push: pushInner, pop: popInner } = useModalStack()

  const enrichment = useTMDBEnrichment(item?.tmdb_id, item?.media_type)
  const tmdbMovie = enrichment.movie
  const tmdbTV = enrichment.tv
  const tmdbLoading = enrichment.loading

  const effectiveTmdbId = item?.tmdb_id
  const effectiveMediaType = resolveMediaType(item?.media_type)

  const seasonNumbers = useMemo(
    () => tmdbTV?.seasons ? regularSeasonNumbers(tmdbTV.seasons) : [],
    [tmdbTV],
  )

  const {
    overseerrStatus,
    requesting,
    requestSuccess,
    requestError,
    selectedSeasons,
    allSeasons,
    canRequest,
    handleRequest,
    toggleSeason,
    toggleAllSeasons,
  } = useOverseerrRequest({
    overseerrConfigured,
    effectiveTmdbId,
    effectiveMediaType,
    seasonNumbers,
  })

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

  const title = item?.title || tmdbMovie?.title || tmdbTV?.name || ''
  const overview = tmdbMovie?.overview || tmdbTV?.overview || item?.summary
  const backdrop = tmdbMovie?.backdrop_path || tmdbTV?.backdrop_path
  const tmdbPoster = tmdbMovie?.poster_path || tmdbTV?.poster_path
  const serverThumbSrc = item?.thumb_url
    ? thumbUrl(item.server_id, item.thumb_url)
    : undefined
  const posterSrc = tmdbPoster ? `${TMDB_IMG}/w342${tmdbPoster}` : serverThumbSrc
  const tagline = tmdbMovie?.tagline || tmdbTV?.tagline
  const year = tmdbMovie?.release_date?.slice(0, 4)
    || tmdbTV?.first_air_date?.slice(0, 4)
    || (item?.year ? String(item.year) : undefined)
  const runtime = tmdbMovie?.runtime
  const durationStr = item?.duration_ms ? formatDuration(item.duration_ms) : (runtime ? `${runtime} min` : undefined)
  const rating = tmdbMovie?.vote_average ?? tmdbTV?.vote_average ?? item?.rating
  const contentRating = item?.content_rating

  const tmdbGenres = tmdbMovie?.genres || tmdbTV?.genres
  const serverGenres = item?.genres
  const tmdbCast = tmdbMovie?.credits?.cast || tmdbTV?.credits?.cast
  const tmdbCrew = tmdbMovie?.credits?.crew || tmdbTV?.credits?.crew
  const tmdbDirectors = tmdbCrew?.filter((c: TMDBCrew) => c.job === 'Director')
  const directorNames = tmdbDirectors && tmdbDirectors.length > 0
    ? tmdbDirectors.map(d => d.name)
    : item?.directors && item.directors.length > 0
      ? item.directors
      : null
  const hasTMDBCast = tmdbCast && tmdbCast.length > 0
  const collection = tmdbMovie?.belongs_to_collection
  const tvStatus = tmdbTV?.status
  const networks = tmdbTV?.networks
  const seasonCount = tmdbTV?.number_of_seasons
  const episodeCount = tmdbTV?.number_of_episodes

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
  const genreBadgeClass = item ? accent.badge : 'bg-accent/10 text-accent'
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
                      <img src={posterSrc} alt={title}
                        className="w-44 md:w-52 rounded-lg shadow-lg" />
                    </div>
                  )}
                  {!posterSrc && (
                    <div className="shrink-0 hidden sm:flex w-44 md:w-52 aspect-[2/3] rounded-lg bg-gray-200 dark:bg-white/10 items-center justify-center">
                      <span className="text-6xl opacity-20">
                        {mediaTypeIcons[item.media_type] ?? '🎵'}
                      </span>
                    </div>
                  )}

                  <div className="flex-1 min-w-0 space-y-3">
                    <div>
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
                        {tvStatus && (
                          <>
                            <span>·</span>
                            <TVStatusBadge status={tvStatus} />
                          </>
                        )}
                      </div>

                      {overseerrStatus && overseerrStatus > MEDIA_STATUS.UNKNOWN && (
                        <div className="mt-2">{mediaStatusBadge(overseerrStatus)}</div>
                      )}

                      {networks && networks.length > 0 && (
                        <div className="mt-2">
                          <NetworkLogos networks={networks} />
                        </div>
                      )}

                      {seasonCount != null && (
                        <div className="text-sm text-muted dark:text-muted-dark mt-1">
                          {seasonCount} season{seasonCount !== 1 ? 's' : ''}
                          {episodeCount != null && (
                            <span> · {episodeCount} episode{episodeCount !== 1 ? 's' : ''}</span>
                          )}
                        </div>
                      )}
                    </div>

                    {tagline && (
                      <p className="text-sm italic text-muted dark:text-muted-dark">&ldquo;{tagline}&rdquo;</p>
                    )}

                    {(tmdbGenres?.length || serverGenres?.length) ? (
                      <div className="flex flex-wrap gap-2">
                        {tmdbGenres
                          ? tmdbGenres.map(g => (
                              <span key={g.id} className={`px-2.5 py-1 text-xs font-medium rounded-full ${genreBadgeClass}`}>
                                {g.name}
                              </span>
                            ))
                          : serverGenres!.map(genre => (
                              <span key={genre} className={`px-2.5 py-1 text-xs font-medium rounded-full ${genreBadgeClass}`}>
                                {genre}
                              </span>
                            ))}
                      </div>
                    ) : null}

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

                {tmdbLoading && !hasTMDBCast && item.tmdb_id && (
                  <div className="flex items-center gap-2 text-xs text-muted dark:text-muted-dark">
                    <div className="w-3 h-3 border border-accent border-t-transparent rounded-full animate-spin" />
                    Loading additional details...
                  </div>
                )}

                {collection && (
                  <div className="bg-gray-50 dark:bg-white/5 rounded-lg p-3">
                    <div className="flex items-center gap-3">
                      {collection.poster_path && (
                        <img src={`${TMDB_IMG}/w92${collection.poster_path}`} alt={collection.name}
                          className="w-12 rounded shadow" />
                      )}
                      <div>
                        <div className="text-xs text-muted dark:text-muted-dark">Part of</div>
                        <div className="text-sm font-medium text-gray-900 dark:text-gray-100">{collection.name}</div>
                      </div>
                    </div>
                  </div>
                )}

                <TechInfo item={item} />
                <WatchHistory item={item} />

                {effectiveMediaType === 'tv' && tmdbTV?.seasons && tmdbTV.seasons.length > 0 && canRequest && (
                  <div className="space-y-2 border-t border-border dark:border-border-dark pt-4">
                    <div className="text-sm font-medium">Select Seasons to Request</div>
                    <div className="flex flex-wrap gap-2">
                      <button
                        onClick={toggleAllSeasons}
                        aria-label="Select all seasons"
                        className={`px-3 py-1.5 text-xs font-medium rounded-full transition-colors ${
                          allSeasons
                            ? 'bg-accent text-gray-900'
                            : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/20'
                        }`}
                      >
                        All Seasons
                      </button>
                      {tmdbTV.seasons
                        .filter(s => s.season_number > 0)
                        .map(season => (
                          <button
                            key={season.season_number}
                            onClick={() => toggleSeason(season.season_number)}
                            aria-label={`Season ${season.season_number}`}
                            className={`px-3 py-1.5 text-xs font-medium rounded-full transition-colors ${
                              !allSeasons && selectedSeasons.includes(season.season_number)
                                ? 'bg-accent text-gray-900'
                                : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/20'
                            }`}
                          >
                            S{season.season_number}
                            <span className="ml-1 opacity-60">({season.episode_count})</span>
                          </button>
                        ))}
                    </div>
                  </div>
                )}

                {canRequest && (
                  <div className="border-t border-border dark:border-border-dark pt-4">
                    {requestSuccess ? (
                      <div className="text-sm text-green-600 dark:text-green-400 font-medium">
                        Request submitted successfully!
                      </div>
                    ) : (
                      <button
                        onClick={handleRequest}
                        disabled={requesting || (effectiveMediaType === 'tv' && !allSeasons && selectedSeasons.length === 0)}
                        className="px-5 py-2.5 text-sm font-semibold rounded-lg bg-accent text-gray-900
                                   hover:bg-accent/90 disabled:opacity-50 transition-colors"
                      >
                        {requestButtonLabel(requesting, overseerrStatus, effectiveMediaType!)}
                      </button>
                    )}
                    {requestError && (
                      <div className="text-sm text-red-500 dark:text-red-400 mt-2">{requestError}</div>
                    )}
                  </div>
                )}

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
          overseerrConfigured={overseerrConfigured}
          libraryIds={new Set()}
        />
      )}
    </>
  )
}
