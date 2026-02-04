import type { GeoResult } from '../../types'

export interface LocationColumn {
  header: string
  accessor: (loc: GeoResult, idx: number) => React.ReactNode
  className?: string
}

interface LocationTableProps {
  locations: GeoResult[]
  columns: LocationColumn[]
  rowKey: (loc: GeoResult, idx: number) => string
  className?: string
}

export function LocationTable({ locations, columns, rowKey, className = '' }: LocationTableProps) {
  return (
    <div className={`overflow-x-auto ${className}`}>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border dark:border-border-dark text-left text-muted dark:text-muted-dark">
            {columns.map((col, idx) => (
              <th
                key={col.header}
                className={`py-2 font-medium ${idx < columns.length - 1 ? 'pr-4' : ''}`}
              >
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {locations.map((loc, idx) => (
            <tr key={rowKey(loc, idx)} className="border-b border-border/50 dark:border-border-dark/50">
              {columns.map((col, colIdx) => (
                <td
                  key={col.header}
                  className={`py-2 ${colIdx < columns.length - 1 ? 'pr-4' : ''} ${col.className || ''}`}
                >
                  {col.accessor(loc, idx)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
