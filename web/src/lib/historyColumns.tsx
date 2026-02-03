import { Link } from 'react-router-dom'
import type { WatchHistoryEntry } from '../types'
import { formatDuration, formatDate, formatEpisode, parseSeasonFromTitle, formatLocation } from './format'
import { mediaTypeLabels } from './constants'

export interface ColumnDef {
  id: string
  label: string
  defaultVisible: boolean
  render: (entry: WatchHistoryEntry) => React.ReactNode
  sortValue?: (entry: WatchHistoryEntry) => string | number
  className?: string
  responsiveClassName?: string
}

export function EntryTitle({ entry }: { entry: WatchHistoryEntry }) {
  if (entry.media_type === 'episode' && entry.grandparent_title) {
    const season = entry.season_number ?? parseSeasonFromTitle(entry.parent_title)
    const episode = entry.episode_number
    const episodeInfo = formatEpisode(season, episode)
    const subtitle = episodeInfo ? `${episodeInfo} · ${entry.title}` : entry.title

    return (
      <div>
        <div className="font-medium text-gray-900 dark:text-gray-50 truncate">
          {entry.grandparent_title}
        </div>
        <div className="text-xs text-muted dark:text-muted-dark truncate">
          {subtitle}
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

function UserLink({ name }: { name: string }) {
  return (
    <Link
      to={`/users/${encodeURIComponent(name)}`}
      className="font-medium text-accent-dim dark:text-accent hover:underline"
    >
      {name}
    </Link>
  )
}

export const HISTORY_COLUMNS: ColumnDef[] = [
  {
    id: 'user',
    label: 'User',
    defaultVisible: true,
    render: (e) => <UserLink name={e.user_name} />,
    sortValue: (e) => e.user_name.toLowerCase(),
  },
  {
    id: 'title',
    label: 'Title',
    defaultVisible: true,
    render: (e) => <EntryTitle entry={e} />,
    sortValue: (e) => (e.grandparent_title || e.title).toLowerCase(),
    className: 'max-w-[300px]',
  },
  {
    id: 'type',
    label: 'Type',
    defaultVisible: true,
    render: (e) => (
      <span className="badge badge-muted">
        {mediaTypeLabels[e.media_type]}
      </span>
    ),
    sortValue: (e) => e.media_type,
  },
  {
    id: 'player',
    label: 'Player',
    defaultVisible: true,
    render: (e) => e.player,
    sortValue: (e) => e.player.toLowerCase(),
    className: 'text-muted dark:text-muted-dark',
    responsiveClassName: 'hidden lg:table-cell',
  },
  {
    id: 'platform',
    label: 'Platform',
    defaultVisible: true,
    render: (e) => e.platform,
    sortValue: (e) => e.platform.toLowerCase(),
    className: 'text-muted dark:text-muted-dark',
    responsiveClassName: 'hidden lg:table-cell',
  },
  {
    id: 'location',
    label: 'Location',
    defaultVisible: true,
    render: (e) => formatLocation(e.city, e.country),
    sortValue: (e) => formatLocation(e.city, e.country),
    className: 'text-muted dark:text-muted-dark',
    responsiveClassName: 'hidden xl:table-cell',
  },
  {
    id: 'isp',
    label: 'ISP',
    defaultVisible: true,
    render: (e) => e.isp || '—',
    sortValue: (e) => (e.isp || '').toLowerCase(),
    className: 'text-muted dark:text-muted-dark text-xs',
    responsiveClassName: 'hidden xl:table-cell',
  },
  {
    id: 'duration',
    label: 'Duration',
    defaultVisible: true,
    render: (e) => formatDuration(e.watched_ms),
    sortValue: (e) => e.watched_ms,
    className: 'font-mono text-xs',
  },
  {
    id: 'date',
    label: 'Date',
    defaultVisible: true,
    render: (e) => formatDate(e.started_at),
    sortValue: (e) => new Date(e.started_at).getTime(),
    className: 'text-muted dark:text-muted-dark whitespace-nowrap',
  },
]

export function getDefaultVisibleColumns(columns: ColumnDef[], excludeColumns: string[] = []): string[] {
  return columns
    .filter(c => c.defaultVisible && !excludeColumns.includes(c.id))
    .map(c => c.id)
}
