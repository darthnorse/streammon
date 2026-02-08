import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { render } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { UserDetail } from '../pages/UserDetail'
import { ApiError } from '../lib/api'
import { baseUser, baseHistoryEntry } from './fixtures'
import type { WatchHistoryEntry, PaginatedResult, UserDetailStats } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

vi.mock('../components/LocationMap', () => ({
  LocationMap: ({ userName }: { userName: string }) => (
    <div data-testid="location-map">{userName}</div>
  ),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

const noData = () => ({ data: null, loading: false, error: null, refetch: vi.fn() })

beforeEach(() => {
  localStorage.clear()
  // Default fallback for any useFetch call (child components like UserTrustScoreCard, UserHouseholdCard)
  mockUseFetch.mockReturnValue(noData())
})

afterEach(() => {
  vi.restoreAllMocks()
})

const testHistory: PaginatedResult<WatchHistoryEntry> = {
  items: [baseHistoryEntry],
  total: 1,
  page: 1,
  per_page: 20,
}

const testStats: UserDetailStats = {
  session_count: 42,
  total_hours: 12.5,
  locations: [
    { city: 'New York', country: 'US', session_count: 30, percentage: 71.4, last_seen: '2024-01-15T12:00:00Z' },
    { city: 'London', country: 'UK', session_count: 12, percentage: 28.6, last_seen: '2024-01-14T10:00:00Z' },
  ],
  devices: [
    { player: 'Chrome', platform: 'Windows', session_count: 25, percentage: 59.5, last_seen: '2024-01-15T12:00:00Z' },
    { player: 'Plex TV', platform: 'Android', session_count: 17, percentage: 40.5, last_seen: '2024-01-14T10:00:00Z' },
  ],
  isps: [
    { isp: 'Comcast', session_count: 35, percentage: 83.3, last_seen: '2024-01-15T12:00:00Z' },
    { isp: 'BT', session_count: 7, percentage: 16.7, last_seen: '2024-01-14T10:00:00Z' },
  ],
}

function renderAtRoute(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/users/:name" element={<UserDetail />} />
      </Routes>
    </MemoryRouter>
  )
}

// Helper: set specific values for the first N useFetch calls, fallback handles the rest
function mockFetchSequence(...values: ReturnType<typeof noData>[]) {
  let chain = mockUseFetch
  for (const v of values) {
    chain = chain.mockReturnValueOnce(v) as typeof mockUseFetch
  }
}

describe('UserDetail', () => {
  it('shows loading state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderAtRoute('/users/alice')
    expect(screen.getByText(/loading user/i)).toBeDefined()
  })

  it('shows page with username when user record not found (404)', () => {
    const err = new ApiError(404, 'not found')
    mockFetchSequence(
      { data: null, loading: false, error: err, refetch: vi.fn() },
      { data: testStats, loading: false, error: null, refetch: vi.fn() },
      { data: testHistory, loading: false, error: null, refetch: vi.fn() },
    )
    renderAtRoute('/users/nobody')
    expect(screen.getByRole('heading', { name: 'nobody' })).toBeDefined()
  })

  it('shows generic error for non-404', () => {
    const err = new ApiError(500, 'server error')
    mockFetchSequence(
      { data: null, loading: false, error: err, refetch: vi.fn() },
    )
    renderAtRoute('/users/alice')
    expect(screen.getByText(/failed to load user/i)).toBeDefined()
  })

  it('renders user header with role badge', () => {
    mockFetchSequence(
      { data: baseUser, loading: false, error: null, refetch: vi.fn() },
      { data: testStats, loading: false, error: null, refetch: vi.fn() },
      { data: testHistory, loading: false, error: null, refetch: vi.fn() },
    )
    renderAtRoute('/users/alice')
    expect(screen.getByText('alice')).toBeDefined()
    expect(screen.getByText('admin')).toBeDefined()
  })

  it('renders watch history tab by default', () => {
    mockFetchSequence(
      { data: baseUser, loading: false, error: null, refetch: vi.fn() },
      { data: testStats, loading: false, error: null, refetch: vi.fn() },
      { data: testHistory, loading: false, error: null, refetch: vi.fn() },
    )
    renderAtRoute('/users/alice')
    expect(screen.getAllByText('Inception').length).toBeGreaterThan(0)
  })

  it('hides user column in history table (hideUser prop)', () => {
    mockFetchSequence(
      { data: baseUser, loading: false, error: null, refetch: vi.fn() },
      { data: testStats, loading: false, error: null, refetch: vi.fn() },
      { data: testHistory, loading: false, error: null, refetch: vi.fn() },
    )
    renderAtRoute('/users/alice')
    const table = document.querySelector('table')
    const userLinks = table?.querySelectorAll('a[href="/users/alice"]')
    expect(userLinks?.length ?? 0).toBe(0)
  })

  it('excludes user column from column settings', () => {
    mockFetchSequence(
      { data: baseUser, loading: false, error: null, refetch: vi.fn() },
      { data: testStats, loading: false, error: null, refetch: vi.fn() },
      { data: testHistory, loading: false, error: null, refetch: vi.fn() },
    )
    renderAtRoute('/users/alice')
    const headers = document.querySelectorAll('table th')
    const headerTexts = Array.from(headers).map(h => h.textContent?.toLowerCase())
    expect(headerTexts).not.toContain('user')
  })

  it('renders stats cards with session count and watch time', () => {
    mockFetchSequence(
      { data: baseUser, loading: false, error: null, refetch: vi.fn() },
      { data: testStats, loading: false, error: null, refetch: vi.fn() },
      { data: testHistory, loading: false, error: null, refetch: vi.fn() },
    )
    renderAtRoute('/users/alice')
    expect(screen.getByText('42')).toBeDefined()
    expect(screen.getByText('12.5h')).toBeDefined()
  })

  it('renders locations card with percentage bars', () => {
    mockFetchSequence(
      { data: baseUser, loading: false, error: null, refetch: vi.fn() },
      { data: testStats, loading: false, error: null, refetch: vi.fn() },
      { data: testHistory, loading: false, error: null, refetch: vi.fn() },
    )
    renderAtRoute('/users/alice')
    expect(screen.getByText('New York, US')).toBeDefined()
    expect(screen.getByText('London, UK')).toBeDefined()
    expect(screen.getByText('71%')).toBeDefined()
  })

  it('renders devices card with percentage bars', () => {
    mockFetchSequence(
      { data: baseUser, loading: false, error: null, refetch: vi.fn() },
      { data: testStats, loading: false, error: null, refetch: vi.fn() },
      { data: testHistory, loading: false, error: null, refetch: vi.fn() },
    )
    renderAtRoute('/users/alice')
    expect(screen.getByText('Chrome (Windows)')).toBeDefined()
    expect(screen.getByText('Plex TV (Android)')).toBeDefined()
    expect(screen.getByText('60%')).toBeDefined()
  })

  it('shows error message when stats fetch fails', () => {
    const statsErr = new ApiError(500, 'server error')
    mockFetchSequence(
      { data: baseUser, loading: false, error: null, refetch: vi.fn() },
      { data: null, loading: false, error: statsErr, refetch: vi.fn() },
      { data: testHistory, loading: false, error: null, refetch: vi.fn() },
    )
    renderAtRoute('/users/alice')
    expect(screen.getByText(/failed to load user statistics/i)).toBeDefined()
  })
})
