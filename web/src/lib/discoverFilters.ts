import { DISCOVER_CATEGORIES } from './constants'
import type { DropdownOption } from '../components/Dropdown'

export type DiscoverSort = 'popularity' | 'rating' | 'newest'
export type DiscoverMediaType = 'movie' | 'tv'

export interface DiscoverFilters {
  year: number | null
  genres: readonly number[]
  sort: DiscoverSort
  rating: number | null
  hideOwned: boolean
  type: DiscoverMediaType | null
  countries: readonly string[]
  languages: readonly string[]
}

export interface DiscoverCategoryCaps {
  readonly year: boolean
  readonly type: boolean
  readonly mediaType: DiscoverMediaType | null
}

const FROZEN_EMPTY_GENRES: readonly number[] = Object.freeze([])
const FROZEN_EMPTY_CODES: readonly string[] = Object.freeze([])

export const EMPTY_FILTERS: DiscoverFilters = Object.freeze({
  year: null,
  genres: FROZEN_EMPTY_GENRES,
  sort: 'popularity',
  rating: null,
  hideOwned: false,
  type: null,
  countries: FROZEN_EMPTY_CODES,
  languages: FROZEN_EMPTY_CODES,
})

export const MAX_GENRES = 5
export const MAX_CODES = 5

// Curated rather than TMDB's full lists (~250 countries, ~190 languages), which
// would bury the common picks in an unsearchable dropdown.
export const COUNTRY_OPTIONS: readonly DropdownOption[] = [
  { value: 'US', label: 'United States' },
  { value: 'GB', label: 'United Kingdom' },
  { value: 'CA', label: 'Canada' },
  { value: 'AU', label: 'Australia' },
  { value: 'IE', label: 'Ireland' },
  { value: 'NZ', label: 'New Zealand' },
  { value: 'FR', label: 'France' },
  { value: 'DE', label: 'Germany' },
  { value: 'ES', label: 'Spain' },
  { value: 'IT', label: 'Italy' },
  { value: 'NL', label: 'Netherlands' },
  { value: 'BE', label: 'Belgium' },
  { value: 'SE', label: 'Sweden' },
  { value: 'NO', label: 'Norway' },
  { value: 'DK', label: 'Denmark' },
  { value: 'FI', label: 'Finland' },
  { value: 'IS', label: 'Iceland' },
  { value: 'PL', label: 'Poland' },
  { value: 'JP', label: 'Japan' },
  { value: 'KR', label: 'South Korea' },
  { value: 'CN', label: 'China' },
  { value: 'HK', label: 'Hong Kong' },
  { value: 'TW', label: 'Taiwan' },
  { value: 'IN', label: 'India' },
  { value: 'TH', label: 'Thailand' },
  { value: 'TR', label: 'Turkey' },
  { value: 'MX', label: 'Mexico' },
  { value: 'BR', label: 'Brazil' },
  { value: 'AR', label: 'Argentina' },
  { value: 'CO', label: 'Colombia' },
  { value: 'ZA', label: 'South Africa' },
]

export const LANGUAGE_OPTIONS: readonly DropdownOption[] = [
  { value: 'en', label: 'English' },
  { value: 'es', label: 'Spanish' },
  { value: 'fr', label: 'French' },
  { value: 'de', label: 'German' },
  { value: 'it', label: 'Italian' },
  { value: 'pt', label: 'Portuguese' },
  { value: 'nl', label: 'Dutch' },
  { value: 'sv', label: 'Swedish' },
  { value: 'no', label: 'Norwegian' },
  { value: 'da', label: 'Danish' },
  { value: 'fi', label: 'Finnish' },
  { value: 'is', label: 'Icelandic' },
  { value: 'pl', label: 'Polish' },
  { value: 'ru', label: 'Russian' },
  { value: 'tr', label: 'Turkish' },
  { value: 'ar', label: 'Arabic' },
  { value: 'he', label: 'Hebrew' },
  { value: 'hi', label: 'Hindi' },
  { value: 'ta', label: 'Tamil' },
  { value: 'te', label: 'Telugu' },
  { value: 'th', label: 'Thai' },
  { value: 'ja', label: 'Japanese' },
  { value: 'ko', label: 'Korean' },
  { value: 'zh', label: 'Mandarin' },
  { value: 'cn', label: 'Cantonese' },
]

// TMDB genre ids are 5 digits; this ceiling is generous and, more importantly,
// keeps every accepted value inside both JS's safe-integer range and Go's int.
const MAX_GENRE_ID = 1_000_000

