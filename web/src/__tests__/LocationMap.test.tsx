import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { LocationMap } from '../components/LocationMap'
import type { GeoResult } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map">{children}</div>
  ),
  TileLayer: () => null,
  CircleMarker: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="marker">{children}</div>
  ),
  Popup: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  useMap: () => ({
    setView: vi.fn(),
    fitBounds: vi.fn(),
  }),
}))

vi.mock('react-leaflet-heatmap-layer-v3', () => ({
  HeatmapLayer: () => <div data-testid="heatmap" />,
}))

import { useFetch } from '../hooks/useFetch'
import { MS_PER_DAY } from '../lib/constants'

const mockUseFetch = vi.mocked(useFetch)

const locations: GeoResult[] = [
  { ip: '1.2.3.4', lat: 40.7, lng: -74.0, city: 'New York', country: 'US', last_seen: new Date().toISOString() },
  { ip: '5.6.7.8', lat: 51.5, lng: -0.1, city: 'London', country: 'GB', last_seen: new Date(Date.now() - MS_PER_DAY * 2).toISOString() },
]

describe('LocationMap', () => {
  it('shows loading state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<LocationMap userName="alice" />)
    expect(screen.getByText(/loading locations/i)).toBeInTheDocument()
  })

  it('shows error state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: new Error('fail'), refetch: vi.fn() })
    renderWithRouter(<LocationMap userName="alice" />)
    expect(screen.getByText(/failed to load locations/i)).toBeInTheDocument()
  })

  it('shows empty state when no locations', () => {
    mockUseFetch.mockReturnValue({ data: [], loading: false, error: null, refetch: vi.fn() })
    renderWithRouter(<LocationMap userName="alice" />)
    expect(screen.getByText(/no location data/i)).toBeInTheDocument()
  })

  it('renders map and table with locations', () => {
    mockUseFetch.mockReturnValue({ data: locations, loading: false, error: null, refetch: vi.fn() })
    renderWithRouter(<LocationMap userName="alice" />)

    expect(screen.getByTestId('map')).toBeInTheDocument()
    expect(screen.getAllByText('1.2.3.4').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('5.6.7.8').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('New York, US').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('London, GB').length).toBeGreaterThanOrEqual(1)
  })

  it('renders markers for each location (markers mode)', () => {
    mockUseFetch.mockReturnValue({ data: locations, loading: false, error: null, refetch: vi.fn() })
    const { container } = renderWithRouter(<LocationMap userName="alice" />)

    const markers = container.querySelectorAll('[data-testid="marker"]')
    expect(markers.length).toBe(2)
  })

  it('fetches locations for the correct user', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<LocationMap userName="bob smith" />)
    const url = mockUseFetch.mock.calls[0][0] as string
    expect(url).toBe('/api/users/bob%20smith/locations')
  })
})
