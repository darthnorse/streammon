import { useState, useEffect } from 'react'
import { api } from '../lib/api'

interface FetchState<T> {
  data: T | null
  loading: boolean
  error: Error | null
}

export function useFetch<T>(url: string | null): FetchState<T> {
  const [state, setState] = useState<FetchState<T>>({
    data: null,
    loading: true,
    error: null,
  })

  useEffect(() => {
    if (!url) {
      setState({ data: null, loading: false, error: null })
      return
    }

    let cancelled = false
    setState(prev => ({ ...prev, loading: true, error: null }))

    api.get<T>(url)
      .then(data => {
        if (!cancelled) setState({ data, loading: false, error: null })
      })
      .catch(err => {
        if (!cancelled) setState({ data: null, loading: false, error: err as Error })
      })

    return () => { cancelled = true }
  }, [url])

  return state
}
