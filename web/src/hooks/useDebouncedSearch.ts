import { useState, useEffect, useCallback, useRef } from 'react'
import { SEARCH_DEBOUNCE_MS } from '../lib/constants'

interface UseDebouncedSearchResult {
  searchInput: string
  setSearchInput: (value: string) => void
  search: string
  resetSearch: () => void
}

export function useDebouncedSearch(onSearchChange?: () => void): UseDebouncedSearchResult {
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  const onChangeRef = useRef(onSearchChange)

  // Keep ref updated without triggering effect
  useEffect(() => {
    onChangeRef.current = onSearchChange
  }, [onSearchChange])

  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchInput !== search) {
        setSearch(searchInput)
        onChangeRef.current?.()
      }
    }, SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [searchInput, search])

  const resetSearch = useCallback(() => {
    setSearchInput('')
    setSearch('')
  }, [])

  return { searchInput, setSearchInput, search, resetSearch }
}
