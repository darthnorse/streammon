import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { render } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { UserDetail } from '../pages/UserDetail'
import { ApiError } from '../lib/api'
import { baseUser, baseHistoryEntry } from './fixtures'
import type { WatchHistoryEntry, PaginatedResult } from '../types'

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

beforeEach(() => {
  localStorage.clear()
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

function renderAtRoute(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/users/:name" element={<UserDetail />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('UserDetail', () => {
  it('shows loading state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderAtRoute('/users/alice')
    expect(screen.getByText(/loading user/i)).toBeDefined()
  })

  it('shows page with username when user record not found (404)', () => {
    const err = new ApiError(404, 'not found')
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: err, refetch: vi.fn() })
    renderAtRoute('/users/nobody')
    // User may not have a StreamMon account but we still show their history page
    expect(screen.getByRole('heading', { name: 'nobody' })).toBeDefined()
  })

  it('shows generic error for non-404', () => {
    const err = new ApiError(500, 'server error')
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: err, refetch: vi.fn() })
    renderAtRoute('/users/alice')
    expect(screen.getByText(/failed to load user/i)).toBeDefined()
  })

  it('renders user header with role badge', () => {
    mockUseFetch
      .mockReturnValueOnce({ data: baseUser, loading: false, error: null, refetch: vi.fn() })
      .mockReturnValueOnce({ data: testHistory, loading: false, error: null, refetch: vi.fn() })
    renderAtRoute('/users/alice')
    expect(screen.getByText('alice')).toBeDefined()
    expect(screen.getByText('admin')).toBeDefined()
  })

  it('renders watch history tab by default', () => {
    mockUseFetch
      .mockReturnValueOnce({ data: baseUser, loading: false, error: null, refetch: vi.fn() })
      .mockReturnValueOnce({ data: testHistory, loading: false, error: null, refetch: vi.fn() })
    renderAtRoute('/users/alice')
    expect(screen.getAllByText('Inception').length).toBeGreaterThan(0)
  })

  it('hides user column in history table (hideUser prop)', () => {
    mockUseFetch
      .mockReturnValueOnce({ data: baseUser, loading: false, error: null, refetch: vi.fn() })
      .mockReturnValueOnce({ data: testHistory, loading: false, error: null, refetch: vi.fn() })
    renderAtRoute('/users/alice')
    // The table should not contain a link to the user since hideUser is true
    const table = document.querySelector('table')
    const userLinks = table?.querySelectorAll('a[href="/users/alice"]')
    expect(userLinks?.length ?? 0).toBe(0)
  })

  it('excludes user column from column settings', () => {
    mockUseFetch
      .mockReturnValueOnce({ data: baseUser, loading: false, error: null, refetch: vi.fn() })
      .mockReturnValueOnce({ data: testHistory, loading: false, error: null, refetch: vi.fn() })
    renderAtRoute('/users/alice')
    // The User column should not appear in the table header since hideUser excludes it
    const headers = document.querySelectorAll('table th')
    const headerTexts = Array.from(headers).map(h => h.textContent?.toLowerCase())
    expect(headerTexts).not.toContain('user')
  })
})
