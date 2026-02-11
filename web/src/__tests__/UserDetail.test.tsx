import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { UserDetail } from '../pages/UserDetail'
import { ApiError } from '../lib/api'
import { baseUser, baseHistoryEntry } from './fixtures'
import type { WatchHistoryEntry, PaginatedResult, UserDetailStats, GeoResult } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

vi.mock('../components/shared/LeafletMap', () => ({
  LeafletMap: () => <div data-testid="leaflet-map" />,
}))

vi.mock('../components/shared/ViewModeToggle', () => ({
  ViewModeToggle: ({ viewMode, onChange }: { viewMode: string; onChange: (m: string) => void }) => (
    <div data-testid="view-mode-toggle">
      <button onClick={() => onChange('heatmap')}>Heatmap</button>
      <button onClick={() => onChange('markers')}>Markers</button>
      <span data-testid="current-mode">{viewMode}</span>
    </div>
  ),
}))

vi.mock('../context/AuthContext', () => ({
  useAuth: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'
import { useAuth } from '../context/AuthContext'

const mockUseFetch = vi.mocked(useFetch)
const mockUseAuth = vi.mocked(useAuth)

function fetchResult<T>(data: T | null, error: Error | null = null) {
  return { data, loading: false, error, refetch: vi.fn() }
}

const noData = () => fetchResult(null)

const adminAuth = {
  user: { id: 1, name: 'admin', email: '', role: 'admin' as const, thumb_url: '', has_password: true, created_at: '', updated_at: '' },
  loading: false,
  setupRequired: false,
  setUser: vi.fn(),
  clearSetupRequired: vi.fn(),
  refreshUser: vi.fn(),
  logout: vi.fn(),
}

const viewerAuth = {
  ...adminAuth,
  user: { ...adminAuth.user, id: 2, name: 'viewer', role: 'viewer' as const },
}

beforeEach(() => {
  localStorage.clear()
  // Default: admin user viewing the page
  mockUseAuth.mockReturnValue(adminAuth)
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

function mockFetchSequence(...values: ReturnType<typeof noData>[]) {
  let chain = mockUseFetch
  for (const v of values) {
    chain = chain.mockReturnValueOnce(v) as typeof mockUseFetch
  }
}

// useFetch calls in UserDetail, in hook order:
//   1. trust-visibility (null URL for admin or non-own-page viewer → falls through to default noData mock)
//   2. user
//   3. stats
//   4. locations (always fetched)
//   5. history (only when tab === 'history')
//   6. violations (null URL unless showTrustScore && tab === 'violations')
// Admin and non-own-page viewer tests don't need a trust-visibility mock;
// the default noData() handles the null-URL call.
function mockStandardPage(userOverride?: Partial<typeof baseUser>) {
  const user = userOverride ? { ...baseUser, ...userOverride } : baseUser
  mockFetchSequence(
    noData(),           // trust-visibility (null URL for admin)
    fetchResult(user),
    fetchResult(testStats),
    noData(),           // locations
    fetchResult(testHistory),
  )
}

describe('UserDetail', () => {
  it('shows loading state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderAtRoute('/users/alice')
    expect(screen.getByText(/loading user/i)).toBeDefined()
  })

  it('shows page with username when user record not found (404)', () => {
    mockFetchSequence(
      noData(),           // trust-visibility (null URL for admin)
      fetchResult(null, new ApiError(404, 'not found')),
      fetchResult(testStats),
      noData(),           // locations
      fetchResult(testHistory),
    )
    renderAtRoute('/users/nobody')
    expect(screen.getByRole('heading', { name: 'nobody' })).toBeDefined()
  })

  it('shows generic error for non-404', () => {
    mockFetchSequence(
      noData(),           // trust-visibility (null URL for admin)
      fetchResult(null, new ApiError(500, 'server error')),
    )
    renderAtRoute('/users/alice')
    expect(screen.getByText(/failed to load user/i)).toBeDefined()
  })

  it('renders user header with role badge', () => {
    mockStandardPage()
    renderAtRoute('/users/alice')
    expect(screen.getByText('alice')).toBeDefined()
    expect(screen.getByText('admin')).toBeDefined()
  })

  it('renders watch history tab by default', () => {
    mockStandardPage()
    renderAtRoute('/users/alice')
    expect(screen.getAllByText('Inception').length).toBeGreaterThan(0)
  })

  it('hides user column in history table (hideUser prop)', () => {
    mockStandardPage()
    renderAtRoute('/users/alice')
    const table = document.querySelector('table')
    const userLinks = table?.querySelectorAll('a[href="/users/alice"]')
    expect(userLinks?.length ?? 0).toBe(0)
  })

  it('excludes user column from column settings', () => {
    mockStandardPage()
    renderAtRoute('/users/alice')
    const headers = document.querySelectorAll('table th')
    const headerTexts = Array.from(headers).map(h => h.textContent?.toLowerCase())
    expect(headerTexts).not.toContain('user')
  })

  it('renders stats cards with session count and watch time', () => {
    mockStandardPage()
    renderAtRoute('/users/alice')
    expect(screen.getByText('42')).toBeDefined()
    expect(screen.getByText('12.5h')).toBeDefined()
  })

  it('renders locations card with percentage bars', () => {
    mockStandardPage()
    renderAtRoute('/users/alice')
    expect(screen.getByText('New York, US')).toBeDefined()
    expect(screen.getByText('London, UK')).toBeDefined()
    expect(screen.getByText('71%')).toBeDefined()
  })

  it('renders devices card with percentage bars', () => {
    mockStandardPage()
    renderAtRoute('/users/alice')
    expect(screen.getByText('Chrome (Windows)')).toBeDefined()
    expect(screen.getByText('Plex TV (Android)')).toBeDefined()
    expect(screen.getByText('60%')).toBeDefined()
  })

  it('shows error message when stats fetch fails', () => {
    mockFetchSequence(
      noData(),           // trust-visibility (null URL for admin)
      fetchResult(baseUser),
      fetchResult(null, new ApiError(500, 'server error')),
      noData(),           // locations
      fetchResult(testHistory),
    )
    renderAtRoute('/users/alice')
    expect(screen.getByText(/failed to load user statistics/i)).toBeDefined()
  })

  it('shows violations tab for admin users', () => {
    mockStandardPage()
    renderAtRoute('/users/alice')
    expect(screen.getByText('Violations')).toBeDefined()
  })

  it('hides violations tab for non-admin users viewing other profiles', () => {
    mockUseAuth.mockReturnValue(viewerAuth)
    mockStandardPage()
    renderAtRoute('/users/alice')
    expect(screen.queryByText('Violations')).toBeNull()
  })

  it('hides household card for non-admin users', () => {
    mockUseAuth.mockReturnValue(viewerAuth)
    mockStandardPage()
    renderAtRoute('/users/alice')
    const grid = document.querySelector('.lg\\:grid-cols-3')
    expect(grid).not.toBeNull()
  })

  it('uses 4-column grid for admin users', () => {
    mockStandardPage()
    renderAtRoute('/users/alice')
    const grid = document.querySelector('.lg\\:grid-cols-4')
    expect(grid).not.toBeNull()
  })

  it('accepts userName prop and uses it instead of URL param', () => {
    mockStandardPage({ name: 'bob' })
    render(
      <MemoryRouter>
        <UserDetail userName="bob" />
      </MemoryRouter>
    )
    expect(screen.getByRole('heading', { name: 'bob' })).toBeDefined()
  })

  describe('inline location map', () => {
    const testLocations: GeoResult[] = [
      { lat: 40.7, lng: -74.0, city: 'New York', country: 'US', last_seen: '2024-01-15T12:00:00Z' },
    ]

    it('renders map and toggle when locations are available', () => {
      mockFetchSequence(
        noData(),
        fetchResult(baseUser),
        fetchResult(testStats),
        fetchResult(testLocations),
        fetchResult(testHistory),
      )
      renderAtRoute('/users/alice')
      expect(screen.getByTestId('leaflet-map')).toBeDefined()
      expect(screen.getByTestId('view-mode-toggle')).toBeDefined()
    })

    it('hides map when no locations available', () => {
      mockStandardPage()
      renderAtRoute('/users/alice')
      expect(screen.queryByTestId('leaflet-map')).toBeNull()
      expect(screen.queryByTestId('view-mode-toggle')).toBeNull()
    })

    it('shows error when locations fetch fails', () => {
      mockFetchSequence(
        noData(),
        fetchResult(baseUser),
        fetchResult(testStats),
        fetchResult(null, new ApiError(500, 'server error')),
        fetchResult(testHistory),
      )
      renderAtRoute('/users/alice')
      expect(screen.getByText(/failed to load location map/i)).toBeDefined()
      expect(screen.queryByTestId('leaflet-map')).toBeNull()
    })
  })
})
