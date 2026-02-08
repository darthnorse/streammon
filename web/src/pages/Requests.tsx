import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'
import { useDebouncedSearch } from '../hooks/useDebouncedSearch'
import { useAuth } from '../context/AuthContext'
import { EmptyState } from '../components/EmptyState'
import { OverseerrDetailModal } from '../components/OverseerrDetailModal'
import { TMDB_IMG, mediaStatusBadge, requestStatusBadge } from '../lib/overseerr'
import type {
  OverseerrSearchResult,
  OverseerrMediaResult,
  OverseerrRequestList,
  OverseerrRequestCount,
  OverseerrRequest,
} from '../types'

type Tab = 'discover' | 'requests'

function MediaCard({ item, onClick }: { item: OverseerrMediaResult; onClick: () => void }) {
  const title = item.title || item.name || 'Unknown'
  const year = item.releaseDate?.slice(0, 4) || item.firstAirDate?.slice(0, 4)
  const mediaStatus = item.mediaInfo?.status

  return (
    <button
      onClick={onClick}
      className="text-left group relative rounded-lg overflow-hidden
                 bg-surface dark:bg-surface-dark border border-border dark:border-border-dark
                 hover:border-accent/40 transition-all duration-200 focus:outline-none
                 focus:ring-2 focus:ring-accent/50"
    >
      <div className="aspect-[2/3] bg-gray-200 dark:bg-gray-800 relative">
        {item.posterPath ? (
          <img
            src={`${TMDB_IMG}/w300${item.posterPath}`}
            alt={title}
            className="w-full h-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center text-muted dark:text-muted-dark text-3xl">
            {item.mediaType === 'movie' ? '🎬' : '📺'}
          </div>
        )}
        {mediaStatus && mediaStatus > 1 && (
          <div className="absolute top-2 right-2">
            {mediaStatusBadge(mediaStatus)}
          </div>
        )}
        <div className="absolute top-2 left-2">
          <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-black/60 text-white">
            {item.mediaType === 'movie' ? 'Movie' : 'TV'}
          </span>
        </div>
      </div>
      <div className="p-2.5">
        <h3 className="text-sm font-medium truncate group-hover:text-accent transition-colors">
          {title}
        </h3>
        <div className="flex items-center gap-2 mt-0.5">
          {year && <span className="text-xs text-muted dark:text-muted-dark">{year}</span>}
          {item.voteAverage != null && item.voteAverage > 0 && (
            <span className="text-xs text-muted dark:text-muted-dark">
              ★ {item.voteAverage.toFixed(1)}
            </span>
          )}
        </div>
      </div>
    </button>
  )
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

  const title = request.type === 'movie' ? 'Movie' : 'TV Show'
  const requester = request.requestedBy?.username || request.requestedBy?.plexUsername || request.requestedBy?.email || 'Unknown'

  return (
    <div className="card p-4 flex items-center gap-4">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-gray-100 dark:bg-white/10">
            {title}
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

export function Requests() {
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'

  const [tab, setTab] = useState<Tab>('discover')
  const [searchResults, setSearchResults] = useState<OverseerrSearchResult | null>(null)
  const [trending, setTrending] = useState<OverseerrSearchResult | null>(null)
  const [searching, setSearching] = useState(false)
  const [trendingLoading, setTrendingLoading] = useState(false)
  const [trendingError, setTrendingError] = useState('')
  const [error, setError] = useState('')
  const [selectedItem, setSelectedItem] = useState<OverseerrMediaResult | null>(null)

  const { data: configStatus } = useFetch<{ configured: boolean }>('/api/overseerr/configured')
  const configured = !!configStatus?.configured

  const resetPage = useCallback(() => {
    setSearchResults(null)
  }, [])
  const { searchInput, setSearchInput, search } = useDebouncedSearch(resetPage)

  const [requestFilter, setRequestFilter] = useState('all')
  const [requestsTick, setRequestsTick] = useState(0)
  // _t is a cache-busting parameter incremented after approve/decline actions
  const requestsUrl = tab === 'requests' && configured
    ? `/api/overseerr/requests?take=20&skip=0&filter=${requestFilter}&sort=added&_t=${requestsTick}`
    : null
  const { data: requests, loading: requestsLoading } = useFetch<OverseerrRequestList>(requestsUrl)
  const { data: counts } = useFetch<OverseerrRequestCount>(configured ? '/api/overseerr/requests/count' : null)

  useEffect(() => {
    if (!configured) return
    const controller = new AbortController()
    setTrendingLoading(true)
    api.get<OverseerrSearchResult>('/api/overseerr/discover/trending', controller.signal)
      .then(data => setTrending(data))
      .catch(err => {
        if ((err as Error).name !== 'AbortError') {
          setTrendingError((err as Error).message)
        }
      })
      .finally(() => setTrendingLoading(false))
    return () => controller.abort()
  }, [configured])

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

  const displayResults = search ? searchResults?.results : trending?.results
  const filteredResults = displayResults?.filter(r => r.mediaType === 'movie' || r.mediaType === 'tv')
  const isLoading = search ? searching : trendingLoading

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
        <button
          onClick={() => setTab('discover')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            tab === 'discover'
              ? 'border-accent text-accent'
              : 'border-transparent text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
          }`}
        >
          Discover
        </button>
        <button
          onClick={() => setTab('requests')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            tab === 'requests'
              ? 'border-accent text-accent'
              : 'border-transparent text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
          }`}
        >
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

          {(error || trendingError) && (
            <div className="card p-4 mb-4 text-center text-red-500 dark:text-red-400">
              {error || trendingError}
            </div>
          )}

          {!search && !trendingLoading && trending && (
            <h2 className="text-sm font-medium text-muted dark:text-muted-dark mb-3 uppercase tracking-wider">
              Trending
            </h2>
          )}

          {search && searchResults && !searching && (
            <p className="text-sm text-muted dark:text-muted-dark mb-3">
              {searchResults.totalResults} result{searchResults.totalResults !== 1 ? 's' : ''} for &ldquo;{search}&rdquo;
            </p>
          )}

          {isLoading && <EmptyState icon="&#8635;" title="Loading..." />}

          {!isLoading && filteredResults && filteredResults.length === 0 && search && (
            <EmptyState icon="&#128270;" title="No results" description={`Nothing found for "${search}"`} />
          )}

          {!isLoading && filteredResults && filteredResults.length > 0 && (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
              {filteredResults.map(item => (
                <MediaCard
                  key={`${item.mediaType}-${item.id}`}
                  item={item}
                  onClick={() => setSelectedItem(item)}
                />
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
          mediaType={selectedItem.mediaType as 'movie' | 'tv'}
          mediaId={selectedItem.id}
          onClose={() => setSelectedItem(null)}
        />
      )}
    </div>
  )
}
