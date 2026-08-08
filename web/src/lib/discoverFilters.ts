export type DiscoverSort = 'popularity' | 'rating' | 'newest'
export type DiscoverMediaType = 'movie' | 'tv'

export interface DiscoverFilters {
  year: number | null
  genres: number[]
  sort: DiscoverSort
  rating: number | null
  hideOwned: boolean
  type: DiscoverMediaType | null
}

export interface DiscoverCategoryCaps {
  readonly year: boolean
  readonly type: boolean
  readonly mediaType: DiscoverMediaType | null
}

// Declared separately so `genres` keeps its number[] type; Object.freeze's
// array overload would otherwise widen it to a non-assignable readonly type.
const FROZEN_EMPTY_GENRES: number[] = []
Object.freeze(FROZEN_EMPTY_GENRES)

export const EMPTY_FILTERS: DiscoverFilters = Object.freeze({
  year: null,
  genres: FROZEN_EMPTY_GENRES,
  sort: 'popularity',
  rating: null,
  hideOwned: false,
  type: null,
})

export const MAX_GENRES = 5

const CAPS: Record<string, DiscoverCategoryCaps> = {
  trending: Object.freeze({ year: true, type: true, mediaType: null }),
  movies: Object.freeze({ year: true, type: false, mediaType: 'movie' }),
  'movies/upcoming': Object.freeze({ year: false, type: false, mediaType: 'movie' }),
  tv: Object.freeze({ year: true, type: false, mediaType: 'tv' }),
  'tv/upcoming': Object.freeze({ year: false, type: false, mediaType: 'tv' }),
}

// Returns the shared frozen object so consumers can memoise on identity.
export function categoryCaps(path: string): DiscoverCategoryCaps | null {
  return CAPS[path] ?? null
}

const SORTS: DiscoverSort[] = ['popularity', 'rating', 'newest']

function parseSort(raw: string | null): DiscoverSort {
  return SORTS.find(s => s === raw) ?? 'popularity'
}

function parseType(raw: string | null): DiscoverMediaType | null {
  return raw === 'movie' || raw === 'tv' ? raw : null
}

function parseGenres(raw: string | null): number[] {
  if (!raw) return []
  const ids = raw.split(',').map(part => Number(part))
  if (ids.length > MAX_GENRES) return []
  if (ids.some(id => !Number.isInteger(id) || id <= 0)) return []
  return ids
}

// The ceiling matches the backend: a future floor is unsatisfiable on a
// non-upcoming category and contradicts the newest sort's today ceiling.
function parseYear(raw: string | null, now: Date): number | null {
  if (!raw) return null
  const year = Number(raw)
  if (!Number.isInteger(year) || year < 1900 || year > now.getUTCFullYear()) return null
  return year
}

function parseRating(raw: string | null): number | null {
  if (!raw) return null
  const rating = Number(raw)
  if (!Number.isFinite(rating) || rating <= 0 || rating > 10) return null
  return rating
}

// On a mixed category (caps.type), the backend 400s any filtered request
// that lacks ?type=, so a missing type must drop the other server-side
// filters here rather than let an incoherent combination reach filtersToQuery.
export function filtersFromParams(params: URLSearchParams, caps: DiscoverCategoryCaps): DiscoverFilters {
  const type = caps.type ? parseType(params.get('type')) : null
  const hideOwned = params.get('hide_owned') === '1'
  if (caps.type && type === null) {
    return { ...EMPTY_FILTERS, hideOwned }
  }
  return {
    year: caps.year ? parseYear(params.get('year'), new Date()) : null,
    genres: parseGenres(params.get('genres')),
    sort: parseSort(params.get('sort')),
    rating: parseRating(params.get('rating')),
    hideOwned,
    type,
  }
}

// apiParams holds everything the backend understands; hideOwned is client-side only.
function apiParams(filters: DiscoverFilters, caps: DiscoverCategoryCaps): URLSearchParams {
  const params = new URLSearchParams()
  if (caps.year && filters.year !== null) params.set('year', String(filters.year))
  if (filters.genres.length > 0) params.set('genres', filters.genres.join(','))
  if (filters.sort !== 'popularity') params.set('sort', filters.sort)
  if (filters.rating !== null) params.set('rating', String(filters.rating))
  if (caps.type && filters.type !== null) params.set('type', filters.type)
  return params
}

export function filtersToParams(filters: DiscoverFilters, caps: DiscoverCategoryCaps): URLSearchParams {
  const params = apiParams(filters, caps)
  if (filters.hideOwned) params.set('hide_owned', '1')
  return params
}

export function filtersToQuery(filters: DiscoverFilters, caps: DiscoverCategoryCaps): string {
  return apiParams(filters, caps).toString()
}

export function activeFilterCount(filters: DiscoverFilters, caps: DiscoverCategoryCaps): number {
  let count = 0
  if (caps.year && filters.year !== null) count++
  if (filters.genres.length > 0) count++
  if (filters.sort !== 'popularity') count++
  if (filters.rating !== null) count++
  if (caps.type && filters.type !== null) count++
  if (filters.hideOwned) count++
  return count
}

// Genre IDs differ between movie and TV, and the backend rejects server-side
// filters that arrive without a type, so both transitions must drop state.
export function setMediaType(filters: DiscoverFilters, type: DiscoverMediaType | null): DiscoverFilters {
  if (type === filters.type) return filters
  if (type === null) return { ...EMPTY_FILTERS, hideOwned: filters.hideOwned }
  return { ...filters, type, genres: [] }
}

const OLDER_YEARS = [2020, 2015, 2010, 2000]

export function yearOptions(now: Date): number[] {
  const current = now.getUTCFullYear()
  const recent = [0, 1, 2, 3, 4].map(offset => current - offset)
  const oldest = recent[recent.length - 1]
  return [...recent, ...OLDER_YEARS.filter(year => year < oldest)]
}
