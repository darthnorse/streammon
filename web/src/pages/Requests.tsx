import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'
import { useDebouncedSearch } from '../hooks/useDebouncedSearch'
import { useHorizontalScroll } from '../hooks/useHorizontalScroll'
import { useAuth } from '../context/AuthContext'
import { DISCOVER_CATEGORIES, MEDIA_GRID_CLASS } from '../lib/constants'
import { EmptyState } from '../components/EmptyState'
import { MediaCard } from '../components/MediaCard'
import { ChevronIcon } from '../components/ChevronIcon'
import { OverseerrDetailModal } from '../components/OverseerrDetailModal'
import { TMDB_IMG, requestStatusBadge } from '../lib/overseerr'
import type {
  OverseerrSearchResult,
  OverseerrMediaResult,
  OverseerrRequestList,
  OverseerrRequestCount,
  OverseerrRequest,
  OverseerrMovieDetails,
  OverseerrTVDetails,
} from '../types'

type Tab = 'discover' | 'requests'
type SelectedItem = OverseerrMediaResult & { mediaType: 'movie' | 'tv' }

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

function RequestCard({
  request,
  isAdmin,
  onAction,
}: {
  request: OverseerrRequest
  isAdmin: boolean
  onAction: () => void
}) {
  const [acting, setActing] = useState(false)

  const detailUrl = request.media?.tmdbId
    ? `/api/overseerr/${request.type === 'movie' ? 'movie' : 'tv'}/${request.media.tmdbId}`
    : null
  const { data: details } = useFetch<OverseerrMovieDetails | OverseerrTVDetails>(detailUrl)

  const mediaTitle = details
    ? ('title' in details ? details.title : details.name)
    : null
  const posterPath = details?.posterPath
  const year = details
    ? ('releaseDate' in details
        ? (details as OverseerrMovieDetails).releaseDate?.slice(0, 4)
        : (details as OverseerrTVDetails).firstAirDate?.slice(0, 4))
    : null

  async function handleAction(action: 'approve' | 'decline') {
    setActing(true)
    try {
      await api.post(`/api/overseerr/requests/${request.id}/${action}`)
      onAction()
    } catch {
      // silently fail — refetch will show current state
    } finally {
      setActing(false)
    }
  }

  const typeBadge = request.type === 'movie' ? 'Movie' : 'TV Show'
  const requester = request.requestedBy?.username || request.requestedBy?.plexUsername || request.requestedBy?.email || 'Unknown'

  return (
    <div className="card p-4 flex items-start gap-4">
      {posterPath ? (
        <img
          src={`${TMDB_IMG}/w92${posterPath}`}
          alt={mediaTitle ?? ''}
          className="w-12 h-[72px] rounded object-cover bg-gray-200 dark:bg-gray-800 shrink-0"
          loading="lazy"
        />
      ) : (
        <div className="w-12 h-[72px] rounded bg-gray-200 dark:bg-gray-800 shrink-0 flex items-center justify-center text-muted dark:text-muted-dark text-lg">
          {request.type === 'movie' ? '\uD83C\uDFAC' : '\uD83D\uDCFA'}
        </div>
      )}
      <div className="flex-1 min-w-0">
        <p className="font-medium truncate">
          {mediaTitle ?? <span className="text-muted dark:text-muted-dark">Loading...</span>}
          {year && <span className="text-sm text-muted dark:text-muted-dark ml-1.5">({year})</span>}
        </p>
        <div className="flex items-center gap-2 flex-wrap mt-1">
          <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-gray-100 dark:bg-white/10">
            {typeBadge}
          </span>
          {requestStatusBadge(request.status)}
          {request.is4k && (
            <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-400">4K</span>
          )}
        </div>
        <p className="text-sm text-muted dark:text-muted-dark mt-1">
          Requested by <span className="font-medium text-foreground dark:text-foreground-dark">{requester}</span>
          {' · '}
          {new Date(request.createdAt).toLocaleDateString()}
        </p>
      </div>
      {isAdmin && request.status === 1 && (
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={() => handleAction('approve')}
            disabled={acting}
            className="px-3 py-1.5 text-xs font-medium rounded-md bg-green-500/20 text-green-400
                       hover:bg-green-500/30 transition-colors disabled:opacity-50"
          >
            Approve
          </button>
          <button
            onClick={() => handleAction('decline')}
            disabled={acting}
            className="px-3 py-1.5 text-xs font-medium rounded-md bg-red-500/20 text-red-400
                       hover:bg-red-500/30 transition-colors disabled:opacity-50"
          >
            Decline
          </button>
        </div>
      )}
    </div>
  )
}

function DiscoverSection({
  path,
  title,
  data,
  loading,
  onSelect,
}: {
  path: string
  title: string
  data: OverseerrSearchResult | null
  loading: boolean
  onSelect: (item: SelectedItem) => void
}) {
  const { canScrollLeft, canScrollRight, scrollBy, ...scrollHandlers } = useHorizontalScroll()
  const items = data?.results?.filter(
    (r): r is SelectedItem => r.mediaType === 'movie' || r.mediaType === 'tv'
  )
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
              to={`/requests/discover/${path}`}
              className="text-xs text-muted dark:text-muted-dark hover:text-accent transition-colors mr-1"
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
            <div key={i} className="shrink-0 w-[120px] rounded-lg overflow-hidden bg-surface dark:bg-surface-dark border border-border dark:border-border-dark">
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
              key={`${item.mediaType}-${item.id}`}
              item={item}
              onClick={() => onSelect(item)}
              className="shrink-0 w-[120px]"
            />
          ))}
        </div>
      )}
    </div>
  )
}

