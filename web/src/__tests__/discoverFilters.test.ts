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

  // The argument is the route splat, i.e. attacker-chosen via a crafted link;
  // an inherited Object.prototype member must not satisfy the | null contract.
  it('returns null for inherited Object.prototype keys', () => {
    expect(categoryCaps('constructor')).toBeNull()
    expect(categoryCaps('__proto__')).toBeNull()
    expect(categoryCaps('toString')).toBeNull()
  })

  it('returns a stable reference so hook memoisation holds', () => {
    expect(categoryCaps('tv')).toBe(categoryCaps('tv'))
  })

  it('freezes each caps entry so mutation cannot corrupt the shared singleton', () => {
    expect(Object.isFrozen(categoryCaps('tv'))).toBe(true)
    expect(Object.isFrozen(categoryCaps('trending'))).toBe(true)
  })
})

describe('EMPTY_FILTERS', () => {
  it('is frozen, including its genres array, so it cannot be mutated in place', () => {
    expect(Object.isFrozen(EMPTY_FILTERS)).toBe(true)
    expect(Object.isFrozen(EMPTY_FILTERS.genres)).toBe(true)
  })

  it('still supports spread-based derivation', () => {
    expect({ ...EMPTY_FILTERS, hideOwned: true }).toEqual({
      year: null,
      genres: [],
      sort: 'popularity',
      rating: null,
      hideOwned: true,
      type: null,
    })
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

  // The case above uses trendingCaps (caps.type === true), where type=person
  // fails parseType and the mixed-category early return fires before parseYear,
  // parseSort or parseRating ever run — so it doesn't actually exercise them.
  // tvCaps has caps.type === false, so that early return cannot fire and every
  // parser genuinely runs on the malformed input.
  it('discards malformed values on a category with no type param to short-circuit on', () => {
    const params = new URLSearchParams('year=abc&genres=drama,18&sort=sideways&rating=99&type=person')
    expect(filtersFromParams(params, tvCaps)).toEqual(EMPTY_FILTERS)
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

  // Number.isInteger accepts values beyond JS's safe range and Go's int; such
  // values break the parse-and-reserialise contract (e.g. re-serialise in
  // exponential notation), so the backend 400s them on the round trip.
  it('rejects a genre id in exponential notation beyond the safe integer range', () => {
    expect(filtersFromParams(new URLSearchParams('genres=1e21'), tvCaps).genres).toEqual([])
  })

  it('rejects a genre id far beyond Go int range', () => {
    expect(filtersFromParams(new URLSearchParams('genres=99999999999999999999'), tvCaps).genres).toEqual([])
  })

  // Number() accepts exponential, hex, trailing-decimal, sign-prefixed, and
  // whitespace-padded forms that Go's strconv.Atoi rejects, so a raw token
  // must be canonical base-10 digits before conversion.
  it('rejects genre ids that are not canonical base-10 digit strings', () => {
    for (const raw of ['1e3', '0x50', '80.0', '+80', ' 80', '080']) {
      expect(filtersFromParams(new URLSearchParams(`genres=${raw}`), tvCaps).genres).toEqual([])
    }
  })
})

// On a mixed category (caps.type === true), the backend 400s any filtered
// request that lacks ?type=. Filters and type must hydrate as a single unit.
describe('filtersFromParams on mixed categories', () => {
  const filtered = 'year=2024&genres=80,18&sort=rating&rating=7'

  it('drops every server-side filter when type is missing on trending', () => {
    expect(filtersFromParams(new URLSearchParams(filtered), trendingCaps)).toEqual(EMPTY_FILTERS)
  })

  it('hydrates normally once type is present', () => {
    expect(filtersFromParams(new URLSearchParams(`${filtered}&type=movie`), trendingCaps)).toEqual({
      year: 2024,
      genres: [80, 18],
      sort: 'rating',
      rating: 7,
      hideOwned: false,
      type: 'movie',
    })
  })

  it('keeps hideOwned even when the other filters are dropped', () => {
    expect(filtersFromParams(new URLSearchParams('year=2024&hide_owned=1'), trendingCaps)).toEqual({
      ...EMPTY_FILTERS,
      hideOwned: true,
    })
  })

  it('never lets filtersToQuery emit a server-side param without type on trending', () => {
    const scenarios = ['year=2024', 'genres=80,18', 'sort=rating', 'rating=7', filtered]
    for (const raw of scenarios) {
      const filters = filtersFromParams(new URLSearchParams(raw), trendingCaps)
      expect(filtersToQuery(filters, trendingCaps)).toBe('')
    }
  })

  it('leaves non-mixed categories unaffected', () => {
    expect(filtersFromParams(new URLSearchParams(filtered), tvCaps)).toEqual({
      year: 2024,
      genres: [80, 18],
      sort: 'rating',
      rating: 7,
      hideOwned: false,
      type: null,
    })
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

  // apiParams (which filtersToParams wraps) must enforce the same backend
  // invariants filtersFromParams does, or the write and read sides disagree
  // and state silently reverts on the next render.
  it('round-trips to the normalised form on a mixed category with no type set', () => {
    const filters = { year: 2024, genres: [80, 18], sort: 'rating' as const, rating: 7, hideOwned: true, type: null }
    const normalised = { ...EMPTY_FILTERS, hideOwned: true }
    expect(filtersFromParams(filtersToParams(filters, trendingCaps), trendingCaps)).toEqual(normalised)
  })

  it('round-trips to the normalised form when more than MAX_GENRES genres are set', () => {
    const tooMany = Array.from({ length: MAX_GENRES + 1 }, (_, i) => i + 1)
    const filters = { ...EMPTY_FILTERS, genres: tooMany }
    const normalised = { ...EMPTY_FILTERS, genres: tooMany.slice(0, MAX_GENRES) }
    expect(filtersFromParams(filtersToParams(filters, tvCaps), tvCaps)).toEqual(normalised)
  })

  it('never leaves a phantom param in the URL when type is missing on a mixed category', () => {
    expect(filtersToQuery({ ...EMPTY_FILTERS, sort: 'rating' }, trendingCaps)).toBe('')
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

  // On a mixed category with no type set, apiParams (via filtersToParams) emits
  // nothing but hideOwned, so the badge must agree: 0 filters, or 1 with hideOwned.
  it('counts nothing but hideOwned on a mixed category with no type set', () => {
    const filters = { year: 2024, genres: [80, 18], sort: 'rating' as const, rating: 7, hideOwned: false, type: null }
    expect(activeFilterCount(filters, trendingCaps)).toBe(0)
    expect(activeFilterCount({ ...filters, hideOwned: true }, trendingCaps)).toBe(1)
  })

  // URLSearchParams.prototype.size shipped in 2023 (Chrome 113 / Safari 17 /
  // Firefox 112), newer than this project's Vite build target; Node always
  // has it, so this simulates a browser without it to pin that the counting
  // implementation never depends on it.
  it('counts correctly even when the runtime has no URLSearchParams.size', () => {
    const descriptor = Object.getOwnPropertyDescriptor(URLSearchParams.prototype, 'size')
    Object.defineProperty(URLSearchParams.prototype, 'size', { value: undefined, configurable: true })
    try {
      expect(activeFilterCount({ ...EMPTY_FILTERS, year: 2024, genres: [80, 18], hideOwned: true }, tvCaps)).toBe(3)
    } finally {
      Object.defineProperty(URLSearchParams.prototype, 'size', descriptor!)
    }
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
