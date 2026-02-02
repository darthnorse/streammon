import { Link } from 'react-router-dom'
import type { WatchHistoryEntry, MediaType } from '../types'

interface HistoryTableProps {
  entries: WatchHistoryEntry[]
  hideUser?: boolean
}

const mediaTypeLabels: Record<MediaType, string> = {
  movie: 'Movie',
  episode: 'TV',
  livetv: 'Live TV',
  track: 'Music',
  audiobook: 'Audiobook',
  book: 'Book',
}

function formatDuration(ms: number): string {
  const totalMin = Math.floor(ms / 60000)
  const h = Math.floor(totalMin / 60)
  const m = totalMin % 60
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

function EntryTitle({ entry }: { entry: WatchHistoryEntry }) {
  if (entry.media_type === 'episode' && entry.grandparent_title) {
    return (
      <div>
        <div className="font-medium text-gray-900 dark:text-gray-50 truncate">
          {entry.grandparent_title}
        </div>
        <div className="text-xs text-muted dark:text-muted-dark truncate">
          {entry.parent_title} &middot; {entry.title}
        </div>
      </div>
    )
  }
  return (
    <div className="font-medium text-gray-900 dark:text-gray-50 truncate">
      {entry.title}
    </div>
  )
}

function HistoryCard({ entry, hideUser }: { entry: WatchHistoryEntry; hideUser?: boolean }) {
  return (
    <div className="card p-4" data-testid="history-row">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          {!hideUser && (
            <Link
              to={`/users/${encodeURIComponent(entry.user_name)}`}
              className="text-sm font-medium text-accent-dim dark:text-accent hover:underline"
            >
              {entry.user_name}
            </Link>
          )}
          <div className="mt-0.5">
            <EntryTitle entry={entry} />
          </div>
        </div>
        <span className="badge badge-muted shrink-0">
          {mediaTypeLabels[entry.media_type]}
        </span>
      </div>
      <div className="flex items-center gap-3 mt-2 text-xs text-muted dark:text-muted-dark">
        <span>{formatDate(entry.started_at)}</span>
        <span>&middot;</span>
        <span>{formatDuration(entry.watched_ms)}</span>
        <span className="hidden sm:inline">&middot;</span>
        <span className="hidden sm:inline">{entry.player}</span>
      </div>
    </div>
  )
}

export function HistoryTable({ entries, hideUser }: HistoryTableProps) {
  if (entries.length === 0) {
    return (
      <div className="card p-12 text-center">
        <div className="text-4xl mb-3 opacity-30">☰</div>
        <p className="text-muted dark:text-muted-dark">No history yet</p>
      </div>
    )
  }

  return (
    <>
      {/* Mobile: card list */}
      <div className="md:hidden space-y-3">
        {entries.map(entry => (
          <HistoryCard key={entry.id} entry={entry} hideUser={hideUser} />
        ))}
      </div>

      {/* Desktop: table */}
      <div className="hidden md:block card overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border dark:border-border-dark text-left text-xs
                          text-muted dark:text-muted-dark uppercase tracking-wider">
              {!hideUser && <th className="px-4 py-3 font-medium">User</th>}
              <th className="px-4 py-3 font-medium">Title</th>
              <th className="px-4 py-3 font-medium">Type</th>
              <th className="px-4 py-3 font-medium hidden lg:table-cell">Player</th>
              <th className="px-4 py-3 font-medium hidden lg:table-cell">Platform</th>
              <th className="px-4 py-3 font-medium">Duration</th>
              <th className="px-4 py-3 font-medium">Date</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border dark:divide-border-dark">
            {entries.map(entry => (
              <tr key={entry.id} data-testid="history-row"
                  className="hover:bg-gray-50 dark:hover:bg-white/[0.02] transition-colors">
                {!hideUser && (
                  <td className="px-4 py-3">
                    <Link
                      to={`/users/${encodeURIComponent(entry.user_name)}`}
                      className="font-medium text-accent-dim dark:text-accent hover:underline"
                    >
                      {entry.user_name}
                    </Link>
                  </td>
                )}
                <td className="px-4 py-3 max-w-[300px]">
                  <EntryTitle entry={entry} />
                </td>
                <td className="px-4 py-3">
                  <span className="badge badge-muted">
                    {mediaTypeLabels[entry.media_type]}
                  </span>
                </td>
                <td className="px-4 py-3 hidden lg:table-cell text-muted dark:text-muted-dark">
                  {entry.player}
                </td>
                <td className="px-4 py-3 hidden lg:table-cell text-muted dark:text-muted-dark">
                  {entry.platform}
                </td>
                <td className="px-4 py-3 font-mono text-xs">
                  {formatDuration(entry.watched_ms)}
                </td>
                <td className="px-4 py-3 text-muted dark:text-muted-dark whitespace-nowrap">
                  {formatDate(entry.started_at)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}
