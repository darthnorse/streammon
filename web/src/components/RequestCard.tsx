import { useState } from 'react'
import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'
import { TMDB_IMG, requestStatusBadge } from '../lib/overseerr'
import type { OverseerrRequest, OverseerrMovieDetails, OverseerrTVDetails } from '../types'

function getMediaMeta(details: OverseerrMovieDetails | OverseerrTVDetails | null) {
  if (!details) return { title: null, posterPath: null, year: null }
  if ('title' in details) {
    return {
      title: details.title,
      posterPath: details.posterPath ?? null,
      year: details.releaseDate?.slice(0, 4) ?? null,
    }
  }
  return {
    title: details.name,
    posterPath: details.posterPath ?? null,
    year: details.firstAirDate?.slice(0, 4) ?? null,
  }
}

export function RequestCard({
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
  const { title: mediaTitle, posterPath, year } = getMediaMeta(details)

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
