import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { LocationsCard } from '../components/stats/LocationsCard'
import type { GeoResult, ViewMode } from '../types'

vi.mock('../components/shared/LeafletMap', () => ({
  LeafletMap: ({ viewMode }: { locations: GeoResult[]; viewMode?: ViewMode }) => (
    <div data-testid="map">
      {viewMode === 'markers'
        ? <div data-testid="marker" />
        : <div data-testid="heatmap" />}
    </div>
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

  it('renders the map', () => {
    render(<LocationsCard locations={mockLocations} />)

    expect(screen.getByTestId('map')).toBeInTheDocument()
  })

  it('renders heatmap by default', () => {
    render(<LocationsCard locations={mockLocations} />)

    expect(screen.getByTestId('heatmap')).toBeInTheDocument()
  })

  it('renders location table with correct data', () => {
    render(<LocationsCard locations={mockLocations} />)

    expect(screen.getByText('Location')).toBeInTheDocument()
    expect(screen.getByText('Users')).toBeInTheDocument()

    expect(screen.getByText('New York, US')).toBeInTheDocument()
    expect(screen.getByText('Los Angeles, US')).toBeInTheDocument()

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

    const cells = screen.getAllByRole('cell')
    const usersCell = cells.find((cell: HTMLElement) => cell.textContent === '—')
    expect(usersCell).toBeInTheDocument()
  })

  it('shows all users in table', () => {
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

    expect(screen.getByText('user1, user2, user3, user4, user5, user6, user7')).toBeInTheDocument()
  })

  it('shows view mode toggle when locations exist', () => {
    render(<LocationsCard locations={mockLocations} />)

    expect(screen.getByText('Heatmap')).toBeInTheDocument()
    expect(screen.getByText('Markers')).toBeInTheDocument()
  })

  it('switches to markers view when clicking Markers button', () => {
    render(<LocationsCard locations={mockLocations} />)

    expect(screen.getByTestId('heatmap')).toBeInTheDocument()

    fireEvent.click(screen.getByText('Markers'))

    expect(screen.queryByTestId('heatmap')).not.toBeInTheDocument()
    expect(screen.getByTestId('marker')).toBeInTheDocument()
  })

  it('switches back to heatmap view when clicking Heatmap button', () => {
    render(<LocationsCard locations={mockLocations} />)

    fireEvent.click(screen.getByText('Markers'))
    expect(screen.queryByTestId('heatmap')).not.toBeInTheDocument()

    fireEvent.click(screen.getByText('Heatmap'))
    expect(screen.getByTestId('heatmap')).toBeInTheDocument()
  })

  it('has correct aria-pressed attributes on toggle buttons', () => {
    render(<LocationsCard locations={mockLocations} />)

    const heatmapBtn = screen.getByText('Heatmap')
    const markersBtn = screen.getByText('Markers')

    expect(heatmapBtn).toHaveAttribute('aria-pressed', 'true')
    expect(markersBtn).toHaveAttribute('aria-pressed', 'false')

    fireEvent.click(markersBtn)

    expect(heatmapBtn).toHaveAttribute('aria-pressed', 'false')
    expect(markersBtn).toHaveAttribute('aria-pressed', 'true')
  })
})
