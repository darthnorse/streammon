import { useState, useEffect, useRef } from 'react'
import { api } from '../lib/api'
import { useFetch } from '../hooks/useFetch'
import { TMDB_IMG, requestStatusBadge } from '../lib/overseerr'
import { ConfirmDialog } from './shared/ConfirmDialog'
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

type Props = {
  request: OverseerrRequest
  isAdmin: boolean
  onAction: () => void
  onTitleResolved?: (requestId: number, title: string) => void
}

export function RequestCard({ request, isAdmin, onAction, onTitleResolved }: Props) {
  const [acting, setActing] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  const detailUrl = request.media?.tmdbId
    ? `/api/overseerr/${request.type === 'movie' ? 'movie' : 'tv'}/${request.media.tmdbId}`
    : null
  const { data: details } = useFetch<OverseerrMovieDetails | OverseerrTVDetails>(detailUrl)
  const { title: mediaTitle, posterPath, year } = getMediaMeta(details)

  const onTitleResolvedRef = useRef(onTitleResolved)
  onTitleResolvedRef.current = onTitleResolved

  useEffect(() => {
    if (mediaTitle && onTitleResolvedRef.current) {
      onTitleResolvedRef.current(request.id, mediaTitle)
    }
  }, [mediaTitle, request.id])

  async function handleAction(action: 'approve' | 'decline') {
    setActing(true)
    try {
      await api.post(`/api/overseerr/requests/${request.id}/${action}`)
    } catch {
      // swallow — refetch below will show current state
    } finally {
      setActing(false)
      onAction()
    }
  }

  async function handleDelete() {
    setActing(true)
    try {
      await api.del(`/api/overseerr/requests/${request.id}`)
    } catch {
      // swallow — refetch below will show current state
    } finally {
      setActing(false)
      setShowDeleteConfirm(false)
      onAction()
    }
  }

  const typeBadge = request.type === 'movie' ? 'Movie' : 'TV Show'
  const displayTitle = mediaTitle ?? 'this request'

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
          {isAdmin && (
            <>Requested by <span className="font-medium text-foreground dark:text-foreground-dark">
              {request.requestedBy?.username || request.requestedBy?.plexUsername || request.requestedBy?.email || 'Unknown'}
            </span>{' · '}</>
          )}
          {new Date(request.createdAt).toLocaleDateString()}
        </p>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        {isAdmin && request.status === 1 && (
          <>
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
          </>
        )}
        <button
          onClick={() => setShowDeleteConfirm(true)}
          disabled={acting}
          className="p-1.5 text-muted dark:text-muted-dark hover:text-red-400 transition-colors disabled:opacity-50"
          title="Delete request"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
              d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </button>
      </div>

      {showDeleteConfirm && (
        <ConfirmDialog
          title="Delete Request"
          message={`Are you sure you want to delete the request for "${displayTitle}"?`}
          confirmLabel="Delete"
          onConfirm={handleDelete}
          onCancel={() => setShowDeleteConfirm(false)}
          isDestructive
          disabled={acting}
        />
      )}
    </div>
  )
}
