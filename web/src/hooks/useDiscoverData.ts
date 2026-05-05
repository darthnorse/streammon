import { useMemo } from 'react'
import { useFetch } from './useFetch'
import { useOverseerrMediaStatuses } from './useOverseerrMediaStatuses'

export function useDiscoverData() {
  const { data: configStatus } = useFetch<{ configured: boolean }>('/api/overseerr/configured')
  const overseerrConfigured = !!configStatus?.configured
  const { data: libraryData } = useFetch<{ ids: string[] }>('/api/library/tmdb-ids')
  const libraryIds = useMemo(() => new Set(libraryData?.ids ?? []), [libraryData])
  const mediaStatuses = useOverseerrMediaStatuses(overseerrConfigured)

  return { overseerrConfigured, libraryIds, mediaStatuses }
}
