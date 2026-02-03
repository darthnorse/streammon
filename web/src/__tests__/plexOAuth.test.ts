import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { getClientId, plexHeaders, requestPin, checkPin, getAuthUrl, fetchResources } from '../lib/plexOAuth'

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('getClientId', () => {
  it('returns persistent UUID from localStorage', () => {
    const id1 = getClientId()
    const id2 = getClientId()
    expect(id1).toBe(id2)
    expect(id1).toMatch(/^[0-9a-f-]{36}$/)
  })

  it('returns existing value from localStorage', () => {
    localStorage.setItem('streammon_plex_client_id', 'existing-id')
    expect(getClientId()).toBe('existing-id')
  })
})

describe('plexHeaders', () => {
  it('returns required Plex headers', () => {
    const headers = plexHeaders()
    expect(headers['X-Plex-Product']).toBe('StreamMon')
    expect(headers['X-Plex-Client-Identifier']).toBeTruthy()
    expect(headers['Accept']).toBe('application/json')
  })
})

describe('requestPin', () => {
  it('calls plex.tv/api/v2/pins with POST', async () => {
    const pin = { id: 123, code: 'ABCD', authToken: null }
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(pin), { status: 200 })
    )

    const result = await requestPin()
    expect(result).toEqual(pin)

    const call = vi.mocked(fetch).mock.calls[0]
    expect(call[0]).toBe('https://plex.tv/api/v2/pins?strong=true')
    expect((call[1] as RequestInit).method).toBe('POST')
  })

  it('throws on non-OK response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('error', { status: 500 })
    )
    await expect(requestPin()).rejects.toThrow()
  })
})

describe('checkPin', () => {
  it('returns authToken when ready', async () => {
    const pin = { id: 123, code: 'ABCD', authToken: 'my-token' }
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(pin), { status: 200 })
    )

    const result = await checkPin(123)
    expect(result.authToken).toBe('my-token')

    const call = vi.mocked(fetch).mock.calls[0]
    expect(call[0]).toBe('https://plex.tv/api/v2/pins/123')
    expect((call[1] as RequestInit).method).toBe('GET')
  })
})

describe('getAuthUrl', () => {
  it('returns plex auth URL with clientID and code', () => {
    const url = getAuthUrl('my-client-id', 'ABCD')
    expect(url).toContain('https://app.plex.tv/auth/#!?')
    expect(url).toContain('clientID=my-client-id')
    expect(url).toContain('code=ABCD')
    expect(url).toContain('context%5Bdevice%5D%5Bproduct%5D=StreamMon')
  })
})

describe('fetchResources', () => {
  it('returns server list filtered to provides=server', async () => {
    const resources = [
      { name: 'My Server', clientIdentifier: 'abc', accessToken: 'tok', provides: 'server', connections: [{ uri: 'https://1.2.3.4:32400', local: false, relay: false, protocol: 'https' }] },
      { name: 'Player', clientIdentifier: 'def', accessToken: 'tok2', provides: 'player', connections: [] },
    ]
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(resources), { status: 200 })
    )

    const result = await fetchResources('my-token')
    expect(result).toHaveLength(1)
    expect(result[0].name).toBe('My Server')

    const call = vi.mocked(fetch).mock.calls[0]
    expect(call[0]).toBe('https://plex.tv/api/v2/resources?includeHttps=1&includeRelay=1')
  })

  it('throws on non-OK response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('error', { status: 401 })
    )
    await expect(fetchResources('bad-token')).rejects.toThrow()
  })
})
