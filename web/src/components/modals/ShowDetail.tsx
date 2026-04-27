import type { ItemDetails, ModalEntry } from '../../types'

interface ShowDetailProps {
  item: ItemDetails | null
  loading: boolean
  onClose: () => void
  pushModal: (entry: ModalEntry) => void
  active: boolean
  overseerrConfigured: boolean
}

export function ShowDetail(_props: ShowDetailProps) {
  return null
}
