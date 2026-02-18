import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { Statistics } from '../pages/Statistics'
import { createMockStats } from './fixtures'
import type { Server } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

vi.mock('../components/DatePicker', () => ({
  DatePicker: ({ value, onChange, label }: { value: string; onChange: (v: string) => void; label: string }) => (
    <input
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      aria-label={label}
    />
  ),
}))

vi.mock('../components/DailyChart', () => ({
  DailyChart: () => <div data-testid="daily-chart" />,
}))

vi.mock('../components/stats/LibraryCards', () => ({
  LibraryCards: () => <div data-testid="library-cards" />,
}))

vi.mock('../components/stats/TopMediaCard', () => ({
  TopMediaCard: () => <div data-testid="top-media-card" />,
}))

vi.mock('../components/stats/TopUsersCard', () => ({
  TopUsersCard: () => <div data-testid="top-users-card" />,
}))

vi.mock('../components/stats/LocationsCard', () => ({
  LocationsCard: () => <div data-testid="locations-card" />,
}))

vi.mock('../components/stats/ActivityByDayChart', () => ({
  ActivityByDayChart: () => <div data-testid="activity-day" />,
}))

vi.mock('../components/stats/ActivityByHourChart', () => ({
  ActivityByHourChart: () => <div data-testid="activity-hour" />,
}))

vi.mock('../components/stats/DistributionDonut', () => ({
  DistributionDonut: () => <div data-testid="donut" />,
}))

