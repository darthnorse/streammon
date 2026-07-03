import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { baseServer } from './fixtures'
import type { Server } from '../types'

vi.mock('../lib/api', () => ({
  api: { get: vi.fn() },
}))

import { api } from '../lib/api'
const mockApi = vi.mocked(api)

const serverA: Server = baseServer
const serverB: Server = { ...baseServer, id: 2, name: 'Jellyfin', type: 'jellyfin' }

describe('useServers', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
  })

  it('fetches servers once and shares the cache across hook instances', async () => {
    mockApi.get.mockResolvedValue([serverA])
    const { useServers } = await import('../hooks/useServers')

    const { result: r1 } = renderHook(() => useServers())
    await waitFor(() => expect(r1.current).toHaveLength(1))

    const { result: r2 } = renderHook(() => useServers())
    expect(r2.current).toHaveLength(1)
    expect(mockApi.get).toHaveBeenCalledTimes(1)
  })

  it('invalidateServers refetches and updates already-mounted consumers', async () => {
    mockApi.get.mockResolvedValueOnce([serverA])
    const { useServers, invalidateServers } = await import('../hooks/useServers')

    const { result } = renderHook(() => useServers())
    await waitFor(() => expect(result.current).toHaveLength(1))
    expect(mockApi.get).toHaveBeenCalledTimes(1)

    mockApi.get.mockResolvedValueOnce([serverA, serverB])
    await act(async () => {
      await invalidateServers()
    })

    expect(result.current).toHaveLength(2)
    expect(mockApi.get).toHaveBeenCalledTimes(2)
  })

  it('a fresh hook mount after invalidation sees the updated cache without refetching again', async () => {
    mockApi.get.mockResolvedValueOnce([serverA])
    const { useServers, invalidateServers } = await import('../hooks/useServers')

    const { result: r1 } = renderHook(() => useServers())
    await waitFor(() => expect(r1.current).toHaveLength(1))

    mockApi.get.mockResolvedValueOnce([serverA, serverB])
    await act(async () => {
      await invalidateServers()
    })

    const { result: r2 } = renderHook(() => useServers())
    expect(r2.current).toHaveLength(2)
    expect(mockApi.get).toHaveBeenCalledTimes(2)
  })
})
