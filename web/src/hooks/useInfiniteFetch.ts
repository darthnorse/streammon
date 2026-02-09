import { useState, useEffect, useRef, useCallback } from 'react'
import { api } from '../lib/api'

interface OffsetResponse<T> {
  results: T[]
  pageInfo: { pages: number }
}

interface PageResponse<T> {
  results: T[]
  totalPages: number
}

type PagedResponse<T> = OffsetResponse<T> | PageResponse<T>

interface UseInfiniteFetchReturn<T> {
  items: T[]
  loading: boolean
  loadingMore: boolean
  hasMore: boolean
  error: string | null
  sentinelRef: React.RefObject<HTMLDivElement>
  retry: () => void
  refetch: () => void
}

export function useInfiniteFetch<T>(
  baseUrl: string | null,
  pageSize: number,
  mode: 'offset' | 'page' = 'offset',
): UseInfiniteFetchReturn<T> {
  const [items, setItems] = useState<T[]>([])
  const [hasMore, setHasMore] = useState(true)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [resetTick, setResetTick] = useState(0)

  const sentinelRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)
  const fetchingRef = useRef(false)
  // -1 = no page fetched yet; on success set to the fetched page number
  const pageRef = useRef(-1)

  const fetchPage = useCallback((pageNum: number) => {
    if (!baseUrl) return

    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    const isFirst = pageNum === 0
    fetchingRef.current = true

    if (isFirst) {
      setLoading(true)
      setError(null)
    } else {
      setLoadingMore(true)
    }

    const sep = baseUrl.includes('?') ? '&' : '?'
    const params = mode === 'page'
      ? `page=${pageNum + 1}`
      : `take=${pageSize}&skip=${pageNum * pageSize}`
    api.get<PagedResponse<T>>(
      `${baseUrl}${sep}${params}`,
      controller.signal,
    )
      .then(data => {
        if (controller.signal.aborted) return
        if (isFirst) setItems(data.results)
        else setItems(prev => [...prev, ...data.results])
        const totalPages = mode === 'page'
          ? (data as PageResponse<T>).totalPages
          : (data as OffsetResponse<T>).pageInfo.pages
        setHasMore((pageNum + 1) < totalPages)
        pageRef.current = pageNum
      })
      .catch(err => {
        if ((err as Error).name === 'AbortError') return
        setHasMore(false)
        setError((err as Error).message || 'Failed to load')
      })
      .finally(() => {
        if (controller.signal.aborted) return
        fetchingRef.current = false
        if (isFirst) setLoading(false)
        else setLoadingMore(false)
      })
  }, [baseUrl, pageSize, mode])

  useEffect(() => {
    abortRef.current?.abort()
    fetchingRef.current = false
    pageRef.current = -1
    setItems([])
    setHasMore(true)
    setLoadingMore(false)
    setError(null)

    if (baseUrl) {
      fetchPage(0)
    } else {
      setLoading(false)
    }

    return () => { abortRef.current?.abort() }
  }, [fetchPage, resetTick])

  useEffect(() => {
    if (!hasMore || loading) return
    const sentinel = sentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(entries => {
      if (entries[0].isIntersecting && !fetchingRef.current) {
        fetchPage(pageRef.current + 1)
      }
    }, { rootMargin: '200px' })

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [hasMore, loading, fetchPage])

  const retry = useCallback(() => {
    setError(null)
    setHasMore(true)
    fetchingRef.current = false
    fetchPage(pageRef.current + 1)
  }, [fetchPage])

  const refetch = useCallback(() => {
    setResetTick(t => t + 1)
  }, [])

  return { items, loading, loadingMore, hasMore, error, sentinelRef, retry, refetch }
}
