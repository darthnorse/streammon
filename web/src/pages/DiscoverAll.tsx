import { useState, useEffect } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useFetch } from '../hooks/useFetch'
import { DISCOVER_CATEGORIES, MEDIA_GRID_CLASS } from '../lib/constants'
import { MediaCard } from '../components/MediaCard'
import { ChevronIcon } from '../components/ChevronIcon'
import { Pagination } from '../components/Pagination'
import { OverseerrDetailModal } from '../components/OverseerrDetailModal'
import { EmptyState } from '../components/EmptyState'
import type { OverseerrSearchResult, OverseerrMediaResult } from '../types'

type SelectedItem = OverseerrMediaResult & { mediaType: 'movie' | 'tv' }

const backLinkClass = `p-1.5 rounded-md text-gray-500 dark:text-gray-300
  hover:text-gray-900 dark:hover:text-white hover:bg-gray-200 dark:hover:bg-white/10
  transition-colors`

function BackLink() {
  return (
    <Link to="/requests" className={backLinkClass} aria-label="Back to Requests">
      <ChevronIcon direction="left" />
    </Link>
  )
}

function isValidCategory(path: string): boolean {
  return DISCOVER_CATEGORIES.some(c => c.path === path)
}

function categoryTitle(path: string): string {
  const cat = DISCOVER_CATEGORIES.find(c => c.path === path)
  return cat?.title ?? path
}

export function DiscoverAll() {
  const { '*': splat } = useParams()
  const category = splat ?? ''
  const valid = isValidCategory(category)
  const title = categoryTitle(category)

  const [page, setPage] = useState(1)
  const [selectedItem, setSelectedItem] = useState<SelectedItem | null>(null)

  useEffect(() => {
    setPage(1)
  }, [category])

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [page])

  const { data, loading, error } = useFetch<OverseerrSearchResult>(
    valid ? `/api/overseerr/discover/${category}?page=${page}` : null
  )

  const items = data?.results?.filter(
    (r): r is SelectedItem => r.mediaType === 'movie' || r.mediaType === 'tv'
  )

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
        <div>
          <h1 className="text-2xl font-semibold">{title}</h1>
          {data && (
            <p className="text-sm text-muted dark:text-muted-dark mt-0.5">
              {(data.totalResults ?? 0).toLocaleString()} titles
            </p>
          )}
        </div>
      </div>

      {loading && <EmptyState icon="&#8635;" title="Loading..." />}

      {!loading && error && (
        <div className="card p-4 mb-4 text-center text-red-500 dark:text-red-400">
          {error.message || 'Failed to load results'}
        </div>
      )}

      {!loading && !error && (!items || items.length === 0) && (
        <EmptyState icon="&#128270;" title="No results" description="Nothing found in this category." />
      )}

      {!loading && !error && items && items.length > 0 && (
        <>
          <div className={MEDIA_GRID_CLASS}>
            {items.map(item => (
              <MediaCard
                key={`${item.mediaType}-${item.id}`}
                item={item}
                onClick={() => setSelectedItem(item)}
              />
            ))}
          </div>
          <Pagination page={page} totalPages={data?.totalPages ?? 1} onPageChange={setPage} />
        </>
      )}

      {selectedItem && (
        <OverseerrDetailModal
          mediaType={selectedItem.mediaType}
          mediaId={selectedItem.id}
          onClose={() => setSelectedItem(null)}
        />
      )}
    </div>
  )
}
