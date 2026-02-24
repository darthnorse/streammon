import { useState, useEffect, useCallback, useMemo } from 'react'
import { Link } from 'react-router-dom'
import type {
  ItemDetails,
  TMDBMovieDetails,
  TMDBTVDetails,
  TMDBMovieEnvelope,
  TMDBTVEnvelope,
  TMDBCrew,
  LibraryMatch,
} from '../types'
import { formatDuration, formatBitrate, formatAudioCodec, formatVideoCodec, formatDate, thumbUrl } from '../lib/format'
import { getAudioCodecIcon, getVideoCodecIcon, getResolutionIcon, getChannelsIcon } from '../lib/mediaFlags'
import { useTMDBEnrichment } from '../hooks/useTMDBEnrichment'
import { useFetch } from '../hooks/useFetch'
import { useModalStack } from '../hooks/useModalStack'
import { lockBodyScroll, unlockBodyScroll } from '../lib/bodyScroll'
import { TMDB_IMG } from '../lib/tmdb'
import { api } from '../lib/api'
import { mediaStatusBadge } from '../lib/overseerr'
import { useOverseerrRequest } from '../hooks/useOverseerrRequest'
import { CastChip } from './CastChip'
import { ModalStackRenderer } from './ModalStackRenderer'

interface LibraryEntryProps {
  item: ItemDetails | null
  loading: boolean
  onClose: () => void
}

interface TmdbEntryProps {
  mediaType: 'movie' | 'tv'
  mediaId: number
  overseerrConfigured: boolean
  onClose: () => void
  onPersonClick?: (personId: number) => void
  active?: boolean
}

export type MediaDetailModalProps = LibraryEntryProps | TmdbEntryProps

function isLibraryEntry(props: MediaDetailModalProps): props is LibraryEntryProps {
  return 'item' in props
}

const serverAccent: Record<string, { bar: string; badge: string }> = {
  plex: { bar: 'bg-warn', badge: 'bg-warn/20 text-amber-700 dark:text-amber-300' },
  emby: { bar: 'bg-emby', badge: 'bg-emby/20 text-green-700 dark:text-green-300' },
  jellyfin: { bar: 'bg-jellyfin', badge: 'bg-jellyfin/20 text-purple-700 dark:text-purple-300' },
}

const defaultAccent = { bar: 'bg-accent', badge: 'bg-accent/20 text-blue-700 dark:text-blue-300' }

const mediaTypeIcons: Record<string, string> = { movie: '\uD83C\uDFAC', episode: '\uD83D\uDCFA' }

function StarRating({ rating }: { rating: number }) {
  const stars = Math.max(0, Math.min(5, Math.round(rating / 2)))
  return (
    <span className="text-amber-500" title={`${rating.toFixed(1)} / 10`}>
      {'★'.repeat(stars)}{'☆'.repeat(5 - stars)}
    </span>
  )
}

function MediaFlagIcon({ src, alt }: { src: string | null; alt: string }) {
  if (!src) return null
  return <img src={src} alt={alt} className="h-4 object-contain invert dark:invert-0" loading="lazy" />
}

function TechInfo({ item }: { item: ItemDetails }) {
  const video = formatVideoCodec(item.video_codec, item.video_resolution)
  const audio = formatAudioCodec(item.audio_codec, item.audio_channels)
  const bitrate = item.bitrate ? formatBitrate(item.bitrate) : null
  const container = item.container?.toUpperCase()

  const resolutionIcon = getResolutionIcon(item.video_resolution)
  const videoCodecIcon = getVideoCodecIcon(item.video_codec)
  const audioCodecIcon = getAudioCodecIcon(item.audio_codec)
  const channelsIcon = getChannelsIcon(item.audio_channels)

  const hasIcons = resolutionIcon || videoCodecIcon || audioCodecIcon || channelsIcon
  const hasDetails = video || audio || bitrate || container
  if (!hasIcons && !hasDetails) return null

  return (
    <div className="bg-gray-50 dark:bg-white/5 rounded-lg p-3 space-y-3">
      <div className="text-xs font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wide">
        Technical Details
      </div>
      {hasIcons && (
        <div className="flex flex-wrap items-center gap-3">
          <MediaFlagIcon src={resolutionIcon} alt={item.video_resolution || ''} />
          <MediaFlagIcon src={videoCodecIcon} alt={item.video_codec || ''} />
          <MediaFlagIcon src={audioCodecIcon} alt={item.audio_codec || ''} />
          <MediaFlagIcon src={channelsIcon} alt={`${item.audio_channels} channels`} />
        </div>
      )}
      <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
        {video && (
          <>
            <span className="text-muted dark:text-muted-dark">Video</span>
            <span className="text-gray-900 dark:text-gray-100">{video}</span>
          </>
        )}
        {audio && (
          <>
            <span className="text-muted dark:text-muted-dark">Audio</span>
            <span className="text-gray-900 dark:text-gray-100">{audio}</span>
          </>
        )}
        {container && (
          <>
            <span className="text-muted dark:text-muted-dark">Container</span>
            <span className="text-gray-900 dark:text-gray-100">{container}</span>
          </>
        )}
        {bitrate && (
          <>
            <span className="text-muted dark:text-muted-dark">Bitrate</span>
            <span className="text-gray-900 dark:text-gray-100">{bitrate}</span>
          </>
        )}
      </div>
    </div>
  )
}

