import { useState, useCallback } from 'react'
import type { TitleClickHandler } from '../types'
import { useItemDetails } from './useItemDetails'
import { MediaDetailModal } from '../components/MediaDetailModal'

export function useMediaDetailModal() {
  const [selectedItem, setSelectedItem] = useState<{ serverId: number; itemId: string } | null>(null)
  const { data: itemDetails, loading: detailsLoading } = useItemDetails(
    selectedItem?.serverId ?? 0,
    selectedItem?.itemId ?? null
  )

  const handleTitleClick: TitleClickHandler = useCallback((serverId, itemId) => {
    setSelectedItem({ serverId, itemId })
  }, [])

  const close = useCallback(() => setSelectedItem(null), [])

  const modal = selectedItem ? (
    <MediaDetailModal
      item={itemDetails}
      loading={detailsLoading}
      onClose={close}
    />
  ) : null

  return { handleTitleClick, modal }
}
