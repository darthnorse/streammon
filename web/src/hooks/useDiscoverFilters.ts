import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  DISCOVER_DEFAULT_KEY,
  activeFilterCount,
  defaultFromFilters,
  defaultParamsFor,
  filtersFromParams,
  filtersToParams,
  filtersToQuery,
  parseStoredDefault,
  type DiscoverCategoryCaps,
  type DiscoverFilters,
  type StoredDiscoverDefault,
} from '../lib/discoverFilters'

function readStoredDefault(): StoredDiscoverDefault | null {
  try {
    return parseStoredDefault(localStorage.getItem(DISCOVER_DEFAULT_KEY))
  } catch {
    return null
  }
}

function writeStoredDefault(value: StoredDiscoverDefault | null) {
  try {
    if (value) localStorage.setItem(DISCOVER_DEFAULT_KEY, JSON.stringify(value))
    else localStorage.removeItem(DISCOVER_DEFAULT_KEY)
  } catch {
    return
  }
}

export function useDiscoverFilters(caps: DiscoverCategoryCaps) {
  const [searchParams, setSearchParams] = useSearchParams()
  const [storedDefault, setStoredDefault] = useState(readStoredDefault)

  const defaultParams = useMemo(
    () => (storedDefault ? defaultParamsFor(storedDefault, caps) : null),
    [storedDefault, caps],
  )

  // The default fills an empty URL once per category visit. Until the effect
  // writes it to the URL, the first render already reads it, so the page never
  // fetches unfiltered results first. Tracked per caps rather than per URL:
  // re-applying on every empty URL would put the default straight back after
  // the user clears it.
  const appliedFor = useRef<DiscoverCategoryCaps | null>(null)
  const pendingDefault =
    appliedFor.current !== caps && searchParams.toString() === '' && !!defaultParams && defaultParams.toString() !== ''
  const effectiveParams = pendingDefault && defaultParams ? defaultParams : searchParams

  useEffect(() => {
    appliedFor.current = caps
    if (pendingDefault && defaultParams) setSearchParams(defaultParams, { replace: true })
  }, [caps, pendingDefault, defaultParams, setSearchParams])

  const filters = useMemo(() => filtersFromParams(effectiveParams, caps), [effectiveParams, caps])
  const apiQuery = useMemo(() => filtersToQuery(filters, caps), [filters, caps])
  const activeCount = useMemo(() => activeFilterCount(filters, caps), [filters, caps])
  const isDefault =
    defaultParams !== null && defaultParams.toString() !== '' &&
    defaultParams.toString() === filtersToParams(filters, caps).toString()

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

  const saveDefault = useCallback(() => {
    const value = defaultFromFilters(filters, caps)
    writeStoredDefault(value)
    setStoredDefault(value)
  }, [filters, caps])

  const removeDefault = useCallback(() => {
    writeStoredDefault(null)
    setStoredDefault(null)
  }, [])

  return {
    filters,
    setFilters,
    clear,
    apiQuery,
    activeCount,
    hasDefault: storedDefault !== null,
    isDefault,
    saveDefault,
    removeDefault,
  }
}
