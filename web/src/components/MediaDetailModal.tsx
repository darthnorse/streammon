import { useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import type { ItemDetails } from '../types'
import { formatDuration, formatBitrate, formatAudioCodec, formatVideoCodec, formatDate } from '../lib/format'
import { getAudioCodecIcon, getVideoCodecIcon, getResolutionIcon, getChannelsIcon } from '../lib/mediaFlags'

const serverAccent: Record<string, { bar: string; badge: string }> = {
  plex: { bar: 'bg-warn', badge: 'bg-warn/20 text-amber-700 dark:text-amber-300' },
  emby: { bar: 'bg-emby', badge: 'bg-emby/20 text-green-700 dark:text-green-300' },
  jellyfin: { bar: 'bg-jellyfin', badge: 'bg-jellyfin/20 text-purple-700 dark:text-purple-300' },
}

const defaultAccent = { bar: 'bg-accent', badge: 'bg-accent/20 text-blue-700 dark:text-blue-300' }

const mediaTypeIcons: Record<string, string> = {
  movie: '🎬',
  episode: '📺',
}
const defaultMediaIcon = '🎵'

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
  const stars = Math.round(rating / 2)
  return (
    <span className="text-amber-500" title={`${rating.toFixed(1)} / 10`}>
      {'★'.repeat(stars)}{'☆'.repeat(5 - stars)}
    </span>
  )
}

function CastChip({ name, role, thumbUrl }: { name: string; role?: string; thumbUrl?: string }) {
  if (!name) return null
  const initials = name.split(' ').map(n => n[0]).join('').slice(0, 2).toUpperCase()
  return (
    <div className="flex items-center gap-2 px-2 py-1 rounded-full bg-gray-100 dark:bg-white/10 shrink-0">
      {thumbUrl ? (
        <img
          src={thumbUrl}
          alt={name}
          className="w-6 h-6 rounded-full object-cover bg-gray-300 dark:bg-white/20"
          loading="lazy"
        />
      ) : (
        <div className="w-6 h-6 rounded-full bg-gray-300 dark:bg-white/20 flex items-center justify-center text-[10px] font-medium text-gray-600 dark:text-gray-300">
          {initials}
        </div>
      )}
      <div className="text-xs">
        <div className="font-medium text-gray-900 dark:text-gray-100">{name}</div>
        {role && <div className="text-gray-500 dark:text-gray-400 text-[10px]">{role}</div>}
      </div>
    </div>
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
  return <img src={src} alt={alt} className="h-4 object-contain" loading="lazy" />
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
                className="font-medium text-accent-dim dark:text-accent hover:underline truncate"
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

interface ItemContentProps {
  item: ItemDetails
  accent: { bar: string; badge: string }
}

function ItemContent({ item, accent }: ItemContentProps) {
  return (
    <div className="flex flex-col md:flex-row overflow-y-auto max-h-[calc(90vh-4px)]">
      <div className="md:w-1/3 shrink-0 p-4 md:p-6">
        {item.thumb_url ? (
          <img
            src={item.thumb_url}
            alt={item.title}
            className="w-full aspect-[2/3] object-cover rounded-lg shadow-lg"
          />
        ) : (
          <div className="w-full aspect-[2/3] rounded-lg bg-gray-200 dark:bg-white/10 flex items-center justify-center">
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
          </div>
        </div>

        {item.genres && item.genres.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {item.genres.map(genre => (
              <span
                key={genre}
                className={`px-2.5 py-1 text-xs font-medium rounded-full ${accent.badge}`}
              >
                {genre}
              </span>
            ))}
          </div>
        )}

        {item.summary && (
          <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed">
            {item.summary}
          </p>
        )}

        {item.directors && item.directors.length > 0 && (
          <div className="text-sm">
            <span className="text-muted dark:text-muted-dark">Directed by </span>
            <span className="text-gray-900 dark:text-gray-100">
              {item.directors.join(', ')}
            </span>
          </div>
        )}

        {item.cast && item.cast.length > 0 && (
          <div className="space-y-2">
            <div className="text-sm font-medium text-gray-900 dark:text-gray-100">Cast</div>
            <div className="flex gap-2 overflow-x-auto pb-2 -mx-4 px-4 md:mx-0 md:px-0">
              {item.cast.slice(0, 6).map((member, idx) => (
                <CastChip key={`${member.name}-${idx}`} name={member.name} role={member.role} thumbUrl={member.thumb_url} />
              ))}
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

  const accent = item ? (serverAccent[item.server_type] ?? defaultAccent) : defaultAccent

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fade-in"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-title"
    >
      <div
        className="relative w-full max-w-6xl max-h-[90vh] overflow-hidden rounded-xl bg-panel dark:bg-panel-dark shadow-2xl animate-slide-up"
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
        {!loading && item && <ItemContent item={item} accent={accent} />}
        {!loading && !item && <ErrorState />}
      </div>
    </div>
  )
}
