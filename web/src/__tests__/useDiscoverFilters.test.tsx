import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useDiscoverFilters } from '../hooks/useDiscoverFilters'
import { useTMDBGenres } from '../hooks/useTMDBGenres'
import { categoryCaps } from '../lib/discoverFilters'

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

describe('useTMDBGenres', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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

    const { result, rerender } = renderHook(
      ({ mediaType }: { mediaType: 'movie' | 'tv' }) => useTMDBGenres(mediaType),
      { wrapper: wrapperAt('/'), initialProps: { mediaType: 'movie' as const } },
    )
    await waitFor(() => expect(result.current.genres).toHaveLength(1))

    let resolveTv: (value: { genres: { id: number; name: string }[] }) => void = () => {}
    mockApi.get.mockImplementationOnce((() => new Promise(resolve => { resolveTv = resolve })) as typeof api.get)

    rerender({ mediaType: 'tv' })
    expect(result.current.genres).toEqual([])

    await act(async () => {
      resolveTv({ genres: [{ id: 10765, name: 'Sci-Fi & Fantasy' }] })
    })
    await waitFor(() => expect(result.current.genres[0].id).toBe(10765))
  })
})
