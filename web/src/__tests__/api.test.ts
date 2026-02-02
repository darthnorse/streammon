import { describe, it, expect, vi, beforeEach } from 'vitest'
import { api } from '../lib/api'

describe('api', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('get fetches and returns JSON', async () => {
    const data = { status: 'ok' }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(data),
    }))
    const result = await api.get<{ status: string }>('/api/health')
    expect(result).toEqual(data)
    expect(fetch).toHaveBeenCalledWith('/api/health', {
      method: 'GET',
      headers: { 'Content-Type': 'application/json' },
    })
  })

  it('post sends body and returns JSON', async () => {
    const body = { name: 'test' }
    const response = { id: 1, name: 'test' }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(response),
    }))
    const result = await api.post<{ id: number; name: string }>('/api/servers', body)
    expect(result).toEqual(response)
    expect(fetch).toHaveBeenCalledWith('/api/servers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  })

  it('throws on non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: 'not found' }),
    }))
    await expect(api.get('/api/nope')).rejects.toThrow()
  })
})
