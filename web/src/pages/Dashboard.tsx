import { useSSE } from '../hooks/useSSE'
import { StreamCard } from '../components/StreamCard'
import { DailyChart } from '../components/DailyChart'
import { EmptyState } from '../components/EmptyState'

export function Dashboard() {
  const { sessions, connected } = useSSE('/api/dashboard/sse')

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">Dashboard</h1>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            {sessions.length > 0
              ? `${sessions.length} active stream${sessions.length !== 1 ? 's' : ''}`
              : 'Monitoring streams'}
          </p>
        </div>
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

      {sessions.length === 0 ? (
        <EmptyState icon="▣" title="No active streams" description="Streams will appear here when someone starts watching" />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {sessions.map(stream => (
            <StreamCard key={`${stream.server_id}:${stream.session_id}`} stream={stream} />
          ))}
        </div>
      )}
      <div className="mt-6">
        <DailyChart />
      </div>
    </div>
  )
}
