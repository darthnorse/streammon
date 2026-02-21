import type { CSSProperties } from 'react'
import { PersonModal } from './PersonModal'
import { TMDBDetailModal } from './TMDBDetailModal'
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
}

export function ModalStackRenderer({
  stack,
  pushModal,
  popModal,
  overseerrConfigured,
  libraryIds,
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
                  active={isTop}
                />
              </div>
            )
          case 'tmdb':
            return (
              <div key={`tmdb-${entry.mediaType}-${entry.mediaId}-${i}`} style={style} aria-hidden={ariaHidden}>
                <TMDBDetailModal
                  mediaType={entry.mediaType}
                  mediaId={entry.mediaId}
                  overseerrConfigured={overseerrConfigured}
                  onClose={popModal}
                  onPersonClick={id => pushModal({ type: 'person', personId: id })}
                  active={isTop}
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
