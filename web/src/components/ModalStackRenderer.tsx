import type { CSSProperties } from 'react'
import { PersonModal } from './PersonModal'
import { MediaDetailModal } from './MediaDetailModal'
import { ShowDetail } from './modals/ShowDetail'
import { SeasonDetail } from './modals/SeasonDetail'
import { EpisodeDetail } from './modals/EpisodeDetail'
import { useItemDetails } from '../hooks/useItemDetails'
import type { ModalEntry } from '../types'

const HIDDEN_STYLE: CSSProperties = {
  visibility: 'hidden',
  pointerEvents: 'none',
}

interface ModalStackRendererProps {
  stack: ModalEntry[]
  pushModal: (entry: ModalEntry) => void
  popModal: () => void
  overseerrConfigured: boolean
  libraryIds: Set<string>
  mediaStatuses?: Map<string, number>
}

export function ModalStackRenderer({
  stack,
  pushModal,
  popModal,
  overseerrConfigured,
  libraryIds,
  mediaStatuses,
}: ModalStackRendererProps) {
  return (
    <>
      {stack.map((entry, i) => {
        const isTop = i === stack.length - 1
        const style = isTop ? undefined : HIDDEN_STYLE
        const ariaHidden = isTop ? undefined : true

        switch (entry.type) {
          case 'person':
            return (
              <div key={`person-${entry.personId}-${i}`} style={style} aria-hidden={ariaHidden}>
                <PersonModal
                  personId={entry.personId}
                  onClose={popModal}
                  onMediaClick={(type, id) => pushModal({ type: 'tmdb', mediaType: type, mediaId: id })}
                  libraryIds={libraryIds}
                  mediaStatuses={mediaStatuses}
                  active={isTop}
                />
              </div>
            )
          case 'tmdb':
            return (
              <div key={`tmdb-${entry.mediaType}-${entry.mediaId}-${i}`} style={style} aria-hidden={ariaHidden}>
                <MediaDetailModal
                  mediaType={entry.mediaType}
                  mediaId={entry.mediaId}
                  overseerrConfigured={overseerrConfigured}
                  onClose={popModal}
                  onPersonClick={id => pushModal({ type: 'person', personId: id })}
                  active={isTop}
                />
              </div>
            )
          case 'library':
          case 'show':
          case 'season':
          case 'episode':
            return (
              <div key={`${entry.type}-${entry.serverId}-${entry.itemId}-${i}`} style={style} aria-hidden={ariaHidden}>
                <LibraryEntryModal
                  entry={entry}
                  onClose={popModal}
                  pushModal={pushModal}
                  active={isTop}
                  overseerrConfigured={overseerrConfigured}
                  libraryIds={libraryIds}
                  mediaStatuses={mediaStatuses}
                />
              </div>
            )
          default: {
            const _exhaustive: never = entry
            return _exhaustive
          }
        }
      })}
    </>
  )
}

interface LibraryEntryModalProps {
  entry: { type: 'library' | 'show' | 'season' | 'episode'; serverId: number; itemId: string }
  onClose: () => void
  pushModal: (entry: ModalEntry) => void
  active: boolean
  overseerrConfigured: boolean
  libraryIds?: Set<string>
  mediaStatuses?: Map<string, number>
}

function LibraryEntryModal({ entry, onClose, pushModal, active, overseerrConfigured, libraryIds, mediaStatuses }: LibraryEntryModalProps) {
  const { data, loading } = useItemDetails(entry.serverId, entry.itemId)

  const level = entry.type === 'library' ? (data?.level ?? '') : entry.type

  if (level === 'season') {
    return (
      <SeasonDetail
        item={data}
        loading={loading}
        onClose={onClose}
        pushModal={pushModal}
        active={active}
      />
    )
  }
  if (level === 'episode') {
    return (
      <EpisodeDetail
        item={data}
        loading={loading}
        onClose={onClose}
        pushModal={pushModal}
        active={active}
        libraryIds={libraryIds}
        mediaStatuses={mediaStatuses}
      />
    )
  }
  return (
    <ShowDetail
      item={data}
      loading={loading}
      onClose={onClose}
      pushModal={pushModal}
      active={active}
      overseerrConfigured={overseerrConfigured}
      libraryIds={libraryIds}
      mediaStatuses={mediaStatuses}
    />
  )
}
