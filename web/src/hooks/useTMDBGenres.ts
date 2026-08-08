import { useFetch } from './useFetch'
import type { DiscoverMediaType } from '../lib/discoverFilters'
import type { TMDBGenre } from '../types'

const EMPTY: TMDBGenre[] = []

// useFetch retains the previous url's data while a new url loads, so a type
// switch is gated on the url/loading state here instead, rather than in
// useFetch, whose other callers depend on the retain-while-loading behaviour.
export function useTMDBGenres(mediaType: DiscoverMediaType | null) {
  const url = mediaType ? `/api/tmdb/genres/${mediaType}` : null
  const { data, loading } = useFetch<{ genres: TMDBGenre[] }>(url)
  return { genres: loading || !url ? EMPTY : data?.genres ?? EMPTY, loading }
}
