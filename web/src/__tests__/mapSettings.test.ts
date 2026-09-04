import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import {
  DEFAULT_TILE_URL,
  DEFAULT_TILE_ATTRIBUTION,
  DARK_TILE_FILTER,
  tileAttributionHtml,
  validateTileUrl,
  validateTileAttribution,
} from '../lib/mapUtils'
import type * as UnitsModule from '../lib/units'

// initDisplaySettings latches a module-level "already fetched" flag, so each
// test needs a fresh copy of the module rather than a reset hook exported
// from production code.
async function freshUnits(): Promise<typeof UnitsModule> {
  vi.resetModules()
  return import('../lib/units')
}

describe('map tile constants', () => {
  it('uses the plain OpenStreetMap host with no subdomain sharding', () => {
    expect(DEFAULT_TILE_URL).toBe('https://tile.openstreetmap.org/{z}/{x}/{y}.png')
    expect(DEFAULT_TILE_URL).not.toContain('{s}')
    expect(DEFAULT_TILE_URL).not.toContain('cartocdn')
  })

  it('credits OpenStreetMap and links the fixthemap reporting page', () => {
    expect(DEFAULT_TILE_ATTRIBUTION).toContain('OpenStreetMap')
    expect(DEFAULT_TILE_ATTRIBUTION).toContain('https://www.openstreetmap.org/copyright')
    expect(DEFAULT_TILE_ATTRIBUTION).toContain('https://www.openstreetmap.org/fixthemap')
    expect(DEFAULT_TILE_ATTRIBUTION).not.toContain('carto')
  })

  it('greys out before inverting so the basemap keeps no colour cast', () => {
    expect(DARK_TILE_FILTER).toBe('grayscale(1) invert(1) brightness(0.8) contrast(1.1)')
    expect(DARK_TILE_FILTER).not.toContain('hue-rotate')
  })
})

const CUSTOM_TILE_URL = 'https://tiles.example.com/{z}/{x}/{y}.png'

describe('tileAttributionHtml', () => {
  it('falls back to the OpenStreetMap credit when the default basemap is in use', () => {
    expect(tileAttributionHtml('', DEFAULT_TILE_URL)).toBe(DEFAULT_TILE_ATTRIBUTION)
  })

  it('never credits OpenStreetMap for a third-party basemap', () => {
    expect(tileAttributionHtml('', CUSTOM_TILE_URL)).toBe('')
  })

  it('escapes a custom attribution so it can never inject markup', () => {
    expect(tileAttributionHtml('<img src=x onerror=alert(1)>', DEFAULT_TILE_URL)).toBe(
      '&lt;img src=x onerror=alert(1)&gt;'
    )
    expect(tileAttributionHtml('A & B "quoted"', DEFAULT_TILE_URL)).toBe('A &amp; B &quot;quoted&quot;')
  })

  it('leaves an ordinary provider credit readable', () => {
    expect(tileAttributionHtml('© CARTO, © OpenStreetMap contributors', CUSTOM_TILE_URL)).toBe(
      '© CARTO, © OpenStreetMap contributors'
    )
  })
})

describe('client validators mirror the server rules', () => {
  it('rejects a tile URL over the server 500-character cap', () => {
    const long = `https://tiles.example.com/{z}/{x}/{y}.png?pad=${'a'.repeat(500)}`
    expect(validateTileUrl(long)).toMatch(/500/)
    expect(validateTileUrl(CUSTOM_TILE_URL)).toBeNull()
  })

  it('rejects an attribution over the server 500-character cap', () => {
    expect(validateTileAttribution('a'.repeat(501))).toMatch(/500/)
    expect(validateTileAttribution('© Example')).toBeNull()
  })

  it('names an unknown placeholder rather than letting Leaflet throw on every tile', () => {
    const msg = validateTileUrl('https://tiles.example.com/{z}/{x}/{y}.png?key={apikey}')
    expect(msg).toContain('{apikey}')
    expect(validateTileUrl('https://tiles.example.com/{z}/{x}/{y}{r}.png')).toBeNull()
    expect(validateTileUrl('https://{s}.example.com/{z}/{x}/{y}.png')).toBeNull()
  })
})

describe('map settings persistence', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('defaults to OpenStreetMap tiles with the dark filter enabled', async () => {
    const { getMapSettings } = await freshUnits()
    expect(getMapSettings()).toEqual({
      tileUrl: DEFAULT_TILE_URL,
      attribution: '',
      darkFilter: true,
    })
  })

  it('adopts the server values on init and notifies listeners', async () => {
    const { getMapSettings, initDisplaySettings } = await freshUnits()
    const serverSettings = {
      unit_system: 'metric',
      discover_region: '',
      map_tile_url: 'https://tiles.example.com/{z}/{x}/{y}.png',
      map_tile_attribution: '© Example',
      map_dark_filter: false,
    }
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify(serverSettings), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      )
    )

    const events: unknown[] = []
    const handler = (e: Event) => events.push((e as CustomEvent).detail)
    window.addEventListener('map-settings-changed', handler)

    await initDisplaySettings()

    window.removeEventListener('map-settings-changed', handler)

    expect(getMapSettings()).toEqual({
      tileUrl: 'https://tiles.example.com/{z}/{x}/{y}.png',
      attribution: '© Example',
      darkFilter: false,
    })
    expect(events).toEqual([
      { tileUrl: 'https://tiles.example.com/{z}/{x}/{y}.png', attribution: '© Example', darkFilter: false },
    ])
  })

  it('keeps the cached settings when the server is unreachable', async () => {
    const { getMapSettings, initDisplaySettings } = await freshUnits()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')))
    vi.spyOn(console, 'warn').mockImplementation(() => {})

    await initDisplaySettings()

    expect(getMapSettings().tileUrl).toBe(DEFAULT_TILE_URL)
  })

  it('persists an update and pushes it to the server', async () => {
    const { getMapSettings, setMapSettings } = await freshUnits()
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(null, { status: 204, headers: { 'content-length': '0' } })
    )
    vi.stubGlobal('fetch', fetchMock)

    await setMapSettings({ tileUrl: 'https://tiles.example.com/{z}/{x}/{y}.png' })

    expect(getMapSettings()).toEqual({
      tileUrl: 'https://tiles.example.com/{z}/{x}/{y}.png',
      attribution: '',
      darkFilter: true,
    })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/settings/display',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({ map_tile_url: 'https://tiles.example.com/{z}/{x}/{y}.png' }),
      })
    )
  })

  it('rolls the cache back when the server refuses the update', async () => {
    const { getMapSettings, setMapSettings } = await freshUnits()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: 'invalid tile URL' }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        })
      )
    )

    const before = getMapSettings()
    await expect(setMapSettings({ tileUrl: CUSTOM_TILE_URL })).rejects.toThrow()

    expect(getMapSettings()).toEqual(before)
  })

  it('sends only the fields being changed', async () => {
    const { getMapSettings, setMapSettings } = await freshUnits()
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(null, { status: 204, headers: { 'content-length': '0' } })
    )
    vi.stubGlobal('fetch', fetchMock)

    await setMapSettings({ darkFilter: false })

    expect(getMapSettings().darkFilter).toBe(false)
    expect(fetchMock.mock.calls[0][1]).toMatchObject({
      body: JSON.stringify({ map_dark_filter: false }),
    })
  })
})
