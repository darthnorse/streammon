import { describe, it, expect, beforeAll, beforeEach, afterAll, vi } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { LeafletMap } from '../components/shared/LeafletMap'
import { DARK_TILE_FILTER } from '../lib/mapUtils'
import type { GeoResult } from '../types'

// The map fetches display settings on mount. jsdom has no origin for a
// relative URL, so let the request fail deterministically and keep the cached
// (localStorage) values as the single source of truth for these tests.
beforeAll(() => {
  vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('no server in tests')))
  vi.spyOn(console, 'warn').mockImplementation(() => {})
})

// Leaflet only creates tile <img> elements for a container with a real size;
// jsdom reports every element as 0x0, so the map would otherwise render an
// empty tile pane and the basemap assertions would pass vacuously.
beforeAll(() => {
  for (const [prop, size] of [['clientWidth', 800], ['clientHeight', 600]] as const) {
    Object.defineProperty(HTMLElement.prototype, prop, { configurable: true, value: size })
  }
})

afterAll(() => {
  for (const prop of ['clientWidth', 'clientHeight'] as const) {
    Object.defineProperty(HTMLElement.prototype, prop, { configurable: true, value: 0 })
  }
})

// jsdom ships no canvas implementation, so the heatmap layer gets a stub that
// accepts every 2d call simpleheat makes while drawing.
function stubContext(canvas: HTMLCanvasElement): CanvasRenderingContext2D {
  const noop = () => {}
  return {
    canvas,
    arc: noop,
    beginPath: noop,
    clearRect: noop,
    closePath: noop,
    createLinearGradient: () => ({ addColorStop: noop }),
    drawImage: noop,
    fill: noop,
    fillRect: noop,
    getImageData: (_x: number, _y: number, width: number, height: number) => ({
      width,
      height,
      data: new Uint8ClampedArray(Math.max(width, 1) * Math.max(height, 1) * 4),
    }),
    putImageData: noop,
  } as unknown as CanvasRenderingContext2D
}

beforeAll(() => {
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(function (
    this: HTMLCanvasElement
  ) {
    return stubContext(this)
  } as unknown as HTMLCanvasElement['getContext'])
})

afterAll(() => {
  vi.restoreAllMocks()
})

const locations: GeoResult[] = [
  { ip: '1.1.1.1', lat: 40.7, lng: -74.0, city: 'New York', country: 'US', users: ['alice', 'bob'] },
  { ip: '2.2.2.2', lat: 34.0, lng: -118.2, city: 'Los Angeles', country: 'US', users: ['carol'] },
]

function heatmapCanvases(container: HTMLElement): HTMLCanvasElement[] {
  return Array.from(container.querySelectorAll<HTMLCanvasElement>('.leaflet-overlay-pane canvas'))
}

describe('LeafletMap', () => {
  it('draws a heatmap canvas onto the overlay pane in heatmap view', () => {
    const { container } = render(<LeafletMap locations={locations} viewMode="heatmap" />)

    expect(heatmapCanvases(container)).toHaveLength(1)
    expect(screen.queryByText('Map failed to load')).not.toBeInTheDocument()
  })

  it('draws circle markers and no heatmap canvas in markers view', () => {
    const { container } = render(<LeafletMap locations={locations} viewMode="markers" />)

    expect(heatmapCanvases(container)).toHaveLength(0)
    expect(container.querySelectorAll('.leaflet-overlay-pane path')).toHaveLength(locations.length)
  })

  it('draws no heatmap when there are no locations', () => {
    const { container } = render(<LeafletMap locations={[]} viewMode="heatmap" />)

    expect(heatmapCanvases(container)).toHaveLength(0)
    expect(screen.queryByText('Map failed to load')).not.toBeInTheDocument()
  })

  // Identity, not count: a remount detaches the old canvas before attaching the
  // new one, so a length assertion cannot tell "reused" from "torn down and rebuilt".
  it('reuses the same heatmap canvas when the locations change', () => {
    const { container, rerender } = render(<LeafletMap locations={locations} viewMode="heatmap" />)
    const firstCanvas = heatmapCanvases(container)[0]

    rerender(<LeafletMap locations={[locations[0]]} viewMode="heatmap" />)

    expect(heatmapCanvases(container)).toHaveLength(1)
    expect(heatmapCanvases(container)[0]).toBe(firstCanvas)
    expect(screen.queryByText('Map failed to load')).not.toBeInTheDocument()
  })

  it('removes the heatmap canvas when switching to markers view', () => {
    const { container, rerender } = render(<LeafletMap locations={locations} viewMode="heatmap" />)
    expect(heatmapCanvases(container)).toHaveLength(1)

    rerender(<LeafletMap locations={locations} viewMode="markers" />)

    expect(heatmapCanvases(container)).toHaveLength(0)
  })
})

