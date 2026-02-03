import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { LocationMap } from '../components/LocationMap'
import type { GeoResult } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

vi.mock('react-simple-maps', () => ({
  ComposableMap: ({ children }: { children: React.ReactNode }) => <div data-testid="map">{children}</div>,
  Geographies: ({ children }: { children: (props: { geographies: never[] }) => React.ReactNode }) => <>{children({ geographies: [] })}</>,
  Geography: () => null,
  Marker: ({ children }: { children: React.ReactNode }) => <div data-testid="marker">{children}</div>,
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

const MS_PER_DAY = 86_400_000

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
    expect(screen.getByText('1.2.3.4')).toBeInTheDocument()
    expect(screen.getByText('5.6.7.8')).toBeInTheDocument()
    expect(screen.getByText('New York, US')).toBeInTheDocument()
    expect(screen.getByText('London, GB')).toBeInTheDocument()
  })

  it('renders markers for each location', () => {
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
