import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter } from '../test-utils'
import { RecentMedia, metaLine } from '../components/RecentMedia'
import type { LibraryItem } from '../types'

function makeItem(overrides: Partial<LibraryItem> = {}): LibraryItem {
  return {
    item_id: 'id',
    title: 'Title',
    media_type: 'movie',
    added_at: new Date().toISOString(),
    server_id: 1,
    server_name: 'Server',
    server_type: 'plex',
    ...overrides,
  }
}

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

describe('metaLine', () => {
  it('renders the year for movies', () => {
    expect(metaLine(makeItem({ media_type: 'movie', year: 2024 }))).toBe('2024')
  })

  it('returns empty string for movies without a year', () => {
    expect(metaLine(makeItem({ media_type: 'movie' }))).toBe('')
  })

  it('renders S{n} · E{m} for episodes with full numbering', () => {
    expect(metaLine(makeItem({ media_type: 'episode', season_number: 5, episode_number: 14 }))).toBe('S5 · E14')
  })

  it('renders Season {n} · {count} episodes for season-batch with leafCount', () => {
    expect(metaLine(makeItem({
      media_type: 'episode',
      season_batch: true,
      season_number: 1,
      episode_count: 5,
    }))).toBe('Season 1 · 5 episodes')
  })

  it('uses singular "episode" for season-batch with one episode', () => {
    expect(metaLine(makeItem({
      media_type: 'episode',
      season_batch: true,
      season_number: 1,
      episode_count: 1,
    }))).toBe('Season 1 · 1 episode')
  })

  it('renders Season {n} when season-batch has no episode count', () => {
    expect(metaLine(makeItem({
      media_type: 'episode',
      season_batch: true,
      season_number: 2,
    }))).toBe('Season 2')
  })

  it('falls back to year for an episode missing the episode number', () => {
    expect(metaLine(makeItem({
      media_type: 'episode',
      season_number: 5,
      year: 2024,
    }))).toBe('2024')
  })
})
