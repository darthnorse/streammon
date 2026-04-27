import type { ItemDetails, ModalEntry } from '../../types'

interface EpisodeDetailProps {
  item: ItemDetails | null
  loading: boolean
  onClose: () => void
  pushModal: (entry: ModalEntry) => void
  active: boolean
}

export function EpisodeDetail(_props: EpisodeDetailProps) {
  return null
}
