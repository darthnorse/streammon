import { Link } from 'react-router-dom'
import type { ActiveStream } from '../types'
import { formatTimestamp, formatBitrate, formatChannels } from '../lib/format'
import { mediaTypeLabels } from '../lib/constants'
import { GeoIPPopover } from './GeoIPPopover'

const serverAccent: Record<string, { bar: string; progress: string; badge: string }> = {
  plex: { bar: 'bg-warn', progress: 'bg-warn', badge: 'badge-warn' },
  emby: { bar: 'bg-emby', progress: 'bg-emby', badge: 'badge-emby' },
  jellyfin: { bar: 'bg-jellyfin', progress: 'bg-jellyfin', badge: 'badge-jellyfin' },
}

const defaultAccent = { bar: 'bg-accent', progress: 'bg-accent', badge: 'badge-accent' }

interface StreamCardProps {
  stream: ActiveStream
}

function MediaTitle({ stream }: { stream: ActiveStream }) {
  if (stream.media_type === 'episode' && stream.grandparent_title) {
    return (
      <>
        <div className="font-semibold text-gray-900 dark:text-gray-50 truncate text-[15px] leading-snug">
          {stream.grandparent_title}
        </div>
        <div className="text-xs text-muted dark:text-muted-dark truncate mt-0.5">
          {stream.parent_title} &middot; {stream.title}
        </div>
      </>
    )
  }
  return (
    <>
      <div className="font-semibold text-gray-900 dark:text-gray-50 truncate text-[15px] leading-snug">
        {stream.title}
      </div>
      {stream.year > 0 && (
        <div className="text-xs text-muted dark:text-muted-dark mt-0.5">{stream.year}</div>
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
  const src = [stream.video_resolution, stream.video_codec?.toUpperCase()].filter(Boolean).join(' ')
  if (!src) return ''
  if (stream.video_decision === 'direct play') return `${src} - Direct Play`
  if (stream.video_decision === 'copy') return `${src} - Direct Stream`
  const dst = [stream.transcode_video_codec?.toUpperCase(), formatBitrate(stream.bandwidth ?? 0)].filter(Boolean).join(' ')
  const hw = stream.transcode_hw_accel ? ' (HW)' : ''
  return `${src} \u2192 ${dst}${hw}`
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
  ].filter(l => l.value)

  return (
    <div className="mt-2 grid grid-cols-[auto_1fr] gap-x-2 gap-y-px text-[10px] leading-relaxed font-mono">
      {lines.map((l, i) => (
        <div key={i} className="contents">
          <span className="text-muted dark:text-muted-dark/50 select-none">{l.label}</span>
          <span className="text-muted dark:text-muted-dark truncate">{l.value}</span>
        </div>
      ))}
    </div>
  )
}

export function StreamCard({ stream }: StreamCardProps) {
  const progress = stream.duration_ms > 0
    ? Math.round((stream.progress_ms / stream.duration_ms) * 100)
    : 0

  const accent = serverAccent[stream.server_type] ?? defaultAccent

  return (
    <div className="card card-hover overflow-hidden group">
      {/* Top accent bar */}
      <div className={`h-0.5 ${accent.bar}`} />

      <div className="flex gap-3 p-3 h-full">
        {/* Poster */}
        <div className="shrink-0 flex flex-col items-center gap-1.5">
          {stream.thumb_url ? (
            <div className="relative">
              <img
                src={stream.thumb_url}
                alt=""
                className="w-16 h-24 object-cover rounded bg-gray-200 dark:bg-white/5"
              />
              <div className="absolute inset-0 rounded bg-gradient-to-t from-black/40 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
            </div>
          ) : (
            <div className="w-16 h-24 rounded bg-gray-100 dark:bg-white/5 flex items-center justify-center">
              <span className="text-2xl opacity-20">
                {stream.media_type === 'movie' ? '🎬' : stream.media_type === 'episode' ? '📺' : '🎵'}
              </span>
            </div>
          )}
          <span className={`badge text-[9px] py-0 px-1.5 ${accent.badge}`}>
            {mediaTypeLabels[stream.media_type]}
          </span>
        </div>

        {/* Content */}
        <div className="min-w-0 flex-1 flex flex-col">
          {/* Header row */}
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0 flex-1">
              <MediaTitle stream={stream} />
            </div>
            <div className="text-right shrink-0 space-y-0.5">
              <div className="text-[10px] text-muted dark:text-muted-dark font-mono leading-tight">
                {stream.server_name}
              </div>
              <div className="text-[10px] text-muted dark:text-muted-dark leading-tight">
                {stream.player}
              </div>
              {stream.ip_address && (
                <GeoIPPopover ip={stream.ip_address}>
                  <span className="text-[10px] font-mono text-muted dark:text-muted-dark hover:text-accent dark:hover:text-accent transition-colors inline-block leading-tight">
                    {stream.ip_address}
                  </span>
                </GeoIPPopover>
              )}
            </div>
          </div>

          <TranscodeInfo stream={stream} />

          {/* Progress — always pinned to bottom */}
          <div className="mt-auto pt-2">
            <div className="flex items-baseline justify-between text-[10px] font-mono mb-0.5">
              <span className="text-muted dark:text-muted-dark">{formatTimestamp(stream.progress_ms)} / {formatTimestamp(stream.duration_ms)}</span>
              <Link
                to={`/users/${encodeURIComponent(stream.user_name)}`}
                className="text-xs font-medium text-accent-dim dark:text-accent hover:underline truncate ml-2"
              >
                {stream.user_name}
              </Link>
            </div>
            <div
              role="progressbar"
              aria-valuenow={progress}
              aria-valuemin={0}
              aria-valuemax={100}
              className="h-1 rounded-full bg-gray-200 dark:bg-white/10 overflow-hidden"
            >
              <div
                className={`h-full rounded-full ${accent.progress} transition-all duration-500`}
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
