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
      status: 200,
      headers: new Headers({ 'content-length': '15' }),
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

  it('refetches when url changes', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-length': '10' }),
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

  it('resets data to null when url changes', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() =>
      new Promise(resolve =>
        setTimeout(() => resolve({
          ok: true,
          status: 200,
          headers: new Headers({ 'content-length': '10' }),
          json: () => Promise.resolve({ page: 1 }),
        }), 50)
      )
    ))
    const { result, rerender } = renderHook(
      ({ url }) => useFetch<{ page: number }>(url),
      { initialProps: { url: '/api/page1' } }
    )
    await waitFor(() => expect(result.current.data).toEqual({ page: 1 }))
    rerender({ url: '/api/page2' })
    // After URL change, data should be reset to null while loading
    expect(result.current.data).toBeNull()
    expect(result.current.loading).toBe(true)
  })
})
