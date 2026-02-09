import type { ConcurrentPeaks } from '../../types'

interface ConcurrentStreamsCardProps {
  peaks: ConcurrentPeaks
}

type NumericPeakKey = 'total' | 'direct_play' | 'direct_stream' | 'transcode'

const ROWS: { label: string; key: NumericPeakKey }[] = [
  { label: 'Total', key: 'total' },
  { label: 'Direct Play', key: 'direct_play' },
  { label: 'Direct Stream', key: 'direct_stream' },
  { label: 'Transcode', key: 'transcode' },
]

export function ConcurrentStreamsCard({ peaks }: ConcurrentStreamsCardProps) {
  return (
    <div className="card p-4">
      <h3 className="text-sm font-medium text-muted dark:text-muted-dark mb-3">
        Peak Concurrent Streams
      </h3>
      <div className="space-y-2">
        {ROWS.map(({ label, key }) => (
          <div key={key} className="flex items-center justify-between">
            <span className="text-sm">{label}</span>
            <span className="text-sm font-medium tabular-nums">{peaks[key]}</span>
          </div>
        ))}
      </div>
      {peaks.peak_at && (
        <div className="mt-3 pt-2 border-t border-border dark:border-border-dark text-xs text-muted dark:text-muted-dark">
          Peak at {new Date(peaks.peak_at).toLocaleString()}
        </div>
      )}
    </div>
  )
}
