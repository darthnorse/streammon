import { useState, useEffect, useCallback } from 'react'
import { lockBodyScroll, unlockBodyScroll } from '../lib/bodyScroll'
import { api } from '../lib/api'
import { TMDB_IMG } from '../lib/tmdb'
import { mediaStatusBadge } from '../lib/overseerr'
import { CastChip } from './CastChip'
import type {
  TMDBMovieEnvelope,
  TMDBTVEnvelope,
  TMDBMovieDetails,
  TMDBTVDetails,
  TMDBCrew,
  LibraryMatch,
} from '../types'

interface TMDBDetailModalProps {
  mediaType: 'movie' | 'tv'
  mediaId: number
  overseerrConfigured: boolean
  onClose: () => void
  onPersonClick?: (personId: number) => void
  active?: boolean
}

function regularSeasonNumbers(seasons: { season_number: number }[]): number[] {
  return seasons.filter(s => s.season_number > 0).map(s => s.season_number)
}

export function TMDBDetailModal({ mediaType, mediaId, overseerrConfigured, onClose, onPersonClick, active = true }: TMDBDetailModalProps) {
  const [movie, setMovie] = useState<TMDBMovieDetails | null>(null)
  const [tv, setTv] = useState<TMDBTVDetails | null>(null)
  const [libraryItems, setLibraryItems] = useState<LibraryMatch[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [overseerrStatus, setOverseerrStatus] = useState<number | undefined>()
  const [requesting, setRequesting] = useState(false)
  const [requestSuccess, setRequestSuccess] = useState(false)
  const [requestError, setRequestError] = useState('')
  const [selectedSeasons, setSelectedSeasons] = useState<number[]>([])
  const [allSeasons, setAllSeasons] = useState(true)

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.stopImmediatePropagation()
      onClose()
    }
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

  useEffect(() => {
    setLoading(true)
    setError('')
    const controller = new AbortController()
    const endpoint = mediaType === 'movie'
      ? `/api/tmdb/movie/${mediaId}`
      : `/api/tmdb/tv/${mediaId}`

    api.get<TMDBMovieEnvelope | TMDBTVEnvelope>(endpoint, controller.signal)
      .then(data => {
        if (controller.signal.aborted) return
        setLibraryItems(data.library_items ?? [])
        if (mediaType === 'movie') {
          setMovie((data as TMDBMovieEnvelope).tmdb)
        } else {
          const tvData = (data as TMDBTVEnvelope).tmdb
          setTv(tvData)
          if (tvData.seasons) {
            setSelectedSeasons(regularSeasonNumbers(tvData.seasons))
          }
        }
      })
      .catch(err => {
        if (err instanceof Error && err.name === 'AbortError') return
        if (!controller.signal.aborted) setError((err as Error).message)
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [mediaType, mediaId])

  useEffect(() => {
    if (!overseerrConfigured) return
    const controller = new AbortController()
    const endpoint = mediaType === 'movie'
      ? `/api/overseerr/movie/${mediaId}`
      : `/api/overseerr/tv/${mediaId}`
    api.get<{ mediaInfo?: { status?: number } }>(endpoint, controller.signal)
      .then(data => {
        if (!controller.signal.aborted) setOverseerrStatus(data.mediaInfo?.status)
      })
      .catch(() => { /* ignore — just means no status badge */ })
    return () => controller.abort()
  }, [overseerrConfigured, mediaType, mediaId])

  async function handleRequest() {
    setRequesting(true)
    setRequestError('')
    try {
      const body: Record<string, unknown> = { mediaType, mediaId }
      if (mediaType === 'tv') {
        body.seasons = selectedSeasons
      }
      await api.post('/api/overseerr/requests', body)
      setRequestSuccess(true)
    } catch (err) {
      setRequestError((err as Error).message)
    } finally {
      setRequesting(false)
    }
  }

  function toggleSeason(num: number) {
    if (allSeasons) {
      setAllSeasons(false)
      setSelectedSeasons([num])
      return
    }
    setSelectedSeasons(prev =>
      prev.includes(num) ? prev.filter(n => n !== num) : [...prev, num]
    )
  }

  function toggleAllSeasons() {
    if (allSeasons) {
      setAllSeasons(false)
      setSelectedSeasons([])
    } else {
      setAllSeasons(true)
      if (tv?.seasons) {
        setSelectedSeasons(regularSeasonNumbers(tv.seasons))
      }
    }
  }

  const details = mediaType === 'movie' ? movie : tv
  const title = movie?.title || tv?.name || ''
  const overview = details?.overview
  const backdrop = details?.backdrop_path
  const poster = details?.poster_path
  const genres = details?.genres
  const tagline = details?.tagline
  const cast = details?.credits?.cast
  const crew = details?.credits?.crew
  const directors = crew?.filter((c: TMDBCrew) => c.job === 'Director')
  const year = movie?.release_date?.slice(0, 4) || tv?.first_air_date?.slice(0, 4)
  const runtime = movie?.runtime
  const rating = details?.vote_average
  const collection = movie?.belongs_to_collection

  const alreadyRequested = overseerrStatus && overseerrStatus >= 2 && overseerrStatus <= 5
  const canRequest = overseerrConfigured && !alreadyRequested && !requestSuccess

  return (
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center p-4 pb-20 lg:pb-4 bg-black/70 backdrop-blur-sm animate-fade-in"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="tmdb-modal-title"
    >
      <div
        className="relative w-full max-w-3xl max-h-[90dvh] overflow-y-auto rounded-xl
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

        {loading && (
          <div className="flex items-center justify-center py-20">
            <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {!loading && error && (
          <div className="p-8 text-center text-red-500 dark:text-red-400">{error}</div>
        )}

        {!loading && !error && details && (
          <div>
            {backdrop && (
              <div className="relative h-48 sm:h-64 overflow-hidden">
                <img
                  src={`${TMDB_IMG}/w1280${backdrop}`}
                  alt=""
                  className="w-full h-full object-cover"
                />
                <div className="absolute inset-0 bg-gradient-to-t from-panel dark:from-panel-dark via-transparent to-transparent" />
              </div>
            )}

            <div className={`p-5 sm:p-6 space-y-4 ${backdrop ? '-mt-20 relative' : ''}`}>
              <div className="flex gap-4">
                {poster && (
                  <div className="shrink-0 hidden sm:block">
                    <img
                      src={`${TMDB_IMG}/w300${poster}`}
                      alt={title}
                      className="w-32 rounded-lg shadow-lg"
                    />
                  </div>
                )}

                <div className="flex-1 min-w-0">
                  <h2 id="tmdb-modal-title" className="text-2xl font-bold">
                    {title}
                  </h2>
                  <div className="flex flex-wrap items-center gap-2 mt-1.5 text-sm text-muted dark:text-muted-dark">
                    {year && <span>{year}</span>}
                    {runtime && (
                      <>
                        <span>&middot;</span>
                        <span>{runtime} min</span>
                      </>
                    )}
                    {tv?.number_of_episodes && (
                      <>
                        <span>&middot;</span>
                        <span>{tv.number_of_episodes} episodes</span>
                      </>
                    )}
                    {rating != null && rating > 0 && (
                      <>
                        <span>&middot;</span>
                        <span className="text-amber-500">★ {rating.toFixed(1)}</span>
                      </>
                    )}
                    {mediaType === 'movie' ? (
                      <span className="text-xs px-1.5 py-0.5 rounded bg-gray-100 dark:bg-white/10">Movie</span>
                    ) : (
                      <span className="text-xs px-1.5 py-0.5 rounded bg-gray-100 dark:bg-white/10">TV</span>
                    )}
                  </div>

                  {overseerrStatus && overseerrStatus > 1 && (
                    <div className="mt-2">{mediaStatusBadge(overseerrStatus)}</div>
                  )}

                  {libraryItems.length > 0 && (
                    <div className="flex flex-wrap gap-1.5 mt-2">
                      {libraryItems.map(li => (
                        <span
                          key={`${li.server_id}-${li.item_id}`}
                          className="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-green-600/20 text-green-500"
                        >
                          On {li.server_name}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              </div>

              {tagline && (
                <p className="text-sm italic text-muted dark:text-muted-dark">&ldquo;{tagline}&rdquo;</p>
              )}

              {genres && genres.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {genres.map(g => (
                    <span key={g.id} className="px-2.5 py-1 text-xs font-medium rounded-full bg-accent/10 text-accent">
                      {g.name}
                    </span>
                  ))}
                </div>
              )}

              {overview && (
                <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed">
                  {overview}
                </p>
              )}

              {directors && directors.length > 0 && (
                <div className="text-sm">
                  <span className="text-muted dark:text-muted-dark">Directed by </span>
                  <span className="font-medium">{directors.map(d => d.name).join(', ')}</span>
                </div>
              )}

              {cast && cast.length > 0 && (
                <div className="space-y-2">
                  <div className="text-sm font-medium">Cast</div>
                  <div className="flex gap-2 overflow-x-auto pb-2 -mx-5 px-5 sm:-mx-6 sm:px-6">
                    {cast.slice(0, 8).map(person => (
                      <CastChip
                        key={person.id}
                        name={person.name}
                        character={person.character}
                        profilePath={person.profile_path}
                        onClick={onPersonClick ? () => onPersonClick(person.id) : undefined}
                      />
                    ))}
                  </div>
                </div>
              )}

              {collection && (
                <div className="border-t border-border dark:border-border-dark pt-4">
                  <div className="flex items-center gap-3">
                    {collection.poster_path && (
                      <img
                        src={`${TMDB_IMG}/w92${collection.poster_path}`}
                        alt={collection.name}
                        className="w-12 rounded shadow"
                      />
                    )}
                    <div>
                      <div className="text-sm text-muted dark:text-muted-dark">Part of</div>
                      <div className="text-sm font-medium">{collection.name}</div>
                    </div>
                  </div>
                </div>
              )}

              {mediaType === 'tv' && tv?.seasons && tv.seasons.length > 0 && canRequest && (
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
                    {tv.seasons
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
                      disabled={requesting || (mediaType === 'tv' && !allSeasons && selectedSeasons.length === 0)}
                      className="px-5 py-2.5 text-sm font-semibold rounded-lg bg-accent text-gray-900
                                 hover:bg-accent/90 disabled:opacity-50 transition-colors"
                    >
                      {requesting ? 'Requesting...' : `Request ${mediaType === 'movie' ? 'Movie' : 'TV Show'}`}
                    </button>
                  )}
                  {requestError && (
                    <div className="text-sm text-red-500 dark:text-red-400 mt-2">{requestError}</div>
                  )}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
