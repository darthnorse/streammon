import { useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  activeFilterCount,
  filtersFromParams,
  filtersToParams,
  filtersToQuery,
  type DiscoverCategoryCaps,
  type DiscoverFilters,
} from '../lib/discoverFilters'

export function useDiscoverFilters(caps: DiscoverCategoryCaps) {
  const [searchParams, setSearchParams] = useSearchParams()

  const filters = useMemo(() => filtersFromParams(searchParams, caps), [searchParams, caps])
  const apiQuery = useMemo(() => filtersToQuery(filters, caps), [filters, caps])
  const activeCount = useMemo(() => activeFilterCount(filters, caps), [filters, caps])

  // replace, not push: a dropdown row would otherwise add a history entry per
  // interaction and Back would take many presses to leave the page.
  const setFilters = useCallback(
    (next: DiscoverFilters) => setSearchParams(filtersToParams(next, caps), { replace: true }),
    [setSearchParams, caps],
  )

  const clear = useCallback(
    () => setSearchParams(new URLSearchParams(), { replace: true }),
    [setSearchParams],
  )

  return { filters, setFilters, clear, apiQuery, activeCount }
}
