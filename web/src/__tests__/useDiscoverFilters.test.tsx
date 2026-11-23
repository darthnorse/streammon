import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useDiscoverFilters } from '../hooks/useDiscoverFilters'
import { useTMDBGenres } from '../hooks/useTMDBGenres'
import { categoryCaps, defaultFromFilters, EMPTY_FILTERS, DISCOVER_DEFAULT_KEY } from '../lib/discoverFilters'

vi.mock('../lib/api', () => ({
  api: { get: vi.fn() },
}))

import { api } from '../lib/api'
const mockApi = vi.mocked(api)

const tvCaps = categoryCaps('tv')!
const trendingCaps = categoryCaps('trending')!

function wrapperAt(path: string) {
  return ({ children }: { children: ReactNode }) => (
    <MemoryRouter initialEntries={[path]}>{children}</MemoryRouter>
  )
}

describe('useDiscoverFilters', () => {
  it('initialises from the URL', () => {
    const { result } = renderHook(() => useDiscoverFilters(tvCaps), {
      wrapper: wrapperAt('/discover/tv?year=2024&genres=80,18'),
    })
    expect(result.current.filters.year).toBe(2024)
    expect(result.current.filters.genres).toEqual([80, 18])
    expect(result.current.activeCount).toBe(2)
  })

  it('writes changes back to the URL and exposes an API query', () => {
    const { result } = renderHook(() => useDiscoverFilters(tvCaps), {
      wrapper: wrapperAt('/discover/tv'),
    })

    act(() => {
      result.current.setFilters({ ...result.current.filters, year: 2024 })
    })

    expect(result.current.filters.year).toBe(2024)
    expect(result.current.apiQuery).toBe('year=2024')
  })

  it('excludes hideOwned from the API query but keeps it in state', () => {
    const { result } = renderHook(() => useDiscoverFilters(tvCaps), {
      wrapper: wrapperAt('/discover/tv?hide_owned=1'),
    })
    expect(result.current.filters.hideOwned).toBe(true)
    expect(result.current.apiQuery).toBe('')
  })

  it('clears every filter', () => {
    const { result } = renderHook(() => useDiscoverFilters(trendingCaps), {
      wrapper: wrapperAt('/discover/trending?type=movie&year=2024&hide_owned=1'),
    })

    act(() => result.current.clear())

    expect(result.current.activeCount).toBe(0)
    expect(result.current.apiQuery).toBe('')
  })

  it('keeps apiQuery referentially stable across re-renders', () => {
    const { result, rerender } = renderHook(() => useDiscoverFilters(tvCaps), {
      wrapper: wrapperAt('/discover/tv?year=2024'),
    })
    const first = result.current.apiQuery
    rerender()
    expect(result.current.apiQuery).toBe(first)
  })
})

describe('useDiscoverFilters saved default', () => {
  const movieCaps = categoryCaps('movies')!

  beforeEach(() => localStorage.clear())

  function store(filters = { ...EMPTY_FILTERS, countries: ['US', 'GB'], rating: 7 }) {
    localStorage.setItem(DISCOVER_DEFAULT_KEY, JSON.stringify(defaultFromFilters(filters, movieCaps)))
  }

  it('applies the saved default when the URL carries no filters', async () => {
    store()
    const { result } = renderHook(() => useDiscoverFilters(tvCaps), { wrapper: wrapperAt('/discover/tv') })
    await waitFor(() => expect(result.current.filters.countries).toEqual(['US', 'GB']))
    expect(result.current.filters.rating).toBe(7)
    expect(result.current.savedDefault.isDefault).toBe(true)
  })

  it('uses the default from the first render, so nothing is fetched unfiltered', () => {
    store()
    const queries: string[] = []
    renderHook(() => {
      const value = useDiscoverFilters(tvCaps)
      queries.push(value.apiQuery)
      return value
    }, { wrapper: wrapperAt('/discover/tv') })
    expect(queries[0]).toContain('countries=US%2CGB')
  })

  it('leaves an explicit URL alone', () => {
    store()
    const { result } = renderHook(() => useDiscoverFilters(tvCaps), { wrapper: wrapperAt('/discover/tv?year=2024') })
    expect(result.current.filters.countries).toEqual([])
    expect(result.current.filters.year).toBe(2024)
  })

  it('does not re-apply the default after the filters are cleared', async () => {
    store()
    const { result } = renderHook(() => useDiscoverFilters(tvCaps), { wrapper: wrapperAt('/discover/tv') })
    await waitFor(() => expect(result.current.activeCount).toBe(2))
    act(() => result.current.clear())
    expect(result.current.activeCount).toBe(0)
    expect(result.current.savedDefault.hasDefault).toBe(true)
  })

  it('saves and removes the default in localStorage', () => {
    const { result } = renderHook(() => useDiscoverFilters(tvCaps), { wrapper: wrapperAt('/discover/tv?languages=en') })
    expect(result.current.savedDefault.hasDefault).toBe(false)
    expect(result.current.savedDefault.isDefault).toBe(false)

    act(() => result.current.savedDefault.onSave())
    expect(result.current.savedDefault.hasDefault).toBe(true)
    expect(result.current.savedDefault.isDefault).toBe(true)
    expect(localStorage.getItem(DISCOVER_DEFAULT_KEY)).toContain('languages=en')

    act(() => result.current.savedDefault.onRemove())
    expect(result.current.savedDefault.hasDefault).toBe(false)
    expect(localStorage.getItem(DISCOVER_DEFAULT_KEY)).toBeNull()
  })

  it('applies the default on trending when a type is picked, and reports it as the default', () => {
    store({ ...EMPTY_FILTERS, countries: ['US'], rating: 7 })
    const { result } = renderHook(() => useDiscoverFilters(trendingCaps), { wrapper: wrapperAt('/discover/trending') })
    expect(result.current.savedDefault.isDefault).toBe(false)
    act(() => result.current.setFilters({ ...result.current.filters, type: 'tv' }))
    expect(result.current.filters).toEqual({ ...EMPTY_FILTERS, type: 'tv', countries: ['US'], rating: 7 })
    expect(result.current.savedDefault.isDefault).toBe(true)
  })

  it('cannot save a trending selection that only picks a type', () => {
    const { result } = renderHook(() => useDiscoverFilters(trendingCaps), { wrapper: wrapperAt('/discover/trending?type=tv') })
    expect(result.current.savedDefault.canSave).toBe(false)
  })

  it('applies no default for an unknown category', () => {
    store()
    const { result } = renderHook(() => useDiscoverFilters(null), { wrapper: wrapperAt('/discover/bogus') })
    expect(result.current.activeCount).toBe(0)
    expect(result.current.savedDefault.hasDefault).toBe(false)
  })

  it('ignores a corrupt stored default', () => {
    localStorage.setItem(DISCOVER_DEFAULT_KEY, '{oops')
    const { result } = renderHook(() => useDiscoverFilters(tvCaps), { wrapper: wrapperAt('/discover/tv') })
    expect(result.current.savedDefault.hasDefault).toBe(false)
    expect(result.current.activeCount).toBe(0)
  })

  it('keeps working when localStorage throws', () => {
    const spy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    const { result } = renderHook(() => useDiscoverFilters(tvCaps), { wrapper: wrapperAt('/discover/tv?languages=en') })
    expect(result.current.savedDefault.hasDefault).toBe(false)
    act(() => result.current.savedDefault.onSave())
    expect(result.current.filters.languages).toEqual(['en'])
    expect(result.current.savedDefault.hasDefault).toBe(false)
    spy.mockRestore()
    vi.restoreAllMocks()
  })
})

