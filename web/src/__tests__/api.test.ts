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
      status: 200,
      headers: new Headers({ 'content-length': '15' }),
      json: () => Promise.resolve(data),
    }))
    const result = await api.get<{ status: string }>('/api/health')
    expect(result).toEqual(data)
    expect(fetch).toHaveBeenCalledWith('/api/health', {
      method: 'GET',
      headers: { 'Content-Type': 'application/json' },
      signal: undefined,
    })
  })

  it('get passes abort signal', async () => {
    const controller = new AbortController()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-length': '2' }),
      json: () => Promise.resolve({}),
    }))
    await api.get('/api/test', controller.signal)
    expect(fetch).toHaveBeenCalledWith('/api/test', {
      method: 'GET',
      headers: { 'Content-Type': 'application/json' },
      signal: controller.signal,
    })
  })

  it('post sends body and returns JSON', async () => {
    const body = { name: 'test' }
    const response = { id: 1, name: 'test' }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-length': '20' }),
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

  it('del handles 204 No Content without parsing JSON', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
      headers: new Headers(),
      json: () => { throw new Error('should not call json()') },
    }))
    await expect(api.del('/api/servers/1')).resolves.toBeUndefined()
  })

  it('post handles 204 No Content without parsing JSON, defaulting to void', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
      headers: new Headers(),
      json: () => { throw new Error('should not call json()') },
    }))
    // No explicit type arg: T defaults to void, matching the empty-body contract.
    const result = await api.post('/api/servers/1/restore', {})
    expect(result).toBeUndefined()
  })

  it('put still returns JSON when an explicit type arg is given', async () => {
    const response = { id: 1, name: 'renamed' }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-length': '25' }),
      json: () => Promise.resolve(response),
    }))
    const result = await api.put<{ id: number; name: string }>('/api/servers/1', { name: 'renamed' })
    expect(result).toEqual(response)
  })
})
