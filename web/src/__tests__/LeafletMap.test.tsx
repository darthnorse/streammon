import { describe, it, expect, beforeAll, afterAll, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { LeafletMap } from '../components/shared/LeafletMap'
import type { GeoResult } from '../types'

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
