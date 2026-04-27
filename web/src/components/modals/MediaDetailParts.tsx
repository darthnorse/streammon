import { Link } from 'react-router-dom'
import type { ItemDetails } from '../../types'
import { formatBitrate, formatAudioCodec, formatVideoCodec, formatDate } from '../../lib/format'
import { getAudioCodecIcon, getVideoCodecIcon, getResolutionIcon, getChannelsIcon } from '../../lib/mediaFlags'
import { TMDB_IMG } from '../../lib/tmdb'
import { MEDIA_STATUS } from '../../lib/overseerr'

export const serverAccent: Record<string, { bar: string; badge: string }> = {
  plex: { bar: 'bg-warn', badge: 'bg-warn/20 text-amber-700 dark:text-amber-300' },
  emby: { bar: 'bg-emby', badge: 'bg-emby/20 text-green-700 dark:text-green-300' },
  jellyfin: { bar: 'bg-jellyfin', badge: 'bg-jellyfin/20 text-purple-700 dark:text-purple-300' },
}

export const defaultAccent = { bar: 'bg-accent', badge: 'bg-accent/20 text-blue-700 dark:text-blue-300' }

export const mediaTypeIcons: Record<string, string> = { movie: '🎬', episode: '📺' }

export function StarRating({ rating }: { rating: number }) {
  const stars = Math.max(0, Math.min(5, Math.round(rating / 2)))
  return (
    <span className="text-amber-500" title={`${rating.toFixed(1)} / 10`}>
      {'★'.repeat(stars)}{'☆'.repeat(5 - stars)}
    </span>
  )
}

export function MediaFlagIcon({ src, alt }: { src: string | null; alt: string }) {
  if (!src) return null
  return <img src={src} alt={alt} className="h-4 object-contain invert dark:invert-0" loading="lazy" />
}

export function TechInfo({ item }: { item: ItemDetails }) {
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

export function WatchHistory({ item }: { item: ItemDetails }) {
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

export const tvStatusColors: Record<string, string> = {
  'Ended': 'bg-red-500/15 text-red-600 dark:text-red-400',
  'Canceled': 'bg-red-500/15 text-red-600 dark:text-red-400',
  'Returning Series': 'bg-green-500/15 text-green-600 dark:text-green-400',
  'In Production': 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
  'Planned': 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
}

export function TVStatusBadge({ status }: { status: string }) {
  const color = tvStatusColors[status] || 'bg-gray-500/15 text-gray-600 dark:text-gray-400'
  return <span className={`px-2 py-0.5 text-xs font-medium rounded-full ${color}`}>{status}</span>
}

export function NetworkLogos({ networks }: { networks: { id: number; name: string; logo_path?: string }[] }) {
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

export function regularSeasonNumbers(seasons: { season_number: number }[]): number[] {
  return seasons.filter(s => s.season_number > 0).map(s => s.season_number)
}

export function requestButtonLabel(requesting: boolean, status: number | undefined, mediaType: 'movie' | 'tv'): string {
  if (requesting) return 'Requesting...'
  if (status === MEDIA_STATUS.PARTIALLY_AVAILABLE && mediaType === 'tv') return 'Request More'
  return `Request ${mediaType === 'movie' ? 'Movie' : 'TV Show'}`
}

export function resolveMediaType(mediaType: string | undefined): 'movie' | 'tv' | null {
  if (mediaType === 'episode') return 'tv'
  if (mediaType === 'movie') return 'movie'
  return null
}
