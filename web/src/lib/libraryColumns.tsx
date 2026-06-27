import type { ColumnDef } from './historyColumns'
import type { LibraryItemDetail, TitleClickHandler, LibraryType } from '../types'
import { CLICKABLE_TITLE_CLASS, getMediaLabel } from './constants'
import { formatSize, formatHours } from './format'

export const LIBRARY_COLUMN_STORAGE_KEY = 'library-columns'

function StatusBadges({ row }: { row: LibraryItemDetail }) {
  return (
    <span className="flex flex-wrap gap-1">
      {row.tmdb_status && <span className="badge badge-muted">{row.tmdb_status}</span>}
      {row.flagged_for_deletion && <span className="badge badge-warn">Flagged</span>}
      {row.protected && <span className="badge badge-muted">Protected</span>}
    </span>
  )
}

export function getLibraryColumns(
  onTitleClick?: TitleClickHandler,
  libraryType?: LibraryType,
): ColumnDef<LibraryItemDetail>[] {
  const cols: ColumnDef<LibraryItemDetail>[] = [
    {
      id: 'added', label: 'Added At', defaultVisible: true, sortKey: 'added_at',
      render: (r) => new Date(r.added_at).toLocaleDateString(),
      className: 'text-muted dark:text-muted-dark whitespace-nowrap',
    },
    {
      id: 'title', label: 'Title', defaultVisible: true, sortKey: 'title',
      className: 'max-w-[320px]',
      render: (r) => {
        const clickable = onTitleClick && r.server_id && r.item_id
        return (
          <div className="flex items-center gap-2 min-w-0">
            <span className="badge badge-muted shrink-0">{getMediaLabel(r.media_type)}</span>
            <span
              className={`font-medium text-gray-900 dark:text-gray-50 truncate ${clickable ? CLICKABLE_TITLE_CLASS : ''}`}
              onClick={clickable ? () => onTitleClick!(r.server_id, r.item_id) : undefined}
            >
              {r.title}
            </span>
          </div>
        )
      },
    },
    {
      id: 'last_played', label: 'Last Played', defaultVisible: true, sortKey: 'last_played',
      render: (r) => (r.last_played_at ? new Date(r.last_played_at).toLocaleString() : 'Never'),
      className: 'text-muted dark:text-muted-dark whitespace-nowrap',
    },
    {
      id: 'plays', label: 'Plays', defaultVisible: true, sortKey: 'plays',
      render: (r) => r.plays,
    },
    {
      id: 'status', label: 'Status', defaultVisible: true,
      render: (r) => <StatusBadges row={r} />,
    },
    {
      id: 'total_time', label: 'Total Time', defaultVisible: false, sortKey: 'total_time',
      render: (r) => formatHours(r.total_hours),
      className: 'font-mono text-xs',
    },
    {
      id: 'viewers', label: 'Viewers', defaultVisible: false, sortKey: 'viewers',
      render: (r) => (r.last_viewer ? `${r.unique_viewers} (last: ${r.last_viewer})` : String(r.unique_viewers)),
      className: 'text-muted dark:text-muted-dark',
    },
    {
      id: 'episodes', label: 'Episodes', defaultVisible: false,
      render: (r) => `${r.episodes_watched ?? 0} / ${r.episode_count ?? 0}`,
      className: 'font-mono text-xs',
    },
    {
      id: 'size', label: 'Size', defaultVisible: false, sortKey: 'size',
      render: (r) => formatSize(r.file_size),
      className: 'font-mono text-xs',
    },
    {
      id: 'resolution', label: 'Resolution', defaultVisible: false,
      render: (r) => r.video_resolution || '—',
      className: 'text-muted dark:text-muted-dark',
    },
  ]
  if (libraryType !== 'show') {
    return cols.filter(c => c.id !== 'episodes')
  }
  return cols
}
