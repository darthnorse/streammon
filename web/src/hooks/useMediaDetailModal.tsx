import { useCallback, useMemo } from 'react'
import type { TitleClickHandler } from '../types'
import { useModalStack } from './useModalStack'
import { useFetch } from './useFetch'
import { useOverseerrMediaStatuses } from './useOverseerrMediaStatuses'
import { ModalStackRenderer } from '../components/ModalStackRenderer'

export function useMediaDetailModal() {
  const { stack, push, pop } = useModalStack()

  const handleTitleClick: TitleClickHandler = useCallback((serverId, itemId) => {
    push({ type: 'library', serverId, itemId })
  }, [push])

  const { data: configData } = useFetch<{ configured: boolean }>(
    stack.length > 0 ? '/api/overseerr/configured' : null,
  )
  const overseerrConfigured = !!configData?.configured
  const { data: libraryIdsData } = useFetch<{ ids: string[] }>(
    stack.length > 0 ? '/api/library/tmdb-ids' : null,
  )
  const libraryIds = useMemo(() => new Set(libraryIdsData?.ids ?? []), [libraryIdsData])
  const mediaStatuses = useOverseerrMediaStatuses(overseerrConfigured && stack.length > 0)

  const modal = stack.length > 0 ? (
    <ModalStackRenderer
      stack={stack}
      pushModal={push}
      popModal={pop}
      overseerrConfigured={overseerrConfigured}
      libraryIds={libraryIds}
      mediaStatuses={mediaStatuses}
    />
  ) : null

  return { handleTitleClick, modal }
}
