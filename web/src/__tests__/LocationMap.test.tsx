import { describe, it, expect, vi, afterEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { LocationMap } from '../components/LocationMap'
import type { GeoResult } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

vi.mock('../hooks/useIsDark', () => ({
  useIsDark: () => true,
}))

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="marker">{children}</div>
  ),
  Popup: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="popup">{children}</div>
  ),
  useMap: () => ({ fitBounds: vi.fn(), setView: vi.fn() }),
}))

vi.mock('leaflet', () => ({
  default: {
    Icon: class { constructor() {} },
    latLngBounds: () => ({}),
  },
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

afterEach(() => {
  vi.restoreAllMocks()
})

const locations: GeoResult[] = [
  { ip: '1.2.3.4', lat: 40.7, lng: -74.0, city: 'New York', country: 'US' },
  { ip: '5.6.7.8', lat: 51.5, lng: -0.1, city: 'London', country: 'GB' },
]

describe('LocationMap', () => {
  it('shows loading state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<LocationMap userName="alice" />)
    expect(screen.getByText(/loading map/i)).toBeDefined()
  })

  it('shows error state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: new Error('fail'), refetch: vi.fn() })
    renderWithRouter(<LocationMap userName="alice" />)
    expect(screen.getByText(/failed to load locations/i)).toBeDefined()
  })

  it('shows empty state when no locations', () => {
    mockUseFetch.mockReturnValue({ data: [], loading: false, error: null, refetch: vi.fn() })
    renderWithRouter(<LocationMap userName="alice" />)
    expect(screen.getByText(/no location data/i)).toBeDefined()
  })

  it('renders map with markers when data exists', () => {
    mockUseFetch.mockReturnValue({ data: locations, loading: false, error: null, refetch: vi.fn() })
    renderWithRouter(<LocationMap userName="alice" />)
    expect(screen.getByTestId('map-container')).toBeDefined()
    expect(screen.getAllByTestId('marker')).toHaveLength(2)
    expect(screen.getByText('New York, US')).toBeDefined()
    expect(screen.getByText('London, GB')).toBeDefined()
  })

  it('fetches locations for the correct user', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<LocationMap userName="bob smith" />)
    const url = mockUseFetch.mock.calls[0][0] as string
    expect(url).toBe('/api/users/bob%20smith/locations')
  })
})
