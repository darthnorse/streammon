import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api'

interface FetchState<T> {
  data: T | null
  // The url `data` was fetched for, so a consumer can tell fresh data from a
  // stale value retained from a previous url while a new fetch is in flight.
  // Optional so existing mocks of this hook's return value, which predate
  // this field, keep compiling unchanged.
  dataUrl?: string | null
  loading: boolean
  error: Error | null
}

export function useFetch<T>(url: string | null): FetchState<T> & { refetch: () => void } {
  const [state, setState] = useState<FetchState<T>>({
    data: null,
    dataUrl: null,
    loading: true,
    error: null,
  })
  const [tick, setTick] = useState(0)

  useEffect(() => {
    if (!url) {
      setState({ data: null, dataUrl: null, loading: false, error: null })
      return
    }

    const controller = new AbortController()
    setState(prev => ({ data: prev.data, dataUrl: prev.dataUrl, loading: true, error: null }))

    api.get<T>(url, controller.signal)
      .then(data => {
        if (!controller.signal.aborted) setState({ data, dataUrl: url, loading: false, error: null })
      })
      .catch(err => {
        if (!controller.signal.aborted) setState({ data: null, dataUrl: url, loading: false, error: err as Error })
      })

    return () => { controller.abort() }
  }, [url, tick])

  const refetch = useCallback(() => setTick(t => t + 1), [])

  return { ...state, refetch }
}
