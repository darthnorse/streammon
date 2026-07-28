import { useState, useEffect, useRef, useCallback } from 'react'
import { api } from '../lib/api'

interface OffsetResponse<T> {
  results: T[]
  pageInfo: { pages: number }
}

interface PageResponse<T> {
  results: T[]
  totalPages?: number
  total_pages?: number // TMDB API uses snake_case
}

type PagedResponse<T> = OffsetResponse<T> | PageResponse<T>

// Consumers render a filtered subset, so a short list can keep the sentinel
// visible forever — without this bound it would drain the whole dataset.
const MAX_AUTO_FILL = 10

interface UseInfiniteFetchReturn<T> {
  items: T[]
  loading: boolean
  loadingMore: boolean
  hasMore: boolean
  error: string | null
  sentinelRef: React.RefObject<HTMLDivElement>
  retry: () => void
  refetch: () => void
  capped: boolean
  loadMore: () => void
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
  const [fetchTick, setFetchTick] = useState(0)
  const [capped, setCapped] = useState(false)

  const sentinelRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)
  const fetchingRef = useRef(false)
  // -1 = no page fetched yet; on success set to the fetched page number
  const pageRef = useRef(-1)
  const autoFillRef = useRef(0)

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
        const pageData = data as PageResponse<T>
        const totalPages = mode === 'page'
          ? (pageData.totalPages ?? pageData.total_pages ?? 0)
          : (data as OffsetResponse<T>).pageInfo.pages
        setHasMore((pageNum + 1) < totalPages)
        pageRef.current = pageNum
      })
      .catch(err => {
        if ((err as Error).name === 'AbortError') return
        if (controller.signal.aborted) return
        setHasMore(false)
        setError((err as Error).message || 'Failed to load')
      })
      .finally(() => {
        if (controller.signal.aborted) return
        fetchingRef.current = false
        if (isFirst) setLoading(false)
        else setLoadingMore(false)
        // Must trail fetchingRef, or the re-observe races the in-flight guard.
        setFetchTick(t => t + 1)
      })
  }, [baseUrl, pageSize, mode])

  useEffect(() => {
    abortRef.current?.abort()
    fetchingRef.current = false
    pageRef.current = -1
    autoFillRef.current = 0
    setItems([])
    setHasMore(true)
    setLoadingMore(false)
    setError(null)
    setCapped(false)

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
      if (!entries[0].isIntersecting) {
        autoFillRef.current = 0
        setCapped(false)
        return
      }
      if (fetchingRef.current) return
      if (autoFillRef.current >= MAX_AUTO_FILL) {
        setCapped(true)
        return
      }
      autoFillRef.current += 1
      fetchPage(pageRef.current + 1)
    }, { rootMargin: '200px' })

    observer.observe(sentinel)
    return () => observer.disconnect()
    // fetchTick re-observes after every completed page. IntersectionObserver only
    // fires on transitions, so without it the hook stalls once the content fits
    // the viewport and there is nothing left to scroll.
  }, [hasMore, loading, fetchPage, fetchTick])

  const retry = useCallback(() => {
    setError(null)
    setHasMore(true)
    fetchingRef.current = false
    fetchPage(pageRef.current + 1)
  }, [fetchPage])

  const refetch = useCallback(() => {
    setResetTick(t => t + 1)
  }, [])

  const loadMore = useCallback(() => {
    if (fetchingRef.current) return
    autoFillRef.current = 0
    setCapped(false)
    fetchPage(pageRef.current + 1)
  }, [fetchPage])

  return { items, loading, loadingMore, hasMore, error, sentinelRef, retry, refetch, capped, loadMore }
}
