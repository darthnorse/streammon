import { useState, useMemo } from 'react'
import { Link } from 'react-router-dom'
import type { WatchHistoryEntry } from '../types'
import { formatDuration, formatDate, formatLocation } from '../lib/format'
import { mediaTypeLabels } from '../lib/constants'
import { HISTORY_COLUMNS, EntryTitle } from '../lib/historyColumns'
import { useColumnConfig } from '../hooks/useColumnConfig'
import { ColumnSettings } from './ColumnSettings'

interface HistoryTableProps {
  entries: WatchHistoryEntry[]
  hideUser?: boolean
}

type SortDirection = 'asc' | 'desc'

interface SortState {
  columnId: string
  direction: SortDirection
}

function SortIcon({ direction, active }: { direction: SortDirection; active: boolean }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 16 16"
      fill="currentColor"
      className={`w-3 h-3 ml-1 inline-block transition-opacity ${active ? 'opacity-100' : 'opacity-0 group-hover:opacity-40'}`}
    >
      {direction === 'asc' ? (
        <path fillRule="evenodd" d="M8 14a.75.75 0 0 1-.75-.75V4.56L4.03 7.78a.75.75 0 0 1-1.06-1.06l4.5-4.5a.75.75 0 0 1 1.06 0l4.5 4.5a.75.75 0 0 1-1.06 1.06L8.75 4.56v8.69A.75.75 0 0 1 8 14Z" clipRule="evenodd" />
      ) : (
        <path fillRule="evenodd" d="M8 2a.75.75 0 0 1 .75.75v8.69l3.22-3.22a.75.75 0 1 1 1.06 1.06l-4.5 4.5a.75.75 0 0 1-1.06 0l-4.5-4.5a.75.75 0 0 1 1.06-1.06l3.22 3.22V2.75A.75.75 0 0 1 8 2Z" clipRule="evenodd" />
      )}
    </svg>
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
      {(entry.city || entry.country) && (
        <div className="mt-1 text-xs text-muted dark:text-muted-dark">
          {formatLocation(entry.city, entry.country)}
          {entry.isp && <span className="ml-2 opacity-75">({entry.isp})</span>}
        </div>
      )}
    </div>
  )
}

const EMPTY_EXCLUDE: string[] = []
const USER_EXCLUDE = ['user']

export function HistoryTable({ entries, hideUser }: HistoryTableProps) {
  const excludeColumns = hideUser ? USER_EXCLUDE : EMPTY_EXCLUDE
  const { visibleColumns, toggleColumn, moveColumn, resetToDefaults } = useColumnConfig(
    HISTORY_COLUMNS,
    excludeColumns
  )
  const [sort, setSort] = useState<SortState | null>(null)

  const orderedColumns = useMemo(() =>
    visibleColumns
      .map(id => HISTORY_COLUMNS.find(c => c.id === id))
      .filter((c): c is typeof HISTORY_COLUMNS[number] => c !== undefined),
    [visibleColumns]
  )

  const sortedEntries = useMemo(() => {
    if (!sort) return entries
    const column = HISTORY_COLUMNS.find(c => c.id === sort.columnId)
    if (!column?.sortValue) return entries

    return [...entries].sort((a, b) => {
      const aVal = column.sortValue!(a)
      const bVal = column.sortValue!(b)
      let cmp = 0
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        cmp = aVal - bVal
      } else {
        cmp = String(aVal).localeCompare(String(bVal))
      }
      return sort.direction === 'asc' ? cmp : -cmp
    })
  }, [entries, sort])

  function handleSort(columnId: string) {
    const column = HISTORY_COLUMNS.find(c => c.id === columnId)
    if (!column?.sortValue) return

    setSort(prev => {
      if (prev?.columnId === columnId) {
        if (prev.direction === 'asc') {
          return { columnId, direction: 'desc' }
        }
        return null // Third click removes sort
      }
      return { columnId, direction: 'asc' }
    })
  }

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
      <div className="md:hidden space-y-3">
        {sortedEntries.map(entry => (
          <HistoryCard key={entry.id} entry={entry} hideUser={hideUser} />
        ))}
      </div>

      <div className="hidden md:block card overflow-hidden">
        <div className="flex items-center justify-between px-4 py-2 border-b border-border dark:border-border-dark">
          <span className="text-xs text-muted dark:text-muted-dark uppercase tracking-wider">
            {entries.length} {entries.length === 1 ? 'entry' : 'entries'}
          </span>
          <ColumnSettings
            columns={HISTORY_COLUMNS}
            visibleColumns={visibleColumns}
            excludeColumns={excludeColumns}
            onToggle={toggleColumn}
            onMove={moveColumn}
            onReset={resetToDefaults}
          />
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border dark:border-border-dark text-left text-xs
                            text-muted dark:text-muted-dark uppercase tracking-wider">
                {orderedColumns.map(col => {
                  const isSortable = !!col.sortValue
                  const isActive = sort?.columnId === col.id
                  return (
                    <th
                      key={col.id}
                      className={`px-4 py-3 font-medium ${col.responsiveClassName || ''} ${isSortable ? 'cursor-pointer select-none group' : ''}`}
                      onClick={isSortable ? () => handleSort(col.id) : undefined}
                    >
                      <span className="inline-flex items-center">
                        {col.label}
                        {isSortable && (
                          <SortIcon
                            direction={isActive ? sort!.direction : 'asc'}
                            active={isActive}
                          />
                        )}
                      </span>
                    </th>
                  )
                })}
              </tr>
            </thead>
            <tbody className="divide-y divide-border dark:divide-border-dark">
              {sortedEntries.map(entry => (
                <tr key={entry.id} data-testid="history-row"
                    className="hover:bg-gray-50 dark:hover:bg-white/[0.02] transition-colors">
                  {orderedColumns.map(col => (
                    <td
                      key={col.id}
                      className={`px-4 py-3 ${col.className || ''} ${col.responsiveClassName || ''}`}
                    >
                      {col.render(entry)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </>
  )
}
