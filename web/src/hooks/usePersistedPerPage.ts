import { useState, useCallback } from 'react'
import { PER_PAGE, PER_PAGE_OPTIONS } from '../lib/constants'
import { safeGetItem, safeSetItem } from '../lib/storage'

const STORAGE_KEY = 'streammon:per_page'

function getStoredPerPage(): number {
  const stored = safeGetItem(STORAGE_KEY)
  if (stored) {
    const parsed = parseInt(stored, 10)
    if ((PER_PAGE_OPTIONS as readonly number[]).includes(parsed)) {
      return parsed
    }
  }
  return PER_PAGE
}

export function usePersistedPerPage(): [number, (value: number) => void] {
  const [perPage, setPerPageState] = useState(getStoredPerPage)

  const setPerPage = useCallback((value: number) => {
    setPerPageState(value)
    safeSetItem(STORAGE_KEY, String(value))
  }, [])

  return [perPage, setPerPage]
}
