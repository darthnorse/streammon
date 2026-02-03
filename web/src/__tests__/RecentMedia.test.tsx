import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { RecentMedia } from '../components/RecentMedia'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

describe('RecentMedia', () => {
  it('shows loading state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<RecentMedia />)
    expect(screen.getByText(/loading recent media/i)).toBeInTheDocument()
  })

  it('shows error state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: new Error('fail'), refetch: vi.fn() })
    renderWithRouter(<RecentMedia />)
    expect(screen.getByText(/failed to load recent media/i)).toBeInTheDocument()
  })

  it('returns null when no data', () => {
    mockUseFetch.mockReturnValue({ data: [], loading: false, error: null, refetch: vi.fn() })
    const { container } = renderWithRouter(<RecentMedia />)
    expect(container.firstChild).toBeNull()
  })

  it('renders media cards', () => {
    mockUseFetch.mockReturnValue({
      data: [
        {
          title: 'Test Movie',
          year: 2024,
          server_name: 'My Plex',
          server_type: 'plex',
          server_id: 1,
          media_type: 'movie',
          added_at: new Date().toISOString(),
        },
      ],
      loading: false,
      error: null,
      refetch: vi.fn(),
    })
    renderWithRouter(<RecentMedia />)
    expect(screen.getByText('Recently Added')).toBeInTheDocument()
    expect(screen.getByText('Test Movie')).toBeInTheDocument()
  })

  it('shows server indicator dot', () => {
    mockUseFetch.mockReturnValue({
      data: [
        {
          title: 'Jellyfin Movie',
          year: 2023,
          server_name: 'My Jellyfin',
          server_type: 'jellyfin',
          server_id: 2,
          media_type: 'movie',
          added_at: new Date().toISOString(),
        },
      ],
      loading: false,
      error: null,
      refetch: vi.fn(),
    })
    const { container } = renderWithRouter(<RecentMedia />)
    const dot = container.querySelector('.bg-purple-500')
    expect(dot).toBeInTheDocument()
  })
})
