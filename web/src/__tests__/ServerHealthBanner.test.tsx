import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { ServerHealthBanner } from '../components/ServerHealthBanner'
import type { ServerHealthResponse } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'
const mockUseFetch = vi.mocked(useFetch)

function mockHealth(data: ServerHealthResponse | null, refetch = vi.fn()) {
  mockUseFetch.mockReturnValue({ data, loading: false, error: null, refetch })
  return refetch
}

describe('ServerHealthBanner', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('fetches server health from the admin endpoint', () => {
    mockHealth({ failing: [] })
    renderWithRouter(<ServerHealthBanner />)
    expect(mockUseFetch).toHaveBeenCalledWith('/api/dashboard/server-health')
  })

  it('renders nothing when no server is failing', () => {
    mockHealth({ failing: [] })
    const { container } = renderWithRouter(<ServerHealthBanner />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing before data loads', () => {
    mockHealth(null)
    const { container } = renderWithRouter(<ServerHealthBanner />)
    expect(container).toBeEmptyDOMElement()
  })

  it('warns about each failing server with its error and a link to settings', () => {
    mockHealth({
      failing: [
        { server_id: 3, server_name: 'Jellyfin', error: 'jellyfin returned status 401', failing_since: '2026-10-29T03:37:36Z' },
        { server_id: 5, server_name: 'Emby', error: 'connection refused', failing_since: '2026-11-19T10:00:00Z' },
      ],
    })
    renderWithRouter(<ServerHealthBanner />)

    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('Jellyfin')
    expect(alert).toHaveTextContent('jellyfin returned status 401')
    expect(alert).toHaveTextContent('Emby')
    expect(alert).toHaveTextContent('connection refused')
    expect(alert).toHaveTextContent(/watch history may be incomplete/i)
    expect(screen.getByRole('link', { name: /server settings/i })).toHaveAttribute('href', '/settings')
  })

  it('refetches periodically and stops on unmount', () => {
    vi.useFakeTimers()
    const refetch = mockHealth({ failing: [] })
    const { unmount } = renderWithRouter(<ServerHealthBanner />)

    vi.advanceTimersByTime(30_000)
    expect(refetch).toHaveBeenCalledTimes(1)

    unmount()
    vi.advanceTimersByTime(60_000)
    expect(refetch).toHaveBeenCalledTimes(1)
  })
})
