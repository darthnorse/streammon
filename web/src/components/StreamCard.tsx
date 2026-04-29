import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { ActiveStream, TitleClickHandler } from '../types'
import { formatTimestamp, formatBitrate, formatChannels, formatEpisode, parseSeasonFromTitle, thumbUrl } from '../lib/format'
import { getMediaLabel, CLICKABLE_TITLE_CLASS } from '../lib/constants'
import { GeoIPPopover } from './GeoIPPopover'
import { TerminateSessionDialog } from './TerminateSessionDialog'

const serverAccent: Record<string, { bar: string; progress: string; badge: string }> = {
  plex: { bar: 'bg-warn', progress: 'bg-warn', badge: 'badge-warn' },
  emby: { bar: 'bg-emby', progress: 'bg-emby', badge: 'badge-emby' },
  jellyfin: { bar: 'bg-jellyfin', progress: 'bg-jellyfin', badge: 'badge-jellyfin' },
}

const defaultAccent = { bar: 'bg-accent', progress: 'bg-accent', badge: 'badge-accent' }

interface StreamCardProps {
  stream: ActiveStream
  isAdmin?: boolean
  onTitleClick?: TitleClickHandler
}

function MediaTitle({ stream, onTitleClick }: { stream: ActiveStream; onTitleClick?: TitleClickHandler }) {
  const canClickSeries = onTitleClick && stream.server_id && stream.grandparent_item_id
  const canClickItem = onTitleClick && stream.server_id && stream.item_id

  if (stream.media_type === 'episode' && stream.grandparent_title) {
    const season = stream.season_number ?? parseSeasonFromTitle(stream.parent_title)
    const episodeInfo = formatEpisode(season, stream.episode_number)
    const subtitle = episodeInfo ? `${episodeInfo} · ${stream.title}` : stream.title

    return (
      <>
        <div
          className={`font-semibold text-gray-900 dark:text-gray-50 truncate text-base leading-snug ${canClickSeries ? CLICKABLE_TITLE_CLASS : ''}`}
          onClick={canClickSeries ? () => onTitleClick(stream.server_id, stream.grandparent_item_id!) : undefined}
        >
          {stream.grandparent_title}
        </div>
        <div
          className={`text-sm text-gray-600 dark:text-gray-300 truncate mt-1 ${canClickItem ? CLICKABLE_TITLE_CLASS : ''}`}
          onClick={canClickItem ? () => onTitleClick(stream.server_id, stream.item_id!) : undefined}
        >
          {subtitle}
        </div>
      </>
    )
  }
  if (stream.media_type === 'livetv') {
    const showProgram = stream.title && stream.title !== stream.grandparent_title
    const subtitle = stream.parent_title
      ? `${stream.title} · ${stream.parent_title}`
      : stream.title
    return (
      <>
        <div className="font-semibold text-gray-900 dark:text-gray-50 truncate text-base leading-snug">
          {stream.grandparent_title || stream.title}
        </div>
        {showProgram && (
          <div className="text-sm text-gray-600 dark:text-gray-300 truncate mt-1">
            {subtitle}
          </div>
        )}
      </>
    )
  }
  if (stream.extra_type && stream.parent_title) {
    return (
      <>
        <div className="font-semibold text-gray-900 dark:text-gray-50 truncate text-base leading-snug">
          {stream.parent_title}
        </div>
        <div className="text-sm text-gray-600 dark:text-gray-300 truncate mt-1">
          {stream.title}
        </div>
      </>
    )
  }
  return (
    <>
      <div
        className={`font-semibold text-gray-900 dark:text-gray-50 truncate text-base leading-snug ${canClickItem ? CLICKABLE_TITLE_CLASS : ''}`}
        onClick={canClickItem ? () => onTitleClick(stream.server_id, stream.item_id!) : undefined}
      >
        {stream.title}
      </div>
      {stream.year > 0 && (
        <div className="text-sm text-gray-600 dark:text-gray-300 mt-1">{stream.year}</div>
      )}
    </>
  )
}

function formatStreamLine(stream: ActiveStream): string {
  const src = [stream.container?.toUpperCase(), formatBitrate(stream.bitrate ?? 0)].filter(Boolean).join(' ')
  if (stream.video_decision === 'direct play') return src ? `${src} - Direct Play` : 'Direct Play'
  const dst = [stream.transcode_container?.toUpperCase(), formatBitrate(stream.bandwidth ?? 0)].filter(Boolean).join(' ')
  if (src && dst) return `${src} \u2192 ${dst}`
  return dst || src
}

function formatVideoLine(stream: ActiveStream): string {
  const srcCodec = stream.video_codec?.toUpperCase()
  const srcRes = stream.video_resolution
  const src = [srcCodec, srcRes].filter(Boolean).join(' ')
  if (!src) return ''

  if (stream.video_decision === 'direct play') return `${src} - Direct Play`
  if (stream.video_decision === 'copy') return `${src} - Direct Stream`

  const dstCodec = stream.transcode_video_codec?.toUpperCase()
  const dstRes = stream.transcode_video_resolution
  const dst = [dstCodec, dstRes].filter(Boolean).join(' ')
  if (!dst) return src
  const dstHw = stream.transcode_hw_encode ? ' (HW)' : ''
  return `${src} \u2192 ${dst}${dstHw}`
}