function WatchHistory({ item }: { item: ItemDetails }) {
  if (!item.watch_history?.length) return null
  return (
    <div className="bg-gray-50 dark:bg-white/5 rounded-lg p-3 space-y-2">
      <div className="text-xs font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wide">
        Watch History
      </div>
      <div className="space-y-2 max-h-40 overflow-y-auto">
        {item.watch_history.map(entry => (
          <div key={entry.id} className="flex items-center justify-between text-sm">
            <div className="flex items-center gap-2 min-w-0">
              <Link
                to={`/users/${encodeURIComponent(entry.user_name)}`}
                className="font-medium hover:text-accent hover:underline truncate"
              >
                {entry.user_name}
              </Link>
              <span className="text-muted dark:text-muted-dark truncate">
                {entry.grandparent_title ? entry.title : ''}
              </span>
            </div>
            <div className="text-xs text-muted dark:text-muted-dark whitespace-nowrap ml-2">
              {formatDate(entry.started_at)}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

const tvStatusColors: Record<string, string> = {
  'Ended': 'bg-red-500/15 text-red-600 dark:text-red-400',
  'Canceled': 'bg-red-500/15 text-red-600 dark:text-red-400',
  'Returning Series': 'bg-green-500/15 text-green-600 dark:text-green-400',
  'In Production': 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
  'Planned': 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
}

function TVStatusBadge({ status }: { status: string }) {
  const color = tvStatusColors[status] || 'bg-gray-500/15 text-gray-600 dark:text-gray-400'
  return <span className={`px-2 py-0.5 text-xs font-medium rounded-full ${color}`}>{status}</span>
}

function NetworkLogos({ networks }: { networks: { id: number; name: string; logo_path?: string }[] }) {
  if (!networks.length) return null
  return (
    <div className="flex items-center gap-3">
      {networks.map(net => (
        net.logo_path ? (
          <img key={net.id} src={`${TMDB_IMG}/w92${net.logo_path}`} alt={net.name} title={net.name}
            className="h-5 object-contain dark:invert dark:brightness-200" loading="lazy" />
        ) : (
          <span key={net.id} className="text-xs text-muted dark:text-muted-dark">{net.name}</span>
        )
      ))}
    </div>
  )
}

function regularSeasonNumbers(seasons: { season_number: number }[]): number[] {
  return seasons.filter(s => s.season_number > 0).map(s => s.season_number)
}

function resolveMediaType(mediaType: string | undefined): 'movie' | 'tv' | null {
  if (mediaType === 'episode') return 'tv'
  if (mediaType === 'movie') return 'movie'
  return null
}

export function MediaDetailModal(props: MediaDetailModalProps) {
  const libraryMode = isLibraryEntry(props)
  const onClose = props.onClose

  const libProps = libraryMode ? (props as LibraryEntryProps) : null
  const tmdbProps = libraryMode ? null : (props as TmdbEntryProps)

  const libraryItem = libProps?.item ?? null
  const libraryLoading = libProps?.loading ?? false
  const tmdbMediaType = tmdbProps?.mediaType ?? null
  const tmdbMediaId = tmdbProps?.mediaId ?? null
  const parentOnPersonClick = tmdbProps?.onPersonClick
  const active = tmdbProps?.active ?? true

  const { stack, current: currentModal, push: pushModal, pop: popModal } = useModalStack()

  const enrichment = useTMDBEnrichment(
    libraryMode ? libraryItem?.tmdb_id : undefined,
    libraryMode ? libraryItem?.media_type : undefined,
  )

  // Overseerr config (library-first fetches, TMDB-first uses prop)
  const { data: configData } = useFetch<{ configured: boolean }>(
    libraryMode ? '/api/overseerr/configured' : null,
  )
  const overseerrConfigured = libraryMode
    ? !!configData?.configured
    : !!tmdbProps?.overseerrConfigured

  const { data: libraryIdsData } = useFetch<{ ids: string[] }>(
    libraryMode ? '/api/library/tmdb-ids' : null,
  )
  const libraryIds = useMemo(() => new Set(libraryIdsData?.ids ?? []), [libraryIdsData])

  const [fetchedMovie, setFetchedMovie] = useState<TMDBMovieDetails | null>(null)
  const [fetchedTV, setFetchedTV] = useState<TMDBTVDetails | null>(null)
  const [fetchLoading, setFetchLoading] = useState(!libraryMode)
  const [fetchError, setFetchError] = useState('')
  const [libraryMatches, setLibraryMatches] = useState<LibraryMatch[]>([])

  useEffect(() => {
    if (!tmdbMediaType || tmdbMediaId == null) return
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

  const tmdbMovie = libraryMode ? enrichment.movie : fetchedMovie
  const tmdbTV = libraryMode ? enrichment.tv : fetchedTV
  const tmdbLoading = libraryMode ? enrichment.loading : fetchLoading

  const effectiveTmdbId = libraryMode ? libraryItem?.tmdb_id : tmdbMediaId?.toString()
  const effectiveMediaType = libraryMode
    ? resolveMediaType(libraryItem?.media_type)
    : tmdbMediaType

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
    if (libraryMode && currentModal) return
    e.stopImmediatePropagation()
    onClose()
  }, [onClose, currentModal, libraryMode])

  useEffect(() => {
    if (!active) return
    const useCapture = !libraryMode
    document.addEventListener('keydown', handleKeyDown, useCapture)
    return () => document.removeEventListener('keydown', handleKeyDown, useCapture)
  }, [handleKeyDown, active, libraryMode])

  useEffect(() => {
    if (!active) return
    lockBodyScroll()
    return () => unlockBodyScroll()
  }, [active])

  const handlePersonClick = useCallback((personId: number) => {
    if (parentOnPersonClick) {
      parentOnPersonClick(personId)
    } else {
      pushModal({ type: 'person', personId })
    }
  }, [parentOnPersonClick, pushModal])

  const isLoading = libraryMode ? libraryLoading : fetchLoading
  const hasError = !libraryMode && !!fetchError

  const title = libraryItem?.title || tmdbMovie?.title || tmdbTV?.name || ''
  const overview = tmdbMovie?.overview || tmdbTV?.overview || libraryItem?.summary
  const backdrop = tmdbMovie?.backdrop_path || tmdbTV?.backdrop_path
  const tmdbPoster = tmdbMovie?.poster_path || tmdbTV?.poster_path
  const serverThumbSrc = libraryItem?.thumb_url
    ? thumbUrl(libraryItem.server_id, libraryItem.thumb_url)
    : undefined
  const posterSrc = tmdbPoster ? `${TMDB_IMG}/w342${tmdbPoster}` : serverThumbSrc
  const tagline = tmdbMovie?.tagline || tmdbTV?.tagline
  const year = tmdbMovie?.release_date?.slice(0, 4)
    || tmdbTV?.first_air_date?.slice(0, 4)
    || (libraryItem?.year ? String(libraryItem.year) : undefined)
  const runtime = tmdbMovie?.runtime
  const durationStr = libraryItem?.duration_ms ? formatDuration(libraryItem.duration_ms) : (runtime ? `${runtime} min` : undefined)
  const rating = tmdbMovie?.vote_average ?? tmdbTV?.vote_average ?? libraryItem?.rating
  const contentRating = libraryItem?.content_rating

  const tmdbGenres = tmdbMovie?.genres || tmdbTV?.genres
  const serverGenres = libraryItem?.genres
  const tmdbCast = tmdbMovie?.credits?.cast || tmdbTV?.credits?.cast
  const tmdbCrew = tmdbMovie?.credits?.crew || tmdbTV?.credits?.crew
  const tmdbDirectors = tmdbCrew?.filter((c: TMDBCrew) => c.job === 'Director')
  const directorNames = tmdbDirectors && tmdbDirectors.length > 0
    ? tmdbDirectors.map(d => d.name)
    : libraryItem?.directors && libraryItem.directors.length > 0
      ? libraryItem.directors
      : null
  const hasTMDBCast = tmdbCast && tmdbCast.length > 0
  const collection = tmdbMovie?.belongs_to_collection
  const tvStatus = tmdbTV?.status
  const networks = tmdbTV?.networks
  const seasonCount = tmdbTV?.number_of_seasons
  const episodeCount = tmdbTV?.number_of_episodes

  const serverThumbByName = useMemo(() => {
    const map = new Map<string, string>()
    if (libraryItem?.cast) {
      for (const m of libraryItem.cast) {
        if (m.thumb_url) map.set(m.name, m.thumb_url)
      }
    }
    return map
  }, [libraryItem?.cast])

  const accent = libraryItem
    ? (serverAccent[libraryItem.server_type] ?? defaultAccent)
    : defaultAccent

  const genreBadgeClass = libraryItem ? accent.badge : 'bg-accent/10 text-accent'

  const zIndex = libraryMode ? 'z-[60]' : 'z-[70]'
  const titleId = libraryMode
    ? `modal-title-${libraryItem?.id ?? 'loading'}`
    : `modal-title-tmdb-${tmdbMediaId}`

  return (
    <>
      <div
        className={`fixed inset-0 ${zIndex} flex items-center justify-center p-4 pb-20 lg:pb-4 bg-black/70 backdrop-blur-sm animate-fade-in`}
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
          {!backdrop && libraryItem && <div className={`h-1 ${accent.bar}`} />}

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

          {!isLoading && !hasError && (libraryItem || tmdbMovie || tmdbTV) && (
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
                  {!posterSrc && libraryItem && (
                    <div className="shrink-0 hidden sm:flex w-44 md:w-52 aspect-[2/3] rounded-lg bg-gray-200 dark:bg-white/10 items-center justify-center">
                      <span className="text-6xl opacity-20">
                        {mediaTypeIcons[libraryItem.media_type] ?? '\uD83C\uDFB5'}
                      </span>
                    </div>
                  )}

                  <div className="flex-1 min-w-0 space-y-3">
                    <div>
                      {libraryItem?.series_title && (
                        <div className="text-sm text-muted dark:text-muted-dark mb-1">
                          {libraryItem.series_title}
                          {libraryItem.season_number != null && libraryItem.episode_number != null && (
                            <span> &middot; S{libraryItem.season_number}E{libraryItem.episode_number}</span>
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
                            <span>&middot;</span>
                            <span>{durationStr}</span>
                          </>
                        )}
                        {contentRating && (
                          <>
                            <span>&middot;</span>
                            <span className="px-1.5 py-0.5 text-xs border border-current rounded">
                              {contentRating}
                            </span>
                          </>
                        )}
                        {rating != null && rating > 0 && (
                          <>
                            <span>&middot;</span>
                            {libraryItem ? <StarRating rating={rating} /> : (
                              <span className="text-amber-500">★ {rating.toFixed(1)}</span>
                            )}
                          </>
                        )}
                        {tvStatus && (
                          <>
                            <span>&middot;</span>
                            <TVStatusBadge status={tvStatus} />
                          </>
                        )}
                        {!libraryItem && tmdbMediaType && (
                          <span className="text-xs px-1.5 py-0.5 rounded bg-gray-100 dark:bg-white/10">
                            {tmdbMediaType === 'movie' ? 'Movie' : 'TV'}
                          </span>
                        )}
                      </div>

                      {overseerrStatus && overseerrStatus > 1 && (
                        <div className="mt-2">{mediaStatusBadge(overseerrStatus)}</div>
                      )}

                      {!libraryMode && libraryMatches.length > 0 && (
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
                            <span> &middot; {episodeCount} episode{episodeCount !== 1 ? 's' : ''}</span>
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

                {(hasTMDBCast || (libraryItem?.cast && libraryItem.cast.length > 0)) && (
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
                                imgSrc={serverThumb && libraryItem ? thumbUrl(libraryItem.server_id, serverThumb) : undefined}
                                onClick={() => handlePersonClick(person.id)}
                              />
                            )
                          })
                        : libraryItem!.cast!.slice(0, 6).map((member, idx) => (
                            <CastChip
                              key={`${member.name}-${idx}`}
                              name={member.name}
                              character={member.role}
                              imgSrc={member.thumb_url ? thumbUrl(libraryItem!.server_id, member.thumb_url) : undefined}
                            />
                          ))
                      }
                    </div>
                  </div>
                )}

                {tmdbLoading && !hasTMDBCast && libraryItem?.tmdb_id && (
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

                {libraryItem && <TechInfo item={libraryItem} />}
                {libraryItem && <WatchHistory item={libraryItem} />}

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
                        {requesting ? 'Requesting...' : `Request ${effectiveMediaType === 'movie' ? 'Movie' : 'TV Show'}`}
                      </button>
                    )}
                    {requestError && (
                      <div className="text-sm text-red-500 dark:text-red-400 mt-2">{requestError}</div>
                    )}
                  </div>
                )}

                {libraryItem && (
                  <div className="pt-2 flex items-center justify-between text-xs text-muted dark:text-muted-dark border-t border-border dark:border-border-dark">
                    <span>{libraryItem.studio}</span>
                    <span>{libraryItem.server_name}</span>
                  </div>
                )}
              </div>
            </div>
          )}

          {!isLoading && !hasError && !libraryItem && !tmdbMovie && !tmdbTV && (
            <div className="p-8 text-center text-muted dark:text-muted-dark">
              Failed to load item details
            </div>
          )}
        </div>
      </div>

      {libraryMode && stack.length > 0 && (
        <ModalStackRenderer
          stack={stack}
          pushModal={pushModal}
          popModal={popModal}
          overseerrConfigured={overseerrConfigured}
          libraryIds={libraryIds}
        />
      )}
    </>
  )
}