describe('LeafletMap basemap', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
  })

  // Cleared in beforeEach, not afterEach: an afterEach runs before Testing
  // Library unmounts, so toggling the class there would fire useIsDark's
  // observer on a still-mounted map and warn about a state update outside act.
  afterAll(() => {
    document.documentElement.classList.remove('dark')
  })

  function tilePane(container: HTMLElement): HTMLElement {
    const pane = container.querySelector<HTMLElement>('.leaflet-tile-pane')
    if (!pane) throw new Error('no tile pane')
    return pane
  }

  function overlayPane(container: HTMLElement): HTMLElement {
    const pane = container.querySelector<HTMLElement>('.leaflet-overlay-pane')
    if (!pane) throw new Error('no overlay pane')
    return pane
  }

  function attributionHtml(container: HTMLElement): string {
    return container.querySelector('.leaflet-control-attribution')?.innerHTML ?? ''
  }

  it('loads tiles from OpenStreetMap, not from CARTO', () => {
    const { container } = render(<LeafletMap locations={locations} viewMode="markers" />)

    const tiles = Array.from(container.querySelectorAll<HTMLImageElement>('.leaflet-tile-pane img'))
    expect(tiles.length).toBeGreaterThan(0)
    for (const tile of tiles) {
      expect(tile.src).toMatch(/^https:\/\/tile\.openstreetmap\.org\/\d+\/-?\d+\/-?\d+\.png$/)
    }
  })

  it('credits OpenStreetMap and links the map issue reporter', () => {
    const { container } = render(<LeafletMap locations={locations} viewMode="markers" />)

    const html = attributionHtml(container)
    expect(html).toContain('OpenStreetMap')
    expect(html).toContain('https://www.openstreetmap.org/fixthemap')
    expect(html.toLowerCase()).not.toContain('carto')
  })

  it('inverts the tile pane in dark mode and leaves markers alone', () => {
    document.documentElement.classList.add('dark')

    const { container } = render(<LeafletMap locations={locations} viewMode="markers" />)

    expect(tilePane(container).style.filter).toBe(DARK_TILE_FILTER)
    expect(overlayPane(container).style.filter).toBe('')
  })

  it('leaves the tiles unfiltered in light mode', () => {
    const { container } = render(<LeafletMap locations={locations} viewMode="markers" />)

    expect(tilePane(container).style.filter).toBe('')
  })

  it('drops the filter when the theme switches back to light', async () => {
    document.documentElement.classList.add('dark')
    const { container } = render(<LeafletMap locations={locations} viewMode="markers" />)
    expect(tilePane(container).style.filter).toBe(DARK_TILE_FILTER)

    // useIsDark observes the class attribute; MutationObserver callbacks are
    // microtasks, so the update needs an async act to flush.
    await act(async () => {
      document.documentElement.classList.remove('dark')
    })

    expect(tilePane(container).style.filter).toBe('')
  })

  it('uses the configured tile url and escapes its attribution', () => {
    localStorage.setItem('streammon:map-tile-url', 'https://tiles.example.com/{z}/{x}/{y}.png')
    localStorage.setItem('streammon:map-tile-attribution', '© Example <script>')

    const { container } = render(<LeafletMap locations={locations} viewMode="markers" />)

    const tiles = Array.from(container.querySelectorAll<HTMLImageElement>('.leaflet-tile-pane img'))
    expect(tiles.length).toBeGreaterThan(0)
    for (const tile of tiles) {
      expect(tile.src).toContain('https://tiles.example.com/')
    }

    const attribution = container.querySelector('.leaflet-control-attribution')
    expect(attribution?.querySelector('script')).toBeNull()
    expect(attribution?.textContent).toContain('© Example <script>')
  })

  it('honours the dark filter being switched off for a custom dark basemap', () => {
    document.documentElement.classList.add('dark')
    localStorage.setItem('streammon:map-dark-filter', 'false')

    const { container } = render(<LeafletMap locations={locations} viewMode="markers" />)

    expect(tilePane(container).style.filter).toBe('')
  })
})

describe('LeafletMap viewport', () => {
  function tileZooms(container: HTMLElement): string[] {
    return Array.from(container.querySelectorAll<HTMLImageElement>('.leaflet-tile-pane img'))
      .map((t) => new URL(t.src).pathname.split('/')[1])
  }

  it('keeps a zoom the user chose when the same locations arrive as a new array', () => {
    const { container, rerender } = render(<LeafletMap locations={locations} viewMode="markers" />)

    const zoomIn = container.querySelector<HTMLAnchorElement>('.leaflet-control-zoom-in')
    if (!zoomIn) throw new Error('no zoom control')
    // A real click always delivers mousedown first; that is the signal the map
    // uses to tell a user gesture from a programmatic move.
    act(() => {
      zoomIn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }))
      zoomIn.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }))
    })
    const zoomed = tileZooms(container)[0]

    // The poller hands down a fresh array of the same places every tick.
    act(() => {
      rerender(<LeafletMap locations={locations.map((l) => ({ ...l }))} viewMode="markers" />)
    })

    expect(tileZooms(container)[0]).toBe(zoomed)
  })
})