vi.mock('../components/stats/ConcurrentStreamsChart', () => ({
  ConcurrentStreamsChart: () => <div data-testid="concurrent" />,
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

const mockServers: Server[] = [
  { id: 1, name: 'Plex Server', type: 'plex', url: 'http://localhost:32400', enabled: true, show_recent_media: true, created_at: '', updated_at: '' },
  { id: 2, name: 'Jellyfin Server', type: 'jellyfin', url: 'http://localhost:8096', enabled: true, show_recent_media: true, created_at: '', updated_at: '' },
]

beforeEach(() => {
  localStorage.clear()
  mockUseFetch.mockReset()
})

function mockStats(servers: Server[] | null = null) {
  mockUseFetch.mockImplementation((url: string) => {
    if (url === '/api/servers') {
      return { data: servers, loading: false, error: null, refetch: vi.fn() }
    }
    return { data: createMockStats(), loading: false, error: null, refetch: vi.fn() }
  })
}

function getStatsUrl(): string {
  const calls = mockUseFetch.mock.calls
  const statsCalls = calls.filter(([url]) => (url as string).startsWith('/api/stats'))
  const last = statsCalls[statsCalls.length - 1]
  return last ? (last[0] as string) : ''
}

describe('Statistics', () => {
  it('defaults to 30 days filter', () => {
    mockStats()
    renderWithRouter(<Statistics />)
    const btn30 = screen.getByText('30 days')
    expect(btn30.className).toContain('text-accent')
  })

  it('renders filter options in correct order: 7d, 30d, All time, Custom', () => {
    mockStats()
    renderWithRouter(<Statistics />)
    const buttons = screen.getAllByRole('button').filter(btn =>
      ['7 days', '30 days', 'All time', 'Custom'].includes(btn.textContent ?? '')
    )
    expect(buttons).toHaveLength(4)
    expect(buttons[0]).toHaveTextContent('7 days')
    expect(buttons[1]).toHaveTextContent('30 days')
    expect(buttons[2]).toHaveTextContent('All time')
    expect(buttons[3]).toHaveTextContent('Custom')
  })

  it('shows date pickers when Custom is selected', async () => {
    mockStats()
    renderWithRouter(<Statistics />)
    expect(screen.queryByLabelText('Start date')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('End date')).not.toBeInTheDocument()

    await userEvent.click(screen.getByText('Custom'))

    expect(screen.getByLabelText('Start date')).toBeInTheDocument()
    expect(screen.getByLabelText('End date')).toBeInTheDocument()
  })

  it('hides date pickers for non-custom filters', async () => {
    mockStats()
    renderWithRouter(<Statistics />)

    await userEvent.click(screen.getByText('Custom'))
    expect(screen.getByLabelText('Start date')).toBeInTheDocument()

    await userEvent.click(screen.getByText('7 days'))
    expect(screen.queryByLabelText('Start date')).not.toBeInTheDocument()
  })

  it('pre-fills dates from current filter when switching to Custom', async () => {
    mockStats()
    renderWithRouter(<Statistics />)
    await userEvent.click(screen.getByText('Custom'))

    const startInput = screen.getByLabelText('Start date') as HTMLInputElement
    const endInput = screen.getByLabelText('End date') as HTMLInputElement
    expect(startInput.value).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(endInput.value).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })

  it('pre-fills dates immediately so stats keep showing', async () => {
    mockStats()
    renderWithRouter(<Statistics />)
    await userEvent.click(screen.getByText('Custom'))
    const url = getStatsUrl()
    expect(url).toContain('start_date=')
    expect(url).toContain('end_date=')
  })

  it('shows server picker when multiple servers exist', () => {
    mockStats(mockServers)
    renderWithRouter(<Statistics />)
    expect(screen.getByText('All Servers')).toBeInTheDocument()
  })

  it('hides server picker when only one server exists', () => {
    mockStats([mockServers[0]])
    renderWithRouter(<Statistics />)
    expect(screen.queryByText('All Servers')).not.toBeInTheDocument()
  })

  it('hides server picker when no servers loaded', () => {
    mockStats(null)
    renderWithRouter(<Statistics />)
    expect(screen.queryByText('All Servers')).not.toBeInTheDocument()
  })

  it('persists selected filter to localStorage', async () => {
    mockStats()
    renderWithRouter(<Statistics />)
    await userEvent.click(screen.getByText('7 days'))
    expect(localStorage.getItem('streammon:stats-days')).toBe('7')
  })

  it('keeps filters visible during loading state', () => {
    mockUseFetch.mockImplementation((url: string) => {
      if (url === '/api/servers') {
        return { data: null, loading: false, error: null, refetch: vi.fn() }
      }
      return { data: null, loading: true, error: null, refetch: vi.fn() }
    })
    renderWithRouter(<Statistics />)
    expect(screen.getByText('Loading statistics...')).toBeInTheDocument()
    expect(screen.getByText('Statistics')).toBeInTheDocument()
    expect(screen.getByText('30 days')).toBeInTheDocument()
  })

  it('keeps filters visible during error state', () => {
    mockUseFetch.mockImplementation((url: string) => {
      if (url === '/api/servers') {
        return { data: null, loading: false, error: null, refetch: vi.fn() }
      }
      return { data: null, loading: false, error: new Error('fail'), refetch: vi.fn() }
    })
    renderWithRouter(<Statistics />)
    expect(screen.getByText('Error loading statistics')).toBeInTheDocument()
    expect(screen.getByText('Statistics')).toBeInTheDocument()
    expect(screen.getByText('7 days')).toBeInTheDocument()
  })

  it('fetches stats with days=30 by default', () => {
    mockStats()
    renderWithRouter(<Statistics />)
    expect(getStatsUrl()).toBe('/api/stats?days=30')
  })

  it('fetches stats with days=7 after clicking 7 days', async () => {
    mockStats()
    renderWithRouter(<Statistics />)
    await userEvent.click(screen.getByText('7 days'))
    expect(getStatsUrl()).toBe('/api/stats?days=7')
  })

  it('fetches stats with no days param for All time', async () => {
    mockStats()
    renderWithRouter(<Statistics />)
    await userEvent.click(screen.getByText('All time'))
    expect(getStatsUrl()).toBe('/api/stats')
  })

  it('fetches stats with start_date and end_date for custom range', async () => {
    mockStats()
    renderWithRouter(<Statistics />)
    await userEvent.click(screen.getByText('Custom'))
    await userEvent.clear(screen.getByLabelText('Start date'))
    await userEvent.type(screen.getByLabelText('Start date'), '2024-01-01')
    await userEvent.clear(screen.getByLabelText('End date'))
    await userEvent.type(screen.getByLabelText('End date'), '2024-02-01')
    const url = getStatsUrl()
    expect(url).toContain('start_date=2024-01-01')
    expect(url).toContain('end_date=2024-02-01')
    expect(url).not.toContain('days=')
  })
})
