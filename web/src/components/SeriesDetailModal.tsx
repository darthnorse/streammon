import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api'
import { TMDB_IMG, mediaStatusBadge } from '../lib/overseerr'
import type {
  OverseerrTVDetails,
  OverseerrCast,
  OverseerrCrew,
  OverseerrSeason,
  SonarrSeriesDetails,
  SonarrSeason,
} from '../types'

interface SeriesDetailModalProps {
  tmdbId: number | null
  sonarrSeriesId: number
  overseerrAvailable: boolean
  onClose: () => void
}

type DataSource = 'overseerr' | 'sonarr'

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

function regularSeasonNumbers(seasons: { seasonNumber: number }[]): number[] {
  return seasons.filter(s => s.seasonNumber > 0).map(s => s.seasonNumber)
}

interface NormalizedSeason {
  seasonNumber: number
  episodeCount?: number
}

function normalizeSeasons(overseerr?: OverseerrSeason[], sonarr?: SonarrSeason[]): NormalizedSeason[] {
  if (overseerr) {
    return overseerr
      .filter(s => s.seasonNumber > 0)
      .map(s => ({ seasonNumber: s.seasonNumber, episodeCount: s.episodeCount }))
  }
  if (sonarr) {
    return sonarr
      .filter(s => s.seasonNumber > 0)
      .map(s => ({ seasonNumber: s.seasonNumber, episodeCount: s.statistics?.episodeCount }))
  }
  return []
}

interface DisplayData {
  title: string
  overview?: string
  backdrop: string | null
  poster: string | null
  genres?: string[]
  year?: string
  network?: string
  status?: string
  rating?: number
  episodeCount?: number
  runtime?: number
  mediaStatus?: number
  cast?: OverseerrCast[]
  crew?: OverseerrCrew[]
  tagline?: string
}

function buildDisplay(
  source: DataSource,
  overseerr: OverseerrTVDetails | null,
  sonarr: SonarrSeriesDetails | null,
  sonarrSeriesId: number,
): DisplayData {
  if (source === 'overseerr' && overseerr) {
    return {
      title: overseerr.name ?? '',
      overview: overseerr.overview,
      backdrop: overseerr.backdropPath ?? null,
      poster: overseerr.posterPath ? `${TMDB_IMG}/w300${overseerr.posterPath}` : null,
      genres: overseerr.genres?.map(g => g.name),
      year: overseerr.firstAirDate?.slice(0, 4),
      network: overseerr.networks?.[0]?.name,
      status: overseerr.status,
      rating: overseerr.voteAverage,
      episodeCount: overseerr.numberOfEpisodes,
      runtime: overseerr.episodeRunTime?.[0],
      mediaStatus: overseerr.mediaInfo?.status,
      cast: overseerr.credits?.cast,
      crew: overseerr.credits?.crew,
      tagline: overseerr.tagline,
    }
  }
  return {
    title: sonarr?.title ?? '',
    overview: sonarr?.overview,
    backdrop: null,
    poster: `/api/sonarr/poster/${sonarrSeriesId}`,
    genres: sonarr?.genres,
    year: sonarr?.year ? String(sonarr.year) : undefined,
    network: sonarr?.network,
    status: sonarr?.status,
    rating: sonarr?.ratings?.value,
    episodeCount: sonarr?.statistics?.episodeCount,
    runtime: sonarr?.runtime,
  }
}

