import { useSSE } from '../hooks/useSSE'
import { StreamCard } from '../components/StreamCard'
import { EmptyState } from '../components/EmptyState'
import { RecentMedia } from '../components/RecentMedia'
import { WatchStats } from '../components/WatchStats'
import { formatBitrate } from '../lib/format'

export function Dashboard() {
  const { sessions, connected } = useSSE('/api/dashboard/sse')

  const totalBandwidth = sessions.reduce((sum, s) => sum + (s.bandwidth ?? 0), 0)

  return (
    <div>
      <div className="flex items-center justify-end mb-6">
        <div className="flex items-center gap-4">
          {sessions.length > 0 && (
            <span className="text-sm text-muted dark:text-muted-dark">
              {sessions.length} active stream{sessions.length !== 1 ? 's' : ''}{totalBandwidth > 0 ? ` (${formatBitrate(totalBandwidth)})` : ''}
            </span>
          )}
          <div className="flex items-center gap-2">
            <span
              aria-hidden="true"
              className={`w-2 h-2 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`}
            />
            <span className="text-xs text-muted dark:text-muted-dark font-mono">
              {connected ? 'Live' : 'Reconnecting'}
            </span>
          </div>
        </div>
      </div>

      {sessions.length === 0 ? (
        <EmptyState icon="▣" title="No active streams" description="Streams will appear here when someone starts watching" />
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5 max-w-5xl">
          {[...sessions].sort((a, b) => `${a.server_id}:${a.session_id}`.localeCompare(`${b.server_id}:${b.session_id}`)).map(stream => (
            <StreamCard key={`${stream.server_id}:${stream.session_id}`} stream={stream} />
          ))}
        </div>
      )}
      <div className="mt-8">
        <RecentMedia />
      </div>
      <div className="mt-8">
        <WatchStats />
      </div>
    </div>
  )
}
