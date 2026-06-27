import type { ColumnDef } from '../lib/historyColumns'
import type { SortState } from './HistoryTable'
import type { LibraryItemDetail } from '../types'

interface LibraryItemsTableProps {
  items: LibraryItemDetail[]
  columns: ColumnDef<LibraryItemDetail>[]
  sort: SortState | null
  onSort: (s: SortState | null) => void
}

function nextSort(current: SortState | null, columnId: string): SortState | null {
  if (current?.columnId !== columnId) return { columnId, direction: 'desc' }
  if (current.direction === 'desc') return { columnId, direction: 'asc' }
  return null
}

export function LibraryItemsTable({ items, columns, sort, onSort }: LibraryItemsTableProps) {
  return (
    <div className="card overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-gray-50 dark:bg-white/5 border-b border-border dark:border-border-dark text-left">
            {columns.map(col => {
              const sortable = !!col.sortKey
              const active = sort?.columnId === col.id
              return (
                <th
                  key={col.id}
                  className={`px-4 py-2 font-medium text-muted dark:text-muted-dark ${col.responsiveClassName ?? ''} ${sortable ? 'cursor-pointer select-none' : ''}`}
                  onClick={sortable ? () => onSort(nextSort(sort, col.id)) : undefined}
                >
                  {col.label}
                  {active && <span className="text-accent ml-1">{sort!.direction === 'asc' ? '▲' : '▼'}</span>}
                </th>
              )
            })}
          </tr>
        </thead>
        <tbody>
          {items.map(item => (
            <tr
              key={item.id}
              data-testid={`library-row-${item.id}`}
              className={`border-b border-border dark:border-border-dark hover:bg-gray-50 dark:hover:bg-white/5 transition-colors ${item.plays === 0 ? 'border-l-2 border-l-amber-400/60' : ''}`}
            >
              {columns.map(col => (
                <td key={col.id} className={`px-4 py-2 ${col.className ?? ''} ${col.responsiveClassName ?? ''}`}>
                  {col.render(item)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
