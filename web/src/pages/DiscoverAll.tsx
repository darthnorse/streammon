import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useInfiniteFetch } from '../hooks/useInfiniteFetch'
import { useFetch } from '../hooks/useFetch'
import { DISCOVER_CATEGORIES, MEDIA_GRID_CLASS, isSelectableMedia } from '../lib/constants'
import { MediaCard } from '../components/MediaCard'
import { ChevronIcon } from '../components/ChevronIcon'
import { TMDBDetailModal } from '../components/TMDBDetailModal'
import { PersonModal } from '../components/PersonModal'
import { EmptyState } from '../components/EmptyState'
import type { TMDBMediaResult, SelectedMedia } from '../types'

function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="card p-4 mb-4 text-center text-red-500 dark:text-red-400">
      {message}
    </div>
  )
}

const backLinkClass = `p-1.5 rounded-md text-gray-500 dark:text-gray-300
  hover:text-gray-900 dark:hover:text-white hover:bg-gray-200 dark:hover:bg-white/10
  transition-colors`

function BackLink() {
  return (
    <Link to="/discover" className={backLinkClass} aria-label="Back to Discover">
      <ChevronIcon direction="left" />
    </Link>
  )
}

function findCategory(path: string) {
  return DISCOVER_CATEGORIES.find(c => c.path === path)
}

export function DiscoverAll() {
  const { '*': splat } = useParams()
  const category = splat ?? ''
  const cat = findCategory(category)
  const valid = !!cat
  const title = cat?.title ?? category

  const [selectedMedia, setSelectedMedia] = useState<SelectedMedia | null>(null)
  const [selectedPerson, setSelectedPerson] = useState<number | null>(null)

  const { data: configStatus } = useFetch<{ configured: boolean }>('/api/overseerr/configured')
  const overseerrConfigured = !!configStatus?.configured

  const url = valid ? `/api/tmdb/discover/${category}` : null
  const { items, loading, loadingMore, hasMore, error, sentinelRef } =
    useInfiniteFetch<TMDBMediaResult>(url, 20, 'page')

  const filtered = items.filter(isSelectableMedia)

  if (!valid) {
    return (
      <div>
        <div className="flex items-center gap-3 mb-6">
          <BackLink />
          <h1 className="text-2xl font-semibold">Unknown Category</h1>
        </div>
        <EmptyState icon="?" title="Category not found" description="This discover category does not exist." />
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <BackLink />
        <h1 className="text-2xl font-semibold">{title}</h1>
      </div>

      {loading && <EmptyState icon="&#8635;" title="Loading..." />}

      {!loading && error && filtered.length === 0 && <ErrorBanner message={error} />}

      {!loading && !error && filtered.length === 0 && (
        <EmptyState icon="&#128270;" title="No results" description="Nothing found in this category." />
      )}

      {filtered.length > 0 && (
        <>
          <div className={MEDIA_GRID_CLASS}>
            {filtered.map(item => (
              <MediaCard
                key={`${item.media_type}-${item.id}`}
                item={item}
                onClick={() => setSelectedMedia({ mediaType: item.media_type as 'movie' | 'tv', mediaId: item.id })}
              />
            ))}
          </div>
          <div ref={sentinelRef} />
          {loadingMore && (
            <div className="flex justify-center py-6">
              <div className="h-6 w-6 border-2 border-accent border-t-transparent rounded-full animate-spin" />
            </div>
          )}
          {error && <ErrorBanner message={error} />}
          {!hasMore && !error && (
            <p className="text-center text-sm text-muted dark:text-muted-dark py-4">No more results</p>
          )}
        </>
      )}

      {selectedMedia && (
        <TMDBDetailModal
          mediaType={selectedMedia.mediaType}
          mediaId={selectedMedia.mediaId}
          overseerrConfigured={overseerrConfigured}
          onClose={() => setSelectedMedia(null)}
          onPersonClick={id => {
            setSelectedMedia(null)
            setSelectedPerson(id)
          }}
        />
      )}

      {selectedPerson && (
        <PersonModal
          personId={selectedPerson}
          onClose={() => setSelectedPerson(null)}
          onMediaClick={(type, id) => {
            setSelectedPerson(null)
            setSelectedMedia({ mediaType: type, mediaId: id })
          }}
        />
      )}
    </div>
  )
}
