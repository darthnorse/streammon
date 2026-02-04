import { Link } from 'react-router-dom'
import type { WatchHistoryEntry } from '../types'
import { formatDuration, formatDate, formatEpisode, parseSeasonFromTitle, formatLocation } from './format'
import { mediaTypeLabels } from './constants'
import { GeoIPPopover } from '../components/GeoIPPopover'

export interface ColumnDef {
  id: string
  label: string
  defaultVisible: boolean
  render: (entry: WatchHistoryEntry) => React.ReactNode
  sortValue?: (entry: WatchHistoryEntry) => string | number
  sortKey?: string // Backend API sort_by key for server-side sorting
  className?: string
  responsiveClassName?: string
}

interface EntryTitleProps {
  entry: WatchHistoryEntry
  onTitleClick?: (serverId: number, itemId: string) => void
}

export function EntryTitle({ entry, onTitleClick }: EntryTitleProps) {
  const canClickSeries = onTitleClick && entry.server_id && entry.grandparent_item_id
  const canClickItem = onTitleClick && entry.server_id && entry.item_id

  if (entry.media_type === 'episode' && entry.grandparent_title) {
    const season = entry.season_number ?? parseSeasonFromTitle(entry.parent_title)
    const episode = entry.episode_number
    const episodeInfo = formatEpisode(season, episode)
    const subtitle = episodeInfo ? `${episodeInfo} · ${entry.title}` : entry.title

    return (
      <div>
        <div
          className={`font-medium text-gray-900 dark:text-gray-50 truncate ${canClickSeries ? 'cursor-pointer hover:text-accent dark:hover:text-accent transition-colors' : ''}`}
          onClick={canClickSeries ? () => onTitleClick(entry.server_id, entry.grandparent_item_id!) : undefined}
        >
          {entry.grandparent_title}
        </div>
        <div
          className={`text-xs text-muted dark:text-muted-dark truncate ${canClickItem ? 'cursor-pointer hover:text-accent dark:hover:text-accent transition-colors' : ''}`}
          onClick={canClickItem ? () => onTitleClick(entry.server_id, entry.item_id!) : undefined}
        >
          {subtitle}
        </div>
      </div>
    )
  }
  return (
    <div
      className={`font-medium text-gray-900 dark:text-gray-50 truncate ${canClickItem ? 'cursor-pointer hover:text-accent dark:hover:text-accent transition-colors' : ''}`}
      onClick={canClickItem ? () => onTitleClick(entry.server_id, entry.item_id!) : undefined}
    >
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

export type TitleClickHandler = (serverId: number, itemId: string) => void

export function getHistoryColumns(onTitleClick?: TitleClickHandler): ColumnDef[] {
  return [
    {
      id: 'user',
      label: 'User',
      defaultVisible: true,
      render: (e) => <UserLink name={e.user_name} />,
      sortValue: (e) => e.user_name.toLowerCase(),
      sortKey: 'user',
    },
    {
      id: 'title',
      label: 'Title',
      defaultVisible: true,
      render: (e) => <EntryTitle entry={e} onTitleClick={onTitleClick} />,
      sortValue: (e) => (e.grandparent_title || e.title).toLowerCase(),
      sortKey: 'title',
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
      sortKey: 'type',
    },
    {
      id: 'player',
      label: 'Player',
      defaultVisible: true,
      render: (e) => e.player,
      sortValue: (e) => e.player.toLowerCase(),
      sortKey: 'player',
      className: 'text-muted dark:text-muted-dark',
      responsiveClassName: 'hidden lg:table-cell',
    },
    {
      id: 'platform',
      label: 'Platform',
      defaultVisible: true,
      render: (e) => e.platform,
      sortValue: (e) => e.platform.toLowerCase(),
      sortKey: 'platform',
      className: 'text-muted dark:text-muted-dark',
      responsiveClassName: 'hidden lg:table-cell',
    },
    {
      id: 'ip',
      label: 'IP Address',
      defaultVisible: true,
      render: (e) => e.ip_address ? (
        <GeoIPPopover ip={e.ip_address}>
          <span className="font-mono text-xs text-muted dark:text-muted-dark hover:text-accent dark:hover:text-accent transition-colors cursor-pointer">
            {e.ip_address}
          </span>
        </GeoIPPopover>
      ) : '—',
      className: 'font-mono text-xs',
      responsiveClassName: 'hidden lg:table-cell',
    },
    {
      id: 'location',
      label: 'Location',
      defaultVisible: true,
      render: (e) => formatLocation(e.city, e.country),
      sortValue: (e) => formatLocation(e.city, e.country),
      sortKey: 'location',
      className: 'text-muted dark:text-muted-dark',
      responsiveClassName: 'hidden xl:table-cell',
    },
    {
      id: 'isp',
      label: 'ISP',
      defaultVisible: true,
      render: (e) => e.isp || '—',
      className: 'text-muted dark:text-muted-dark text-xs',
      responsiveClassName: 'hidden xl:table-cell',
    },
    {
      id: 'duration',
      label: 'Duration',
      defaultVisible: true,
      render: (e) => formatDuration(e.watched_ms),
      sortValue: (e) => e.watched_ms,
      sortKey: 'watched',
      className: 'font-mono text-xs',
    },
    {
      id: 'date',
      label: 'Date',
      defaultVisible: true,
      render: (e) => formatDate(e.started_at),
      sortValue: (e) => new Date(e.started_at).getTime(),
      sortKey: 'started_at',
      className: 'text-muted dark:text-muted-dark whitespace-nowrap',
    },
  ]
}

// Default columns for backwards compatibility
export const HISTORY_COLUMNS = getHistoryColumns()

export function getDefaultVisibleColumns(columns: ColumnDef[], excludeColumns: string[] = []): string[] {
  return columns
    .filter(c => c.defaultVisible && !excludeColumns.includes(c.id))
    .map(c => c.id)
}
