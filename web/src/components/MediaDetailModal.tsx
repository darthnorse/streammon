import { useEffect, useCallback, useMemo } from 'react'
import { Link } from 'react-router-dom'
import type { ItemDetails, TMDBMovieDetails, TMDBTVDetails, TMDBCrew } from '../types'
import { formatDuration, formatBitrate, formatAudioCodec, formatVideoCodec, formatDate, thumbUrl } from '../lib/format'
import { getAudioCodecIcon, getVideoCodecIcon, getResolutionIcon, getChannelsIcon } from '../lib/mediaFlags'
import { useTMDBEnrichment } from '../hooks/useTMDBEnrichment'
import { useFetch } from '../hooks/useFetch'
import { useModalStack } from '../hooks/useModalStack'
import { lockBodyScroll, unlockBodyScroll } from '../lib/bodyScroll'
import { TMDB_IMG } from '../lib/tmdb'
import { CastChip } from './CastChip'
import { ModalStackRenderer } from './ModalStackRenderer'

const serverAccent: Record<string, { bar: string; badge: string }> = {
  plex: { bar: 'bg-warn', badge: 'bg-warn/20 text-amber-700 dark:text-amber-300' },
  emby: { bar: 'bg-emby', badge: 'bg-emby/20 text-green-700 dark:text-green-300' },
  jellyfin: { bar: 'bg-jellyfin', badge: 'bg-jellyfin/20 text-purple-700 dark:text-purple-300' },
}

const defaultAccent = { bar: 'bg-accent', badge: 'bg-accent/20 text-blue-700 dark:text-blue-300' }

const mediaTypeIcons: Record<string, string> = {
  movie: '\uD83C\uDFAC',
  episode: '\uD83D\uDCFA',
}
const defaultMediaIcon = '\uD83C\uDFB5'

interface MediaDetailModalProps {
  item: ItemDetails | null
  loading: boolean
  onClose: () => void
}

function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center py-20">
      <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin" />
    </div>
  )
}

function StarRating({ rating }: { rating: number }) {
  const stars = Math.max(0, Math.min(5, Math.round(rating / 2)))
  return (
    <span className="text-amber-500" title={`${rating.toFixed(1)} / 10`}>
      {'★'.repeat(stars)}{'☆'.repeat(5 - stars)}
    </span>
  )
}

