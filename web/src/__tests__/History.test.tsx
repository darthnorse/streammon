import { describe, it, expect, vi, afterEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { History } from '../pages/History'
import { baseHistoryEntry, baseServer } from './fixtures'
import type { WatchHistoryEntry, PaginatedResult } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

afterEach(() => {
  vi.restoreAllMocks()
})

const emptyFetch = { data: null, loading: false, error: null, refetch: vi.fn() }

function mockHistory(data: PaginatedResult<WatchHistoryEntry> | null, opts?: { loading?: boolean; error?: Error | null }) {
  mockUseFetch.mockImplementation((url: string | null) => {
    if (url === '/api/servers') return { ...emptyFetch, data: [] }
    return { data, loading: opts?.loading ?? false, error: opts?.error ?? null, refetch: vi.fn() }
  })
}

describe('History', () => {
  it('shows loading state', () => {
    mockHistory(null, { loading: true })
    renderWithRouter(<History />)
    expect(screen.getByText('Loading...')).toBeDefined()
  })

  it('renders history entries', () => {
    const data: PaginatedResult<WatchHistoryEntry> = {
      items: [baseHistoryEntry],
      total: 1,
      page: 1,
      per_page: 20,
    }
    mockHistory(data)
    renderWithRouter(<History />)
    expect(screen.getAllByText('alice').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Inception').length).toBeGreaterThan(0)
  })

  it('shows pagination when multiple pages', () => {
    const data: PaginatedResult<WatchHistoryEntry> = {
      items: [baseHistoryEntry],
      total: 50,
      page: 1,
      per_page: 20,
    }
    mockHistory(data)
    renderWithRouter(<History />)
    expect(screen.getByText(/next/i)).toBeDefined()
  })

  it('shows error state', () => {
    mockHistory(null, { error: new Error('fail') })
    renderWithRouter(<History />)
    expect(screen.getByText(/error/i)).toBeDefined()
  })

  it('shows server filter when multiple servers', () => {
    const servers = [
      baseServer,
      { ...baseServer, id: 2, name: 'Jellyfin' },
    ]
    const data: PaginatedResult<WatchHistoryEntry> = {
      items: [baseHistoryEntry],
      total: 1,
      page: 1,
      per_page: 20,
    }
    mockUseFetch.mockImplementation((url: string | null) => {
      if (url === '/api/servers') return { ...emptyFetch, data: servers }
      return { data, loading: false, error: null, refetch: vi.fn() }
    })
    renderWithRouter(<History />)
    expect(screen.getByText('All Servers')).toBeDefined()
  })

  it('passes server_ids to useFetch when servers selected', async () => {
    const servers = [
      baseServer,
      { ...baseServer, id: 2, name: 'Jellyfin' },
    ]
    const data: PaginatedResult<WatchHistoryEntry> = {
      items: [baseHistoryEntry],
      total: 1,
      page: 1,
      per_page: 20,
    }
    const calledUrls: string[] = []
    mockUseFetch.mockImplementation((url: string | null) => {
      if (url) calledUrls.push(url)
      if (url === '/api/servers') return { ...emptyFetch, data: servers }
      return { data, loading: false, error: null, refetch: vi.fn() }
    })
    renderWithRouter(<History />)

    // Open the server dropdown and select a server
    fireEvent.click(screen.getByText('All Servers'))
    fireEvent.click(screen.getByText('My Plex'))

    await waitFor(() => {
      const historyUrl = calledUrls.find(u => u.includes('server_ids='))
      expect(historyUrl).toBeDefined()
      expect(historyUrl).toContain('server_ids=1')
    })
  })

  it('hides server filter when single server', () => {
    const data: PaginatedResult<WatchHistoryEntry> = {
      items: [baseHistoryEntry],
      total: 1,
      page: 1,
      per_page: 20,
    }
    mockUseFetch.mockImplementation((url: string | null) => {
      if (url === '/api/servers') return { ...emptyFetch, data: [baseServer] }
      return { data, loading: false, error: null, refetch: vi.fn() }
    })
    renderWithRouter(<History />)
    expect(screen.queryByText('All Servers')).toBeNull()
  })
})
