import { useMemo } from 'react'
import { useFetch } from './useFetch'
import { useRequestChangedListener } from './useRequestCount'

export function useOverseerrMediaStatuses(enabled: boolean): Map<string, number> {
  const { data, refetch } = useFetch<{ statuses: Record<string, number> }>(
    enabled ? '/api/overseerr/media-statuses' : null,
  )

  useRequestChangedListener(refetch)

  return useMemo(
    () => new Map(Object.entries(data?.statuses ?? {})),
    [data],
  )
}
