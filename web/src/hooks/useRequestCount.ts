import { useEffect } from 'react'
import { useFetch } from './useFetch'
import type { OverseerrRequestCount } from '../types'

export const REQUEST_CHANGED_EVENT = 'overseerr-request-changed'

export function dispatchRequestChanged() {
  window.dispatchEvent(new Event(REQUEST_CHANGED_EVENT))
}

export function useRequestChangedListener(cb: () => void) {
  useEffect(() => {
    window.addEventListener(REQUEST_CHANGED_EVENT, cb)
    return () => window.removeEventListener(REQUEST_CHANGED_EVENT, cb)
  }, [cb])
}

export function useRequestCount(enabled: boolean) {
  const { data, loading, error, refetch } = useFetch<OverseerrRequestCount>(
    enabled ? '/api/overseerr/requests/count' : null,
  )

  useRequestChangedListener(refetch)

  return { data, loading, error, refetch }
}
