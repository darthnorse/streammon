import { useFetch } from './useFetch'
import type { DiscoverMediaType } from '../lib/discoverFilters'
import type { TMDBGenre } from '../types'

const EMPTY: TMDBGenre[] = []

// useFetch retains the previous url's data while a new url loads, and its
// `loading` flag only flips to true inside the effect - one render after the
// url prop itself already changed. Gating on `dataUrl === url` instead of
// `loading` closes that one-render gap: data is treated as fresh only once it
// actually belongs to the current url, so a movie->tv switch can never
// surface movie genres, even for a single committed render.
export function useTMDBGenres(mediaType: DiscoverMediaType | null) {
  const url = mediaType ? `/api/tmdb/genres/${mediaType}` : null
  const { data, dataUrl, loading } = useFetch<{ genres: TMDBGenre[] }>(url)
  if (!url) return { genres: EMPTY, loading: false }
  const fresh = dataUrl === url
  return { genres: fresh ? data?.genres ?? EMPTY : EMPTY, loading: loading || !fresh }
}
