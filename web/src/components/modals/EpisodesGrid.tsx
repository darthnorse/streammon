import { thumbUrl } from '../../lib/format'
import { TMDB_IMG } from '../../lib/tmdb'
import type { Episode, ModalEntry, TMDBSeasonEpisode } from '../../types'

interface EpisodesGridProps {
  serverId: number
  episodes: Episode[]
  tmdbEpisodes?: TMDBSeasonEpisode[]
  pushModal: (entry: ModalEntry) => void
}

export function EpisodesGrid({ serverId, episodes, tmdbEpisodes, pushModal }: EpisodesGridProps) {
  const tmdbByNum = new Map<number, TMDBSeasonEpisode>(
    (tmdbEpisodes ?? []).map(e => [e.episode_number, e]),
  )

  if (!episodes.length) {
    return <div className="text-sm text-muted dark:text-muted-dark">No episodes</div>
  }

  return (
    <div className="space-y-2">
      <div className="text-sm font-medium text-gray-900 dark:text-gray-100">Episodes</div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {episodes.map(ep => {
          const tmdb = tmdbByNum.get(ep.number)
          const still = tmdb?.still_path ? `${TMDB_IMG}/w300${tmdb.still_path}` : null
          const summary = tmdb?.overview || ep.summary || ''
          return (
            <button
              key={ep.id}
              onClick={() => pushModal({ type: 'episode', serverId, itemId: ep.id })}
              className="flex gap-3 text-left group bg-gray-50 dark:bg-white/5 rounded-lg p-2 hover:bg-gray-100 dark:hover:bg-white/10"
            >
              <div className="shrink-0 w-32 aspect-video rounded overflow-hidden bg-panel dark:bg-panel-dark">
                {still ? (
                  <img src={still} alt="" className="w-full h-full object-cover" loading="lazy" />
                ) : ep.thumb_url ? (
                  <img src={thumbUrl(serverId, ep.thumb_url)} alt="" className="w-full h-full object-cover" loading="lazy" />
                ) : (
                  <div className="w-full h-full flex items-center justify-center text-xl opacity-20">📺</div>
                )}
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium truncate group-hover:text-accent group-hover:underline">
                  {ep.number}. {ep.title}
                </div>
                {summary && (
                  <div className="text-xs text-muted dark:text-muted-dark line-clamp-3 mt-0.5">{summary}</div>
                )}
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}