export function SeriesDetailModal({ tmdbId, sonarrSeriesId, overseerrAvailable, onClose }: SeriesDetailModalProps) {
  const [overseerrData, setOverseerrData] = useState<OverseerrTVDetails | null>(null)
  const [sonarrData, setSonarrData] = useState<SonarrSeriesDetails | null>(null)
  const [dataSource, setDataSource] = useState<DataSource | null>(null)
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
    setOverseerrData(null)
    setSonarrData(null)
    setDataSource(null)
    setRequestSuccess(false)
    setRequestError('')
    const controller = new AbortController()
    let cancelled = false

    async function fetchData() {
      // Try Overseerr first if available and tmdbId present
      if (overseerrAvailable && tmdbId != null) {
        try {
          const data = await api.get<OverseerrTVDetails>(
            `/api/overseerr/tv/${tmdbId}`,
            controller.signal,
          )
          if (cancelled) return
          setOverseerrData(data)
          setDataSource('overseerr')
          if (data.seasons) {
            setSelectedSeasons(regularSeasonNumbers(data.seasons))
          }
          setLoading(false)
          return
        } catch (err) {
          if (cancelled) return
          if ((err as Error).name === 'AbortError') return
          // Fall through to Sonarr
        }
      }

      // Sonarr fallback
      try {
        const data = await api.get<SonarrSeriesDetails>(
          `/api/sonarr/series/${sonarrSeriesId}`,
          controller.signal,
        )
        if (cancelled) return
        setSonarrData(data)
        setDataSource('sonarr')
      } catch (err) {
        if (cancelled) return
        if ((err as Error).name === 'AbortError') return
        setError('Failed to load series details')
      }
      if (!cancelled) setLoading(false)
    }

    fetchData()
    return () => {
      cancelled = true
      controller.abort()
    }
  }, [tmdbId, sonarrSeriesId, overseerrAvailable])

  async function handleRequest() {
    if (tmdbId == null) return
    setRequesting(true)
    setRequestError('')
    try {
      const requestSeasons = allSeasons && overseerrData?.seasons
        ? regularSeasonNumbers(overseerrData.seasons)
        : selectedSeasons
      await api.post('/api/overseerr/requests', {
        mediaType: 'tv',
        mediaId: tmdbId,
        seasons: requestSeasons,
      })
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
      if (overseerrData?.seasons) {
        setSelectedSeasons(regularSeasonNumbers(overseerrData.seasons))
      }
    }
  }

  const isOverseerr = dataSource === 'overseerr'
  const d = dataSource ? buildDisplay(dataSource, overseerrData, sonarrData, sonarrSeriesId) : null
  const directors = d?.crew?.filter((c: OverseerrCrew) => c.job === 'Director')
  const seasons = normalizeSeasons(overseerrData?.seasons, sonarrData?.seasons)

  const alreadyRequested = d?.mediaStatus && d.mediaStatus >= 2 && d.mediaStatus <= 5
  const showRequestSection = isOverseerr && !alreadyRequested

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center p-4 pb-20 lg:pb-4 bg-black/70 backdrop-blur-sm animate-fade-in"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="series-modal-title"
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

        {!loading && !error && d && (
          <div>
            {d.backdrop && (
              <div className="relative h-48 sm:h-64 overflow-hidden">
                <img
                  src={`${TMDB_IMG}/w1280${d.backdrop}`}
                  alt=""
                  className="w-full h-full object-cover"
                />
                <div className="absolute inset-0 bg-gradient-to-t from-panel dark:from-panel-dark via-transparent to-transparent" />
              </div>
            )}

            <div className={`p-5 sm:p-6 space-y-4 ${d.backdrop ? '-mt-20 relative' : ''}`}>
              <div className="flex gap-4">
                {d.poster && (
                  <div className="shrink-0 hidden sm:block">
                    <img
                      src={d.poster}
                      alt={d.title}
                      className="w-32 rounded-lg shadow-lg"
                    />
                  </div>
                )}

                <div className="flex-1 min-w-0">
                  <h2 id="series-modal-title" className="text-2xl font-bold">
                    {d.title}
                  </h2>
                  <div className="flex flex-wrap items-center gap-2 mt-1.5 text-sm text-muted dark:text-muted-dark">
                    {d.year && <span>{d.year}</span>}
                    {d.runtime && (
                      <>
                        <span>&middot;</span>
                        <span>{d.runtime} min</span>
                      </>
                    )}
                    {d.episodeCount != null && d.episodeCount > 0 && (
                      <>
                        <span>&middot;</span>
                        <span>{d.episodeCount} episodes</span>
                      </>
                    )}
                    {d.rating != null && d.rating > 0 && (
                      <>
                        <span>&middot;</span>
                        <span className="text-amber-500">★ {d.rating.toFixed(1)}</span>
                      </>
                    )}
                    {d.network && (
                      <>
                        <span>&middot;</span>
                        <span>{d.network}</span>
                      </>
                    )}
                    {d.status && (
                      <>
                        <span>&middot;</span>
                        <span className="capitalize">{d.status}</span>
                      </>
                    )}
                    <span className="text-xs px-1.5 py-0.5 rounded bg-gray-100 dark:bg-white/10">TV</span>
                  </div>

                  {d.mediaStatus && d.mediaStatus > 1 && (
                    <div className="mt-2">{mediaStatusBadge(d.mediaStatus)}</div>
                  )}
                </div>
              </div>

              {d.tagline && (
                <p className="text-sm italic text-muted dark:text-muted-dark">&ldquo;{d.tagline}&rdquo;</p>
              )}

              {d.genres && d.genres.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {d.genres.map(name => (
                    <span key={name} className="px-2.5 py-1 text-xs font-medium rounded-full bg-accent/10 text-accent">
                      {name}
                    </span>
                  ))}
                </div>
              )}

              {d.overview && (
                <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed">
                  {d.overview}
                </p>
              )}

              {directors && directors.length > 0 && (
                <div className="text-sm">
                  <span className="text-muted dark:text-muted-dark">Directed by </span>
                  <span className="font-medium">{directors.map(dir => dir.name).join(', ')}</span>
                </div>
              )}

              {d.cast && d.cast.length > 0 && (
                <div className="space-y-2">
                  <div className="text-sm font-medium">Cast</div>
                  <div className="flex gap-2 overflow-x-auto pb-2 -mx-5 px-5 sm:-mx-6 sm:px-6">
                    {d.cast.slice(0, 8).map(person => (
                      <CastChip key={person.id} person={person} />
                    ))}
                  </div>
                </div>
              )}

              {seasons.length > 0 && (
                <div className="space-y-2">
                  <div className="text-sm font-medium">Seasons</div>
                  <div className="flex flex-wrap gap-2">
                    {seasons.map(season => (
                      <span
                        key={season.seasonNumber}
                        className="px-3 py-1.5 text-xs font-medium rounded-full bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300"
                        aria-label={`Season ${season.seasonNumber}`}
                      >
                        S{season.seasonNumber}
                        {season.episodeCount != null && <span className="ml-1 opacity-60">({season.episodeCount})</span>}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {showRequestSection && seasons.length > 0 && (
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
                    {seasons.map(season => (
                      <button
                        key={season.seasonNumber}
                        onClick={() => toggleSeason(season.seasonNumber)}
                        aria-label={`Season ${season.seasonNumber}`}
                        className={`px-3 py-1.5 text-xs font-medium rounded-full transition-colors ${
                          !allSeasons && selectedSeasons.includes(season.seasonNumber)
                            ? 'bg-accent text-gray-900'
                            : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/20'
                        }`}
                      >
                        S{season.seasonNumber}
                        {season.episodeCount != null && <span className="ml-1 opacity-60">({season.episodeCount})</span>}
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {showRequestSection && (
                <div className="border-t border-border dark:border-border-dark pt-4">
                  {requestSuccess ? (
                    <div className="text-sm text-green-600 dark:text-green-400 font-medium">
                      Request submitted successfully!
                    </div>
                  ) : (
                    <button
                      onClick={handleRequest}
                      disabled={requesting || (!allSeasons && selectedSeasons.length === 0)}
                      className="px-5 py-2.5 text-sm font-semibold rounded-lg bg-accent text-gray-900
                                 hover:bg-accent/90 disabled:opacity-50 transition-colors"
                    >
                      {requesting ? 'Requesting...' : 'Request TV Show'}
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
