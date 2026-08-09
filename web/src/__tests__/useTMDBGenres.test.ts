import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useTMDBGenres } from '../hooks/useTMDBGenres'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

describe('useTMDBGenres', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // ~15 pre-existing test files mock useFetch with object literals that
  // predate `dataUrl` and omit it entirely; such a mock must still be
  // treated as fresh once loading has finished, not as permanently stale.
  it('treats loaded data as fresh when the mock omits dataUrl', () => {
    mockUseFetch.mockReturnValue({
      data: { genres: [{ id: 80, name: 'Crime' }] },
      loading: false,
      error: null,
      refetch: vi.fn(),
    })

    const { result } = renderHook(() => useTMDBGenres('movie'))

    expect(result.current.loading).toBe(false)
    expect(result.current.genres).toEqual([{ id: 80, name: 'Crime' }])
  })
})
