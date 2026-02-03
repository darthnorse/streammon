import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { LocationsCard } from '../components/stats/LocationsCard'
import type { GeoResult } from '../types'

// Mock react-simple-maps since it requires canvas/svg rendering
vi.mock('react-simple-maps', () => ({
  ComposableMap: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map">{children}</div>
  ),
  Geographies: ({ children }: { children: (args: { geographies: [] }) => React.ReactNode }) => (
    <>{children({ geographies: [] })}</>
  ),
  Geography: () => null,
  Marker: ({ children, coordinates }: { children: React.ReactNode; coordinates: [number, number] }) => (
    <div data-testid={`marker-${coordinates[0]}-${coordinates[1]}`}>{children}</div>
  ),
}))

const mockLocations: GeoResult[] = [
  {
    lat: 40.7,
    lng: -74.0,
    city: 'New York',
    country: 'US',
    users: ['alice', 'bob'],
  },
  {
    lat: 34.0,
    lng: -118.2,
    city: 'Los Angeles',
    country: 'US',
    users: ['carol'],
  },
]

describe('LocationsCard', () => {
  it('renders the component with location count', () => {
    render(<LocationsCard locations={mockLocations} />)

    expect(screen.getByText('Watch Locations')).toBeInTheDocument()
    expect(screen.getByText('(2 locations)')).toBeInTheDocument()
  })

  it('renders empty state when no locations', () => {
    render(<LocationsCard locations={[]} />)

    expect(screen.getByText('No location data available')).toBeInTheDocument()
  })

  it('renders the map with markers', () => {
    render(<LocationsCard locations={mockLocations} />)

    expect(screen.getByTestId('map')).toBeInTheDocument()
    expect(screen.getByTestId('marker--74-40.7')).toBeInTheDocument()
    expect(screen.getByTestId('marker--118.2-34')).toBeInTheDocument()
  })

  it('renders location table with correct data', () => {
    render(<LocationsCard locations={mockLocations} />)

    // Check table headers
    expect(screen.getByText('Location')).toBeInTheDocument()
    expect(screen.getByText('Users')).toBeInTheDocument()

    // Check location data in table
    expect(screen.getByText('New York, US')).toBeInTheDocument()
    expect(screen.getByText('Los Angeles, US')).toBeInTheDocument()

    // Check users in table
    expect(screen.getByText('alice, bob')).toBeInTheDocument()
    expect(screen.getByText('carol')).toBeInTheDocument()
  })

  it('handles missing city gracefully', () => {
    const locationsWithMissingCity: GeoResult[] = [
      {
        lat: 40.7,
        lng: -74.0,
        city: '',
        country: 'US',
        users: ['alice'],
      },
    ]

    render(<LocationsCard locations={locationsWithMissingCity} />)

    // Should show just country when city is missing
    expect(screen.getByText('US')).toBeInTheDocument()
  })

  it('handles missing users gracefully', () => {
    const locationsWithoutUsers: GeoResult[] = [
      {
        lat: 40.7,
        lng: -74.0,
        city: 'New York',
        country: 'US',
      },
    ]

    render(<LocationsCard locations={locationsWithoutUsers} />)

    // Table should show dash for missing users
    const cells = screen.getAllByRole('cell')
    const usersCell = cells.find(cell => cell.textContent === '—')
    expect(usersCell).toBeInTheDocument()
  })

  it('truncates long user lists in tooltip format', () => {
    const locationWithManyUsers: GeoResult[] = [
      {
        lat: 40.7,
        lng: -74.0,
        city: 'New York',
        country: 'US',
        users: ['user1', 'user2', 'user3', 'user4', 'user5', 'user6', 'user7'],
      },
    ]

    render(<LocationsCard locations={locationWithManyUsers} />)

    // Table shows all users (not truncated)
    expect(screen.getByText('user1, user2, user3, user4, user5, user6, user7')).toBeInTheDocument()
  })
})
