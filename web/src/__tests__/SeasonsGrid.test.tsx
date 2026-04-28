import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { SeasonsGrid } from '../components/modals/SeasonsGrid'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

import { useFetch } from '../hooks/useFetch'

const mockUseFetch = vi.mocked(useFetch)

describe('SeasonsGrid', () => {
  beforeEach(() => {
    mockUseFetch.mockReset()
  })

  it('shows loading state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() })
    renderWithRouter(<SeasonsGrid serverId={1} showId="show-1" pushModal={vi.fn()} />)
    expect(screen.getByText(/loading seasons/i)).toBeInTheDocument()
  })

  it('shows error state', () => {
    mockUseFetch.mockReturnValue({ data: null, loading: false, error: new Error('fail'), refetch: vi.fn() })
    renderWithRouter(<SeasonsGrid serverId={1} showId="show-1" pushModal={vi.fn()} />)
    expect(screen.getByText(/failed to load seasons/i)).toBeInTheDocument()
  })

  it('renders the grid when there is one season so users can drill into episodes', () => {
    mockUseFetch.mockReturnValue({
      data: { seasons: [{ id: 's-1', number: 1, title: 'Season 1', episode_count: 8 }] },
      loading: false,
      error: null,
      refetch: vi.fn(),
    })
    renderWithRouter(<SeasonsGrid serverId={1} showId="show-1" pushModal={vi.fn()} />)
    expect(screen.getByText('Season 1')).toBeInTheDocument()
    expect(screen.getByText('8 episodes')).toBeInTheDocument()
  })

  it('renders nothing when there are zero seasons', () => {
    mockUseFetch.mockReturnValue({
      data: { seasons: [] },
      loading: false,
      error: null,
      refetch: vi.fn(),
    })
    const { container } = renderWithRouter(
      <SeasonsGrid serverId={1} showId="show-1" pushModal={vi.fn()} />,
    )
    expect(container.firstChild).toBeNull()
  })

  it('renders a card per season when there are 2+ seasons', () => {
    mockUseFetch.mockReturnValue({
      data: {
        seasons: [
          { id: 's-1', number: 1, title: 'Season 1', episode_count: 8 },
          { id: 's-2', number: 2, title: 'Season 2', episode_count: 10 },
        ],
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    })
    renderWithRouter(<SeasonsGrid serverId={1} showId="show-1" pushModal={vi.fn()} />)
    expect(screen.getByText('Season 1')).toBeInTheDocument()
    expect(screen.getByText('Season 2')).toBeInTheDocument()
    expect(screen.getByText('8 episodes')).toBeInTheDocument()
    expect(screen.getByText('10 episodes')).toBeInTheDocument()
  })

  it('clicking a season card calls pushModal with type=season', async () => {
    const pushModal = vi.fn()
    mockUseFetch.mockReturnValue({
      data: {
        seasons: [
          { id: 's-1', number: 1, title: 'Season 1' },
          { id: 's-2', number: 2, title: 'Season 2' },
        ],
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    })
    renderWithRouter(<SeasonsGrid serverId={42} showId="show-1" pushModal={pushModal} />)

    await userEvent.click(screen.getByText('Season 2'))
    expect(pushModal).toHaveBeenCalledWith({ type: 'season', serverId: 42, itemId: 's-2' })
  })
})