type MediaTypeProp = { mediaType: 'movie' | 'tv' }

describe('useTMDBGenres', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('fetches the genre list for a media type', async () => {
    mockApi.get.mockResolvedValue({ genres: [{ id: 80, name: 'Crime' }] })

    const { result } = renderHook(() => useTMDBGenres('tv'), { wrapper: wrapperAt('/') })

    await waitFor(() => expect(result.current.genres).toHaveLength(1))
    expect(mockApi.get).toHaveBeenCalledWith('/api/tmdb/genres/tv', expect.any(AbortSignal))
    expect(result.current.genres[0].name).toBe('Crime')
  })

  it('fetches nothing when the media type is unknown', () => {
    renderHook(() => useTMDBGenres(null), { wrapper: wrapperAt('/') })
    expect(mockApi.get).not.toHaveBeenCalled()
  })

  it('does not surface the previous type\'s genres while loading a new type', async () => {
    mockApi.get.mockResolvedValueOnce({ genres: [{ id: 878, name: 'Science Fiction' }] })

    const initialProps: MediaTypeProp = { mediaType: 'movie' }
    const { result, rerender } = renderHook(
      ({ mediaType }: MediaTypeProp) => useTMDBGenres(mediaType),
      { wrapper: wrapperAt('/'), initialProps },
    )
    await waitFor(() => expect(result.current.genres).toHaveLength(1))

    let resolveTv: (value: { genres: { id: number; name: string }[] }) => void = () => {}
    mockApi.get.mockImplementationOnce(
      () => new Promise<{ genres: { id: number; name: string }[] }>(resolve => { resolveTv = resolve }),
    )

    rerender({ mediaType: 'tv' })
    expect(result.current.genres).toEqual([])

    await act(async () => {
      resolveTv({ genres: [{ id: 10765, name: 'Sci-Fi & Fantasy' }] })
    })
    await waitFor(() => expect(result.current.genres[0].id).toBe(10765))
  })

  it('never commits a render exposing the previous type\'s genres after a type switch', async () => {
    mockApi.get.mockResolvedValueOnce({ genres: [{ id: 878, name: 'Science Fiction' }] })

    const committed: number[][] = []
    const initialProps: MediaTypeProp = { mediaType: 'movie' }
    const { rerender } = renderHook(
      ({ mediaType }: MediaTypeProp) => {
        const { genres } = useTMDBGenres(mediaType)
        committed.push(genres.map(g => g.id))
        return genres
      },
      { wrapper: wrapperAt('/'), initialProps },
    )
    await waitFor(() => expect(committed.some(ids => ids.includes(878))).toBe(true))

    let resolveTv: (value: { genres: { id: number; name: string }[] }) => void = () => {}
    mockApi.get.mockImplementationOnce(
      () => new Promise<{ genres: { id: number; name: string }[] }>(resolve => { resolveTv = resolve }),
    )

    const beforeSwitch = committed.length
    rerender({ mediaType: 'tv' })

    await act(async () => {
      resolveTv({ genres: [{ id: 10765, name: 'Sci-Fi & Fantasy' }] })
    })
    await waitFor(() => expect(committed.some(ids => ids.includes(10765))).toBe(true))

    expect(committed.slice(beforeSwitch).some(ids => ids.includes(878))).toBe(false)
  })
})