function DiscoverFetchSection({ path, title, onSelect }: {
  path: string
  title: string
  onSelect: (item: SelectedItem) => void
}) {
  const { data, loading } = useFetch<OverseerrSearchResult>(`/api/overseerr/discover/${path}`)
  return <DiscoverSection path={path} title={title} data={data} loading={loading} onSelect={onSelect} />
}

export function Requests() {
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'

  const [tab, setTab] = useState<Tab>('discover')
  const [searchResults, setSearchResults] = useState<OverseerrSearchResult | null>(null)
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState('')
  const [selectedItem, setSelectedItem] = useState<SelectedItem | null>(null)

  const { data: configStatus } = useFetch<{ configured: boolean }>('/api/overseerr/configured')
  const configured = !!configStatus?.configured

  const { searchInput, setSearchInput, search } = useDebouncedSearch(() => setSearchResults(null))

  const [requestFilter, setRequestFilter] = useState('all')
  const [requestsTick, setRequestsTick] = useState(0)
  // _t is a cache-busting parameter incremented after approve/decline actions
  const requestsUrl = tab === 'requests' && configured
    ? `/api/overseerr/requests?take=20&skip=0&filter=${requestFilter}&sort=added&_t=${requestsTick}`
    : null
  const { data: requests, loading: requestsLoading } = useFetch<OverseerrRequestList>(requestsUrl)
  const { data: counts } = useFetch<OverseerrRequestCount>(configured ? '/api/overseerr/requests/count' : null)

  useEffect(() => {
    if (!search || !configured) {
      setSearchResults(null)
      setSearching(false)
      return
    }
    setSearching(true)
    setError('')
    const controller = new AbortController()
    api.get<OverseerrSearchResult>(`/api/overseerr/search?query=${encodeURIComponent(search)}`, controller.signal)
      .then(data => {
        setSearchResults(data)
        setSearching(false)
      })
      .catch(err => {
        if ((err as Error).name !== 'AbortError') {
          setError((err as Error).message)
          setSearching(false)
        }
      })
    return () => controller.abort()
  }, [search, configured])

  if (!configured) {
    return (
      <div>
        <h1 className="text-2xl font-semibold mb-6">Requests</h1>
        <EmptyState
          icon="&#127916;"
          title="Overseerr Not Configured"
          description={isAdmin
            ? 'Configure Overseerr in Settings → Integrations to enable media requests.'
            : 'Media requests are not available yet. Ask an admin to configure Overseerr.'}
        />
      </div>
    )
  }

  const searchFiltered = searchResults?.results?.filter(
    (r): r is SelectedItem => r.mediaType === 'movie' || r.mediaType === 'tv'
  )

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">Requests</h1>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            Search and request movies & TV shows
          </p>
        </div>
        {counts && counts.pending > 0 && isAdmin && (
          <span className="text-xs font-medium px-2.5 py-1 rounded-full bg-yellow-500/20 text-yellow-500">
            {counts.pending} pending
          </span>
        )}
      </div>

      <div className="flex gap-1 mb-6 border-b border-border dark:border-border-dark">
        <button onClick={() => setTab('discover')} className={tabClass(tab === 'discover')}>
          Discover
        </button>
        <button onClick={() => setTab('requests')} className={tabClass(tab === 'requests')}>
          Requests
          {counts && counts.total > 0 && (
            <span className="ml-1.5 text-xs text-muted dark:text-muted-dark">({counts.total})</span>
          )}
        </button>
      </div>

      {tab === 'discover' && (
        <>
          <div className="mb-6">
            <input
              type="text"
              value={searchInput}
              onChange={e => setSearchInput(e.target.value)}
              placeholder="Search movies & TV shows..."
              className="w-full px-4 py-3 rounded-lg text-sm
                bg-surface dark:bg-surface-dark
                border border-border dark:border-border-dark
                focus:outline-none focus:border-accent/50 focus:ring-2 focus:ring-accent/20
                transition-colors placeholder:text-muted/40 dark:placeholder:text-muted-dark/40"
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
                  {searchResults.totalResults} result{searchResults.totalResults !== 1 ? 's' : ''} for &ldquo;{search}&rdquo;
                </p>
              )}

              {!searching && searchFiltered && searchFiltered.length === 0 && (
                <EmptyState icon="&#128270;" title="No results" description={`Nothing found for "${search}"`} />
              )}

              {!searching && searchFiltered && searchFiltered.length > 0 && (
                <div className={MEDIA_GRID_CLASS}>
                  {searchFiltered.map(item => (
                    <MediaCard
                      key={`${item.mediaType}-${item.id}`}
                      item={item}
                      onClick={() => setSelectedItem(item)}
                    />
                  ))}
                </div>
              )}
            </>
          ) : (
            <div className="space-y-8">
              {DISCOVER_CATEGORIES.map(cat => (
                <DiscoverFetchSection key={cat.path} path={cat.path} title={cat.title} onSelect={setSelectedItem} />
              ))}
            </div>
          )}
        </>
      )}

      {tab === 'requests' && (
        <>
          <div className="flex gap-2 mb-4 flex-wrap">
            {['all', 'pending', 'approved', 'processing', 'available'].map(f => (
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

          {!requestsLoading && requests && requests.results.length === 0 && (
            <EmptyState icon="&#128203;" title="No requests" description="No media requests found with this filter." />
          )}

          {!requestsLoading && requests && requests.results.length > 0 && (
            <div className="space-y-3">
              {requests.results.map(req => (
                <RequestCard
                  key={req.id}
                  request={req}
                  isAdmin={isAdmin}
                  onAction={() => setRequestsTick(t => t + 1)}
                />
              ))}
            </div>
          )}
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
