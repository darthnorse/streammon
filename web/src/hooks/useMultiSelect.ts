import { useState, useMemo, useCallback } from 'react'

interface UseMultiSelectResult<T> {
  selected: Set<number>
  setSelected: React.Dispatch<React.SetStateAction<Set<number>>>
  toggleSelect: (id: number) => void
  toggleSelectAll: () => void
  clearSelection: () => void
  allVisibleSelected: boolean
  selectedItems: T[]
}

/**
 * Hook for managing multi-select state in tables
 * @param items - Array of items to select from
 * @param getId - Function to extract the ID from an item
 * @param getFilteredItems - Optional function to get filtered items (for search scenarios)
 */
export function useMultiSelect<T>(
  items: T[],
  getId: (item: T) => number,
  getFilteredItems?: () => T[]
): UseMultiSelectResult<T> {
  const [selected, setSelected] = useState<Set<number>>(new Set())

  const visibleItems = getFilteredItems ? getFilteredItems() : items

  const allVisibleSelected = useMemo(
    () => visibleItems.length > 0 && visibleItems.every(item => selected.has(getId(item))),
    [visibleItems, selected, getId]
  )

  const toggleSelect = useCallback((id: number) => {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const toggleSelectAll = useCallback(() => {
    if (allVisibleSelected) {
      setSelected(prev => {
        const next = new Set(prev)
        visibleItems.forEach(item => next.delete(getId(item)))
        return next
      })
    } else {
      setSelected(prev => {
        const next = new Set(prev)
        visibleItems.forEach(item => next.add(getId(item)))
        return next
      })
    }
  }, [allVisibleSelected, visibleItems, getId])

  const clearSelection = useCallback(() => {
    setSelected(new Set())
  }, [])

  const selectedItems = useMemo(
    () => items.filter(item => selected.has(getId(item))),
    [items, selected, getId]
  )

  return {
    selected,
    setSelected,
    toggleSelect,
    toggleSelectAll,
    clearSelection,
    allVisibleSelected,
    selectedItems,
  }
}
