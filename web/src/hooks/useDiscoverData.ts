import { useMemo } from 'react'
import { useFetch } from './useFetch'

export function useDiscoverData() {
  const { data: configStatus } = useFetch<{ configured: boolean }>('/api/overseerr/configured')
  const overseerrConfigured = !!configStatus?.configured
  const { data: libraryData } = useFetch<{ ids: string[] }>('/api/library/tmdb-ids')
  const libraryIds = useMemo(() => new Set(libraryData?.ids ?? []), [libraryData])
  const { data: statusData } = useFetch<{ statuses: Record<string, number> }>(
    overseerrConfigured ? '/api/overseerr/media-statuses' : null
  )
  const mediaStatuses = useMemo(
    () => new Map(Object.entries(statusData?.statuses ?? {})),
    [statusData]
  )

  return { overseerrConfigured, libraryIds, mediaStatuses }
}
