import { Link } from 'react-router-dom'
import type { ActiveStream } from '../types'
import { formatTimestamp, formatBitrate, formatChannels } from '../lib/format'
import { mediaTypeLabels } from '../lib/constants'
import { GeoIPPopover } from './GeoIPPopover'

interface StreamCardProps {
  stream: ActiveStream
}

function MediaTitle({ stream }: { stream: ActiveStream }) {
  if (stream.media_type === 'episode' && stream.grandparent_title) {
    return (
      <div>
        <div className="font-semibold text-gray-900 dark:text-gray-50 truncate">
          {stream.grandparent_title}
        </div>
        <div className="text-sm text-muted dark:text-muted-dark truncate">
          {stream.parent_title} &middot; {stream.title}
        </div>
      </div>
    )
  }
  return (
    <div>
      <div className="font-semibold text-gray-900 dark:text-gray-50 truncate">
        {stream.title}
      </div>
      {stream.year > 0 && (
        <div className="text-sm text-muted dark:text-muted-dark">{stream.year}</div>
      )}
    </div>
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
    { label: '', value: stream.subtitle_codec ? `Subtitle: ${stream.subtitle_codec.toUpperCase()}` : '' },
  ].filter(l => l.value)

  return (
    <div className="mt-2 text-xs text-muted dark:text-muted-dark font-mono space-y-0.5">
      {lines.map((l, i) => (
        <div key={i}>
          {l.label && <span className="text-muted dark:text-muted-dark/60">{l.label}: </span>}
          <span>{l.value}</span>
        </div>
      ))}
    </div>
  )
}

export function StreamCard({ stream }: StreamCardProps) {
  const progress = stream.duration_ms > 0
    ? Math.round((stream.progress_ms / stream.duration_ms) * 100)
    : 0

  return (
    <div className="card card-hover p-4">
      <div className="flex gap-3">
        {stream.thumb_url && (
          <img src={stream.thumb_url} alt="" className="w-14 h-20 object-cover rounded shrink-0 bg-gray-200 dark:bg-white/10" />
        )}
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 mb-1">
                <Link
                  to={`/users/${encodeURIComponent(stream.user_name)}`}
                  className="text-sm font-medium text-accent-dim dark:text-accent hover:underline truncate"
                >
                  {stream.user_name}
                </Link>
                <span className="badge badge-muted">{mediaTypeLabels[stream.media_type]}</span>
              </div>
              <MediaTitle stream={stream} />
            </div>
            <div className="text-right shrink-0">
              <div className="text-xs text-muted dark:text-muted-dark font-mono">
                {stream.server_name}
              </div>
              <div className="text-xs text-muted dark:text-muted-dark mt-0.5">
                {stream.player}
              </div>
              {stream.ip_address && (
                <GeoIPPopover ip={stream.ip_address}>
                  <span className="text-xs font-mono text-muted dark:text-muted-dark hover:text-accent dark:hover:text-accent transition-colors mt-0.5 inline-block">
                    {stream.ip_address}
                  </span>
                </GeoIPPopover>
              )}
            </div>
          </div>

          <TranscodeInfo stream={stream} />

          <div className="mt-3">
            <div className="flex justify-between text-xs text-muted dark:text-muted-dark font-mono mb-1">
              <span>{formatTimestamp(stream.progress_ms)}</span>
              <span>{formatTimestamp(stream.duration_ms)}</span>
            </div>
            <div
              role="progressbar"
              aria-valuenow={progress}
              aria-valuemin={0}
              aria-valuemax={100}
              className="h-1.5 rounded-full bg-gray-200 dark:bg-white/10 overflow-hidden"
            >
              <div
                className="h-full rounded-full bg-accent transition-all duration-500"
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
