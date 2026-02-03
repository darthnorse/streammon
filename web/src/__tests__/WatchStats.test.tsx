import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { WatchStats } from '../components/WatchStats'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

describe('WatchStats', () => {
  it('renders loading skeleton while fetching', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<WatchStats />)
    expect(screen.getByText('Watch Statistics')).toBeInTheDocument()
    expect(document.querySelector('.animate-pulse')).toBeInTheDocument()
  })

  it('renders error message on failure', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: new Error('fail'), refetch: vi.fn() })
    renderWithRouter(<WatchStats />)
    expect(screen.getByText('Failed to load statistics')).toBeInTheDocument()
  })

  it('renders media stats when data loads', () => {
    mockUseFetch.mockReturnValue({
      data: {
        top_movies: [{ title: 'Test Movie', year: 2024, play_count: 10, total_hours: 5 }],
        top_tv_shows: [{ title: 'Test Show', play_count: 8, total_hours: 4 }],
        top_users: [{ user_name: 'alice', play_count: 15, total_hours: 10 }],
        library: { total_plays: 100, total_hours: 50, unique_users: 5, unique_movies: 10, unique_tv_shows: 5 },
        concurrent_peak: 3,
        locations: [],
        potential_sharers: [],
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    })
    renderWithRouter(<WatchStats />)
    expect(screen.getByText('Most Watched Movies')).toBeInTheDocument()
    expect(screen.getByText('Most Watched TV Shows')).toBeInTheDocument()
    expect(screen.getByText('Test Movie')).toBeInTheDocument()
    expect(screen.getByText('Test Show')).toBeInTheDocument()
  })

  it('has time period dropdown with correct options', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<WatchStats />)
    const select = screen.getByRole('combobox')
    expect(select).toBeInTheDocument()
    expect(screen.getByText('Last 7 days')).toBeInTheDocument()
    expect(screen.getByText('Last 30 days')).toBeInTheDocument()
    expect(screen.getByText('All time')).toBeInTheDocument()
  })
})
