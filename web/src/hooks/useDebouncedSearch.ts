import { useState, useEffect } from 'react'
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

  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchInput !== search) {
        setSearch(searchInput)
        onSearchChange?.()
      }
    }, SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [searchInput, search, onSearchChange])

  const resetSearch = () => {
    setSearchInput('')
    setSearch('')
  }

  return { searchInput, setSearchInput, search, resetSearch }
}
