import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api'

interface FetchState<T> {
  data: T | null
  loading: boolean
  error: Error | null
}

export function useFetch<T>(url: string | null): FetchState<T> & { refetch: () => void } {
  const [state, setState] = useState<FetchState<T>>({
    data: null,
    loading: true,
    error: null,
  })
  const [tick, setTick] = useState(0)

  useEffect(() => {
    if (!url) {
      setState({ data: null, loading: false, error: null })
      return
    }

    const controller = new AbortController()
    setState(prev => ({ data: prev.data, loading: true, error: null }))

    api.get<T>(url, controller.signal)
      .then(data => {
        if (!controller.signal.aborted) setState({ data, loading: false, error: null })
      })
      .catch(err => {
        if (!controller.signal.aborted) setState({ data: null, loading: false, error: err as Error })
      })

    return () => { controller.abort() }
  }, [url, tick])

  const refetch = useCallback(() => setTick(t => t + 1), [])

  return { ...state, refetch }
}
