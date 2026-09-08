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
      const parsed: unknown = JSON.parse(stored)
      if (Array.isArray(parsed) && parsed.every(id => typeof id === 'string')) {
        return parsed as string[]
      }
    } catch {}
  }
  return null
}

// Places id after the last column it canonically follows, so a config the user
// has reordered keeps the new column near its neighbours instead of at the head.
function insertInColumnOrder(visible: string[], id: string, orderOf: (id: string) => number): string[] {
  const target = orderOf(id)
  let insertIdx = 0
  for (let i = visible.length - 1; i >= 0; i--) {
    if (orderOf(visible[i]) < target) {
      insertIdx = i + 1
      break
    }
  }
  const next = [...visible]
  next.splice(insertIdx, 0, id)
  return next
}

function knownColumnsKey(storageKey: string): string {
  return `${storageKey}-known`
}

// A column flagged mergeIntoStoredConfigs is invisible to existing users
// otherwise: the config is persisted on every mount, so nearly everyone has a
// stored list that predates it. Merged in once, then recorded as known so it
// stays hidden if the user turns it off. Only flagged columns are ever merged —
// an absent column is otherwise indistinguishable from one the user hid.
function mergeNewDefaultColumns<T>(
  allColumns: ColumnDef<T>[],
  visible: string[],
  excludeSet: Set<string>,
  storageKey: string,
  orderOf: (id: string) => number,
): string[] {
  const known = new Set(loadStoredColumns(knownColumnsKey(storageKey)) ?? [])
  let result = visible
  for (const col of allColumns) {
    if (!col.mergeIntoStoredConfigs || !col.defaultVisible) continue
    if (known.has(col.id) || excludeSet.has(col.id)) continue
    result = insertInColumnOrder(result, col.id, orderOf)
  }
  return result
}

function loadInitialColumns<T>(allColumns: ColumnDef<T>[], excludeColumns: string[], storageKey: string): string[] {
  const columnIds = new Set(allColumns.map(c => c.id))
  const excludeSet = new Set(excludeColumns)
  const stored = loadStoredColumns(storageKey)
  if (stored) {
    const valid = stored.filter(id => columnIds.has(id) && !excludeSet.has(id))
    if (valid.length > 0) {
      const orderOf = (id: string) => {
        const idx = allColumns.findIndex(c => c.id === id)
        return idx === -1 ? allColumns.length : idx
      }
      return mergeNewDefaultColumns(allColumns, valid, excludeSet, storageKey, orderOf)
    }
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

    const known = new Set(loadStoredColumns(knownColumnsKey(storageKey)) ?? [])
    for (const col of allColumns) {
      if (!excludeSet.has(col.id)) known.add(col.id)
    }
    safeSetItem(knownColumnsKey(storageKey), JSON.stringify([...known]))
  }, [visibleColumns, excludeSet, allColumns, storageKey])

  const orderOf = useCallback(
    (id: string) => columnIndexMap.get(id) ?? allColumns.length,
    [allColumns.length, columnIndexMap]
  )

  const toggleColumn = useCallback((id: string) => {
    if (excludeSet.has(id)) return
    setVisibleColumnsState(prev => {
      if (prev.includes(id)) {
        return prev.filter(c => c !== id)
      }
      return insertInColumnOrder(prev, id, orderOf)
    })
  }, [excludeSet, orderOf])

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
