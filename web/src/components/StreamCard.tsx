import { Link } from 'react-router-dom'
import type { ActiveStream } from '../types'
import { formatTimestamp } from '../lib/format'
import { mediaTypeLabels } from '../lib/constants'

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

function TranscodeInfo({ stream }: { stream: ActiveStream }) {
  if (!stream.video_decision && !stream.video_codec) return null

  const isTranscode = stream.video_decision === 'transcode'
  const decisionLabel = stream.video_decision || 'unknown'

  return (
    <div className="flex flex-wrap gap-1.5 mt-2">
      <span className={isTranscode ? 'badge badge-warn' : 'badge badge-accent'}>
        {decisionLabel}
        {isTranscode && stream.transcode_hw_accel && ' (HW)'}
      </span>
      {stream.video_codec && (
        <span className="badge badge-muted">{stream.video_codec}</span>
      )}
      {stream.video_resolution && (
        <span className="badge badge-muted">{stream.video_resolution}</span>
      )}
      {stream.audio_codec && (
        <span className="badge badge-muted">{stream.audio_codec}</span>
      )}
      {(stream.audio_channels ?? 0) > 0 && (
        <span className="badge badge-muted">{stream.audio_channels}ch</span>
      )}
      {stream.subtitle_codec && (
        <span className="badge badge-muted">SUB: {stream.subtitle_codec}</span>
      )}
    </div>
  )
}

export function StreamCard({ stream }: StreamCardProps) {
  const progress = stream.duration_ms > 0
    ? Math.round((stream.progress_ms / stream.duration_ms) * 100)
    : 0

  return (
    <div className="card card-hover p-4">
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

      {(stream.bandwidth ?? 0) > 0 ? (
        <div className="mt-2 text-xs text-muted dark:text-muted-dark font-mono">
          {((stream.bandwidth ?? 0) / 1000).toFixed(1)} Mbps
        </div>
      ) : null}
    </div>
  )
}