function ErrorState() {
  return (
    <div className="p-8 text-center text-muted dark:text-muted-dark">
      Failed to load item details
    </div>
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
  return (
    <span className={`px-2 py-0.5 text-xs font-medium rounded-full ${color}`}>
      {status}
    </span>
  )
}

function NetworkLogos({ networks }: { networks: { id: number; name: string; logo_path?: string }[] }) {
  if (!networks.length) return null
  return (
    <div className="flex items-center gap-3">
      {networks.map(net => (
        net.logo_path ? (
          <img
            key={net.id}
            src={`${TMDB_IMG}/w92${net.logo_path}`}
            alt={net.name}
            title={net.name}
            className="h-5 object-contain dark:invert dark:brightness-200"
            loading="lazy"
          />
        ) : (
          <span key={net.id} className="text-xs text-muted dark:text-muted-dark">{net.name}</span>
        )
      ))}
    </div>
  )
}

interface ItemContentProps {
  item: ItemDetails
  accent: { bar: string; badge: string }
  tmdbMovie: TMDBMovieDetails | null
  tmdbTV: TMDBTVDetails | null
  tmdbLoading: boolean
  onPersonClick: (personId: number) => void
}

function ItemContent({ item, accent, tmdbMovie, tmdbTV, tmdbLoading, onPersonClick }: ItemContentProps) {
  const tmdbCast = tmdbMovie?.credits?.cast || tmdbTV?.credits?.cast
  const tmdbCrew = tmdbMovie?.credits?.crew || tmdbTV?.credits?.crew
  const tmdbGenres = tmdbMovie?.genres || tmdbTV?.genres
  const tmdbDirectors = tmdbCrew?.filter((c: TMDBCrew) => c.job === 'Director')
  const directorNames = tmdbDirectors && tmdbDirectors.length > 0
    ? tmdbDirectors.map(d => d.name)
    : item.directors && item.directors.length > 0
      ? item.directors
      : null
  const hasTMDBCast = tmdbCast && tmdbCast.length > 0
  const collection = tmdbMovie?.belongs_to_collection

  const serverThumbByName = useMemo(() => {
    const map = new Map<string, string>()
    if (item.cast) {
      for (const m of item.cast) {
        if (m.thumb_url) map.set(m.name, m.thumb_url)
      }
    }
    return map
  }, [item.cast])

  return (
    <div className="flex flex-col md:flex-row overflow-y-auto max-h-[calc(90dvh-4px)]">
      <div className="shrink-0 p-4 md:p-6 flex justify-center md:block md:w-1/3">
        {item.thumb_url ? (
          <img
            src={thumbUrl(item.server_id, item.thumb_url)}
            alt={item.title}
            className="max-h-48 md:max-h-none w-auto md:w-full aspect-[2/3] object-cover rounded-lg shadow-lg"
          />
        ) : (
          <div className="max-h-48 md:max-h-none w-auto md:w-full aspect-[2/3] rounded-lg bg-gray-200 dark:bg-white/10 flex items-center justify-center">
            <span className="text-6xl opacity-20">
              {mediaTypeIcons[item.media_type] ?? defaultMediaIcon}
            </span>
          </div>
        )}
      </div>

      <div className="flex-1 p-4 md:p-6 md:pl-0 space-y-4 overflow-y-auto">
        <div>
          {item.series_title && (
            <div className="text-sm text-muted dark:text-muted-dark mb-1">
              {item.series_title}
              {item.season_number != null && item.episode_number != null && (
                <span> &middot; S{item.season_number}E{item.episode_number}</span>
              )}
            </div>
          )}
          <h2 id="modal-title" className="text-2xl font-bold text-gray-900 dark:text-gray-50">
            {item.title}
          </h2>
          <div className="flex flex-wrap items-center gap-2 mt-2 text-sm text-muted dark:text-muted-dark">
            {item.year && <span>{item.year}</span>}
            {item.duration_ms && (
              <>
                <span>&middot;</span>
                <span>{formatDuration(item.duration_ms)}</span>
              </>
            )}
            {item.content_rating && (
              <>
                <span>&middot;</span>
                <span className="px-1.5 py-0.5 text-xs border border-current rounded">
                  {item.content_rating}
                </span>
              </>
            )}
            {item.rating && item.rating > 0 && (
              <>
                <span>&middot;</span>
                <StarRating rating={item.rating} />
              </>
            )}
            {tmdbTV?.status && (
              <>
                <span>&middot;</span>
                <TVStatusBadge status={tmdbTV.status} />
              </>
            )}
          </div>

          {tmdbTV?.networks && tmdbTV.networks.length > 0 && (
            <div className="mt-2">
              <NetworkLogos networks={tmdbTV.networks} />
            </div>
          )}

          {tmdbTV?.number_of_seasons != null && (
            <div className="text-sm text-muted dark:text-muted-dark mt-1">
              {tmdbTV.number_of_seasons} season{tmdbTV.number_of_seasons !== 1 ? 's' : ''}
              {tmdbTV.number_of_episodes != null && (
                <span> &middot; {tmdbTV.number_of_episodes} episodes</span>
              )}
            </div>
          )}
        </div>

        {(tmdbGenres || item.genres) && (tmdbGenres?.length || item.genres?.length) ? (
          <div className="flex flex-wrap gap-2">
            {tmdbGenres
              ? tmdbGenres.map(g => (
                  <span
                    key={g.id}
                    className={`px-2.5 py-1 text-xs font-medium rounded-full ${accent.badge}`}
                  >
                    {g.name}
                  </span>
                ))
              : item.genres!.map(genre => (
                  <span
                    key={genre}
                    className={`px-2.5 py-1 text-xs font-medium rounded-full ${accent.badge}`}
                  >
                    {genre}
                  </span>
                ))}
          </div>
        ) : null}

        {item.summary && (
          <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed">
            {item.summary}
          </p>
        )}

        {directorNames && (
          <div className="text-sm">
            <span className="text-muted dark:text-muted-dark">Directed by </span>
            <span className="text-gray-900 dark:text-gray-100">
              {directorNames.join(', ')}
            </span>
          </div>
        )}

        {(hasTMDBCast || (item.cast && item.cast.length > 0)) && (
          <div className="space-y-2">
            <div className="text-sm font-medium text-gray-900 dark:text-gray-100">Cast</div>
            <div className="flex gap-2 overflow-x-auto pb-2 -mx-4 px-4 md:mx-0 md:px-0">
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
                        onClick={() => onPersonClick(person.id)}
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
                <img
                  src={`${TMDB_IMG}/w92${collection.poster_path}`}
                  alt={collection.name}
                  className="w-12 rounded shadow"
                />
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

        <div className="pt-2 flex items-center justify-between text-xs text-muted dark:text-muted-dark border-t border-border dark:border-border-dark">
          <span>{item.studio}</span>
          <span>{item.server_name}</span>
        </div>
      </div>
    </div>
  )
}

export function MediaDetailModal({ item, loading, onClose }: MediaDetailModalProps) {
  const { stack, current: currentModal, push: pushModal, pop: popModal } = useModalStack()

  const { movie: tmdbMovie, tv: tmdbTV, loading: tmdbLoading } = useTMDBEnrichment(
    item?.tmdb_id,
    item?.media_type
  )

  const { data: configStatus } = useFetch<{ configured: boolean }>('/api/overseerr/configured')
  const overseerrConfigured = !!configStatus?.configured

  const { data: libraryData } = useFetch<{ ids: string[] }>('/api/library/tmdb-ids')
  const libraryIds = useMemo(() => new Set(libraryData?.ids ?? []), [libraryData])

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape' && !currentModal) {
      e.stopImmediatePropagation()
      onClose()
    }
  }, [onClose, currentModal])

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [handleKeyDown])

  useEffect(() => {
    lockBodyScroll()
    return () => unlockBodyScroll()
  }, [])

  const accent = item ? (serverAccent[item.server_type] ?? defaultAccent) : defaultAccent

  return (
    <>
      <div
        className="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fade-in"
        onClick={onClose}
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
        aria-hidden={stack.length > 0 || undefined}
      >
        <div
          className="relative w-full max-w-6xl max-h-[90dvh] overflow-hidden rounded-xl bg-panel dark:bg-panel-dark shadow-2xl animate-slide-up"
          onClick={e => e.stopPropagation()}
        >
          <div className={`h-1 ${accent.bar}`} />

          <button
            onClick={onClose}
            className="absolute top-3 right-3 z-10 w-8 h-8 flex items-center justify-center rounded-full bg-black/40 hover:bg-black/60 text-white transition-colors"
            aria-label="Close"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>

          {loading && <LoadingSpinner />}
          {!loading && item && (
            <ItemContent
              item={item}
              accent={accent}
              tmdbMovie={tmdbMovie}
              tmdbTV={tmdbTV}
              tmdbLoading={tmdbLoading}
              onPersonClick={id => pushModal({ type: 'person', personId: id })}
            />
          )}
          {!loading && !item && <ErrorState />}
        </div>
      </div>

      {stack.length > 0 && (
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
