import { useState, useCallback, useEffect, useMemo, useRef } from 'react'
import type { ColumnDef } from '../lib/historyColumns'
import { getDefaultVisibleColumns } from '../lib/historyColumns'

export interface ColumnConfig {
  visibleColumns: string[]
  toggleColumn: (id: string) => void
  moveColumn: (id: string, direction: 'up' | 'down') => void
  resetToDefaults: () => void
}

function safeGetItem(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function safeSetItem(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch {}
}

function loadStoredColumns(storageKey: string): string[] | null {
  const stored = safeGetItem(storageKey)
  if (stored) {
    try {
      return JSON.parse(stored) as string[]
    } catch {}
  }
  return null
}

function loadInitialColumns<T>(allColumns: ColumnDef<T>[], excludeColumns: string[], storageKey: string): string[] {
  const columnIds = new Set(allColumns.map(c => c.id))
  const excludeSet = new Set(excludeColumns)
  const stored = loadStoredColumns(storageKey)
  if (stored) {
    const valid = stored.filter(id => columnIds.has(id) && !excludeSet.has(id))
    if (valid.length > 0) return valid
  }
  return getDefaultVisibleColumns(allColumns, excludeColumns)
}

export function useColumnConfig<T>(
  allColumns: ColumnDef<T>[],
  excludeColumns: string[] = [],
  storageKey: string = 'history-columns',
): ColumnConfig {
  const excludeSet = useMemo(() => new Set(excludeColumns), [excludeColumns])
  const columnIndexMap = useMemo(
    () => new Map(allColumns.map((c, i) => [c.id, i])),
    [allColumns]
  )

  const isInitialMount = useRef(true)

  const getDefaults = useCallback(
    () => getDefaultVisibleColumns(allColumns, excludeColumns),
    [allColumns, excludeColumns]
  )

  const [visibleColumns, setVisibleColumnsState] = useState<string[]>(
    () => loadInitialColumns(allColumns, excludeColumns, storageKey)
  )

  // Re-filter when excludeColumns changes (but not on initial mount)
  useEffect(() => {
    if (isInitialMount.current) {
      isInitialMount.current = false
      return
    }
    setVisibleColumnsState(prev => {
      const filtered = prev.filter(id => !excludeSet.has(id))
      if (filtered.length === prev.length) return prev
      return filtered.length > 0 ? filtered : getDefaults()
    })
  }, [excludeSet, getDefaults])

  // When saving, preserve excluded columns at their original positions
  // This prevents losing column preferences when viewing contexts that exclude certain columns
  useEffect(() => {
    const stored = loadStoredColumns(storageKey) ?? allColumns.filter(c => c.defaultVisible).map(c => c.id)

    // Build result by keeping excluded columns at their stored positions
    // and filling non-excluded slots with visibleColumns in order
    const result: string[] = []
    let visibleIndex = 0

    for (const storedId of stored) {
      if (excludeSet.has(storedId)) {
        // Excluded column - preserve its position
        result.push(storedId)
      } else if (visibleIndex < visibleColumns.length) {
        // Non-excluded slot - take next from visibleColumns
        result.push(visibleColumns[visibleIndex++])
      }
    }

    // Append any remaining visible columns (new ones not in stored)
    while (visibleIndex < visibleColumns.length) {
      result.push(visibleColumns[visibleIndex++])
    }

    safeSetItem(storageKey, JSON.stringify(result))
  }, [visibleColumns, excludeSet, allColumns, storageKey])

  const toggleColumn = useCallback((id: string) => {
    if (excludeSet.has(id)) return
    setVisibleColumnsState(prev => {
      if (prev.includes(id)) {
        return prev.filter(c => c !== id)
      }
      // Insert at position based on original column order
      const colIndex = columnIndexMap.get(id) ?? allColumns.length
      const insertIdx = prev.findIndex(existingId => {
        const existingIndex = columnIndexMap.get(existingId) ?? allColumns.length
        return existingIndex > colIndex
      })
      const newVisible = [...prev]
      newVisible.splice(insertIdx === -1 ? newVisible.length : insertIdx, 0, id)
      return newVisible
    })
  }, [allColumns.length, columnIndexMap, excludeSet])

  const moveColumn = useCallback((id: string, direction: 'up' | 'down') => {
    setVisibleColumnsState(prev => {
      const idx = prev.indexOf(id)
      if (idx === -1) return prev
      const newIdx = direction === 'up' ? idx - 1 : idx + 1
      if (newIdx < 0 || newIdx >= prev.length) return prev
      const newArr = [...prev]
      ;[newArr[idx], newArr[newIdx]] = [newArr[newIdx], newArr[idx]]
      return newArr
    })
  }, [])

  const resetToDefaults = useCallback(() => {
    setVisibleColumnsState(getDefaults())
  }, [getDefaults])

  return {
    visibleColumns,
    toggleColumn,
    moveColumn,
    resetToDefaults,
  }
}
