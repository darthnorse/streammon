import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { WatchStats } from '../components/WatchStats'
import type { StatsResponse } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

function createMockStats(overrides: Partial<StatsResponse> = {}): StatsResponse {
  return {
    top_movies: [],
    top_tv_shows: [],
    top_users: [],
    library: { total_plays: 0, total_hours: 0, unique_users: 0, unique_movies: 0, unique_tv_shows: 0 },
    locations: [],
    potential_sharers: [],
    activity_by_day_of_week: [],
    activity_by_hour: [],
    platform_distribution: [],
    player_distribution: [],
    quality_distribution: [],
    concurrent_time_series: [],
    concurrent_peaks: { total: 0, direct_play: 0, direct_stream: 0, transcode: 0 },
    ...overrides,
  }
}

function mockLoading() {
  mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
}

function mockError() {
  mockUseFetch.mockReturnValue({ data: null, loading: false, error: new Error('fail'), refetch: vi.fn() })
}

function mockData(data: StatsResponse) {
  mockUseFetch.mockReturnValue({ data, loading: false, error: null, refetch: vi.fn() })
}

describe('WatchStats', () => {
  it('renders loading skeleton while fetching', () => {
    mockLoading()
    renderWithRouter(<WatchStats />)
    expect(screen.getByText('Watch Statistics')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Time period' })).toBeInTheDocument()
    expect(document.querySelector('.animate-pulse')).toBeInTheDocument()
  })

  it('renders error message on failure', () => {
    mockError()
    renderWithRouter(<WatchStats />)
    expect(screen.getByText('Failed to load statistics')).toBeInTheDocument()
  })

  it('renders media stats when data loads', () => {
    mockData(createMockStats({
      top_movies: [{ title: 'Test Movie', year: 2024, play_count: 10, total_hours: 5 }],
      top_tv_shows: [{ title: 'Test Show', play_count: 8, total_hours: 4 }],
      top_users: [{ user_name: 'alice', play_count: 15, total_hours: 10 }],
      concurrent_peaks: { total: 5, direct_play: 3, direct_stream: 1, transcode: 1 },
    }))
    renderWithRouter(<WatchStats />)
    expect(screen.getByText('Most Watched Movies')).toBeInTheDocument()
    expect(screen.getByText('Most Watched TV Shows')).toBeInTheDocument()
    expect(screen.getByText('Test Movie')).toBeInTheDocument()
    expect(screen.getByText('Test Show')).toBeInTheDocument()
    expect(screen.getByText('Peak Concurrent Streams')).toBeInTheDocument()
    expect(screen.getByText('Direct Play')).toBeInTheDocument()
    expect(screen.getByText('Direct Stream')).toBeInTheDocument()
    expect(screen.getByText('Transcode')).toBeInTheDocument()
  })

  it('has time period dropdown with correct options and aria-label', () => {
    mockLoading()
    renderWithRouter(<WatchStats />)
    const select = screen.getByRole('combobox', { name: 'Time period' })
    expect(select).toBeInTheDocument()
    expect(screen.getByText('Last 7 days')).toBeInTheDocument()
    expect(screen.getByText('Last 30 days')).toBeInTheDocument()
    expect(screen.getByText('All time')).toBeInTheDocument()
  })

  it('keeps header visible in error state', () => {
    mockError()
    renderWithRouter(<WatchStats />)
    expect(screen.getByText('Watch Statistics')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Time period' })).toBeInTheDocument()
    expect(screen.getByText('Failed to load statistics')).toBeInTheDocument()
  })

  it('renders TopUsersCard with compact mode on dashboard', () => {
    mockData(createMockStats({
      top_users: [
        { user_name: 'alice', play_count: 15, total_hours: 10 },
        { user_name: 'bob', play_count: 10, total_hours: 8 },
      ],
    }))
    renderWithRouter(<WatchStats />)
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
  })
})
