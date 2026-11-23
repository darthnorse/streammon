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
import { safeGetItem, safeRemoveItem, safeSetItem } from '../lib/storage'

export interface SavedDefaultControls {
  hasDefault: boolean
  isDefault: boolean
  canSave: boolean
  onSave: () => void
  onRemove: () => void
}

const NO_CAPS: DiscoverCategoryCaps = { year: false, type: false, mediaType: null }

function readStoredDefault(): StoredDiscoverDefault | null {
  return parseStoredDefault(safeGetItem(DISCOVER_DEFAULT_KEY))
}

// knownCaps is null for an unknown category: filters still parse, but no saved
// default is applied or offered there.
export function useDiscoverFilters(knownCaps: DiscoverCategoryCaps | null) {
  const caps = knownCaps ?? NO_CAPS
  const [searchParams, setSearchParams] = useSearchParams()
  const [storedDefault, setStoredDefault] = useState(readStoredDefault)
  const activeDefault = knownCaps ? storedDefault : null

  const defaultFor = useCallback(
    (type: DiscoverFilters['type']) => {
      if (!activeDefault) return null
      const params = defaultParamsFor(activeDefault, caps, type)
      const saved = new URLSearchParams(params)
      saved.delete('type')
      return saved.toString() ? params : null
    },
    [activeDefault, caps],
  )

  // The default fills an empty URL once per category visit. Until the effect
  // writes it to the URL, the first render already reads it, so the page never
  // fetches unfiltered results first. Tracked per caps rather than per URL:
  // re-applying on every empty URL would put the default straight back after
  // the user clears it.
  const appliedFor = useRef<DiscoverCategoryCaps | null>(null)
  const landingDefault = useMemo(() => defaultFor(null), [defaultFor])
  const pendingDefault = appliedFor.current !== caps && searchParams.toString() === '' ? landingDefault : null

  useEffect(() => {
    appliedFor.current = caps
    if (pendingDefault) setSearchParams(pendingDefault, { replace: true })
  }, [caps, pendingDefault, setSearchParams])

  const filters = useMemo(() => filtersFromParams(pendingDefault ?? searchParams, caps), [pendingDefault, searchParams, caps])
  const apiQuery = useMemo(() => filtersToQuery(filters, caps), [filters, caps])
  const activeCount = useMemo(() => activeFilterCount(filters, caps), [filters, caps])
  const currentQuery = filtersToParams(filters, caps).toString()

  // replace, not push: a dropdown row would otherwise add a history entry per
  // interaction and Back would take many presses to leave the page.
  // Picking a type on a mixed category is what lets the default's server-side
  // filters apply there, so that transition brings them in. hideOwned already
  // applied before the type was picked, so the user's current choice wins.
  const setFilters = useCallback(
    (next: DiscoverFilters) => {
      const typed = caps.type && filters.type === null && next.type !== null ? defaultFor(next.type) : null
      const params = typed
        ? filtersToParams({ ...filtersFromParams(typed, caps), hideOwned: next.hideOwned }, caps)
        : filtersToParams(next, caps)
      setSearchParams(params, { replace: true })
    },
    [setSearchParams, caps, filters.type, defaultFor],
  )

  const clear = useCallback(
    () => setSearchParams(new URLSearchParams(), { replace: true }),
    [setSearchParams],
  )

  const toSave = useMemo(() => defaultFromFilters(filters, caps), [filters, caps])
  const onSave = useCallback(() => {
    if (safeSetItem(DISCOVER_DEFAULT_KEY, JSON.stringify(toSave))) setStoredDefault(toSave)
  }, [toSave])

  const onRemove = useCallback(() => {
    if (safeRemoveItem(DISCOVER_DEFAULT_KEY)) setStoredDefault(null)
  }, [])

  const savedDefault: SavedDefaultControls = {
    hasDefault: activeDefault !== null,
    isDefault: defaultFor(filters.type)?.toString() === currentQuery,
    canSave: knownCaps !== null && toSave.query !== '',
    onSave,
    onRemove,
  }

  return { filters, setFilters, clear, apiQuery, activeCount, savedDefault }
}
