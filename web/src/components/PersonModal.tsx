import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api'
import { TMDB_IMG } from '../lib/tmdb'
import { MediaCard } from './MediaCard'
import { MEDIA_GRID_CLASS } from '../lib/constants'
import type { TMDBPersonDetails, TMDBPersonCredit } from '../types'

interface PersonModalProps {
  personId: number
  onClose: () => void
  onMediaClick?: (mediaType: 'movie' | 'tv', mediaId: number) => void
}

function sortByPopularity(credits: TMDBPersonCredit[]): TMDBPersonCredit[] {
  return [...credits].sort((a, b) => (b.popularity ?? 0) - (a.popularity ?? 0))
}

export function PersonModal({ personId, onClose, onMediaClick }: PersonModalProps) {
  const [person, setPerson] = useState<TMDBPersonDetails | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showFullBio, setShowFullBio] = useState(false)

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.stopImmediatePropagation()
      onClose()
    }
  }, [onClose])

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown, true)
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', handleKeyDown, true)
      document.body.style.overflow = ''
    }
  }, [handleKeyDown])

  useEffect(() => {
    setLoading(true)
    setError('')
    const controller = new AbortController()

    api.get<TMDBPersonDetails>(`/api/tmdb/person/${personId}`, controller.signal)
      .then(data => setPerson(data))
      .catch(err => {
        if ((err as Error).name !== 'AbortError') {
          setError((err as Error).message)
        }
      })
      .finally(() => setLoading(false))
    return () => controller.abort()
  }, [personId])

  const cast = person?.combined_credits?.cast
  const sortedCast = cast ? sortByPopularity(cast) : []
  const uniqueCast = sortedCast.filter((c, i, arr) => arr.findIndex(x => x.id === c.id) === i)
  const biography = person?.biography
  const truncatedBio = biography && biography.length > 400 && !showFullBio
    ? biography.slice(0, 400) + '...'
    : biography

  const age = person?.birthday ? calculateAge(person.birthday, person.deathday) : null

  return (
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center p-4 pb-20 lg:pb-4 bg-black/70 backdrop-blur-sm animate-fade-in"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="person-modal-title"
    >
      <div
        className="relative w-full max-w-3xl max-h-[90dvh] overflow-y-auto rounded-xl
                   bg-panel dark:bg-panel-dark shadow-2xl animate-slide-up"
        onClick={e => e.stopPropagation()}
      >
        <button
          onClick={onClose}
          className="absolute top-3 right-3 z-10 w-8 h-8 flex items-center justify-center
                     rounded-full bg-black/40 hover:bg-black/60 text-white transition-colors"
          aria-label="Close"
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>

        {loading && (
          <div className="flex items-center justify-center py-20">
            <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {!loading && error && (
          <div className="p-8 text-center text-red-500 dark:text-red-400">{error}</div>
        )}

        {!loading && !error && person && (
          <div className="p-5 sm:p-6 space-y-5">
            <div className="flex gap-4">
              {person.profile_path ? (
                <img
                  src={`${TMDB_IMG}/w185${person.profile_path}`}
                  alt={person.name}
                  className="w-24 sm:w-32 rounded-lg shadow-lg shrink-0 object-cover"
                />
              ) : (
                <div className="w-24 sm:w-32 aspect-[2/3] rounded-lg bg-gray-200 dark:bg-gray-800 flex items-center justify-center text-3xl text-muted dark:text-muted-dark shrink-0">
                  &#128100;
                </div>
              )}

              <div className="flex-1 min-w-0">
                <h2 id="person-modal-title" className="text-2xl font-bold">
                  {person.name}
                </h2>
                <div className="flex flex-wrap items-center gap-2 mt-1.5 text-sm text-muted dark:text-muted-dark">
                  {person.known_for_department && (
                    <span>{person.known_for_department}</span>
                  )}
                  {person.birthday && (
                    <>
                      <span>&middot;</span>
                      <span>Born {person.birthday}</span>
                    </>
                  )}
                  {age != null && (
                    <>
                      <span>&middot;</span>
                      <span>{person.deathday ? `Died age ${age}` : `Age ${age}`}</span>
                    </>
                  )}
                </div>
                {person.place_of_birth && (
                  <div className="text-sm text-muted dark:text-muted-dark mt-1">
                    {person.place_of_birth}
                  </div>
                )}
              </div>
            </div>

            {truncatedBio && (
              <div>
                <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed whitespace-pre-line">
                  {truncatedBio}
                </p>
                {biography && biography.length > 400 && (
                  <button
                    onClick={() => setShowFullBio(!showFullBio)}
                    className="text-xs text-muted dark:text-muted-dark hover:text-accent hover:underline mt-1"
                  >
                    {showFullBio ? 'Show less' : 'Read more'}
                  </button>
                )}
              </div>
            )}

            {uniqueCast.length > 0 && (
              <div className="space-y-3">
                <div className="text-sm font-medium">
                  Known For ({uniqueCast.length})
                </div>
                <div className={MEDIA_GRID_CLASS}>
                  {uniqueCast.slice(0, 20).map(credit => (
                    <MediaCard
                      key={`${credit.media_type}-${credit.id}`}
                      item={{
                        id: credit.id,
                        media_type: credit.media_type,
                        title: credit.title,
                        name: credit.name,
                        poster_path: credit.poster_path,
                        release_date: credit.release_date,
                        first_air_date: credit.first_air_date,
                        vote_average: credit.vote_average,
                      }}
                      onClick={() => onMediaClick?.(credit.media_type, credit.id)}
                    />
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function calculateAge(birthday: string, deathday?: string): number | null {
  const birth = new Date(birthday)
  if (isNaN(birth.getTime())) return null
  const end = deathday ? new Date(deathday) : new Date()
  let age = end.getFullYear() - birth.getFullYear()
  const m = end.getMonth() - birth.getMonth()
  if (m < 0 || (m === 0 && end.getDate() < birth.getDate())) age--
  return age
}
