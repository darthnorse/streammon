import { Link, useParams } from 'react-router-dom'
import { useInfiniteFetch } from '../hooks/useInfiniteFetch'
import { useDiscoverData } from '../hooks/useDiscoverData'
import { useModalStack } from '../hooks/useModalStack'
import { DISCOVER_CATEGORIES, MEDIA_GRID_CLASS, isSelectableMedia, loadMoreBtnClass } from '../lib/constants'
import { MediaCard } from '../components/MediaCard'
import { ChevronIcon } from '../components/ChevronIcon'
import { ModalStackRenderer } from '../components/ModalStackRenderer'
import { EmptyState } from '../components/EmptyState'
import { DiscoverFilterBar } from '../components/DiscoverFilterBar'
import { useDiscoverFilters } from '../hooks/useDiscoverFilters'
import { categoryCaps, type DiscoverCategoryCaps } from '../lib/discoverFilters'
import { ownedKey } from '../lib/tmdb'
import type { TMDBMediaResult } from '../types'

function ErrorBanner({ message, onRetry, className }: { message: string; onRetry: () => void; className?: string }) {
  return (
    <div className={`card p-4 text-center ${className ?? ''}`}>
      <p className="text-red-500 dark:text-red-400 mb-2">{message}</p>
      <button onClick={onRetry} className="text-sm hover:text-accent hover:underline">
        Try again
      </button>
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

// caps is null only for an unknown category, which returns early; a module-level
// fallback keeps the hook call unconditional without churning its memo identity.
const EMPTY_CAPS: DiscoverCategoryCaps = { year: false, type: false, mediaType: null }

export function DiscoverAll() {
  const { '*': splat } = useParams()
  const category = splat ?? ''
  const cat = findCategory(category)
  const caps = categoryCaps(category)
  const title = cat?.title ?? category

  const { stack, push: pushModal, pop: popModal } = useModalStack()

  const { overseerrConfigured, libraryIds, mediaStatuses } = useDiscoverData()
  const { filters, setFilters, clear, apiQuery, activeCount } = useDiscoverFilters(caps ?? EMPTY_CAPS)

  const url = cat && caps ? `/api/tmdb/discover/${category}${apiQuery ? `?${apiQuery}` : ''}` : null
  const { items, loading, loadingMore, hasMore, error, sentinelRef, retry, capped, loadMore } =
    useInfiniteFetch<TMDBMediaResult>(url, 20, 'page')

  const filtered = items.filter(
    item => isSelectableMedia(item) && !(filters.hideOwned && libraryIds.has(ownedKey(item))),
  )
  // Auto-fill gave up with pages still to come: the filters are not what emptied
  // the list, so the empty state must not blame them.
  const cappedEarly = capped && hasMore

  if (!cat || !caps) {
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

      <DiscoverFilterBar
        caps={caps}
        filters={filters}
        onChange={setFilters}
        onClear={clear}
        activeCount={activeCount}
      />

      {loading && <EmptyState icon="&#8635;" title="Loading..." />}

      {!loading && error && filtered.length === 0 && (
        <ErrorBanner message={error} onRetry={retry} className="mb-4" />
      )}

      {!loading && !error && filtered.length === 0 && cappedEarly && (
        <EmptyState
          icon="&#128270;"
          title="Nothing new in the first pages"
          description="Everything loaded so far is already in your library."
        />
      )}

      {!loading && !error && filtered.length === 0 && !hasMore && (
        activeCount > 0 ? (
          <EmptyState
            icon="&#128270;"
            title="No results match your filters"
            description="Try widening the year range or removing a genre."
          >
            <button type="button" onClick={clear} className={loadMoreBtnClass}>
              Clear all filters
            </button>
          </EmptyState>
        ) : (
          <EmptyState icon="&#128270;" title="No results" description="Nothing found in this category." />
        )
      )}

      {filtered.length > 0 && (
        <div className={MEDIA_GRID_CLASS}>
          {filtered.map(item => (
            <MediaCard
              key={`${item.media_type}-${item.id}`}
              item={item}
              onClick={() => pushModal({ type: 'tmdb', mediaType: item.media_type as 'movie' | 'tv', mediaId: item.id })}
              available={libraryIds.has(ownedKey(item))}
              fallbackMediaStatus={mediaStatuses.get(`${item.media_type}:${item.id}`)}
            />
          ))}
        </div>
      )}

      {!loading && (
        <>
          <div ref={sentinelRef} />
          {loadingMore && (
            <div className="flex justify-center py-6">
              <div className="h-6 w-6 border-2 border-accent border-t-transparent rounded-full animate-spin" />
            </div>
          )}
          {error && filtered.length > 0 && <ErrorBanner message={error} onRetry={retry} />}
          {cappedEarly && !loadingMore && !error && (
            <div className="flex justify-center py-4">
              <button onClick={loadMore} className={loadMoreBtnClass}>Load more</button>
            </div>
          )}
          {!hasMore && !error && filtered.length > 0 && (
            <p className="text-center text-sm text-muted dark:text-muted-dark py-4">No more results</p>
          )}
        </>
      )}

      {stack.length > 0 && (
        <ModalStackRenderer
          stack={stack}
          pushModal={pushModal}
          popModal={popModal}
          overseerrConfigured={overseerrConfigured}
          libraryIds={libraryIds}
          mediaStatuses={mediaStatuses}
        />
      )}
    </div>
  )
}
