import { useEffect } from 'react'
import { useFetch } from './useFetch'
import type { OverseerrRequestCount } from '../types'

export const REQUEST_CHANGED_EVENT = 'overseerr-request-changed'

export function dispatchRequestChanged() {
  window.dispatchEvent(new Event(REQUEST_CHANGED_EVENT))
}

export function useRequestCount(enabled: boolean) {
  const { data, loading, error, refetch } = useFetch<OverseerrRequestCount>(
    enabled ? '/api/overseerr/requests/count' : null,
  )

  useEffect(() => {
    window.addEventListener(REQUEST_CHANGED_EVENT, refetch)
    return () => window.removeEventListener(REQUEST_CHANGED_EVENT, refetch)
  }, [refetch])

  return { data, loading, error, refetch }
}
