import { describe, it, expect } from 'vitest'
import {
  EMPTY_FILTERS,
  MAX_GENRES,
  categoryCaps,
  filtersFromParams,
  filtersToParams,
  filtersToQuery,
  activeFilterCount,
  setMediaType,
  yearOptions,
} from '../lib/discoverFilters'

const tvCaps = categoryCaps('tv')!
const upcomingCaps = categoryCaps('movies/upcoming')!
const trendingCaps = categoryCaps('trending')!

describe('categoryCaps', () => {
  it('describes each known category', () => {
    expect(categoryCaps('tv')).toEqual({ year: true, type: false, mediaType: 'tv' })
    expect(categoryCaps('movies')).toEqual({ year: true, type: false, mediaType: 'movie' })
    expect(categoryCaps('movies/upcoming')).toEqual({ year: false, type: false, mediaType: 'movie' })
    expect(categoryCaps('tv/upcoming')).toEqual({ year: false, type: false, mediaType: 'tv' })
    expect(categoryCaps('trending')).toEqual({ year: true, type: true, mediaType: null })
  })

  it('returns null for an unknown category', () => {
    expect(categoryCaps('nope')).toBeNull()
  })

  it('returns a stable reference so hook memoisation holds', () => {
    expect(categoryCaps('tv')).toBe(categoryCaps('tv'))
  })
})

describe('filtersFromParams', () => {
  it('defaults to empty filters', () => {
    expect(filtersFromParams(new URLSearchParams(), tvCaps)).toEqual(EMPTY_FILTERS)
  })

  it('reads every supported param', () => {
    const params = new URLSearchParams('year=2024&genres=80,18&sort=rating&rating=7&hide_owned=1')
    expect(filtersFromParams(params, tvCaps)).toEqual({
      year: 2024,
      genres: [80, 18],
      sort: 'rating',
      rating: 7,
      hideOwned: true,
      type: null,
    })
  })

  it('ignores year on categories that do not support it', () => {
    expect(filtersFromParams(new URLSearchParams('year=2024'), upcomingCaps).year).toBeNull()
  })

  it('ignores type on categories that do not support it', () => {
    expect(filtersFromParams(new URLSearchParams('type=movie'), tvCaps).type).toBeNull()
  })

  it('reads type on trending', () => {
    expect(filtersFromParams(new URLSearchParams('type=tv'), trendingCaps).type).toBe('tv')
  })

  it('discards malformed values rather than throwing', () => {
    const params = new URLSearchParams('year=abc&genres=drama,18&sort=sideways&rating=99&type=person')
    expect(filtersFromParams(params, trendingCaps)).toEqual(EMPTY_FILTERS)
  })

  it('rejects rather than truncates an over-long genre list', () => {
    const tooMany = Array.from({ length: MAX_GENRES + 1 }, (_, i) => i + 1).join(',')
    expect(filtersFromParams(new URLSearchParams(`genres=${tooMany}`), tvCaps).genres).toEqual([])
  })

  it('rejects a zero rating, which the backend treats as invalid', () => {
    expect(filtersFromParams(new URLSearchParams('rating=0'), tvCaps).rating).toBeNull()
  })

  it('rejects a future year, matching the backend ceiling', () => {
    expect(filtersFromParams(new URLSearchParams('year=2999'), tvCaps).year).toBeNull()
  })
})

describe('filtersToParams', () => {
  it('round-trips through filtersFromParams', () => {
    const filters = { year: 2024, genres: [80, 18], sort: 'rating' as const, rating: 7, hideOwned: true, type: null }
    expect(filtersFromParams(filtersToParams(filters, tvCaps), tvCaps)).toEqual(filters)
  })

  it('omits defaults', () => {
    expect(filtersToParams(EMPTY_FILTERS, tvCaps).toString()).toBe('')
  })
})

describe('filtersToQuery', () => {
  it('is empty for empty filters', () => {
    expect(filtersToQuery(EMPTY_FILTERS, tvCaps)).toBe('')
  })

  it('serialises genres comma-separated and omits hideOwned', () => {
    const query = filtersToQuery({ ...EMPTY_FILTERS, genres: [80, 18], hideOwned: true }, tvCaps)
    expect(query).toContain('genres=80%2C18')
    expect(query).not.toContain('hide_owned')
  })

  it('includes type for trending', () => {
    expect(filtersToQuery({ ...EMPTY_FILTERS, type: 'movie' }, trendingCaps)).toBe('type=movie')
  })

  it('emits nothing when only hideOwned is set', () => {
    expect(filtersToQuery({ ...EMPTY_FILTERS, hideOwned: true }, tvCaps)).toBe('')
  })
})

describe('activeFilterCount', () => {
  it('counts each set filter including hideOwned', () => {
    expect(activeFilterCount(EMPTY_FILTERS, tvCaps)).toBe(0)
    expect(activeFilterCount({ ...EMPTY_FILTERS, year: 2024 }, tvCaps)).toBe(1)
    expect(activeFilterCount({ ...EMPTY_FILTERS, year: 2024, genres: [80, 18], hideOwned: true }, tvCaps)).toBe(3)
  })

  it('counts a genre selection once regardless of how many genres', () => {
    expect(activeFilterCount({ ...EMPTY_FILTERS, genres: [80, 18, 35] }, tvCaps)).toBe(1)
  })
})

// Genre IDs are not portable between movie and TV, and a request carrying
// filters without a type is a 400 from the backend.
describe('setMediaType', () => {
  const filled = { year: 2024, genres: [80, 18], sort: 'rating' as const, rating: 7, hideOwned: true, type: 'movie' as const }

  it('clears genres when switching between types', () => {
    const next = setMediaType(filled, 'tv')
    expect(next.type).toBe('tv')
    expect(next.genres).toEqual([])
    expect(next.year).toBe(2024)
    expect(next.sort).toBe('rating')
  })

  it('clears every server-side filter when returning to All', () => {
    const next = setMediaType(filled, null)
    expect(next).toEqual({ ...EMPTY_FILTERS, hideOwned: true })
  })

  it('is a no-op when the type is unchanged', () => {
    expect(setMediaType(filled, 'movie')).toEqual(filled)
  })
})

describe('yearOptions', () => {
  it('offers recent years then round older ones, newest first', () => {
    expect(yearOptions(new Date(Date.UTC(2026, 7, 8)))).toEqual([2026, 2025, 2024, 2023, 2022, 2020, 2015, 2010, 2000])
  })

  it('drops older options that overlap the recent run', () => {
    expect(yearOptions(new Date(Date.UTC(2023, 0, 1)))).toEqual([2023, 2022, 2021, 2020, 2019, 2015, 2010, 2000])
  })
})
