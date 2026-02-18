import { useState, useRef, useCallback } from 'react'
import { useClickOutside } from '../hooks/useClickOutside'
import type { ColumnDef } from '../lib/historyColumns'

interface ColumnSettingsProps {
  columns: ColumnDef[]
  visibleColumns: string[]
  excludeColumns?: string[]
  onToggle: (id: string) => void
  onMove: (id: string, direction: 'up' | 'down') => void
  onReset: () => void
}

export function ColumnSettings({
  columns,
  visibleColumns,
  excludeColumns = [],
  onToggle,
  onMove,
  onReset,
}: ColumnSettingsProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const close = useCallback(() => setOpen(false), [])
  useClickOutside(ref, open, close)

  const availableColumns = columns.filter(c => !excludeColumns.includes(c.id))

  return (
    <div ref={ref} className="relative inline-block">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-white/5 text-muted dark:text-muted-dark transition-colors"
        aria-label="Column settings"
        aria-expanded={open}
        title="Configure columns"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 20 20"
          fill="currentColor"
          className="w-4 h-4"
        >
          <path
            fillRule="evenodd"
            d="M7.84 1.804A1 1 0 0 1 8.82 1h2.36a1 1 0 0 1 .98.804l.295 1.473c.497.144.97.342 1.416.587l1.25-.834a1 1 0 0 1 1.262.125l1.67 1.671a1 1 0 0 1 .125 1.262l-.834 1.25c.245.447.443.919.587 1.416l1.473.294a1 1 0 0 1 .804.98v2.361a1 1 0 0 1-.804.98l-1.473.295a6.95 6.95 0 0 1-.587 1.416l.834 1.25a1 1 0 0 1-.125 1.262l-1.67 1.67a1 1 0 0 1-1.262.126l-1.25-.834a6.953 6.953 0 0 1-1.416.587l-.294 1.473a1 1 0 0 1-.98.804H8.82a1 1 0 0 1-.98-.804l-.295-1.473a6.957 6.957 0 0 1-1.416-.587l-1.25.834a1 1 0 0 1-1.262-.125l-1.67-1.67a1 1 0 0 1-.126-1.262l.834-1.25a6.957 6.957 0 0 1-.587-1.416L.804 11.18a1 1 0 0 1-.804-.98V7.819a1 1 0 0 1 .804-.98l1.473-.294c.144-.497.342-.97.587-1.416l-.834-1.25a1 1 0 0 1 .125-1.262l1.67-1.67a1 1 0 0 1 1.262-.126l1.25.834a6.95 6.95 0 0 1 1.416-.587L7.84 1.804ZM10 13a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z"
            clipRule="evenodd"
          />
        </svg>
      </button>
      {open && (
        <div className="absolute z-50 top-full mt-1 right-0 w-56 card p-2 shadow-lg">
          <div className="text-xs font-medium text-muted dark:text-muted-dark uppercase tracking-wider px-2 py-1">
            Show columns
          </div>
          <div className="space-y-0.5 mt-1">
            {availableColumns.map(col => {
              const visibleIdx = visibleColumns.indexOf(col.id)
              const isVisible = visibleIdx !== -1
              const canMoveUp = isVisible && visibleIdx > 0
              const canMoveDown = isVisible && visibleIdx < visibleColumns.length - 1

              return (
                <div
                  key={col.id}
                  className="flex items-center gap-1 px-2 py-1.5 rounded hover:bg-gray-50 dark:hover:bg-white/5"
                >
                  <label className="flex items-center gap-2 flex-1 cursor-pointer text-sm">
                    <input
                      type="checkbox"
                      checked={isVisible}
                      onChange={() => onToggle(col.id)}
                      className="rounded border-gray-300 dark:border-gray-600 text-accent focus:ring-accent"
                    />
                    {col.label}
                  </label>
                  {isVisible && (
                    <div className="flex gap-0.5">
                      <button
                        type="button"
                        onClick={() => onMove(col.id, 'up')}
                        disabled={!canMoveUp}
                        className="p-0.5 text-muted dark:text-muted-dark hover:text-gray-700 dark:hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
                        aria-label={`Move ${col.label} up`}
                      >
                        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" className="w-3.5 h-3.5">
                          <path fillRule="evenodd" d="M8 14a.75.75 0 0 1-.75-.75V4.56L4.03 7.78a.75.75 0 0 1-1.06-1.06l4.5-4.5a.75.75 0 0 1 1.06 0l4.5 4.5a.75.75 0 0 1-1.06 1.06L8.75 4.56v8.69A.75.75 0 0 1 8 14Z" clipRule="evenodd" />
                        </svg>
                      </button>
                      <button
                        type="button"
                        onClick={() => onMove(col.id, 'down')}
                        disabled={!canMoveDown}
                        className="p-0.5 text-muted dark:text-muted-dark hover:text-gray-700 dark:hover:text-gray-300 disabled:opacity-30 disabled:cursor-not-allowed"
                        aria-label={`Move ${col.label} down`}
                      >
                        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" className="w-3.5 h-3.5">
                          <path fillRule="evenodd" d="M8 2a.75.75 0 0 1 .75.75v8.69l3.22-3.22a.75.75 0 1 1 1.06 1.06l-4.5 4.5a.75.75 0 0 1-1.06 0l-4.5-4.5a.75.75 0 0 1 1.06-1.06l3.22 3.22V2.75A.75.75 0 0 1 8 2Z" clipRule="evenodd" />
                        </svg>
                      </button>
                    </div>
                  )}
                </div>
              )
            })}
          </div>
          <div className="border-t border-border dark:border-border-dark mt-2 pt-2 px-2">
            <button
              type="button"
              onClick={() => {
                onReset()
                setOpen(false)
              }}
              className="text-xs text-accent-dim dark:text-accent hover:underline"
            >
              Reset to defaults
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
