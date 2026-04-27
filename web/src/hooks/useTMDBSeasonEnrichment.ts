import { useFetch } from './useFetch'
import type { TMDBSeasonDetails } from '../types'

export function useTMDBSeasonEnrichment(tmdbId: string | undefined, seasonNumber: number | undefined) {
  return useFetch<TMDBSeasonDetails>(
    tmdbId && seasonNumber != null
      ? `/api/tmdb/tv/${encodeURIComponent(tmdbId)}/season/${seasonNumber}`
      : null,
  )
}
