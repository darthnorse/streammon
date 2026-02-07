import { useState, useCallback } from 'react'
import { PER_PAGE } from '../lib/constants'

const STORAGE_KEY = 'streammon:per_page'

function getStoredPerPage(): number {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      const parsed = parseInt(stored, 10)
      if ([10, 25, 50].includes(parsed)) {
        return parsed
      }
    }
  } catch {
    // localStorage not available
  }
  return PER_PAGE
}

export function usePersistedPerPage(): [number, (value: number) => void] {
  const [perPage, setPerPageState] = useState(getStoredPerPage)

  const setPerPage = useCallback((value: number) => {
    setPerPageState(value)
    try {
      localStorage.setItem(STORAGE_KEY, String(value))
    } catch {
      // localStorage not available
    }
  }, [])

  return [perPage, setPerPage]
}
