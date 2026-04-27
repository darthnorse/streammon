import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { SeasonDetail } from '../components/modals/SeasonDetail'
import type { ItemDetails } from '../types'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

function makeSeasonItem(overrides: Partial<ItemDetails> = {}): ItemDetails {
  return {
    id: 'season-1',
    title: 'Season 1',
    media_type: 'episode',
    server_id: 1,
    server_name: 'My Plex',
    server_type: 'plex',
    level: 'season',
    season_number: 1,
    series_title: 'Breaking Bad',
    parent_id: 'show-1',
    tmdb_id: '1396',
    ...overrides,
  }
}

function setUseFetch(opts: {
  childrenData?: { episodes: unknown[] } | null
  tmdbData?: { overview?: string; episodes?: unknown[] } | null
} = {}) {
  mockUseFetch.mockImplementation(((url: string | null) => {
    if (url == null) return { data: null, loading: false, error: null, refetch: vi.fn() }
    if (url.includes('/children/')) {
      return { data: opts.childrenData ?? { episodes: [] }, loading: false, error: null, refetch: vi.fn() }
    }
    if (url.includes('/tmdb/tv/') && url.includes('/season/')) {
      return { data: opts.tmdbData ?? null, loading: false, error: null, refetch: vi.fn() }
    }
    return { data: null, loading: false, error: null, refetch: vi.fn() }
  }) as unknown as typeof useFetch)
}

describe('SeasonDetail', () => {
  beforeEach(() => {
    mockUseFetch.mockReset()
  })

  it('renders the season title', () => {
    setUseFetch()
    renderWithRouter(
      <SeasonDetail
        item={makeSeasonItem()}
        loading={false}
        onClose={() => {}}
        pushModal={vi.fn()}
        active={true}
      />,
    )
    expect(screen.getByRole('heading', { name: 'Season 1' })).toBeInTheDocument()
  })

  it('renders the show title as a clickable button when parent_id is present', async () => {
    setUseFetch()
    const pushModal = vi.fn()
    renderWithRouter(
      <SeasonDetail
        item={makeSeasonItem()}
        loading={false}
        onClose={() => {}}
        pushModal={pushModal}
        active={true}
      />,
    )
    const showButton = screen.getByRole('button', { name: 'Breaking Bad' })
    await userEvent.click(showButton)
    expect(pushModal).toHaveBeenCalledWith({ type: 'show', serverId: 1, itemId: 'show-1' })
  })

  it('does not render a clickable show link when parent_id is missing', () => {
    setUseFetch()
    renderWithRouter(
      <SeasonDetail
        item={makeSeasonItem({ parent_id: undefined })}
        loading={false}
        onClose={() => {}}
        pushModal={vi.fn()}
        active={true}
      />,
    )
    expect(screen.queryByRole('button', { name: 'Breaking Bad' })).not.toBeInTheDocument()
  })

  it('renders TMDB description when available', () => {
    setUseFetch({ tmdbData: { overview: 'Season summary from TMDB' } })
    renderWithRouter(
      <SeasonDetail
        item={makeSeasonItem({ summary: 'Server summary' })}
        loading={false}
        onClose={() => {}}
        pushModal={vi.fn()}
        active={true}
      />,
    )
    expect(screen.getByText('Season summary from TMDB')).toBeInTheDocument()
    expect(screen.queryByText('Server summary')).not.toBeInTheDocument()
  })

  it('falls back to server summary when TMDB enrichment is empty', () => {
    setUseFetch({ tmdbData: null })
    renderWithRouter(
      <SeasonDetail
        item={makeSeasonItem({ summary: 'Server summary fallback' })}
        loading={false}
        onClose={() => {}}
        pushModal={vi.fn()}
        active={true}
      />,
    )
    expect(screen.getByText('Server summary fallback')).toBeInTheDocument()
  })

  it('shows loading spinner when loading prop is true', () => {
    setUseFetch()
    renderWithRouter(
      <SeasonDetail
        item={null}
        loading={true}
        onClose={() => {}}
        pushModal={vi.fn()}
        active={true}
      />,
    )
    expect(document.querySelector('.animate-spin')).toBeInTheDocument()
  })
})
