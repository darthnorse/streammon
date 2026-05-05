import { useState, useEffect, useCallback, useMemo } from 'react'
import type {
  TMDBMovieDetails,
  TMDBTVDetails,
  TMDBMovieEnvelope,
  TMDBTVEnvelope,
  TMDBCrew,
  LibraryMatch,
} from '../types'
import { lockBodyScroll, unlockBodyScroll } from '../lib/bodyScroll'
import { TMDB_IMG } from '../lib/tmdb'
import { api } from '../lib/api'
import { MEDIA_STATUS, mediaStatusBadge } from '../lib/overseerr'
import { useOverseerrRequest } from '../hooks/useOverseerrRequest'
import { CastChip } from './CastChip'
import {
  TVStatusBadge,
  NetworkLogos,
  regularSeasonNumbers,
  requestButtonLabel,
} from './modals/MediaDetailParts'

export interface MediaDetailModalProps {
  mediaType: 'movie' | 'tv'
  mediaId: number
  overseerrConfigured: boolean
  onClose: () => void
  onPersonClick?: (personId: number) => void
  active?: boolean
}

export function MediaDetailModal(props: MediaDetailModalProps) {
  const { mediaType: tmdbMediaType, mediaId: tmdbMediaId, overseerrConfigured, onClose, onPersonClick, active = true } = props

  const [fetchedMovie, setFetchedMovie] = useState<TMDBMovieDetails | null>(null)
  const [fetchedTV, setFetchedTV] = useState<TMDBTVDetails | null>(null)
  const [fetchLoading, setFetchLoading] = useState(true)
  const [fetchError, setFetchError] = useState('')
  const [libraryMatches, setLibraryMatches] = useState<LibraryMatch[]>([])

  useEffect(() => {
    setFetchedMovie(null)
    setFetchedTV(null)
    setLibraryMatches([])
    setFetchLoading(true)
    setFetchError('')
    const controller = new AbortController()
    const endpoint = tmdbMediaType === 'movie'
      ? `/api/tmdb/movie/${tmdbMediaId}`
      : `/api/tmdb/tv/${tmdbMediaId}`

    api.get<TMDBMovieEnvelope | TMDBTVEnvelope>(endpoint, controller.signal)
      .then(data => {
        if (controller.signal.aborted) return
        setLibraryMatches(data.library_items ?? [])
        if (tmdbMediaType === 'movie') {
          setFetchedMovie((data as TMDBMovieEnvelope).tmdb)
        } else {
          setFetchedTV((data as TMDBTVEnvelope).tmdb)
        }
      })
      .catch(err => {
        if (err instanceof Error && err.name === 'AbortError') return
        if (!controller.signal.aborted) setFetchError(err instanceof Error ? err.message : String(err))
      })
      .finally(() => {
        if (!controller.signal.aborted) setFetchLoading(false)
      })
    return () => controller.abort()
  }, [tmdbMediaType, tmdbMediaId])

  const tmdbMovie = fetchedMovie
  const tmdbTV = fetchedTV

  const effectiveTmdbId = tmdbMediaId.toString()
  const effectiveMediaType = tmdbMediaType

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

  const showRequestedPill = !requestSuccess && (overseerrStatus === MEDIA_STATUS.PENDING || overseerrStatus === MEDIA_STATUS.PROCESSING)

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key !== 'Escape') return
    e.stopImmediatePropagation()
    onClose()
  }, [onClose])

  useEffect(() => {
    if (!active) return
    document.addEventListener('keydown', handleKeyDown, true)
    return () => document.removeEventListener('keydown', handleKeyDown, true)
  }, [handleKeyDown, active])

  useEffect(() => {
    if (!active) return
    lockBodyScroll()
    return () => unlockBodyScroll()
  }, [active])

  const handlePersonClick = useCallback((personId: number) => {
    onPersonClick?.(personId)
  }, [onPersonClick])

  const isLoading = fetchLoading
  const hasError = !!fetchError

  const title = tmdbMovie?.title || tmdbTV?.name || ''
  const overview = tmdbMovie?.overview || tmdbTV?.overview
  const backdrop = tmdbMovie?.backdrop_path || tmdbTV?.backdrop_path
  const tmdbPoster = tmdbMovie?.poster_path || tmdbTV?.poster_path
  const posterSrc = tmdbPoster ? `${TMDB_IMG}/w342${tmdbPoster}` : undefined
  const tagline = tmdbMovie?.tagline || tmdbTV?.tagline
  const year = tmdbMovie?.release_date?.slice(0, 4) || tmdbTV?.first_air_date?.slice(0, 4)
  const runtime = tmdbMovie?.runtime
  const durationStr = runtime ? `${runtime} min` : undefined
  const rating = tmdbMovie?.vote_average ?? tmdbTV?.vote_average

  const tmdbGenres = tmdbMovie?.genres || tmdbTV?.genres
  const tmdbCast = tmdbMovie?.credits?.cast || tmdbTV?.credits?.cast
  const tmdbCrew = tmdbMovie?.credits?.crew || tmdbTV?.credits?.crew
  const tmdbDirectors = tmdbCrew?.filter((c: TMDBCrew) => c.job === 'Director')
  const directorNames = tmdbDirectors && tmdbDirectors.length > 0 ? tmdbDirectors.map(d => d.name) : null
  const hasTMDBCast = tmdbCast && tmdbCast.length > 0
  const collection = tmdbMovie?.belongs_to_collection
  const tvStatus = tmdbTV?.status
  const networks = tmdbTV?.networks
  const seasonCount = tmdbTV?.number_of_seasons
  const episodeCount = tmdbTV?.number_of_episodes

  const titleId = `modal-title-tmdb-${tmdbMediaId}`

  return (
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center p-4 pb-20 lg:pb-4 bg-black/70 backdrop-blur-sm animate-fade-in"
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

        {isLoading && (
          <div className="flex items-center justify-center py-20">
            <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {!isLoading && hasError && (
          <div className="p-8 text-center text-red-500 dark:text-red-400">{fetchError}</div>
        )}

        {!isLoading && !hasError && (tmdbMovie || tmdbTV) && (
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
                      {rating != null && rating > 0 && (
                        <>
                          <span>·</span>
                          <span className="text-amber-500">★ {rating.toFixed(1)}</span>
                        </>
                      )}
                      {tvStatus && (
                        <>
                          <span>·</span>
                          <TVStatusBadge status={tvStatus} />
                        </>
                      )}
                      <span className="text-xs px-1.5 py-0.5 rounded bg-gray-100 dark:bg-white/10">
                        {tmdbMediaType === 'movie' ? 'Movie' : 'TV'}
                      </span>
                    </div>

                    {overseerrStatus && overseerrStatus > MEDIA_STATUS.UNKNOWN && !showRequestedPill && (
                      <div className="mt-2">{mediaStatusBadge(overseerrStatus)}</div>
                    )}

                    {libraryMatches.length > 0 && (
                      <div className="flex flex-wrap gap-1.5 mt-2">
                        {libraryMatches.map(li => (
                          <span
                            key={`${li.server_id}-${li.item_id}`}
                            className="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-green-600/20 text-green-500"
                          >
                            On {li.server_name}
                          </span>
                        ))}
                      </div>
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

                  {tmdbGenres?.length ? (
                    <div className="flex flex-wrap gap-2">
                      {tmdbGenres.map(g => (
                        <span key={g.id} className="px-2.5 py-1 text-xs font-medium rounded-full bg-accent/10 text-accent">
                          {g.name}
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

              {hasTMDBCast && (
                <div className="space-y-2">
                  <div className="text-sm font-medium text-gray-900 dark:text-gray-100">Cast</div>
                  <div className="flex gap-2 overflow-x-auto pb-2 -mx-5 px-5 sm:-mx-6 sm:px-6">
                    {tmdbCast.slice(0, 8).map(person => (
                      <CastChip
                        key={person.id}
                        name={person.name}
                        character={person.character}
                        profilePath={person.profile_path}
                        onClick={() => handlePersonClick(person.id)}
                      />
                    ))}
                  </div>
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
                      {requestButtonLabel(requesting, overseerrStatus, effectiveMediaType)}
                    </button>
                  )}
                  {requestError && (
                    <div className="text-sm text-red-500 dark:text-red-400 mt-2">{requestError}</div>
                  )}
                </div>
              )}

              {/* Top-of-modal mediaStatusBadge is hidden only for PENDING/PROCESSING (replaced
                  by this prominent pill). All other Overseerr states (UNKNOWN, AVAILABLE,
                  PARTIALLY_AVAILABLE, BLOCKLISTED, DELETED) keep the regular small badge. */}
              {showRequestedPill && (
                <div className="border-t border-border dark:border-border-dark pt-4">
                  <div className="inline-flex items-center gap-2 px-4 py-2.5 rounded-lg bg-accent/10 text-accent border border-accent/30">
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                    <span className="text-sm font-semibold">Already Requested</span>
                    <span className="text-xs opacity-60">·</span>
                    <span className="text-sm">
                      {overseerrStatus === MEDIA_STATUS.PENDING ? 'Pending Approval' : 'Processing'}
                    </span>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {!isLoading && !hasError && !tmdbMovie && !tmdbTV && (
          <div className="p-8 text-center text-muted dark:text-muted-dark">
            Failed to load item details
          </div>
        )}
      </div>
    </div>
  )
}
