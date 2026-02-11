import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api'
import { TMDB_IMG, mediaStatusBadge } from '../lib/overseerr'
import type {
  OverseerrMovieDetails,
  OverseerrTVDetails,
  OverseerrCast,
  OverseerrCrew,
} from '../types'

interface OverseerrDetailModalProps {
  mediaType: 'movie' | 'tv'
  mediaId: number
  onClose: () => void
}

function regularSeasonNumbers(seasons: { seasonNumber: number }[]): number[] {
  return seasons.filter(s => s.seasonNumber > 0).map(s => s.seasonNumber)
}

function getInitials(name: string): string {
  if (!name) return '?'
  return name.split(' ').filter(Boolean).map(n => n[0]).join('').slice(0, 2).toUpperCase()
}

function CastChip({ person }: { person: OverseerrCast }) {
  return (
    <div className="flex items-center gap-2 px-2 py-1.5 rounded-full bg-gray-100 dark:bg-white/10 shrink-0">
      {person.profilePath ? (
        <img
          src={`${TMDB_IMG}/w92${person.profilePath}`}
          alt={person.name}
          className="w-7 h-7 rounded-full object-cover bg-gray-300 dark:bg-white/20"
          loading="lazy"
        />
      ) : (
        <div className="w-7 h-7 rounded-full bg-gray-300 dark:bg-white/20 flex items-center justify-center text-[10px] font-medium text-gray-600 dark:text-gray-300">
          {getInitials(person.name)}
        </div>
      )}
      <div className="text-xs pr-1">
        <div className="font-medium text-gray-900 dark:text-gray-100">{person.name}</div>
        {person.character && (
          <div className="text-gray-500 dark:text-gray-400 text-[10px]">{person.character}</div>
        )}
      </div>
    </div>
  )
}

export function OverseerrDetailModal({ mediaType, mediaId, onClose }: OverseerrDetailModalProps) {
  const [movie, setMovie] = useState<OverseerrMovieDetails | null>(null)
  const [tv, setTv] = useState<OverseerrTVDetails | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [requesting, setRequesting] = useState(false)
  const [requestSuccess, setRequestSuccess] = useState(false)
  const [requestError, setRequestError] = useState('')
  const [selectedSeasons, setSelectedSeasons] = useState<number[]>([])
  const [allSeasons, setAllSeasons] = useState(true)

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') onClose()
  }, [onClose])

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown)
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.body.style.overflow = ''
    }
  }, [handleKeyDown])

  useEffect(() => {
    setLoading(true)
    setError('')
    const controller = new AbortController()
    const endpoint = mediaType === 'movie'
      ? `/api/overseerr/movie/${mediaId}`
      : `/api/overseerr/tv/${mediaId}`

    api.get<OverseerrMovieDetails | OverseerrTVDetails>(endpoint, controller.signal)
      .then(data => {
        if (mediaType === 'movie') {
          setMovie(data as OverseerrMovieDetails)
        } else {
          const tvData = data as OverseerrTVDetails
          setTv(tvData)
          if (tvData.seasons) {
            setSelectedSeasons(regularSeasonNumbers(tvData.seasons))
          }
        }
      })
      .catch(err => {
        if ((err as Error).name !== 'AbortError') {
          setError((err as Error).message)
        }
      })
      .finally(() => setLoading(false))
    return () => controller.abort()
  }, [mediaType, mediaId])

  async function handleRequest() {
    setRequesting(true)
    setRequestError('')
    try {
      const body: Record<string, unknown> = {
        mediaType,
        mediaId,
      }
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
  const backdrop = details?.backdropPath
  const poster = details?.posterPath
  const genres = details?.genres
  const tagline = details?.tagline
  const mediaStatus = details?.mediaInfo?.status
  const cast = details?.credits?.cast
  const crew = details?.credits?.crew
  const directors = crew?.filter((c: OverseerrCrew) => c.job === 'Director')
  const year = movie?.releaseDate?.slice(0, 4) || tv?.firstAirDate?.slice(0, 4)
  const runtime = movie?.runtime
  const rating = details?.voteAverage

  const alreadyRequested = mediaStatus && mediaStatus >= 2 && mediaStatus <= 5
  const canRequest = !alreadyRequested && !requestSuccess

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center p-4 pb-20 lg:pb-4 bg-black/70 backdrop-blur-sm animate-fade-in"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="overseerr-modal-title"
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
                  <h2 id="overseerr-modal-title" className="text-2xl font-bold">
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
                    {tv?.numberOfEpisodes && (
                      <>
                        <span>&middot;</span>
                        <span>{tv.numberOfEpisodes} episodes</span>
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

                  {mediaStatus && mediaStatus > 1 && (
                    <div className="mt-2">{mediaStatusBadge(mediaStatus)}</div>
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
                      <CastChip key={person.id} person={person} />
                    ))}
                  </div>
                </div>
              )}

              {mediaType === 'tv' && tv?.seasons && tv.seasons.length > 0 && canRequest && (
                <div className="space-y-2 border-t border-border dark:border-border-dark pt-4">
                  <div className="text-sm font-medium">Select Seasons to Request</div>
                  <div className="flex flex-wrap gap-2">
                    <button
                      onClick={toggleAllSeasons}
                      className={`px-3 py-1.5 text-xs font-medium rounded-full transition-colors ${
                        allSeasons
                          ? 'bg-accent text-gray-900'
                          : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/20'
                      }`}
                    >
                      All Seasons
                    </button>
                    {tv.seasons
                      .filter(s => s.seasonNumber > 0)
                      .map(season => (
                        <button
                          key={season.seasonNumber}
                          onClick={() => toggleSeason(season.seasonNumber)}
                          className={`px-3 py-1.5 text-xs font-medium rounded-full transition-colors ${
                            !allSeasons && selectedSeasons.includes(season.seasonNumber)
                              ? 'bg-accent text-gray-900'
                              : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/20'
                          }`}
                        >
                          S{season.seasonNumber}
                          <span className="ml-1 opacity-60">({season.episodeCount})</span>
                        </button>
                      ))}
                  </div>
                </div>
              )}

              {!alreadyRequested && (
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
