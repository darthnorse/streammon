import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { useFetch } from '../hooks/useFetch'

describe('useFetch', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('fetches data and returns it', async () => {
    const data = { items: [1, 2, 3] }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(data),
    }))
    const { result } = renderHook(() => useFetch<{ items: number[] }>('/api/test'))
    expect(result.current.loading).toBe(true)
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toEqual(data)
    expect(result.current.error).toBeNull()
  })

  it('returns error on failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'server error' }),
    }))
    const { result } = renderHook(() => useFetch<unknown>('/api/fail'))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toBeNull()
    expect(result.current.error).toBeTruthy()
  })

  it('refetches when deps change', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ v: 1 }),
    })
    vi.stubGlobal('fetch', mockFetch)
    const { rerender } = renderHook(
      ({ url }) => useFetch<{ v: number }>(url),
      { initialProps: { url: '/api/a' } }
    )
    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(1))
    rerender({ url: '/api/b' })
    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2))
  })
})