function formatAudioLine(stream: ActiveStream): string {
  const src = [stream.audio_codec?.toUpperCase(), formatChannels(stream.audio_channels ?? 0)].filter(Boolean).join(' ')
  if (!src) return ''
  if (stream.audio_decision === 'direct play') return `${src} - Direct Play`
  if (stream.audio_decision === 'copy') return `${src} - Direct Stream`
  const dst = stream.transcode_audio_codec?.toUpperCase() || 'Transcode'
  return `${src} \u2192 ${dst}`
}

function TranscodeInfo({ stream }: { stream: ActiveStream }) {
  if (!stream.video_decision && !stream.video_codec) return null

  const lines = [
    { label: 'Stream', value: formatStreamLine(stream) },
    { label: 'Video', value: formatVideoLine(stream) },
    { label: 'Audio', value: formatAudioLine(stream) },
    { label: 'Sub', value: stream.subtitle_codec ? stream.subtitle_codec.toUpperCase() : '' },
    { label: 'BW', value: stream.bandwidth ? formatBitrate(stream.bandwidth) : '' },
  ].filter(l => l.value)

  return (
    <div className="mt-3 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs leading-relaxed font-mono">
      {lines.map((l, i) => (
        <div key={i} className="contents">
          <span className="text-gray-500 dark:text-gray-400 select-none">{l.label}</span>
          <span className="text-gray-600 dark:text-gray-300 truncate">{l.value}</span>
        </div>
      ))}
    </div>
  )
}

export function StreamCard({ stream, isAdmin, onTitleClick }: StreamCardProps) {
  const [showTerminate, setShowTerminate] = useState(false)

  const progress = stream.duration_ms > 0
    ? Math.round((stream.progress_ms / stream.duration_ms) * 100)
    : 0

  const accent = serverAccent[stream.server_type] ?? defaultAccent

  return (
    <div className="card card-hover overflow-hidden group">
      <div className={`h-1 ${accent.bar}`} />

      <div className="flex gap-4 p-4 h-full">
        <div className="shrink-0 flex flex-col items-center gap-2">
          {stream.thumb_url ? (
            <div className="relative">
              <img
                src={thumbUrl(stream.server_id, stream.thumb_url)}
                alt=""
                className="w-32 aspect-[2/3] object-cover rounded-lg shadow-md bg-gray-200 dark:bg-white/5"
              />
              <div className="absolute inset-0 rounded-lg bg-gradient-to-t from-black/40 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
            </div>
          ) : (
            <div className="w-32 aspect-[2/3] rounded-lg bg-gray-100 dark:bg-white/5 flex items-center justify-center shadow-md">
              <span className="text-3xl opacity-20">
                {stream.media_type === 'movie' ? '🎬' : stream.media_type === 'episode' ? '📺' : '🎵'}
              </span>
            </div>
          )}
          <span className={`badge text-[10px] py-0.5 px-2 ${accent.badge}`}>
            {getMediaLabel(stream.media_type, stream.extra_type)}
          </span>
        </div>

        <div className="min-w-0 flex-1 flex flex-col">
          <div>
            <div className="flex items-start gap-2">
              <div className="min-w-0 flex-1">
                <MediaTitle stream={stream} onTitleClick={onTitleClick} />
              </div>
              {isAdmin && (
                <button
                  onClick={() => setShowTerminate(true)}
                  aria-label="Terminate stream"
                  title="Terminate stream"
                  className="shrink-0 mt-0.5 p-1 rounded text-muted dark:text-muted-dark
                             hover:text-red-500 dark:hover:text-red-400 transition-colors
                             opacity-0 group-hover:opacity-100"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
                    <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z" clipRule="evenodd" />
                  </svg>
                </button>
              )}
            </div>
            <div className="flex items-center gap-2 mt-1 text-xs text-muted dark:text-muted-dark">
              <span className="font-mono">{stream.server_name}</span>
              <span>·</span>
              <span className="truncate">{stream.player}</span>
            </div>
          </div>

          <TranscodeInfo stream={stream} />

          <div className="mt-auto pt-3">
            {stream.ip_address && (
              <div className="flex justify-end mb-1">
                <GeoIPPopover ip={stream.ip_address}>
                  <span className="text-xs font-mono text-muted dark:text-muted-dark hover:text-accent dark:hover:text-accent transition-colors">
                    {stream.ip_address}
                  </span>
                </GeoIPPopover>
              </div>
            )}
            <div className="flex items-baseline justify-between text-xs font-mono mb-1">
              <span className="text-muted dark:text-muted-dark">{formatTimestamp(stream.progress_ms)} / {formatTimestamp(stream.duration_ms)}</span>
              <Link
                to={`/users/${encodeURIComponent(stream.user_name)}`}
                className="text-sm font-medium hover:text-accent hover:underline truncate ml-2"
              >
                {stream.user_name}
              </Link>
            </div>
            <div
              role="progressbar"
              aria-valuenow={progress}
              aria-valuemin={0}
              aria-valuemax={100}
              className="h-1.5 rounded-full bg-gray-200 dark:bg-white/10 overflow-hidden"
            >
              <div
                className={`h-full rounded-full ${accent.progress} transition-all duration-500`}
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        </div>
      </div>
      {showTerminate && (
        <TerminateSessionDialog
          stream={stream}
          onClose={() => setShowTerminate(false)}
        />
      )}
    </div>
  )
}
