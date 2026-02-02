import { describe, it, expect, vi, afterEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { History } from '../pages/History'
import type { WatchHistoryEntry, PaginatedResult } from '../types'

const mockEntry: WatchHistoryEntry = {
  id: 1,
  server_id: 1,
  user_name: 'alice',
  media_type: 'movie',
  title: 'Test Movie',
  parent_title: '',
  grandparent_title: '',
  year: 2024,
  duration_ms: 7200000,
  watched_ms: 7200000,
  player: 'Chrome',
  platform: 'Web',
  ip_address: '10.0.0.1',
  started_at: '2024-01-01T00:00:00Z',
  stopped_at: '2024-01-01T02:00:00Z',
  created_at: '2024-01-01T00:00:00Z',
}

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

afterEach(() => {
  vi.restoreAllMocks()
})

describe('History', () => {
  it('shows loading state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null })
    renderWithRouter(<History />)
    expect(screen.getByText('History')).toBeDefined()
  })

  it('renders history entries', () => {
    const data: PaginatedResult<WatchHistoryEntry> = {
      items: [mockEntry],
      total: 1,
      page: 1,
      per_page: 20,
    }
    mockUseFetch.mockReturnValue({ data, loading: false, error: null })
    renderWithRouter(<History />)
    expect(screen.getAllByText('alice').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Test Movie').length).toBeGreaterThan(0)
  })

  it('shows pagination when multiple pages', () => {
    const data: PaginatedResult<WatchHistoryEntry> = {
      items: [mockEntry],
      total: 50,
      page: 1,
      per_page: 20,
    }
    mockUseFetch.mockReturnValue({ data, loading: false, error: null })
    renderWithRouter(<History />)
    expect(screen.getByText(/next/i)).toBeDefined()
  })

  it('shows error state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: new Error('fail') })
    renderWithRouter(<History />)
    expect(screen.getByText(/error/i)).toBeDefined()
  })
})