type DiscoverPath = (typeof DISCOVER_CATEGORIES)[number]['path']

// Keyed off DISCOVER_CATEGORIES so a new routed category with no caps entry
// here is a compile error rather than a silently disabled filter bar.
const CAPS: Record<DiscoverPath, DiscoverCategoryCaps> = {
  trending: Object.freeze({ year: true, type: true, mediaType: null }),
  movies: Object.freeze({ year: true, type: false, mediaType: 'movie' }),
  'movies/upcoming': Object.freeze({ year: false, type: false, mediaType: 'movie' }),
  tv: Object.freeze({ year: true, type: false, mediaType: 'tv' }),
  'tv/upcoming': Object.freeze({ year: false, type: false, mediaType: 'tv' }),
}

// Returns the shared frozen object so consumers can memoise on identity.
// Object.prototype.hasOwnProperty.call, not `in` or bare indexing, so an
// inherited key like "constructor" cannot masquerade as a known category.
export function categoryCaps(path: string): DiscoverCategoryCaps | null {
  return Object.prototype.hasOwnProperty.call(CAPS, path) ? CAPS[path as DiscoverPath] : null
}

const SORTS: DiscoverSort[] = ['popularity', 'rating', 'newest']

function parseSort(raw: string | null): DiscoverSort {
  return SORTS.find(s => s === raw) ?? 'popularity'
}

function parseType(raw: string | null): DiscoverMediaType | null {
  return raw === 'movie' || raw === 'tv' ? raw : null
}

const CANONICAL_DIGITS = /^[1-9]\d*$/

// Repeats collapse rather than count against the cap, so a hand-edited link
// cannot fill every slot with one value.
function splitDistinct(raw: string | null, max: number): string[] | null {
  if (!raw) return null
  const parts = [...new Set(raw.split(','))]
  return parts.length > max ? null : parts
}

function parseGenres(raw: string | null): number[] {
  const parts = splitDistinct(raw, MAX_GENRES)
  if (!parts) return []
  if (parts.some(part => !CANONICAL_DIGITS.test(part))) return []
  const ids = parts.map(part => Number(part))
  if (ids.some(id => !Number.isSafeInteger(id) || id > MAX_GENRE_ID)) return []
  return ids
}

// All-or-nothing like parseGenres: one bad code drops the list rather than
// silently narrowing to a different selection than the link described.
function parseCodes(raw: string | null, options: readonly DropdownOption[]): string[] {
  const parts = splitDistinct(raw, MAX_CODES)
  if (!parts) return []
  if (parts.some(part => !options.some(o => o.value === part))) return []
  return parts
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
    countries: parseCodes(params.get('countries'), COUNTRY_OPTIONS),
    languages: parseCodes(params.get('languages'), LANGUAGE_OPTIONS),
  }
}

// apiParams holds everything the backend understands; hideOwned is client-side
// only. Mirrors the invariants filtersFromParams enforces on the read side: a
// mixed category with no type emits nothing (the backend 400s otherwise), and
// an over-long genre, country, or language list is clamped rather than sent whole and rejected.
function apiParams(filters: DiscoverFilters, caps: DiscoverCategoryCaps): URLSearchParams {
  const params = new URLSearchParams()
  if (caps.type && filters.type === null) return params
  if (caps.year && filters.year !== null) params.set('year', String(filters.year))
  const genres = filters.genres.slice(0, MAX_GENRES)
  if (genres.length > 0) params.set('genres', genres.join(','))
  if (filters.sort !== 'popularity') params.set('sort', filters.sort)
  if (filters.rating !== null) params.set('rating', String(filters.rating))
  const countries = filters.countries.slice(0, MAX_CODES)
  if (countries.length > 0) params.set('countries', countries.join(','))
  const languages = filters.languages.slice(0, MAX_CODES)
  if (languages.length > 0) params.set('languages', languages.join(','))
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

// Derived from filtersToParams rather than re-enumerated, so a filter added
// to apiParams alone cannot silently under-count the badge. This intentionally
// undercounts genres beyond MAX_GENRES and any server-side filter on a mixed
// category with no type set, matching what apiParams actually sends.
// Array.from(...).length rather than URLSearchParams.size: .size shipped in
// 2023 (Chrome 113 / Safari 17 / Firefox 112), newer than this project's Vite
// build target, and Safari 16 and below would resolve it to undefined.
export function activeFilterCount(filters: DiscoverFilters, caps: DiscoverCategoryCaps): number {
  return Array.from(filtersToParams(filters, caps).keys()).length
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
