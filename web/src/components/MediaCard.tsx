import { TMDB_IMG, mediaStatusBadge } from '../lib/overseerr'
import type { OverseerrMediaResult } from '../types'

interface MediaCardProps {
  item: OverseerrMediaResult
  onClick: () => void
  className?: string
}

export function MediaCard({ item, onClick, className }: MediaCardProps) {
  const title = item.title || item.name || 'Unknown'
  const year = item.releaseDate?.slice(0, 4) || item.firstAirDate?.slice(0, 4)
  const mediaStatus = item.mediaInfo?.status

  return (
    <button
      onClick={onClick}
      className={`text-left group relative rounded-lg overflow-hidden
                 bg-surface dark:bg-surface-dark border border-border dark:border-border-dark
                 hover:border-accent/40 transition-all duration-200 focus:outline-none
                 focus:ring-2 focus:ring-accent/50 ${className ?? ''}`}
    >
      <div className="aspect-[2/3] bg-gray-200 dark:bg-gray-800 relative">
        {item.posterPath ? (
          <img
            src={`${TMDB_IMG}/w185${item.posterPath}`}
            alt={title}
            className="w-full h-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center text-muted dark:text-muted-dark text-xl">
            {item.mediaType === 'movie' ? '🎬' : '📺'}
          </div>
        )}
        {mediaStatus && mediaStatus > 1 && (
          <div className="absolute top-1 right-1">
            {mediaStatusBadge(mediaStatus)}
          </div>
        )}
        <div className="absolute top-1 left-1">
          <span className="text-[10px] font-medium px-1 py-0.5 rounded bg-black/60 text-white">
            {item.mediaType === 'movie' ? 'Movie' : 'TV'}
          </span>
        </div>
      </div>
      <div className="p-1.5">
        <h3 className="text-xs font-medium truncate group-hover:text-accent transition-colors">
          {title}
        </h3>
        <div className="flex items-center gap-1.5 mt-0.5">
          {year && <span className="text-[10px] text-muted dark:text-muted-dark">{year}</span>}
          {item.voteAverage != null && item.voteAverage > 0 && (
            <span className="text-[10px] text-muted dark:text-muted-dark">
              ★ {item.voteAverage.toFixed(1)}
            </span>
          )}
        </div>
      </div>
    </button>
  )
}
