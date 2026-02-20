import { TMDB_IMG } from '../lib/tmdb'
import { mediaStatusBadge } from '../lib/overseerr'
import type { OverseerrMediaResult, TMDBMediaResult } from '../types'

type MediaItem = OverseerrMediaResult | TMDBMediaResult

function isTMDB(item: MediaItem): item is TMDBMediaResult {
  return 'poster_path' in item
}

function normalize(item: MediaItem) {
  if (isTMDB(item)) {
    return {
      title: item.title || item.name || 'Unknown',
      year: item.release_date?.slice(0, 4) || item.first_air_date?.slice(0, 4),
      posterPath: item.poster_path,
      mediaType: item.media_type,
      voteAverage: item.vote_average,
      mediaStatus: undefined as number | undefined,
    }
  }
  return {
    title: item.title || item.name || 'Unknown',
    year: item.releaseDate?.slice(0, 4) || item.firstAirDate?.slice(0, 4),
    posterPath: item.posterPath,
    mediaType: item.mediaType,
    voteAverage: item.voteAverage,
    mediaStatus: item.mediaInfo?.status,
  }
}

interface MediaCardProps {
  item: MediaItem
  onClick: () => void
  className?: string
  available?: boolean
}

export function MediaCard({ item, onClick, className, available }: MediaCardProps) {
  const { title, year, posterPath, mediaType, voteAverage, mediaStatus } = normalize(item)

  return (
    <button
      onClick={onClick}
      className={`text-left group relative rounded-lg overflow-hidden
                 bg-surface dark:bg-surface-dark border border-border dark:border-border-dark
                 hover:border-accent/40 transition-all duration-200 focus:outline-none
                 focus:ring-2 focus:ring-accent/50 ${className ?? ''}`}
    >
      <div className="aspect-[2/3] bg-gray-200 dark:bg-gray-800 relative">
        {posterPath ? (
          <img
            src={`${TMDB_IMG}/w185${posterPath}`}
            alt={title}
            className="w-full h-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center text-muted dark:text-muted-dark text-xl">
            {mediaType === 'movie' ? '🎬' : '📺'}
          </div>
        )}
        {mediaStatus && mediaStatus > 1 ? (
          <div className="absolute top-1 right-1">
            {mediaStatusBadge(mediaStatus)}
          </div>
        ) : available ? (
          <div className="absolute top-1 right-1">
            <span className="text-[10px] font-semibold px-1.5 py-0.5 rounded-full bg-green-600/90 text-white">
              Available
            </span>
          </div>
        ) : null}
        <div className="absolute top-1 left-1">
          <span className="text-[10px] font-medium px-1 py-0.5 rounded bg-black/60 text-white">
            {mediaType === 'movie' ? 'Movie' : 'TV'}
          </span>
        </div>
      </div>
      <div className="p-1.5">
        <h3 className="text-xs font-medium truncate group-hover:text-accent transition-colors">
          {title}
        </h3>
        <div className="flex items-center gap-1.5 mt-0.5">
          {year && <span className="text-[10px] text-muted dark:text-muted-dark">{year}</span>}
          {voteAverage != null && voteAverage > 0 && (
            <span className="text-[10px] text-muted dark:text-muted-dark">
              ★ {voteAverage.toFixed(1)}
            </span>
          )}
        </div>
      </div>
    </button>
  )
}
