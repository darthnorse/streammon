import { useFetch } from './useFetch'
import type { ChildrenSeasons, ChildrenEpisodes } from '../types'

export function useSeasonsChildren(serverId: number, showId: string | null) {
  return useFetch<ChildrenSeasons>(
    showId ? `/api/servers/${serverId}/children/${encodeURIComponent(showId)}` : null,
  )
}

export function useEpisodesChildren(serverId: number, seasonId: string | null) {
  return useFetch<ChildrenEpisodes>(
    seasonId ? `/api/servers/${serverId}/children/${encodeURIComponent(seasonId)}` : null,
  )
}
