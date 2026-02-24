import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useRequestCount, REQUEST_CHANGED_EVENT, dispatchRequestChanged } from '../hooks/useRequestCount'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

describe('useRequestCount', () => {
  const refetch = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    mockUseFetch.mockReturnValue({
      data: {
        pending: 3, total: 10, movie: 5, tv: 5,
        approved: 4, declined: 1, processing: 1, available: 1,
      },
      loading: false, error: null, refetch,
    })
  })

  it('passes the request count URL when enabled', () => {
    renderHook(() => useRequestCount(true))
    expect(mockUseFetch).toHaveBeenCalledWith('/api/overseerr/requests/count')
  })

  it('passes null when disabled', () => {
    renderHook(() => useRequestCount(false))
    expect(mockUseFetch).toHaveBeenCalledWith(null)
  })

  it('switches URL when enabled changes', () => {
    const { rerender } = renderHook(({ enabled }) => useRequestCount(enabled), {
      initialProps: { enabled: false },
    })
    expect(mockUseFetch).toHaveBeenLastCalledWith(null)
    rerender({ enabled: true })
    expect(mockUseFetch).toHaveBeenLastCalledWith('/api/overseerr/requests/count')
  })

  it('triggers refetch on window event', () => {
    renderHook(() => useRequestCount(true))
    act(() => {
      window.dispatchEvent(new Event(REQUEST_CHANGED_EVENT))
    })
    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('cleans up event listener on unmount', () => {
    const addSpy = vi.spyOn(window, 'addEventListener')
    const removeSpy = vi.spyOn(window, 'removeEventListener')

    const { unmount } = renderHook(() => useRequestCount(true))
    expect(addSpy).toHaveBeenCalledWith(REQUEST_CHANGED_EVENT, expect.any(Function))

    unmount()
    expect(removeSpy).toHaveBeenCalledWith(REQUEST_CHANGED_EVENT, expect.any(Function))
  })
})

describe('dispatchRequestChanged', () => {
  it('fires the correct event on window', () => {
    const handler = vi.fn()
    window.addEventListener(REQUEST_CHANGED_EVENT, handler)
    dispatchRequestChanged()
    expect(handler).toHaveBeenCalledTimes(1)
    window.removeEventListener(REQUEST_CHANGED_EVENT, handler)
  })
})
