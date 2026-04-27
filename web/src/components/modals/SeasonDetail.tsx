import type { ItemDetails, ModalEntry } from '../../types'

interface SeasonDetailProps {
  item: ItemDetails | null
  loading: boolean
  onClose: () => void
  pushModal: (entry: ModalEntry) => void
  active: boolean
}

export function SeasonDetail(_props: SeasonDetailProps) {
  return null
}
