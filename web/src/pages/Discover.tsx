import { useState, useEffect, useRef, useMemo, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'
import { useInfiniteFetch } from '../hooks/useInfiniteFetch'
import { useDebouncedSearch } from '../hooks/useDebouncedSearch'
import { useHorizontalScroll } from '../hooks/useHorizontalScroll'
import { useAuth } from '../context/AuthContext'
import { useModalStack } from '../hooks/useModalStack'
import { DISCOVER_CATEGORIES, MEDIA_GRID_CLASS, isSelectableMedia } from '../lib/constants'
import { EmptyState } from '../components/EmptyState'
import { MediaCard } from '../components/MediaCard'
import { ChevronIcon } from '../components/ChevronIcon'
import { RequestCard } from '../components/RequestCard'
import { SearchInput } from '../components/shared/SearchInput'
import { ModalStackRenderer } from '../components/ModalStackRenderer'
import type {
  TMDBSearchResult,
  OverseerrRequestCount,
  OverseerrRequest,
  SelectedMedia,
} from '../types'

type Tab = 'discover' | 'requests'

const scrollBtnClass = `p-1.5 rounded-md text-gray-500 dark:text-gray-300
  hover:text-gray-900 dark:hover:text-white hover:bg-gray-200 dark:hover:bg-white/10
  disabled:opacity-20 disabled:pointer-events-none transition-colors`

function tabClass(active: boolean) {
  return `px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
    active
      ? 'border-accent text-accent'
      : 'border-transparent text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
  }`
}

type DiscoverSectionProps = {
  path: string
  title: string
  data: TMDBSearchResult | null
  loading: boolean
  onSelect: (item: SelectedMedia) => void
  libraryIds: Set<string>
}

function DiscoverSection({ path, title, data, loading, onSelect, libraryIds }: DiscoverSectionProps) {
  const { canScrollLeft, canScrollRight, scrollBy, ...scrollHandlers } = useHorizontalScroll()
  const items = data?.results?.filter(isSelectableMedia)
  if (!loading && (!items || items.length === 0)) return null

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-medium text-muted dark:text-muted-dark uppercase tracking-wider">
          {title}
        </h2>
        {!loading && (
          <div className="flex items-center gap-1">
            <Link
              to={`/discover/${path}`}
              className="text-xs text-muted dark:text-muted-dark hover:text-accent hover:underline transition-colors mr-1"
            >
              See All &rsaquo;
            </Link>
            <button onClick={() => scrollBy('left')} disabled={!canScrollLeft} aria-label="Scroll left" className={scrollBtnClass}>
              <ChevronIcon direction="left" />
            </button>
            <button onClick={() => scrollBy('right')} disabled={!canScrollRight} aria-label="Scroll right" className={scrollBtnClass}>
              <ChevronIcon direction="right" />
            </button>
          </div>
        )}
      </div>
      {loading ? (
        <div className="flex gap-3 overflow-hidden">
          {Array.from({ length: 10 }).map((_, i) => (
            <div key={i} className="shrink-0 w-24 sm:w-[150px] rounded-lg overflow-hidden bg-surface dark:bg-surface-dark border border-border dark:border-border-dark">
              <div className="aspect-[2/3] bg-gray-200 dark:bg-gray-800 animate-pulse" />
              <div className="p-1.5 space-y-1.5">
                <div className="h-3 bg-gray-200 dark:bg-gray-800 rounded animate-pulse w-3/4" />
                <div className="h-2.5 bg-gray-200 dark:bg-gray-800 rounded animate-pulse w-1/2" />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div
          {...scrollHandlers}
          className="flex gap-3 overflow-x-auto pb-2 -mx-4 px-4 scrollbar-thin scrollbar-thumb-gray-300 dark:scrollbar-thumb-gray-600 select-none"
        >
          {items!.map(item => (
            <MediaCard
              key={`${item.media_type}-${item.id}`}
              item={item}
              onClick={() => onSelect({ mediaType: item.media_type as 'movie' | 'tv', mediaId: item.id })}
              className="shrink-0 w-24 sm:w-[150px]"
              available={libraryIds.has(String(item.id))}
            />
          ))}
        </div>
      )}
    </div>
  )
}

type DiscoverFetchSectionProps = Pick<DiscoverSectionProps, 'path' | 'title' | 'onSelect' | 'libraryIds'>

function DiscoverFetchSection({ path, title, onSelect, libraryIds }: DiscoverFetchSectionProps) {
  const { data, loading } = useFetch<TMDBSearchResult>(`/api/tmdb/discover/${path}`)
  return <DiscoverSection path={path} title={title} data={data} loading={loading} onSelect={onSelect} libraryIds={libraryIds} />
}

function matchesRequester(req: OverseerrRequest, query: string): boolean {
  const rb = req.requestedBy
  if (!rb) return false
  return [rb.username, rb.plexUsername, rb.email]
    .some(f => f?.toLowerCase().includes(query))
}

export function Discover() {
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'

  const { data: configStatus } = useFetch<{ configured: boolean }>('/api/overseerr/configured')
  const overseerrConfigured = !!configStatus?.configured
  const { data: libraryData } = useFetch<{ ids: string[] }>('/api/library/tmdb-ids')
  const libraryIds = useMemo(() => new Set(libraryData?.ids ?? []), [libraryData])

  const [tab, setTab] = useState<Tab>('discover')
  const [searchResults, setSearchResults] = useState<TMDBSearchResult | null>(null)
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState('')
  const { stack, push: pushModal, pop: popModal } = useModalStack()

  const handleSelectMedia = useCallback((item: SelectedMedia) => {
    pushModal({ type: 'tmdb', mediaType: item.mediaType, mediaId: item.mediaId })
  }, [pushModal])

  const { searchInput, setSearchInput, search } = useDebouncedSearch(() => setSearchResults(null))

  const [requestFilter, setRequestFilter] = useState('all')
  const {
    searchInput: reqSearchInput,
    setSearchInput: setReqSearchInput,
    search: reqSearch,
  } = useDebouncedSearch()

  const titleMapRef = useRef(new Map<number, string>())
  const [titleMapVersion, setTitleMapVersion] = useState(0)
  const handleTitleResolved = (requestId: number, title: string) => {
    if (titleMapRef.current.get(requestId) !== title) {
      titleMapRef.current.set(requestId, title)
      setTitleMapVersion(v => v + 1)
    }
  }

  const requestsBaseUrl = tab === 'requests' && overseerrConfigured
    ? `/api/overseerr/requests?filter=${requestFilter}&sort=added`
    : null

  useEffect(() => {
    titleMapRef.current.clear()
    setTitleMapVersion(0)
  }, [requestsBaseUrl])

  const {
    items: requestItems,
    loading: requestsLoading,
    loadingMore,
    error: requestsError,
    sentinelRef,
    retry: retryRequests,
    refetch: refetchRequests,
  } = useInfiniteFetch<OverseerrRequest>(requestsBaseUrl, 30)

  const { data: counts } = useFetch<OverseerrRequestCount>(overseerrConfigured && isAdmin ? '/api/overseerr/requests/count' : null)

  useEffect(() => {
    if (!search) {
      setSearchResults(null)
      setSearching(false)
      return
    }
    setSearching(true)
    setError('')
    const controller = new AbortController()
    api.get<TMDBSearchResult>(`/api/tmdb/search?query=${encodeURIComponent(search)}`, controller.signal)
      .then(data => {
        if (controller.signal.aborted) return
        setSearchResults(data)
        setSearching(false)
      })
      .catch(err => {
        if (err instanceof Error && err.name === 'AbortError') return
        if (!controller.signal.aborted) {
          setError((err as Error).message)
          setSearching(false)
        }
      })
    return () => controller.abort()
  }, [search])

  const filteredRequests = reqSearch
    ? requestItems.filter(req => {
        const q = reqSearch.toLowerCase()
        const title = titleMapVersion >= 0 ? titleMapRef.current.get(req.id) : undefined
        if (title && title.toLowerCase().includes(q)) return true
        if (!title) return true
        return matchesRequester(req, q)
      })
    : requestItems

  const searchFiltered = searchResults?.results?.filter(isSelectableMedia)

  const pageTitle = overseerrConfigured ? 'Requests' : 'Discover'
  const pageSubtitle = overseerrConfigured
    ? 'Search and request movies & TV shows'
    : 'Browse trending movies & TV shows'

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">{pageTitle}</h1>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            {pageSubtitle}
          </p>
        </div>
      </div>

      {overseerrConfigured && (
        <div className="flex gap-1 mb-6 border-b border-border dark:border-border-dark">
          <button onClick={() => setTab('discover')} className={tabClass(tab === 'discover')}>
            Discover
          </button>
          <button onClick={() => setTab('requests')} className={tabClass(tab === 'requests')}>
            Requests
            {counts && counts.pending > 0 && isAdmin && (
              <span className="ml-1.5 text-[10px] font-semibold px-1.5 py-0.5 rounded-full bg-yellow-500/20 text-yellow-500">
                {counts.pending}
              </span>
            )}
          </button>
        </div>
      )}

      {tab === 'discover' && (
        <>
          <div className="mb-6">
            <SearchInput
              value={searchInput}
              onChange={setSearchInput}
              placeholder="Search movies & TV shows..."
            />
          </div>

          {error && (
            <div className="card p-4 mb-4 text-center text-red-500 dark:text-red-400">
              {error}
            </div>
          )}

          {search ? (
            <>
              {searching && <EmptyState icon="&#8635;" title="Loading..." />}

              {!searching && searchResults && (
                <p className="text-sm text-muted dark:text-muted-dark mb-3">
                  {searchResults.total_results} result{searchResults.total_results !== 1 ? 's' : ''} for &ldquo;{search}&rdquo;
                </p>
              )}

              {!searching && searchFiltered && searchFiltered.length === 0 && (
                <EmptyState icon="&#128270;" title="No results" description={`Nothing found for "${search}"`} />
              )}

              {!searching && searchFiltered && searchFiltered.length > 0 && (
                <div className={MEDIA_GRID_CLASS}>
                  {searchFiltered.map(item => (
                    <MediaCard
                      key={`${item.media_type}-${item.id}`}
                      item={item}
                      onClick={() => handleSelectMedia({ mediaType: item.media_type as 'movie' | 'tv', mediaId: item.id })}
                      available={libraryIds.has(String(item.id))}
                    />
                  ))}
                </div>
              )}
            </>
          ) : (
            <div className="space-y-8">
              {DISCOVER_CATEGORIES.map(cat => (
                <DiscoverFetchSection key={cat.path} path={cat.path} title={cat.title} onSelect={handleSelectMedia} libraryIds={libraryIds} />
              ))}
            </div>
          )}
        </>
      )}

      {tab === 'requests' && overseerrConfigured && (
        <>
          <div className="mb-4">
            <SearchInput
              value={reqSearchInput}
              onChange={setReqSearchInput}
              placeholder={isAdmin ? 'Search by title or requester...' : 'Search by title...'}
            />
          </div>

          <div className="flex gap-2 mb-4 flex-wrap">
            {['all', 'pending', 'approved', 'declined', 'processing', 'available'].map(f => (
              <button
                key={f}
                onClick={() => setRequestFilter(f)}
                className={`px-3 py-1.5 text-xs font-medium rounded-full transition-colors ${
                  requestFilter === f
                    ? 'bg-accent text-gray-900'
                    : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/20'
                }`}
              >
                {f.charAt(0).toUpperCase() + f.slice(1)}
              </button>
            ))}
          </div>

          {requestsLoading && <EmptyState icon="&#8635;" title="Loading..." />}

          {!requestsLoading && requestsError && requestItems.length === 0 && (
            <div className="card p-4 text-center">
              <p className="text-red-500 dark:text-red-400 mb-2">{requestsError}</p>
              <button onClick={retryRequests} className="text-sm hover:text-accent hover:underline">
                Try again
              </button>
            </div>
          )}

          {!requestsLoading && !requestsError && filteredRequests.length === 0 && (
            <EmptyState
              icon={reqSearch ? '&#128270;' : '&#128203;'}
              title={reqSearch ? 'No matches' : 'No requests'}
              description={reqSearch
                ? `No requests matching "${reqSearch}"`
                : 'No media requests found with this filter.'}
            />
          )}

          {!requestsLoading && filteredRequests.length > 0 && (
            <div className="space-y-3">
              {filteredRequests.map(req => (
                <RequestCard
                  key={req.id}
                  request={req}
                  isAdmin={isAdmin}
                  onAction={refetchRequests}
                  onTitleResolved={handleTitleResolved}
                />
              ))}
              <div ref={sentinelRef} />
              {loadingMore && (
                <div className="flex justify-center py-4">
                  <div className="h-6 w-6 border-2 border-accent border-t-transparent rounded-full animate-spin" />
                </div>
              )}
              {requestsError && (
                <div className="text-center py-4">
                  <p className="text-sm text-red-500 dark:text-red-400 mb-2">{requestsError}</p>
                  <button onClick={retryRequests} className="text-sm hover:text-accent hover:underline">
                    Try again
                  </button>
                </div>
              )}
            </div>
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
        />
      )}
    </div>
  )
}
